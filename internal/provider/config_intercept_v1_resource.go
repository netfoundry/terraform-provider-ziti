package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &interceptV1ConfigResource{}
	_ resource.ResourceWithConfigure   = &interceptV1ConfigResource{}
	_ resource.ResourceWithImportState = &interceptV1ConfigResource{}
)

// NewInterceptV1ConfigResource is a helper function to simplify the provider implementation.
func NewInterceptV1ConfigResource() resource.Resource {
	return &interceptV1ConfigResource{}
}

// interceptV1ConfigResource is the resource implementation.
type interceptV1ConfigResource struct {
	resourceConfig *zitiData
}

var PortRangeModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"low":  types.Int32Type,
		"high": types.Int32Type,
	},
}

var DialOptionsModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"connect_timeout_seconds": types.Int32Type,
		"identity":                types.StringType,
	},
}

var AllowedPortRangeModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"low":  types.Int32Type,
		"high": types.Int32Type,
	},
}

// Configure adds the provider configured client to the resource.
func (r *interceptV1ConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	resourceConfig := req.ProviderData.(*zitiData)
	r.resourceConfig = resourceConfig

	fmt.Printf("Using API Token to create resource: %s\n", r.resourceConfig.apiToken)
	fmt.Printf("Using domain to create resource: %s\n", r.resourceConfig.host)
}

// Metadata returns the resource type name.
func (r *interceptV1ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_intercept_v1_config"
}

