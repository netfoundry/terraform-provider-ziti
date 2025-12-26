package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/rs/zerolog/log"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &postureCheckOSDataSource{}
	_ datasource.DataSourceWithConfigure = &postureCheckOSDataSource{}
)

// NewPostureCheckOSDataSource is a helper function to simplify the provider implementation.
func NewPostureCheckOSDataSource() datasource.DataSource {
	return &postureCheckOSDataSource{}
}

// postureCheckOSDataSource is the datasource implementation.
type postureCheckOSDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *postureCheckOSDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *postureCheckOSDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check_os"
}

// postureCheckOSDataSourceModel maps the datasource schema data.
type postureCheckOSDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	RoleAttributes   types.List   `tfsdk:"role_attributes"`
	OperatingSystems types.Set    `tfsdk:"operating_systems"`
	Tags             types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *postureCheckOSDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Posture Check Data Source, type: OS check",
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
			"operating_systems": schema.SetNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Type of operating system",
						},
						"versions": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Min version and Max version of the os",
						},
					},
				},
				MarkdownDescription: "OS List",
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
func (r *postureCheckOSDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state postureCheckOSDataSourceModel
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

	if osList, ok := data["operatingSystems"].([]interface{}); ok && len(osList) > 0 {
		objects := make([]attr.Value, 0, len(osList))

		for _, osItem := range osList {
			osMap := osItem.(map[string]interface{})

			versionStrs := []string{}
			if vList, ok := osMap["versions"].([]interface{}); ok && len(vList) > 0 {
				for _, v := range vList {
					versionStrs = append(versionStrs, strings.TrimSpace(fmt.Sprintf("%v", v)))
				}
				sort.Strings(versionStrs)
			}

			var versionsList types.List
			if len(versionStrs) == 0 {
				versionsList = types.ListNull(types.StringType)
			} else {
				versionsList, _ = types.ListValueFrom(ctx, types.StringType, versionStrs)
			}

			osType := strings.TrimSpace(osMap["type"].(string))

			objectMap := map[string]attr.Value{
				"type":     types.StringValue(osType),
				"versions": versionsList,
			}

			obj, _ := basetypes.NewObjectValue(OperatingSystemModel.AttrTypes, objectMap)
			objects = append(objects, obj)
		}

		osSetValue, _ := types.SetValueFrom(ctx, OperatingSystemModel, objects)
		state.OperatingSystems = osSetValue
	} else {
		state.OperatingSystems = types.SetNull(OperatingSystemModel)
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
