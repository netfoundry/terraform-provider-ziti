package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &proxyV1ConfigResource{}
	_ resource.ResourceWithConfigure   = &proxyV1ConfigResource{}
	_ resource.ResourceWithImportState = &proxyV1ConfigResource{}
)

// NewProxyV1ConfigResource is a helper function to simplify the provider implementation.
func NewProxyV1ConfigResource() resource.Resource {
	return &proxyV1ConfigResource{}
}

// proxyV1ConfigResource is the resource implementation.
type proxyV1ConfigResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *proxyV1ConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	resourceConfig := req.ProviderData.(*zitiData)
	r.resourceConfig = resourceConfig

	tflog.Debug(ctx, "Configured ziti resource", map[string]any{"host": r.resourceConfig.host})
}

// Metadata returns the resource type name.
func (r *proxyV1ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_proxy_v1_config"
}

// proxyV1ConfigResourceModel maps the resource schema data.
type proxyV1ConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Port         types.Int32  `tfsdk:"port"`
	Protocols    types.List   `tfsdk:"protocols"`
	Binding      types.String `tfsdk:"binding"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Tags         types.Map    `tfsdk:"tags"`
	LastUpdated  types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *proxyV1ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti proxy v1 config Resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Identifier",
			},
			"last_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last Updated Time",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the config",
			},
			"port": schema.Int32Attribute{
				Required: true,
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
				MarkdownDescription: "The port to listen on.",
			},
			"protocols": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("tcp", "udp"),
					),
				},
				MarkdownDescription: "Protocols that the proxy will listen on.",
			},
			"binding": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The binding to use for this proxy config.",
			},
			"config_type_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("proxy.v1"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The Id of a config-type",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Config Tags",
			},
		},
	}
}

type ProxyConfigDTO struct {
	Port      *int32    `json:"port,omitempty"`
	Protocols *[]string `json:"protocols,omitempty"`
	Binding   *string   `json:"binding,omitempty"`
}

func (dto *ProxyConfigDTO) ConvertToZitiResourceModel(ctx context.Context) proxyV1ConfigResourceModel {
	return proxyV1ConfigResourceModel{
		Port:      types.Int32PointerValue(dto.Port),
		Protocols: convertStringList(ctx, dto.Protocols, types.StringType),
		Binding:   types.StringPointerValue(dto.Binding),
	}
}

func (r *proxyV1ConfigResourceModel) ToProxyConfigDTO(ctx context.Context) ProxyConfigDTO {
	return ProxyConfigDTO{
		Port:      r.Port.ValueInt32Pointer(),
		Protocols: ElementsToStringArray(r.Protocols.Elements()),
		Binding:   r.Binding.ValueStringPointer(),
	}
}

// Create a new resource.
func (r *proxyV1ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan proxyV1ConfigResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToProxyConfigDTO(ctx), true, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error marshalling Ziti Config from API",
			"Could not create Ziti Config "+eplan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	jsonObj, _ := json.Marshal(requestObject)
	tflog.Debug(ctx, string(jsonObj))

	name := eplan.Name.ValueString()
	configTypeId := eplan.ConfigTypeId.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	payload := rest_model.ConfigCreate{
		ConfigTypeID: &configTypeId,
		Name:         &name,
		Data:         requestObject,
		Tags:         tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	tflog.Debug(ctx, "create resource payload", map[string]any{"payload": string(jsonData)})

	authUrl := fmt.Sprintf("%s/configs", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating configs", "Could not Create configs, unexpected error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "create response", map[string]any{"response": cresp})
	resourceID := gjson.Get(cresp, "data.id").String()

	// Map response body to schema and populate Computed attribute values
	eplan.ID = types.StringValue(resourceID)
	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *proxyV1ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state proxyV1ConfigResourceModel
	tflog.Debug(ctx, "Reading Proxy Config")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := ReadZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		if errors.Is(err, errNotFound) {
			msg := fmt.Sprintf("Resource not found in backend; removing from state, id: %s", state.ID.ValueString())
			log.Info().Msg(msg)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading configs", "Could not READ configs, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		log.Error().Msgf("Error unmarshalling JSON response from Ziti Resource Response: %v", err)
		return
	}

	stringBody := string(cresp)
	tflog.Debug(ctx, "read response", map[string]any{"response": stringBody})

	data := jsonBody["data"].(map[string]interface{})
	resourceData := data["data"].(map[string]interface{})

	var proxyConfigDto ProxyConfigDTO
	GenericFromObject(resourceData, &proxyConfigDto)
	newState := proxyConfigDto.ConvertToZitiResourceModel(ctx)

	// Manually assign individual values from the map to the struct fields
	state.Name = types.StringValue(data["name"].(string))

	if _tags, ok := data["tags"].(map[string]interface{}); ok {
		if len(_tags) != 0 {
			_tags, diag := types.MapValueFrom(ctx, types.StringType, _tags)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.Tags = _tags
		} else {
			state.Tags = types.MapNull(types.StringType)
		}
	}

	newState.ID = state.ID
	newState.Name = state.Name
	newState.ConfigTypeId = state.ConfigTypeId
	newState.Tags = state.Tags
	newState.LastUpdated = state.LastUpdated
	state = newState

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *proxyV1ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan proxyV1ConfigResourceModel
	tflog.Debug(ctx, "Updating Proxy Config")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state proxyV1ConfigResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToProxyConfigDTO(ctx), true, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error marshalling Ziti Config from API",
			"Could not create Ziti Config "+eplan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	jsonObj, _ := json.Marshal(requestObject)
	tflog.Debug(ctx, string(jsonObj))

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	payload := rest_model.ConfigUpdate{
		Name: &name,
		Data: requestObject,
		Tags: tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	tflog.Debug(ctx, "update resource payload", map[string]any{"payload": string(jsonData)})

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PUT Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating configs", "Could not Update configs, unexpected error: "+err.Error(),
		)
		return
	}

	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *proxyV1ConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state proxyV1ConfigResourceModel
	tflog.Debug(ctx, "Deleting Proxy Config")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting configs", "Could not DELETE configs, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *proxyV1ConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
