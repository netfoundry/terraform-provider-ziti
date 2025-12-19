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
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &posterCheckMultiProcessDataSource{}
	_ datasource.DataSourceWithConfigure = &posterCheckMultiProcessDataSource{}
)

// NewPostureCheckMultiProcessDataSource is a helper function to simplify the provider implementation.
func NewPostureCheckMultiProcessDataSource() datasource.DataSource {
	return &posterCheckMultiProcessDataSource{}
}

// posterCheckMultiProcessDataSource is the datasource implementation.
type posterCheckMultiProcessDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *posterCheckMultiProcessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *posterCheckMultiProcessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check_multi_process"
}

// posterCheckMultiProcessDataSourceModel maps the datasource schema data.
type posterCheckMultiProcessDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	RoleAttributes types.List   `tfsdk:"role_attributes"`
	Semantic       types.String `tfsdk:"semantic"`
	Processes      types.Set    `tfsdk:"processes"`
	Tags           types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *posterCheckMultiProcessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Posture Check Data Source, type: Multi Process check",
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
			"semantic": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Semantic Value",
			},
			"processes": schema.SetNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Path",
						},
						"os_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Operating System type",
						},
						"hashes": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "File hashes list",
						},
						"signer_fingerprints": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Signer fingerprints list",
						},
					},
				},
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
func (r *posterCheckMultiProcessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state posterCheckMultiProcessDataSourceModel
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

	if semanticValue, ok := data["semantic"].(string); ok {
		state.Semantic = types.StringValue(semanticValue)
	}

	if procs, ok := data["processes"].([]interface{}); ok && len(procs) > 0 {
		var processValues []attr.Value

		for _, p := range procs {
			proc, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			attrTypes := MultiProcessModel.AttrTypes
			values := make(map[string]attr.Value)

			if v, ok := proc["path"].(string); ok {
				values["path"] = types.StringValue(v)
			}

			if v, ok := proc["osType"].(string); ok {
				values["os_type"] = types.StringValue(v)
			}

			if fps, ok := proc["signerFingerprints"].([]interface{}); ok && len(fps) > 0 {
				list, diag := types.ListValueFrom(ctx, types.StringType, fps)
				resp.Diagnostics.Append(diag...)
				values["signer_fingerprints"] = list
			} else {
				values["signer_fingerprints"] = types.ListNull(types.StringType)
			}

			if hashes, ok := proc["hashes"].([]interface{}); ok && len(hashes) > 0 {
				list, diag := types.ListValueFrom(ctx, types.StringType, hashes)
				resp.Diagnostics.Append(diag...)
				values["hashes"] = list
			} else {
				values["hashes"] = types.ListNull(types.StringType)
			}

			obj, diag := types.ObjectValue(attrTypes, values)
			resp.Diagnostics.Append(diag...)
			processValues = append(processValues, obj)
		}

		list, diag := types.SetValue(types.ObjectType{AttrTypes: MultiProcessModel.AttrTypes}, processValues)
		resp.Diagnostics.Append(diag...)

		state.Processes = list
	} else {
		state.Processes = types.SetNull(types.ObjectType{AttrTypes: MultiProcessModel.AttrTypes})
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
