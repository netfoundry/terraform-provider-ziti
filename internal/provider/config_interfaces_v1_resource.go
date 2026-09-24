package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	_ resource.Resource                = &interfacesV1ConfigResource{}
	_ resource.ResourceWithConfigure   = &interfacesV1ConfigResource{}
	_ resource.ResourceWithImportState = &interfacesV1ConfigResource{}
)

// NewInterfacesV1ConfigResource is a helper function to simplify the provider implementation.
func NewInterfacesV1ConfigResource() resource.Resource {
	return &interfacesV1ConfigResource{}
}

// interfacesV1ConfigResource is the resource implementation.
type interfacesV1ConfigResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *interfacesV1ConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *interfacesV1ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_interfaces_v1_config"
}

// interfacesV1ConfigResourceModel maps the resource schema data.
type interfacesV1ConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Interfaces   types.List   `tfsdk:"interfaces"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Tags         types.Map    `tfsdk:"tags"`
	LastUpdated  types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *interfacesV1ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti interfaces v1 config Resource",
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
			"interfaces": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "The names of the network interfaces that a tunneler should bind to for this service.",
			},
			"config_type_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("interfaces.v1"),
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

type InterfacesConfigDTO struct {
	Interfaces *[]string `json:"interfaces,omitempty"`
}

func (dto *InterfacesConfigDTO) ConvertToZitiResourceModel(ctx context.Context) interfacesV1ConfigResourceModel {
	return interfacesV1ConfigResourceModel{
		Interfaces: convertStringList(ctx, dto.Interfaces, types.StringType),
	}
}

func (r *interfacesV1ConfigResourceModel) ToInterfacesConfigDTO(ctx context.Context) InterfacesConfigDTO {
	return InterfacesConfigDTO{
		Interfaces: ElementsToStringArray(r.Interfaces.Elements()),
	}
}

// Create a new resource.
func (r *interfacesV1ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan interfacesV1ConfigResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToInterfacesConfigDTO(ctx), true, true)
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
func (r *interfacesV1ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state interfacesV1ConfigResourceModel
	tflog.Debug(ctx, "Reading Interfaces Config")
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

	var interfacesConfigDto InterfacesConfigDTO
	GenericFromObject(resourceData, &interfacesConfigDto)
	newState := interfacesConfigDto.ConvertToZitiResourceModel(ctx)

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
func (r *interfacesV1ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan interfacesV1ConfigResourceModel
	tflog.Debug(ctx, "Updating Interfaces Config")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state interfacesV1ConfigResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToInterfacesConfigDTO(ctx), true, true)
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
func (r *interfacesV1ConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state interfacesV1ConfigResourceModel
	tflog.Debug(ctx, "Deleting Interfaces Config")
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

func (r *interfacesV1ConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
