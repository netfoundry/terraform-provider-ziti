package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	_ resource.Resource                = &hostV2ConfigResource{}
	_ resource.ResourceWithConfigure   = &hostV2ConfigResource{}
	_ resource.ResourceWithImportState = &hostV2ConfigResource{}
)

// NewHostV2ConfigResource is a helper function to simplify the provider implementation.
func NewHostV2ConfigResource() resource.Resource {
	return &hostV2ConfigResource{}
}

// hostV2ConfigResource is the resource implementation.
type hostV2ConfigResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *hostV2ConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *hostV2ConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_v2_config"
}

var HostConfigModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"address":                  types.StringType,
		"port":                     types.Int32Type,
		"protocol":                 types.StringType,
		"forward_protocol":         types.BoolType,
		"forward_port":             types.BoolType,
		"forward_address":          types.BoolType,
		"allowed_protocols":        types.ListType{ElemType: types.StringType},
		"allowed_addresses":        types.ListType{ElemType: types.StringType},
		"allowed_source_addresses": types.ListType{ElemType: types.StringType},
		"allowed_port_ranges":      types.ListType{ElemType: AllowedPortRangeModel},
		"listen_options":           ListenOptionsModel,
		"proxy":                    ProxyModel,
		"http_checks":              types.ListType{ElemType: HTTPCheckModel},
		"port_checks":              types.ListType{ElemType: PortCheckModel},
	},
}

// hostV2ConfigResourceModel maps the resource schema data.
type TerminatorsListDTO struct {
	Terminators []HostConfigDTO `json:"terminators"`
}

type hostV2ConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Terminators  types.List   `tfsdk:"terminators"`
	Tags         types.Map    `tfsdk:"tags"`
	LastUpdated  types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *hostV2ConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti host v2 config Resource",
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
			"config_type_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("host.v2"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The Id of a config-type",
			},
			"terminators": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
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
						"proxy": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"address": schema.StringAttribute{
									Optional: true,
									Computed: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString("http"),
									Validators: []validator.String{
										stringvalidator.OneOf("http"),
									},
								},
							},
							MarkdownDescription: "Proxy details.",
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
									//Default:  stringdefault.StaticString("5s"),
								},
								"cost": schema.Int32Attribute{
									Optional: true,
									Computed: true,
									//Default:  int32default.StaticInt32(0),
									Validators: []validator.Int32{
										int32validator.Between(0, 65535),
									},
								},
								"max_connections": schema.Int32Attribute{
									Optional: true,
									Computed: true,
									//Default:  int32default.StaticInt32(65535),
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
					},
				},
				MarkdownDescription: "List of terminators",
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

type hostV2Config struct {
	Address                types.String `tfsdk:"address"`
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
	Proxy                  types.Object `tfsdk:"proxy"`
	PortChecks             types.List   `tfsdk:"port_checks"`
	HTTPChecks             types.List   `tfsdk:"http_checks"`
}

func (m hostV2Config) ToTerraformObject(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	attrs := map[string]attr.Value{
		"address":                  m.Address,
		"port":                     m.Port,
		"protocol":                 m.Protocol,
		"forward_protocol":         m.ForwardProtocol,
		"forward_port":             m.ForwardPort,
		"forward_address":          m.ForwardAddress,
		"allowed_protocols":        m.AllowedProtocols,
		"allowed_addresses":        m.AllowedAddresses,
		"allowed_source_addresses": m.AllowedSourceAddresses,
		"allowed_port_ranges":      m.AllowedPortRanges,
		"listen_options":           m.ListenOptions,
		"proxy":                    m.Proxy,
		"http_checks":              m.HTTPChecks,
		"port_checks":              m.PortChecks,
	}

	//return basetypes.NewObjectValue(hostV2ConfigModelAttrTypes(), attrs)
	return basetypes.NewObjectValue(HostConfigModel.AttrTypes, attrs)
}

func hostV2ConfigModelType() types.ObjectType {
	return types.ObjectType{AttrTypes: hostV2ConfigModelAttrTypes()}
}

func hostV2ConfigModelAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"address":                  types.StringType,
		"port":                     types.Int32Type,
		"protocol":                 types.StringType,
		"forward_protocol":         types.BoolType,
		"forward_port":             types.BoolType,
		"forward_address":          types.BoolType,
		"allowed_protocols":        types.ListType{ElemType: types.StringType},
		"allowed_addresses":        types.ListType{ElemType: types.StringType},
		"allowed_source_addresses": types.ListType{ElemType: types.StringType},
		"allowed_port_ranges":      types.ListType{ElemType: AllowedPortRangeModel},
		"listen_options":           ListenOptionsModel,
		"proxy":                    ProxyModel,
		"http_checks":              types.ListType{ElemType: HTTPCheckModel},
		"port_checks":              types.ListType{ElemType: PortCheckModel},
	}
}