// interceptV1ConfigResourceModel maps the resource schema data.
type interceptV1ConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Addresses    types.List   `tfsdk:"addresses"`
	DialOptions  types.Object `tfsdk:"dial_options"`
	PortRanges   types.List   `tfsdk:"port_ranges"`
	Protocols    types.List   `tfsdk:"protocols"`
	SourceIP     types.String `tfsdk:"source_ip"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Tags         types.Map    `tfsdk:"tags"`
	LastUpdated  types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *interceptV1ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti intercept v1 config Resource",
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
			"addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "Target host config address",
			},
			"dial_options": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"connect_timeout_seconds": schema.Int32Attribute{
						Optional: true,
						Computed: true,
						Default:  int32default.StaticInt32(0),
					},
					"identity": schema.StringAttribute{
						Optional: true,
					},
				},
				MarkdownDescription: "Dial Options.",
			},
			"protocols": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("tcp", "udp"),
					),
				},
				MarkdownDescription: "Protocols that can be forwarded.",
			},
			"port_ranges": schema.ListNestedAttribute{
				Default:  listdefault.StaticValue(types.ListNull(AllowedPortRangeModel)),
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"low": schema.Int32Attribute{
							Required: true,
							Validators: []validator.Int32{
								int32validator.Between(1, 65535),
							},
						},
						"high": schema.Int32Attribute{
							Required: true,
							Validators: []validator.Int32{
								int32validator.Between(1, 65535),
							},
						},
					},
				},
				Optional:            true,
				MarkdownDescription: "Ports that can be forwarded.",
			},
			"source_ip": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Source IP",
			},
			"config_type_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("g7cIWbcGg"),
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

type ConfigPortsDTO struct {
	Low  int32 `json:"low,omitempty"`
	High int32 `json:"high,omitempty"`
}

type DialOptionsDTO struct {
	ConnectTimeoutSeconds *int32  `json:"connectTimeoutSeconds,omitempty"`
	Identity              *string `json:"identity,omitempty"`
}

type InterceptConfigDTO struct {
	Addresses   *[]string         `json:"addresses,omitempty"`
	DialOptions *DialOptionsDTO   `json:"dialOptions,omitempty"`
	PortRanges  *[]ConfigPortsDTO `json:"portRanges,omitempty"`
	Protocols   *[]string         `json:"protocols,omitempty"`
	SourceIP    *string           `json:"sourceIp,omitempty"`
}

func AttributesToDialOptionsStruct(ctx context.Context, attr map[string]attr.Value) DialOptionsDTO {
	var dialOptions DialOptionsDTO
	attrsNative := AttributesToNativeTypes(ctx, attr)
	attrsNative = convertKeysToCamel(attrsNative)
	GenericFromObject(attrsNative, &dialOptions)
	return dialOptions

}

func (dto *InterceptConfigDTO) ConvertToZitiResourceModel(ctx context.Context) interceptV1ConfigResourceModel {

	res := interceptV1ConfigResourceModel{
		Addresses: convertStringList(ctx, dto.Addresses, types.StringType),
		Protocols: convertStringList(ctx, dto.Protocols, types.StringType),
		SourceIP:  types.StringPointerValue(dto.SourceIP),
	}

	if dto.PortRanges != nil {
		var objects []attr.Value
		for _, allowedRange := range *dto.PortRanges {
			allowedRangeco, _ := JsonStructToObject(ctx, allowedRange, true, false)

			objectMap := NativeBasicTypedAttributesToTerraform(ctx, allowedRangeco, PortRangeModel.AttrTypes)
			object, _ := basetypes.NewObjectValue(PortRangeModel.AttrTypes, objectMap)
			objects = append(objects, object)
		}
		allowedPortRanges, _ := types.ListValueFrom(ctx, PortRangeModel, objects)
		res.PortRanges = allowedPortRanges
	} else {
		res.PortRanges = types.ListNull(PortRangeModel)
	}

	if dto.DialOptions != nil {
		dialOptionsObject, _ := JsonStructToObject(ctx, *dto.DialOptions, true, false)
		dialOptionsObject = convertKeysToSnake(dialOptionsObject)

		dialOptionsMap := NativeBasicTypedAttributesToTerraform(ctx, dialOptionsObject, DialOptionsModel.AttrTypes)

		dialOptionsTf, err := basetypes.NewObjectValue(DialOptionsModel.AttrTypes, dialOptionsMap)
		if err != nil {
			oneerr := err[0]
			tflog.Debug(ctx, "Error converting dialOptionsMap to an object: "+oneerr.Summary()+" | "+oneerr.Detail())
		}
		res.DialOptions = dialOptionsTf
	} else {
		res.DialOptions = types.ObjectNull(DialOptionsModel.AttrTypes)
	}

	return res
}

func (r *interceptV1ConfigResourceModel) ToInterceptConfigDTO(ctx context.Context) InterceptConfigDTO {
	dialOptions := AttributesToDialOptionsStruct(ctx, r.DialOptions.Attributes())

	interceptConfigDto := InterceptConfigDTO{
		Addresses:   ElementsToStringArray(r.Addresses.Elements()),
		DialOptions: &dialOptions,
		Protocols:   ElementsToStringArray(r.Protocols.Elements()),
		SourceIP:    r.SourceIP.ValueStringPointer(),
	}

	if len(r.PortRanges.Elements()) > 0 {
		allowedPortRanges := ElementsToListOfStructs[ConfigPortsDTO](ctx, r.PortRanges.Elements())
		interceptConfigDto.PortRanges = &allowedPortRanges
	}

	return interceptConfigDto
}

// Create a new resource.
func (r *interceptV1ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan interceptV1ConfigResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToInterceptConfigDTO(ctx), true, true)
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
	fmt.Printf("**********************create resource payload***********************:\n %s\n", jsonData)

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

	fmt.Printf("**********************create response************************:\n %s\n", cresp)
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
func (r *interceptV1ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state interceptV1ConfigResourceModel
	tflog.Debug(ctx, "Reading Intercept Config")
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
		if cresp == "" || IsNotFoundError(err) {
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
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	stringBody := string(cresp)
	fmt.Printf("**********************read response************************:\n %s\n", stringBody)

	data := jsonBody["data"].(map[string]interface{})
	resourceData := data["data"].(map[string]interface{})

	var hostConfigDto InterceptConfigDTO
	GenericFromObject(resourceData, &hostConfigDto)
	newState := hostConfigDto.ConvertToZitiResourceModel(ctx)

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
func (r *interceptV1ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan interceptV1ConfigResourceModel
	tflog.Debug(ctx, "Updating Intercept Config")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state interceptV1ConfigResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToInterceptConfigDTO(ctx), true, true)
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
	fmt.Printf("**********************update resource payload***********************:\n %s\n", jsonData)

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
func (r *interceptV1ConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state interceptV1ConfigResourceModel
	tflog.Debug(ctx, "Deleting Intercept Config")
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

func (r *interceptV1ConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
