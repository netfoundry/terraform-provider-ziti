package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &hostV1ConfigResource{}
	_ resource.ResourceWithConfigure   = &hostV1ConfigResource{}
	_ resource.ResourceWithImportState = &hostV1ConfigResource{}
)

// NewHostV1ConfigResource is a helper function to simplify the provider implementation.
func NewHostV1ConfigResource() resource.Resource {
	return &hostV1ConfigResource{}
}

// hostV1ConfigResource is the resource implementation.
type hostV1ConfigResource struct {
	resourceConfig *zitiData
}

var ListenOptionsModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"bind_using_edge_identity": types.BoolType,
		"connect_timeout":          types.StringType,
		"cost":                     types.Int32Type,
		"max_connections":          types.Int32Type,
		"precedence":               types.StringType,
	},
}

var CheckActionModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"trigger":            types.StringType,
		"duration":           types.StringType,
		"action":             types.StringType,
		"consecutive_events": types.Int32Type,
	},
}

var PortCheckModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"address":  types.StringType,
		"interval": types.StringType,
		"timeout":  types.StringType,
		"actions":  types.ListType{ElemType: CheckActionModel},
	},
}

var HTTPCheckModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"url":            types.StringType,
		"method":         types.StringType,
		"body":           types.StringType,
		"expect_status":  types.Int32Type,
		"expect_in_body": types.StringType,
		"interval":       types.StringType,
		"timeout":        types.StringType,
		"actions":        types.ListType{ElemType: CheckActionModel},
	},
}

// Configure adds the provider configured client to the resource.
func (r *hostV1ConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *hostV1ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_v1_config"
}

