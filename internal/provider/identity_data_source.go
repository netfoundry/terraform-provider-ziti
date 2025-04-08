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
	_ datasource.DataSource              = &identityDataSource{}
	_ datasource.DataSourceWithConfigure = &identityDataSource{}
)

// NewIdentityDataSource is a helper function to simplify the provider implementation.
func NewIdentityDataSource() datasource.DataSource {
	return &identityDataSource{}
}

// identityDataSource is the datasource implementation.
type identityDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *identityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *identityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity"
}

// identityDataSourceModel maps the datasource schema data.
type identityDataSourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	RoleAttributes           types.List   `tfsdk:"role_attributes"`
	AuthPolicyID             types.String `tfsdk:"auth_policy_id"`
	ExternalID               types.String `tfsdk:"external_id"`
	IsAdmin                  types.Bool   `tfsdk:"is_admin"`
	DefaultHostingCost       types.Int64  `tfsdk:"default_hosting_cost"`
	DefaultHostingPrecedence types.String `tfsdk:"default_hosting_precedence"`
	ServiceHostingCosts      types.Map    `tfsdk:"service_hosting_costs"`
	ServiceHostingPrecedence types.Map    `tfsdk:"service_hosting_precedence"`
	Tags                     types.Map    `tfsdk:"tags"`
	AppData                  types.Map    `tfsdk:"app_data"`
	Type                     types.String `tfsdk:"type"`
}

// Schema defines the schema for the datasource.
func (r *identityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Identity Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the Identity",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Role Attributes",
			},
			"auth_policy_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Auth Policy ID",
			},
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "External id of the identity.",
			},
			"is_admin": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Flag to controls whether an identity has admin rights",
			},
			"default_hosting_cost": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Cost of the service identity",
			},
			"default_hosting_precedence": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Precedence of the service identity",
			},
			"service_hosting_costs": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Service Hosting Costs",
			},
			"service_hosting_precedence": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Service Hosting Precedence",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Identity Tags",
			},
			"app_data": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "App Data of Identity",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Type of the identity.",
			},
		},
	}
}

// Read datasource information.
func (r *identityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state identityDataSourceModel
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

	authUrl := fmt.Sprintf("%s/identities?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Identity", "Could not READ Identity, unexpected error: "+err.Error(),
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

	identity := jsonBody["data"].([]interface{})

	if len(identity) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(identity) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := identity[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if appData, ok := data["appData"].(map[string]interface{}); ok {
		if len(appData) != 0 {
			appData, diag := types.MapValueFrom(ctx, types.StringType, appData)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.AppData = appData
		} else {
			state.AppData = types.MapNull(types.StringType)
		}
	}

	if authPolicyID, ok := data["authPolicyId"].(string); ok {
		state.AuthPolicyID = types.StringValue(authPolicyID)
	}

	if defaultHostingCost, ok := data["defaultHostingCost"].(float64); ok {
		state.DefaultHostingCost = types.Int64Value(int64(defaultHostingCost))
	}

	if defaultHostingPrecedence, ok := data["defaultHostingPrecedence"].(string); ok {
		state.DefaultHostingPrecedence = types.StringValue(defaultHostingPrecedence)
	}

	if externalID, ok := data["externalId"].(string); ok {
		state.ExternalID = types.StringValue(externalID)
	}

	if isAdmin, ok := data["isAdmin"].(bool); ok {
		state.IsAdmin = types.BoolValue(isAdmin)
	}

	if roleAttributes, ok := data["roleAttributes"].([]interface{}); ok {
		roleAttributes, diag := types.ListValueFrom(ctx, types.StringType, roleAttributes)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.RoleAttributes = roleAttributes
	} else {
		state.RoleAttributes = types.ListNull(types.StringType)
	}

	if serviceHostingCosts, ok := data["serviceHostingCosts"].(map[string]interface{}); ok {
		if len(serviceHostingCosts) > 0 {
			serviceHostingCosts, diag := types.MapValueFrom(ctx, types.Int64Type, serviceHostingCosts)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.ServiceHostingCosts = serviceHostingCosts
		} else {
			state.ServiceHostingCosts = types.MapNull(types.Int64Type)
		}
	}

	if serviceHostingPrecedence, ok := data["serviceHostingPrecedences"].(map[string]interface{}); ok {
		if len(serviceHostingPrecedence) > 0 {
			serviceHostingPrecedence, diag := types.MapValueFrom(ctx, types.StringType, serviceHostingPrecedence)
			resp.Diagnostics = append(resp.Diagnostics, diag...)
			state.ServiceHostingPrecedence = serviceHostingPrecedence
		} else {
			state.ServiceHostingPrecedence = types.MapNull(types.StringType)
		}
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

	_type := data["type"].(map[string]interface{})
	if typeValue, ok := _type["name"].(string); ok {
		state.Type = types.StringValue(typeValue)
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
