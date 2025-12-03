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
	_ datasource.DataSource              = &posterCheckMFADataSource{}
	_ datasource.DataSourceWithConfigure = &posterCheckMFADataSource{}
)

// NewPostureCheckMFADataSource is a helper function to simplify the provider implementation.
func NewPostureCheckMFADataSource() datasource.DataSource {
	return &posterCheckMFADataSource{}
}

// posterCheckMFADataSource is the datasource implementation.
type posterCheckMFADataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *posterCheckMFADataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *posterCheckMFADataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check_mfa"
}

// posterCheckMFADataSourceModel maps the datasource schema data.
type posterCheckMFADataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	RoleAttributes types.List   `tfsdk:"role_attributes"`
	PromptOnUnlock types.Bool   `tfsdk:"prompt_on_unlock"`
	PromptOnWake   types.Bool   `tfsdk:"prompt_on_wake"`
	TimeoutSeconds types.Int64  `tfsdk:"timeout_seconds"`
	Tags           types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *posterCheckMFADataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Posture Check Data Source, type: MFA check",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the Posture Check",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Role Attributes",
			},
			"prompt_on_unlock": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Prompt mfa when device unlocks.",
			},
			"prompt_on_wake": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Prompt mfa when device wakes.",
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "MFA check time out in seconds.",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Posture Check Tags",
			},
		},
	}
}

// Read datasource information.
func (r *posterCheckMFADataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state posterCheckMFADataSourceModel
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

	authUrl := fmt.Sprintf("%s/posture-checks?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading posture check", "Could not READ posture check, unexpected error: "+err.Error(),
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

	service := jsonBody["data"].([]interface{})

	if len(service) > 1 {
		resp.Diagnostics.AddError(
			"Multiple items returned from API upon filter execution!", "Try to narrow down the filter expression"+filter,
		)
	}
	if len(service) == 0 {
		resp.Diagnostics.AddError(
			"No items returned from API upon filter execution!", "Try to relax the filter expression: "+filter,
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data := service[0].(map[string]interface{})

	state.Name = types.StringValue(data["name"].(string))
	state.ID = types.StringValue(data["id"].(string))

	if timeoutSeconds, ok := data["timeoutSeconds"].(float64); ok {
		state.TimeoutSeconds = types.Int64Value(int64(timeoutSeconds))
	}

	if promptOnWake, ok := data["promptOnWake"].(bool); ok {
		state.PromptOnWake = types.BoolValue(promptOnWake)
	} else {
		state.PromptOnWake = types.BoolValue(false)
	}

	if promptOnUnlock, ok := data["promptOnUnlock"].(bool); ok {
		state.PromptOnUnlock = types.BoolValue(promptOnUnlock)
	} else {
		state.PromptOnUnlock = types.BoolValue(false)
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

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
