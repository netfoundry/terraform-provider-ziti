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
	_ datasource.DataSource              = &serviceEdgeRouterPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &serviceEdgeRouterPolicyDataSource{}
)

// NewServiceEdgeRouterPolicyDataSource is a helper function to simplify the provider implementation.
func NewServiceEdgeRouterPolicyDataSource() datasource.DataSource {
	return &serviceEdgeRouterPolicyDataSource{}
}

// serviceEdgeRouterPolicyDataSource is the datasource implementation.
type serviceEdgeRouterPolicyDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *serviceEdgeRouterPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *serviceEdgeRouterPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_edge_router_policy"
}

// serviceEdgeRouterPolicyDataSourceModel maps the datasource schema data.
type serviceEdgeRouterPolicyDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Semantic        types.String `tfsdk:"semantic"`
	ServiceRoles    types.List   `tfsdk:"serviceroles"`
	EdgeRouterRoles types.List   `tfsdk:"edgerouterroles"`
	Tags            types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *serviceEdgeRouterPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Service Edge Router Policy Data source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the service edge router policy",
			},
			"semantic": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Semantic Value",
			},
			"serviceroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Service Roles",
			},
			"edgerouterroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Edge Router Roles",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Service Edge Router Policy Tags",
			},
		},
	}
}

// Read datasource information.
func (r *serviceEdgeRouterPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state serviceEdgeRouterPolicyDataSourceModel
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

	authUrl := fmt.Sprintf("%s/service-edge-router-policies?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading service-edge-router-policies", "Could not READ service-edge-router-policies, unexpected error: "+err.Error(),
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

	serviceEdgeRouterPolicies := jsonBody["data"].([]interface{})

	if len(serviceEdgeRouterPolicies) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(serviceEdgeRouterPolicies) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := serviceEdgeRouterPolicies[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if semanticValue, ok := data["semantic"].(string); ok {
		state.Semantic = types.StringValue(semanticValue)
	}

	if edgeRouterRoles, ok := data["edgeRouterRoles"].([]interface{}); ok {
		edgeRouterRoles, diag := types.ListValueFrom(ctx, types.StringType, edgeRouterRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.EdgeRouterRoles = edgeRouterRoles
	} else {
		state.EdgeRouterRoles = types.ListNull(types.StringType)
	}

	if serviceRoles, ok := data["serviceRoles"].([]interface{}); ok {
		serviceRoles, diag := types.ListValueFrom(ctx, types.StringType, serviceRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.ServiceRoles = serviceRoles
	} else {
		state.ServiceRoles = types.ListNull(types.StringType)
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

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