func (dto *HostConfigDTO) ConvertToZitiResourceModel2(ctx context.Context) hostV2Config {

	res := hostV2Config{
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

	if dto.Proxy != nil {
		proxyObject, _ := JsonStructToObject(ctx, *dto.Proxy, true, false)
		proxyObject = convertKeysToSnake(proxyObject)

		proxyMap := NativeBasicTypedAttributesToTerraform(ctx, proxyObject, ProxyModel.AttrTypes)

		proxyOptionsTf, err := basetypes.NewObjectValue(ProxyModel.AttrTypes, proxyMap)
		if err != nil {
			oneerr := err[0]
			tflog.Debug(ctx, "Error converting proxyMap to an object: "+oneerr.Summary()+" | "+oneerr.Detail())
		}
		res.Proxy = proxyOptionsTf
	} else {
		res.Proxy = types.ObjectNull(ProxyModel.AttrTypes)
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

func (r *hostV2ConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan hostV2ConfigResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var terminators []HostConfigDTO
	if !eplan.Terminators.IsNull() && !eplan.Terminators.IsUnknown() {
		for _, v := range eplan.Terminators.Elements() {
			if obj, ok := v.(types.Object); ok {
				dto := AttributesToStruct[HostConfigDTO](ctx, obj.Attributes())

				// http_checks
				if hcAttr, exists := obj.Attributes()["http_checks"]; exists && !hcAttr.IsNull() && !hcAttr.IsUnknown() {
					hcList, _ := hcAttr.(types.List)
					for _, hcItem := range hcList.Elements() {
						hcObj := hcItem.(types.Object)
						hcDTO := AttributesToStruct[HTTPCheckDTO](ctx, hcObj.Attributes())
						if dto.HTTPChecks == nil {
							dto.HTTPChecks = &[]HTTPCheckDTO{}
						}
						*dto.HTTPChecks = append(*dto.HTTPChecks, hcDTO)
					}
				}

				// port_checks
				if pcAttr, exists := obj.Attributes()["port_checks"]; exists && !pcAttr.IsNull() && !pcAttr.IsUnknown() {
					pcList, _ := pcAttr.(types.List)
					for _, pcItem := range pcList.Elements() {
						pcObj := pcItem.(types.Object)
						pcDTO := AttributesToStruct[PortCheckDTO](ctx, pcObj.Attributes())
						if dto.PortChecks == nil {
							dto.PortChecks = &[]PortCheckDTO{}
						}
						*dto.PortChecks = append(*dto.PortChecks, pcDTO)
					}
				}

				// listen_options
				if loAttr, exists := obj.Attributes()["listen_options"]; exists && !loAttr.IsNull() && !loAttr.IsUnknown() {
					loObj, _ := loAttr.(types.Object)
					loDTO := AttributesToStruct[ListenOptionsDTO](ctx, loObj.Attributes())
					dto.ListenOptions = &loDTO
				}

				// proxy
				if proxyAttr, exists := obj.Attributes()["proxy"]; exists && !proxyAttr.IsNull() && !proxyAttr.IsUnknown() {
					proxyObj, _ := proxyAttr.(types.Object)
					proxyDTO := AttributesToStruct[ProxyDTO](ctx, proxyObj.Attributes())
					dto.Proxy = &proxyDTO
				}

				// allowed_port_ranges
				if aprAttr, exists := obj.Attributes()["allowed_port_ranges"]; exists && !aprAttr.IsNull() && !aprAttr.IsUnknown() {
					aprList, _ := aprAttr.(types.List)
					for _, aprItem := range aprList.Elements() {
						aprObj := aprItem.(types.Object)
						aprDTO := AttributesToStruct[ConfigPortsDTO](ctx, aprObj.Attributes())
						if dto.AllowedPortRanges == nil {
							dto.AllowedPortRanges = &[]ConfigPortsDTO{}
						}
						*dto.AllowedPortRanges = append(*dto.AllowedPortRanges, aprDTO)
					}
				}

				terminators = append(terminators, dto)
			}
		}
	}

	wrapper := TerminatorsListDTO{Terminators: terminators}
	requestObject, err := JsonStructToObject2(ctx, wrapper, true, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error marshalling Ziti Config from API",
			"Could not create Ziti Config "+eplan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	name := eplan.Name.ValueString()
	configTypeId := eplan.ConfigTypeId.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	payload := rest_model.ConfigCreate{
		ConfigTypeID: &configTypeId,
		Name:         &name,
		Data:         requestObject,
		Tags:         tags,
	}

	// Convert payload to JSON for API request
	jsonData, _ := json.Marshal(payload)
	tflog.Debug(ctx, fmt.Sprintf("Final API request payload:\n%s", jsonData))

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

	resourceID := gjson.Get(cresp, "data.id").String()

	// Map response body to schema and populate Computed attribute values
	eplan.ID = types.StringValue(resourceID)
	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
}

// Read resource information.
func (r *hostV2ConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state hostV2ConfigResourceModel
	tflog.Debug(ctx, "Reading Host Config")
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

	resourceData := gjson.Get(cresp, "data.data")
	if !resourceData.Exists() {
		resp.Diagnostics.AddError("Missing data", "The config response had no 'data' object.")
		return
	}

	terminatorsJson := resourceData.Get("terminators")
	if terminatorsJson.Exists() && terminatorsJson.IsArray() {
		resultList := make([]attr.Value, 0)
		for _, term := range terminatorsJson.Array() {
			// Unmarshal terminator into HostConfigDTO
			var dto HostConfigDTO
			if err := json.Unmarshal([]byte(term.Raw), &dto); err != nil {
				resp.Diagnostics.AddError(
					"Failed to Unmarshal Terminator",
					err.Error(),
				)
				return
			}

			// Convert HostConfigDTO to Terraform Object
			tfModel := dto.ConvertToZitiResourceModel2(ctx)

			// Convert tfModel struct to Terraform Object value
			objVal, diag := tfModel.ToTerraformObject(ctx)
			resp.Diagnostics.Append(diag...)
			if resp.Diagnostics.HasError() {
				return
			}
			resultList = append(resultList, objVal)
		}

		// Assign list to state
		terminatorList, diag := types.ListValue(hostV2ConfigModelType(), resultList)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Terminators = terminatorList
	}

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

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *hostV2ConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan hostV2ConfigResourceModel
	tflog.Debug(ctx, "Updating Host Config")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state hostV2ConfigResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var terminators []HostConfigDTO
	if !eplan.Terminators.IsNull() && !eplan.Terminators.IsUnknown() {
		for _, v := range eplan.Terminators.Elements() {
			if obj, ok := v.(types.Object); ok {
				dto := AttributesToStruct[HostConfigDTO](ctx, obj.Attributes())

				// http_checks
				if hcAttr, exists := obj.Attributes()["http_checks"]; exists && !hcAttr.IsNull() && !hcAttr.IsUnknown() {
					hcList, _ := hcAttr.(types.List)
					for _, hcItem := range hcList.Elements() {
						hcObj := hcItem.(types.Object)
						hcDTO := AttributesToStruct[HTTPCheckDTO](ctx, hcObj.Attributes())
						if dto.HTTPChecks == nil {
							dto.HTTPChecks = &[]HTTPCheckDTO{}
						}
						*dto.HTTPChecks = append(*dto.HTTPChecks, hcDTO)
					}
				}

				// port_checks
				if pcAttr, exists := obj.Attributes()["port_checks"]; exists && !pcAttr.IsNull() && !pcAttr.IsUnknown() {
					pcList, _ := pcAttr.(types.List)
					for _, pcItem := range pcList.Elements() {
						pcObj := pcItem.(types.Object)
						pcDTO := AttributesToStruct[PortCheckDTO](ctx, pcObj.Attributes())
						if dto.PortChecks == nil {
							dto.PortChecks = &[]PortCheckDTO{}
						}
						*dto.PortChecks = append(*dto.PortChecks, pcDTO)
					}
				}

				// listen_options
				if loAttr, exists := obj.Attributes()["listen_options"]; exists && !loAttr.IsNull() && !loAttr.IsUnknown() {
					loObj, _ := loAttr.(types.Object)
					loDTO := AttributesToStruct[ListenOptionsDTO](ctx, loObj.Attributes())
					dto.ListenOptions = &loDTO
				}

				// proxy
				if proxyAttr, exists := obj.Attributes()["proxy"]; exists && !proxyAttr.IsNull() && !proxyAttr.IsUnknown() {
					proxyObj, _ := proxyAttr.(types.Object)
					proxyDTO := AttributesToStruct[ProxyDTO](ctx, proxyObj.Attributes())
					dto.Proxy = &proxyDTO
				}

				// allowed_port_ranges
				if aprAttr, exists := obj.Attributes()["allowed_port_ranges"]; exists && !aprAttr.IsNull() && !aprAttr.IsUnknown() {
					aprList, _ := aprAttr.(types.List)
					for _, aprItem := range aprList.Elements() {
						aprObj := aprItem.(types.Object)
						aprDTO := AttributesToStruct[ConfigPortsDTO](ctx, aprObj.Attributes())
						if dto.AllowedPortRanges == nil {
							dto.AllowedPortRanges = &[]ConfigPortsDTO{}
						}
						*dto.AllowedPortRanges = append(*dto.AllowedPortRanges, aprDTO)
					}
				}

				terminators = append(terminators, dto)
			}
		}
	}

	wrapper := TerminatorsListDTO{Terminators: terminators}
	requestObject, err := JsonStructToObject2(ctx, wrapper, true, true)
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
func (r *hostV2ConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state hostV2ConfigResourceModel
	tflog.Debug(ctx, "Deleting Host Config")
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

func (r *hostV2ConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
