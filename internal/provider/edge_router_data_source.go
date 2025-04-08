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
	_ datasource.DataSource              = &edgeRouterDataSource{}
	_ datasource.DataSourceWithConfigure = &edgeRouterDataSource{}
)

// NewEdgeRouterDataSource is a helper function to simplify the provider implementation.
func NewEdgeRouterDataSource() datasource.DataSource {
	return &edgeRouterDataSource{}
}

// edgeRouterDataSource is the datasource implementation.
type edgeRouterDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *edgeRouterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *edgeRouterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_router"
}

// edgeRouterDataSourceModel maps the datasource schema data.
type edgeRouterDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Cost              types.Int64  `tfsdk:"cost"`
	RoleAttributes    types.List   `tfsdk:"role_attributes"`
	IsTunnelerEnabled types.Bool   `tfsdk:"is_tunnelerenabled"`
	NoTraversal       types.Bool   `tfsdk:"no_traversal"`
	Tags              types.Map    `tfsdk:"tags"`
	AppData           types.Map    `tfsdk:"app_data"`
}

// Schema defines the schema for the datasource.
func (r *edgeRouterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Edge Router Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the edge router",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Role Attributes",
			},
			"is_tunnelerenabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Tunneler Enabled Flag",
			},
			"no_traversal": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "No Traversal Flag",
			},
			"cost": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Cost",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Edge Router Tags",
			},
			"app_data": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "App Data of Edge Router",
			},
		},
	}
}

// Read datasource information.
func (r *edgeRouterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state edgeRouterDataSourceModel
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

	authUrl := fmt.Sprintf("%s/edge-routers?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading edge-routers", "Could not READ edge-routers, unexpected error: "+err.Error(),
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

	edgeRouter := jsonBody["data"].([]interface{})

	if len(edgeRouter) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(edgeRouter) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := edgeRouter[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if cost, ok := data["Cost"].(int64); ok {
		state.Cost = types.Int64Value(int64(cost))
	}

	if isTunnelerEnabled, ok := data["isTunnelerEnabled"].(bool); ok {
		state.IsTunnelerEnabled = types.BoolValue(isTunnelerEnabled)
	}

	if noTraversal, ok := data["noTraversal"].(bool); ok {
		state.NoTraversal = types.BoolValue(noTraversal)
	}

	if roleAttributes, ok := data["roleAttributes"].([]interface{}); ok {
		roleAttributes, diag := types.ListValueFrom(ctx, types.StringType, roleAttributes)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.RoleAttributes = roleAttributes
	} else {
		state.RoleAttributes = types.ListNull(types.StringType)
	}

	if _tags, ok := data["tags"].(map[string]interface{}); ok {
		if len(_tags) != 0 {
			_tags, diag := types.MapValueFrom(ctx, types.StringType, _tags)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.Tags = _tags
		} else {
			state.Tags = types.MapNull(types.StringType)
		}
	}

	if appData, ok := data["appData"].(map[string]interface{}); ok {
		if len(appData) != 0 {
			appData, diag := types.MapValueFrom(ctx, types.StringType, appData)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.AppData = appData
		} else {
			state.AppData = types.MapNull(types.StringType)
		}
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