// hostV1ConfigResourceModel maps the resource schema data.
type hostV1ConfigResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Address                types.String `tfsdk:"address"`
	ConfigTypeId           types.String `tfsdk:"config_type_id"`
	Port                   types.Int32  `tfsdk:"port"`
	Protocol               types.String `tfsdk:"protocol"`
	ForwardProtocol        types.Bool   `tfsdk:"forward_protocol"`
	ForwardPort            types.Bool   `tfsdk:"forward_port"`
	ForwardAddress         types.Bool   `tfsdk:"forward_address"`
	AllowedProtocols       types.List   `tfsdk:"allowed_protocols"`
	AllowedAddresses       types.List   `tfsdk:"allowed_addresses"`
	AllowedSourceAddresses types.List   `tfsdk:"allowed_source_addresses"`
	AllowedPortRanges      types.List   `tfsdk:"allowed_port_ranges"`
	ListenOptions          types.Object `tfsdk:"listen_options"`
	PortChecks             types.List   `tfsdk:"port_checks"`
	HTTPChecks             types.List   `tfsdk:"http_checks"`
	Tags                   types.Map    `tfsdk:"tags"`
	LastUpdated            types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *hostV1ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti host v1 config Resource",
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
			"address": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Target host config address",
			},
			"port": schema.Int32Attribute{
				Optional: true,
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
				MarkdownDescription: "Port of a target address",
			},
			"protocol": schema.StringAttribute{
				Validators: []validator.String{
					stringvalidator.OneOf("tcp", "udp"),
				},
				Optional:            true,
				MarkdownDescription: "Protocol which config would be allowed to receive",
			},
			"forward_protocol": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Flag which controls whether to forward allowedProtocols",
			},
			"forward_port": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Flag which controls whether to forward allowedPortRanges",
			},
			"forward_address": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Flag which controls whether to forward allowedAddresses",
			},
			"allowed_addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Addresses that can be forwarded.",
			},
			"allowed_source_addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Source addresses that can be forwarded.",
			},
			"listen_options": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"bind_using_edge_identity": schema.BoolAttribute{
						Optional: true,
					},
					"connect_timeout": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("5s"),
					},
					"cost": schema.Int32Attribute{
						Optional: true,
						Computed: true,
						Default:  int32default.StaticInt32(0),
						Validators: []validator.Int32{
							int32validator.Between(0, 65535),
						},
					},
					"max_connections": schema.Int32Attribute{
						Optional: true,
						Computed: true,
						Default:  int32default.StaticInt32(65535),
						Validators: []validator.Int32{
							int32validator.Between(1, 65535),
						},
					},
					"precedence": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("default"),
						Validators: []validator.String{
							stringvalidator.OneOf("default", "required", "failed"),
						},
					},
				},
				MarkdownDescription: "Listen Options.",
			},
			"http_checks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"url": schema.StringAttribute{
							Required: true,
						},
						"method": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("GET", "PUT", "POST", "PATCH"),
							},
						},
						"body": schema.StringAttribute{
							Optional: true,
						},
						"expect_status": schema.Int32Attribute{
							Optional: true,
							Computed: true,
							Default:  int32default.StaticInt32(200),
							Validators: []validator.Int32{
								int32validator.Between(1, 1000),
							},
						},
						"expect_in_body": schema.StringAttribute{
							Optional: true,
						},
						"interval": schema.StringAttribute{
							Required: true,
						},
						"timeout": schema.StringAttribute{
							Required: true,
						},
						"actions": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"trigger": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf("pass", "fail", "change"),
										},
									},
									"duration": schema.StringAttribute{
										Required: true,
									},
									"action": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.Any(
												stringvalidator.OneOf("mark unhealthy", "mark healthy", "send event"),
												stringvalidator.RegexMatches(
													regexp.MustCompile(`^(increase|decrease) cost (-?\d+)$`),
													"must have a valid syntax(eg 'increase cost 100')",
												),
											),
										},
									},
									"consecutive_events": schema.Int32Attribute{
										Optional: true,
										Computed: true,
										Default:  int32default.StaticInt32(1),
									},
								},
							},
							Required: true,
						},
					},
				},
				MarkdownDescription: "HTTP Checks.",
			},
			"port_checks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required: true,
						},
						"interval": schema.StringAttribute{
							Required: true,
						},
						"timeout": schema.StringAttribute{
							Required: true,
						},
						"actions": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"trigger": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf("pass", "fail", "change"),
										},
									},
									"duration": schema.StringAttribute{
										Required: true,
									},
									"action": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.Any(
												stringvalidator.OneOf("mark unhealthy", "mark healthy", "send event"),
												stringvalidator.RegexMatches(
													regexp.MustCompile(`^(increase|decrease) cost (-?\d+)$`),
													"must have a valid syntax(eg 'increase cost 100')",
												),
											),
										},
									},
									"consecutive_events": schema.Int32Attribute{
										Optional: true,
										Computed: true,
										Default:  int32default.StaticInt32(1),
									},
								},
							},
							Required: true,
						},
					},
				},
				MarkdownDescription: "Port Checks.",
			},
			"allowed_protocols": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(types.ListNull(types.StringType)),
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("tcp", "udp"),
					),
				},
				MarkdownDescription: "Protocols that can be forwarded.",
			},
			"allowed_port_ranges": schema.ListNestedAttribute{
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
			"config_type_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("NH5p4FpGR"),
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

type ListenOptionsDTO struct {
	BindUsingEdgeIdentity *bool   `json:"bindUsingEdgeIdentity,omitempty"`
	ConnectTimeout        *string `json:"connectTimeout,omitempty"`
	Cost                  *int32  `json:"cost,omitempty"`
	MaxConnections        *int32  `json:"maxConnections,omitempty"`
	Precedence            *string `json:"precedence,omitempty"`
}

type CheckActionDTO struct {
	Trigger           *string `json:"trigger"`
	Duration          *string `json:"duration"`
	ConsecutiveEvents *int32  `json:"consecutiveEvents,omitempty"`
	Action            *string `json:"action"`
}

type HTTPCheckDTO struct {
	Url          *string           `json:"url"`
	Method       *string           `json:"method"`
	Body         *string           `json:"body,omitempty"`
	ExpectStatus *int32            `json:"expectStatus,omitempty"`
	ExpectInBody *string           `json:"expectInBody,omitempty"`
	Interval     *string           `json:"interval"`
	Timeout      *string           `json:"timeout"`
	Actions      *[]CheckActionDTO `json:"actions"`
}

type PortCheckDTO struct {
	Address  *string           `json:"address"`
	Interval *string           `json:"interval"`
	Timeout  *string           `json:"timeout"`
	Actions  *[]CheckActionDTO `json:"actions"`
}

type HostConfigDTO struct {
	Address                *string           `json:"address,omitempty"`
	Port                   *int32            `json:"port,omitempty"`
	Protocol               *string           `json:"protocol,omitempty"`
	ForwardProtocol        *bool             `json:"forwardProtocol,omitempty"`
	ForwardPort            *bool             `json:"forwardPort,omitempty"`
	ForwardAddress         *bool             `json:"forwardAddress,omitempty"`
	AllowedProtocols       *[]string         `json:"allowedProtocols,omitempty"`
	AllowedAddresses       *[]string         `json:"allowedAddresses,omitempty"`
	AllowedSourceAddresses *[]string         `json:"allowedSourceAddresses,omitempty"`
	AllowedPortRanges      *[]ConfigPortsDTO `json:"allowedPortRanges,omitempty"`
	ListenOptions          *ListenOptionsDTO `json:"listenOptions,omitempty"`
	HTTPChecks             *[]HTTPCheckDTO   `json:"httpChecks,omitempty"`
	PortChecks             *[]PortCheckDTO   `json:"portChecks,omitempty"`
}

func AttributesToListenOptionsStruct(ctx context.Context, attr map[string]attr.Value) ListenOptionsDTO {
	var listenOptions ListenOptionsDTO
	attrsNative := AttributesToNativeTypes(ctx, attr)
	attrsNative = convertKeysToCamel(attrsNative)
	GenericFromObject(attrsNative, &listenOptions)
	return listenOptions

}

func HandleActions(ctx context.Context, attr map[string]attr.Value) *[]CheckActionDTO {
	if value, exists := attr["actions"]; exists {
		if valueList, ok := value.(types.List); ok {
			actionsArray := []CheckActionDTO{}
			for _, v := range valueList.Elements() {
				if valueObject, ok := v.(types.Object); ok {
					var checkAction CheckActionDTO
					attrsNative := AttributesToNativeTypes(ctx, valueObject.Attributes())
					attrsNative = convertKeysToCamel(attrsNative)

					GenericFromObject(attrsNative, &checkAction)
					actionsArray = append(actionsArray, checkAction)
				}
			}
			if len(actionsArray) > 0 {
				return &actionsArray
			}
		}
	}
	return nil
}

func AttributesToStruct[T any](ctx context.Context, attr map[string]attr.Value) T {
	var result T
	attrsNative := AttributesToNativeTypes(ctx, attr)
	attrsNative = convertKeysToCamel(attrsNative)
	GenericFromObject(attrsNative, &result)

	// Set the actions field, assuming all such structs have Actions
	// Modify this part if the Actions handling differs per type.
	fieldValue := reflect.ValueOf(&result).Elem().FieldByName("Actions")
	if fieldValue.IsValid() && fieldValue.CanSet() {
		fieldValue.Set(reflect.ValueOf(HandleActions(ctx, attr)))
	}

	return result
}

func convertCheckActionToTerraformList(ctx context.Context, actions *[]CheckActionDTO) (types.List, diag.Diagnostics) {
	var actionsTf []attr.Value
	for _, item := range *actions {

		actionObject, _ := JsonStructToObject(ctx, item, true, false)
		actionObject = convertKeysToSnake(actionObject)

		actionMap := NativeBasicTypedAttributesToTerraform(ctx, actionObject, CheckActionModel.AttrTypes)

		actionTf, err := basetypes.NewObjectValue(CheckActionModel.AttrTypes, actionMap)
		if err != nil {
			oneerr := err[0]
			tflog.Debug(ctx, "Error converting actionMap to an object: "+oneerr.Summary()+" | "+oneerr.Detail())

		}
		actionsTf = append(actionsTf, actionTf)
	}
	if len(actionsTf) > 0 {
		actionsList, err := types.ListValueFrom(ctx, CheckActionModel, actionsTf)
		return actionsList, err
	} else {
		return types.ListNull(CheckActionModel), nil

	}
}

func convertChecksToTerraformList(ctx context.Context, checks interface{}, modelAttrs map[string]attr.Type, checkModel attr.Type) types.List {
	var objects []attr.Value

	checkList := reflect.ValueOf(checks)

	for i := 0; i < checkList.Len(); i++ {
		check := checkList.Index(i).Interface()
		checkObject, _ := JsonStructToObject(ctx, check, true, false)
		checkObject = convertKeysToSnake(checkObject)
		delete(checkObject, "actions")
		checkMap := NativeBasicTypedAttributesToTerraform(ctx, checkObject, modelAttrs)

		actionsValue := reflect.ValueOf(check).FieldByName("Actions").Interface()
		if actions, ok := actionsValue.(*[]CheckActionDTO); ok {
			actionsList, err := convertCheckActionToTerraformList(ctx, actions)
			if err != nil {
				tflog.Debug(ctx, "Error converting an array of actions to a list")
			}
			checkMap["actions"] = actionsList

			checkTf, err := basetypes.NewObjectValue(modelAttrs, checkMap)
			if err != nil {
				tflog.Debug(ctx, "Error converting checkMap to ObjectValue")
			}

			objects = append(objects, checkTf)
		}

	}

	checksTf, _ := types.ListValueFrom(ctx, checkModel, objects)

	return checksTf

}

func (dto *HostConfigDTO) ConvertToZitiResourceModel(ctx context.Context) hostV1ConfigResourceModel {

	res := hostV1ConfigResourceModel{
		Address:         types.StringPointerValue(dto.Address),
		Port:            types.Int32PointerValue(dto.Port),
		Protocol:        types.StringPointerValue(dto.Protocol),
		ForwardProtocol: types.BoolPointerValue(dto.ForwardProtocol),
		ForwardPort:     types.BoolPointerValue(dto.ForwardPort),
		ForwardAddress:  types.BoolPointerValue(dto.ForwardAddress),
	}
	res.AllowedProtocols = convertStringList(ctx, dto.AllowedProtocols, types.StringType)
	res.AllowedAddresses = convertStringList(ctx, dto.AllowedAddresses, types.StringType)
	res.AllowedSourceAddresses = convertStringList(ctx, dto.AllowedSourceAddresses, types.StringType)

	if dto.AllowedPortRanges != nil {
		var objects []attr.Value
		for _, allowedRange := range *dto.AllowedPortRanges {
			allowedRangeco, _ := JsonStructToObject(ctx, allowedRange, true, false)

			objectMap := NativeBasicTypedAttributesToTerraform(ctx, allowedRangeco, AllowedPortRangeModel.AttrTypes)
			object, _ := basetypes.NewObjectValue(AllowedPortRangeModel.AttrTypes, objectMap)
			objects = append(objects, object)
		}
		allowedPortRanges, _ := types.ListValueFrom(ctx, AllowedPortRangeModel, objects)
		res.AllowedPortRanges = allowedPortRanges
	} else {
		res.AllowedPortRanges = types.ListNull(AllowedPortRangeModel)
	}

	if dto.ListenOptions != nil {
		listenOptionsObject, _ := JsonStructToObject(ctx, *dto.ListenOptions, true, false)
		listenOptionsObject = convertKeysToSnake(listenOptionsObject)

		listenOptionsMap := NativeBasicTypedAttributesToTerraform(ctx, listenOptionsObject, ListenOptionsModel.AttrTypes)

		listenOptionsTf, err := basetypes.NewObjectValue(ListenOptionsModel.AttrTypes, listenOptionsMap)
		if err != nil {
			oneerr := err[0]
			tflog.Debug(ctx, "Error converting listenOptionsMap to an object: "+oneerr.Summary()+" | "+oneerr.Detail())
		}
		res.ListenOptions = listenOptionsTf
	} else {
		res.ListenOptions = types.ObjectNull(ListenOptionsModel.AttrTypes)
	}

	if dto.HTTPChecks != nil {
		res.HTTPChecks = convertChecksToTerraformList(ctx, *dto.HTTPChecks, HTTPCheckModel.AttrTypes, HTTPCheckModel)
	} else {
		res.HTTPChecks = types.ListNull(HTTPCheckModel)

	}

	if dto.PortChecks != nil {
		res.PortChecks = convertChecksToTerraformList(ctx, *dto.PortChecks, PortCheckModel.AttrTypes, PortCheckModel)
	} else {
		res.PortChecks = types.ListNull(PortCheckModel)
	}

	return res
}

func (r *hostV1ConfigResourceModel) ToHostConfigDTO(ctx context.Context) HostConfigDTO {
	listenOptions := AttributesToListenOptionsStruct(ctx, r.ListenOptions.Attributes())
	var portChecks []PortCheckDTO
	for _, v := range r.PortChecks.Elements() {
		if v, ok := v.(types.Object); ok {
			portCheck := AttributesToStruct[PortCheckDTO](ctx, v.Attributes())
			portChecks = append(portChecks, portCheck)
		}
	}
	var httpChecks []HTTPCheckDTO
	for _, v := range r.HTTPChecks.Elements() {
		if v, ok := v.(types.Object); ok {
			httpCheck := AttributesToStruct[HTTPCheckDTO](ctx, v.Attributes())
			httpChecks = append(httpChecks, httpCheck)
		}
	}

	hostConfigDto := HostConfigDTO{
		Address:                r.Address.ValueStringPointer(),
		Protocol:               r.Protocol.ValueStringPointer(),
		ListenOptions:          &listenOptions,
		PortChecks:             &portChecks,
		HTTPChecks:             &httpChecks,
		ForwardAddress:         r.ForwardAddress.ValueBoolPointer(),
		ForwardPort:            r.ForwardPort.ValueBoolPointer(),
		ForwardProtocol:        r.ForwardProtocol.ValueBoolPointer(),
		Port:                   r.Port.ValueInt32Pointer(),
		AllowedProtocols:       ElementsToStringArray(r.AllowedProtocols.Elements()),
		AllowedAddresses:       ElementsToStringArray(r.AllowedAddresses.Elements()),
		AllowedSourceAddresses: ElementsToStringArray(r.AllowedSourceAddresses.Elements()),
	}

	if len(r.AllowedPortRanges.Elements()) > 0 {
		allowedPortRanges := ElementsToListOfStructs[ConfigPortsDTO](ctx, r.AllowedPortRanges.Elements())
		hostConfigDto.AllowedPortRanges = &allowedPortRanges
	}

	return hostConfigDto
}

// Create a new resource.
func (r *hostV1ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan hostV1ConfigResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToHostConfigDTO(ctx), true, true)
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

	fmt.Printf("**********************create resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

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
func (r *hostV1ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state hostV1ConfigResourceModel
	tflog.Debug(ctx, "Reading Host Config")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, state.ID.ValueString())
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

	var hostConfigDto HostConfigDTO
	GenericFromObject(resourceData, &hostConfigDto)
	newState := hostConfigDto.ConvertToZitiResourceModel(ctx)
	fmt.Printf("**********************newState************************:\n %+v\n", newState)

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
func (r *hostV1ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan hostV1ConfigResourceModel
	tflog.Debug(ctx, "Updating Host Config")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state hostV1ConfigResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestObject, err := JsonStructToObject(ctx, eplan.ToHostConfigDTO(ctx), true, true)
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
	fmt.Printf("**********************update resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, state.ID.ValueString())
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
func (r *hostV1ConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state hostV1ConfigResourceModel
	tflog.Debug(ctx, "Deleting Host Config")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/configs/%s", r.resourceConfig.host, state.ID.ValueString())

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

func (r *hostV1ConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
