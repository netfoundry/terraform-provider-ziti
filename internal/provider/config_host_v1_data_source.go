package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rs/zerolog/log"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &hostV1ConfigDataSource{}
	_ datasource.DataSourceWithConfigure = &hostV1ConfigDataSource{}
)

// NewHostV1ConfigDataSource is a helper function to simplify the provider implementation.
func NewHostV1ConfigDataSource() datasource.DataSource {
	return &hostV1ConfigDataSource{}
}

// hostV1ConfigDataSource is the datasource implementation.
type hostV1ConfigDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *hostV1ConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *hostV1ConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_v1_config"
}

// hostV1ConfigDataSourceModel maps the datasource schema data.
type hostV1ConfigDataSourceModel struct {
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
}

// Schema defines the schema for the datasource.
func (r *hostV1ConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti host v1 config Datasource",
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
			"config_type_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Id of a config-type",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Config Tags",
			},
		},
	}
}

func ResourceModelToDataSourceModel(resourceModel hostV1ConfigResourceModel) hostV1ConfigDataSourceModel {
	dataSourceModel := hostV1ConfigDataSourceModel{
		Name:                   resourceModel.Name,
		Address:                resourceModel.Address,
		Port:                   resourceModel.Port,
		Protocol:               resourceModel.Protocol,
		ForwardProtocol:        resourceModel.ForwardProtocol,
		ForwardPort:            resourceModel.ForwardPort,
		ForwardAddress:         resourceModel.ForwardAddress,
		AllowedProtocols:       resourceModel.AllowedProtocols,
		AllowedAddresses:       resourceModel.AllowedAddresses,
		AllowedSourceAddresses: resourceModel.AllowedSourceAddresses,
		AllowedPortRanges:      resourceModel.AllowedPortRanges,
		ListenOptions:          resourceModel.ListenOptions,
		PortChecks:             resourceModel.PortChecks,
		HTTPChecks:             resourceModel.HTTPChecks,
	}
	return dataSourceModel

}

// Read datasource information.
func (r *hostV1ConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state hostV1ConfigDataSourceModel
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
	datasourceData := data["data"].(map[string]interface{})

	var hostConfigDto HostConfigDTO
	GenericFromObject(datasourceData, &hostConfigDto)
	resourceState := hostConfigDto.ConvertToZitiResourceModel(ctx)
	newState := ResourceModelToDataSourceModel(resourceState)

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

	newState.ID = state.ID
	newState.Name = state.Name
	newState.ConfigTypeId = state.ConfigTypeId
	newState.Tags = state.Tags
	state = newState

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
