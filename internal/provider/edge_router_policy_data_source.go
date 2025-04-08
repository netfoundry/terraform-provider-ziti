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
	_ datasource.DataSource              = &edgeRouterPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &edgeRouterPolicyDataSource{}
)

// NewEdgeRouterPolicyDataSource is a helper function to simplify the provider implementation.
func NewEdgeRouterPolicyDataSource() datasource.DataSource {
	return &edgeRouterPolicyDataSource{}
}

// edgeRouterPolicyDataSource is the datasource implementation.
type edgeRouterPolicyDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *edgeRouterPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	datasourceConfig := req.ProviderData.(*zitiData)
	r.datasourceConfig = datasourceConfig

	fmt.Printf("Using API Token for datasource: %s\n", r.datasourceConfig.apiToken)
	fmt.Printf("Using domain for datasource: %s\n", r.datasourceConfig.host)
}

// Metadata returns the datasource type name.
func (r *edgeRouterPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_router_policy"
}

// edgeRouterPolicyDataSourceModel maps the datasource schema data.
type edgeRouterPolicyDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	EdgeRouterRoles types.List   `tfsdk:"edgerouterroles"`
	IdentityRoles   types.List   `tfsdk:"identityroles"`
	Semantic        types.String `tfsdk:"semantic"`
	Tags            types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *edgeRouterPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Edge Router Policy Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the edge router policy",
			},
			"semantic": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Semantic Value",
			},
			"edgerouterroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Edge Router Roles",
			},
			"identityroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Identity Roles",
			},
			"tags": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

// Read datasource information.
func (r *edgeRouterPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state edgeRouterPolicyDataSourceModel
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

	authUrl := fmt.Sprintf("%s/edge-router-policies?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading ERP", "Could not READ ERP, unexpected error: "+err.Error(),
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

	edgeRouterPolicies := jsonBody["data"].([]interface{})

	if len(edgeRouterPolicies) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(edgeRouterPolicies) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := edgeRouterPolicies[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if semanticValue, ok := data["semantic"].(string); ok {
		state.Semantic = types.StringValue(semanticValue)
	}

	if identityRoles, ok := data["identityRoles"].([]interface{}); ok {
		identityRoles, diag := types.ListValueFrom(ctx, types.StringType, identityRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.IdentityRoles = identityRoles
	} else {
		state.IdentityRoles = types.ListNull(types.StringType)
	}

	if edgeRouterRoles, ok := data["edgeRouterRoles"].([]interface{}); ok {
		edgeRouterRoles, diag := types.ListValueFrom(ctx, types.StringType, edgeRouterRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.EdgeRouterRoles = edgeRouterRoles
	} else {
		state.EdgeRouterRoles = types.ListNull(types.StringType)
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
