package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &hostV2ConfigDataSource{}
	_ datasource.DataSourceWithConfigure = &hostV2ConfigDataSource{}
)

// NewHostV2ConfigDataSource is a helper function to simplify the provider implementation.
func NewHostV2ConfigDataSource() datasource.DataSource {
	return &hostV2ConfigDataSource{}
}

// hostV2ConfigDataSource is the datasource implementation.
type hostV2ConfigDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *hostV2ConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	datasourceConfig := req.ProviderData.(*zitiData)
	r.datasourceConfig = datasourceConfig

	fmt.Printf("Using API Token to create datasource: %s\n", r.datasourceConfig.apiToken)
	fmt.Printf("Using domain to create datasource: %s\n", r.datasourceConfig.host)
}

// Metadata returns the datasource type name.
func (r *hostV2ConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_v2_config"
}

// hostV2ConfigDataSourceModel maps the datasource schema data.
type hostV2ConfigDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Terminators  types.List   `tfsdk:"terminators"`
	Tags         types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *hostV2ConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti host v2 config Datasource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the config",
			},
			"config_type_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Id of a config-type",
			},
			"terminators": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Target host config address",
						},
						"port": schema.Int32Attribute{
							Computed:            true,
							MarkdownDescription: "Port of a target address",
						},
						"protocol": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Protocol which config would be allowed to receive",
						},
						"forward_protocol": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Flag which controls whether to forward allowedProtocols",
						},
						"forward_port": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Flag which controls whether to forward allowedPortRanges",
						},
						"forward_address": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Flag which controls whether to forward allowedAddresses",
						},
						"allowed_addresses": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Addresses that can be forwarded.",
						},
						"allowed_source_addresses": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Source addresses that can be forwarded.",
						},
						"proxy": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"address": schema.StringAttribute{
									Computed: true,
								},
								"type": schema.StringAttribute{
									Computed: true,
								},
							},
							MarkdownDescription: "Proxy details.",
						},
						"listen_options": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"bind_using_edge_identity": schema.BoolAttribute{
									Computed: true,
								},
								"connect_timeout": schema.StringAttribute{
									Computed: true,
								},
								"cost": schema.Int32Attribute{
									Computed: true,
								},
								"max_connections": schema.Int32Attribute{
									Computed: true,
								},
								"precedence": schema.StringAttribute{
									Computed: true,
								},
							},
							MarkdownDescription: "Listen Options.",
						},
						"http_checks": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"url": schema.StringAttribute{
										Computed: true,
									},
									"method": schema.StringAttribute{
										Computed: true,
									},
									"body": schema.StringAttribute{
										Computed: true,
									},
									"expect_status": schema.Int32Attribute{
										Computed: true,
									},
									"expect_in_body": schema.StringAttribute{
										Computed: true,
									},
									"interval": schema.StringAttribute{
										Computed: true,
									},
									"timeout": schema.StringAttribute{
										Computed: true,
									},
									"actions": schema.ListNestedAttribute{
										Computed: true,
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"trigger": schema.StringAttribute{
													Computed: true,
												},
												"duration": schema.StringAttribute{
													Computed: true,
												},
												"action": schema.StringAttribute{
													Computed: true,
												},
												"consecutive_events": schema.Int32Attribute{
													Computed: true,
												},
											},
										},
									},
								},
							},
							MarkdownDescription: "HTTP Checks.",
						},
						"port_checks": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"address": schema.StringAttribute{
										Computed: true,
									},
									"interval": schema.StringAttribute{
										Computed: true,
									},
									"timeout": schema.StringAttribute{
										Computed: true,
									},
									"actions": schema.ListNestedAttribute{
										Computed: true,
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"trigger": schema.StringAttribute{
													Computed: true,
												},
												"duration": schema.StringAttribute{
													Computed: true,
												},
												"action": schema.StringAttribute{
													Computed: true,
												},
												"consecutive_events": schema.Int32Attribute{
													Computed: true,
												},
											},
										},
									},
								},
							},
							MarkdownDescription: "Port Checks.",
						},
						"allowed_protocols": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Protocols that can be forwarded.",
						},
						"allowed_port_ranges": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"low": schema.Int32Attribute{
										Computed: true,
									},
									"high": schema.Int32Attribute{
										Computed: true,
									},
								},
							},
							MarkdownDescription: "Ports that can be forwarded.",
						},
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Config Tags",
			},
		},
	}
}

func (r *hostV2ConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state hostV2ConfigDataSourceModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter := ""
	if state.Name.ValueString() != "" {
		filter = "filter=name=\"" + state.Name.ValueString() + "\""
	}
	if state.ID.ValueString() != "" {
		filter = "filter=id=\"" + state.ID.ValueString() + "\""
	}

	authUrl := fmt.Sprintf("%s/configs?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading configs", "Could not READ configs, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		log.Error().Msgf("Error unmarshalling JSON response from Ziti DataSource Response: %v", err)
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	stringBody := string(cresp)
	fmt.Printf("**********************read response************************:\n %s\n", stringBody)

	_config := jsonBody["data"].([]interface{})
	if len(_config) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(_config) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := _config[0].(map[string]interface{})
	datasourceData := gjson.Get(cresp, "data.0.data")
	if !datasourceData.Exists() {
		resp.Diagnostics.AddError("Missing data", "The config response had no 'data' object.")
		return
	}

	terminatorsJson := datasourceData.Get("terminators")
	if terminatorsJson.Exists() && terminatorsJson.IsArray() {
		resultList := make([]attr.Value, 0)
		for _, term := range terminatorsJson.Array() {
			// Unmarshal 1 terminator into HostConfigDTO
			var dto HostConfigDTO
			if err := json.Unmarshal([]byte(term.Raw), &dto); err != nil {
				resp.Diagnostics.AddError(
					"Failed to Unmarshal Terminator",
					err.Error(),
				)
				return
			}

			// Convert HostConfigDTO → Terraform Object
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
	state.ID = types.StringValue(data["id"].(string))
	state.ConfigTypeId = types.StringValue(data["configTypeId"].(string))

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
