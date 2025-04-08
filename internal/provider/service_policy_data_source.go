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
	_ datasource.DataSource              = &servicePolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &servicePolicyDataSource{}
)

// NewServicePolicyDataSource is a helper function to simplify the provider implementation.
func NewServicePolicyDataSource() datasource.DataSource {
	return &servicePolicyDataSource{}
}

// servicePolicyDataSource is the datasource implementation.
type servicePolicyDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *servicePolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *servicePolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_policy"
}

// servicePolicyDataSourceModel maps the datasource schema data.
type servicePolicyDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Semantic          types.String `tfsdk:"semantic"`
	ServiceRoles      types.List   `tfsdk:"serviceroles"`
	IdentityRoles     types.List   `tfsdk:"identityroles"`
	PostureCheckRoles types.List   `tfsdk:"posturecheckroles"`
	Tags              types.Map    `tfsdk:"tags"`
	Type              types.String `tfsdk:"type"`
}

// Schema defines the schema for the datasource.
func (r *servicePolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Service Policy Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the service policy",
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
			"identityroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Identity Roles",
			},
			"posturecheckroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Posture Check Roles",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Service Policy Tags",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service Policy Type",
			},
		},
	}
}

// Read datasource information.
func (r *servicePolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state servicePolicyDataSourceModel
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

	authUrl := fmt.Sprintf("%s/service-policies?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading service-policies", "Could not READ service-policies, unexpected error: "+err.Error(),
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

	servicePolicies := jsonBody["data"].([]interface{})

	if len(servicePolicies) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(servicePolicies) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := servicePolicies[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if semanticValue, ok := data["semantic"].(string); ok {
		state.Semantic = types.StringValue(semanticValue)
	}

	if typeValue, ok := data["type"].(string); ok {
		state.Type = types.StringValue(typeValue)
	}

	if identityRoles, ok := data["identityRoles"].([]interface{}); ok {
		identityRoles, diag := types.ListValueFrom(ctx, types.StringType, identityRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.IdentityRoles = identityRoles
	} else {
		state.IdentityRoles = types.ListNull(types.StringType)
	}

	if serviceRoles, ok := data["serviceRoles"].([]interface{}); ok {
		serviceRoles, diag := types.ListValueFrom(ctx, types.StringType, serviceRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.ServiceRoles = serviceRoles
	} else {
		state.ServiceRoles = types.ListNull(types.StringType)
	}

	if postureCheckRoles, ok := data["postureCheckRoles"].([]interface{}); ok {
		postureCheckRoles, diag := types.ListValueFrom(ctx, types.StringType, postureCheckRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.PostureCheckRoles = postureCheckRoles
	} else {
		state.PostureCheckRoles = types.ListNull(types.StringType)
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
