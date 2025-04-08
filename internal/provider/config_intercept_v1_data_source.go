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
	_ datasource.DataSource              = &interceptV1ConfigDataSource{}
	_ datasource.DataSourceWithConfigure = &interceptV1ConfigDataSource{}
)

// NewInterceptV1ConfigDataSource is a helper function to simplify the provider implementation.
func NewInterceptV1ConfigDataSource() datasource.DataSource {
	return &interceptV1ConfigDataSource{}
}

// interceptV1ConfigDataSource is the datasource implementation.
type interceptV1ConfigDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *interceptV1ConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *interceptV1ConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_intercept_v1_config"
}

// interceptV1ConfigDataSourceModel maps the datasource schema data.
type interceptV1ConfigDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Addresses    types.List   `tfsdk:"addresses"`
	DialOptions  types.Object `tfsdk:"dial_options"`
	PortRanges   types.List   `tfsdk:"port_ranges"`
	Protocols    types.List   `tfsdk:"protocols"`
	SourceIP     types.String `tfsdk:"source_ip"`
	ConfigTypeId types.String `tfsdk:"config_type_id"`
	Tags         types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *interceptV1ConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti intercept v1 config Data Source",
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
			"addresses": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Target host config address",
			},
			"dial_options": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"connect_timeout_seconds": schema.Int32Attribute{
						Computed: true,
					},
					"identity": schema.StringAttribute{
						Computed: true,
					},
				},
				MarkdownDescription: "Dial Options.",
			},
			"protocols": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Protocols that can be forwarded.",
			},
			"port_ranges": schema.ListNestedAttribute{
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
			"source_ip": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Source IP",
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

func resourceModelToDataSourceModel(resourceModel interceptV1ConfigResourceModel) interceptV1ConfigDataSourceModel {
	dataSourceModel := interceptV1ConfigDataSourceModel{
		Name:        resourceModel.Name,
		Addresses:   resourceModel.Addresses,
		DialOptions: resourceModel.DialOptions,
		PortRanges:  resourceModel.PortRanges,
		Protocols:   resourceModel.Protocols,
		SourceIP:    resourceModel.SourceIP,
	}
	return dataSourceModel

}

// Read datasource information.
func (r *interceptV1ConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state interceptV1ConfigDataSourceModel
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

	var interceptConfigDto InterceptConfigDTO
	GenericFromObject(datasourceData, &interceptConfigDto)
	resourceState := interceptConfigDto.ConvertToZitiResourceModel(ctx)
	newState := resourceModelToDataSourceModel(resourceState)

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
