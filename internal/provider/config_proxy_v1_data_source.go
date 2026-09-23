package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rs/zerolog/log"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &proxyV1ConfigDataSource{}
	_ datasource.DataSourceWithConfigure = &proxyV1ConfigDataSource{}
)

// NewProxyV1ConfigDataSource is a helper function to simplify the provider implementation.
func NewProxyV1ConfigDataSource() datasource.DataSource {
	return &proxyV1ConfigDataSource{}
}

// proxyV1ConfigDataSource is the datasource implementation.
type proxyV1ConfigDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *proxyV1ConfigDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	datasourceConfig := req.ProviderData.(*zitiData)
	r.datasourceConfig = datasourceConfig

	tflog.Debug(ctx, "Configured ziti data source", map[string]any{"host": r.datasourceConfig.host})
}

// Metadata returns the datasource type name.
func (r *proxyV1ConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_proxy_v1_config"
}

// proxyV1ConfigDataSourceModel maps the datasource schema data.
type proxyV1ConfigDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Port         types.Int32  `tfsdk:"port"`
	Protocols    types.List   `tfsdk:"protocols"`
	Binding      types.String `tfsdk:"binding"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Tags         types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *proxyV1ConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti proxy v1 config Data Source",
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
			"port": schema.Int32Attribute{
				Computed:            true,
				MarkdownDescription: "The port to listen on.",
			},
			"protocols": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Protocols that the proxy will listen on.",
			},
			"binding": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The binding to use for this proxy config.",
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

func proxyResourceModelToDataSourceModel(resourceModel proxyV1ConfigResourceModel) proxyV1ConfigDataSourceModel {
	return proxyV1ConfigDataSourceModel{
		Name:      resourceModel.Name,
		Port:      resourceModel.Port,
		Protocols: resourceModel.Protocols,
		Binding:   resourceModel.Binding,
	}
}

// Read datasource information.
func (r *proxyV1ConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state proxyV1ConfigDataSourceModel
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
		return
	}

	stringBody := string(cresp)
	tflog.Debug(ctx, "read response", map[string]any{"response": stringBody})

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

	var proxyConfigDto ProxyConfigDTO
	GenericFromObject(datasourceData, &proxyConfigDto)
	resourceState := proxyConfigDto.ConvertToZitiResourceModel(ctx)
	newState := proxyResourceModelToDataSourceModel(resourceState)

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
