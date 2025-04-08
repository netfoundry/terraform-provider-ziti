package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &edgeRouterResource{}
	_ resource.ResourceWithConfigure   = &edgeRouterResource{}
	_ resource.ResourceWithImportState = &edgeRouterResource{}
)

// NewEdgeRouterResource is a helper function to simplify the provider implementation.
func NewEdgeRouterResource() resource.Resource {
	return &edgeRouterResource{}
}

// edgeRouterResource is the resource implementation.
type edgeRouterResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *edgeRouterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	resourceConfig := req.ProviderData.(*zitiData)
	r.resourceConfig = resourceConfig

	fmt.Printf("Using API Token to create resource: %s\n", r.resourceConfig.apiToken)
	fmt.Printf("Using domain to create resource: %s\n", r.resourceConfig.host)
}

// Metadata returns the resource type name.
func (r *edgeRouterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_router"
}

// edgeRouterResourceModel maps the resource schema data.
type edgeRouterResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Cost              types.Int64  `tfsdk:"cost"`
	RoleAttributes    types.List   `tfsdk:"role_attributes"`
	IsTunnelerEnabled types.Bool   `tfsdk:"is_tunnelerenabled"`
	NoTraversal       types.Bool   `tfsdk:"no_traversal"`
	Tags              types.Map    `tfsdk:"tags"`
	AppData           types.Map    `tfsdk:"app_data"`
	LastUpdated       types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *edgeRouterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Edge Router Resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Identifier",
			},
			"last_updated": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last Updated Time",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the edge router",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Role Attributes",
			},
			"is_tunnelerenabled": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Tunneler Enabled Flag",
			},
			"no_traversal": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "No Traversal Flag",
			},
			"cost": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				Default:  int64default.StaticInt64(0),
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
				MarkdownDescription: "Cost",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Edge Router Tags",
			},
			"app_data": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "App Data of Edge Router",
			},
		},
	}
}

// Create a new resource.
func (r *edgeRouterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan edgeRouterResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	cost_ := eplan.Cost.ValueInt64()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	appData := TagsFromAttributes(eplan.AppData.Elements())
	isTunnelerEnabled := eplan.IsTunnelerEnabled.ValueBool()
	noTraversal := eplan.NoTraversal.ValueBool()

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := rest_model.EdgeRouterCreate{
		Name:              &name,
		RoleAttributes:    &roleAttributes,
		Cost:              &cost_,
		NoTraversal:       &noTraversal,
		IsTunnelerEnabled: isTunnelerEnabled,
		Tags:              tags,
		AppData:           appData,
	}

	fmt.Printf("**********************create resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/edge-routers", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating edge-routers", "Could not Create edge-routers, unexpected error: "+err.Error(),
		)
		return
	}

	fmt.Printf("**********************create response************************:\n %s\n", cresp)
	resourceID := gjson.Get(cresp, "data.id").String()

	// Map response body to schema and populate Computed attribute values
	eplan.ID = types.StringValue(resourceID)
	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *edgeRouterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state edgeRouterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/edge-routers/%s", r.resourceConfig.host, state.ID.ValueString())
	cresp, err := ReadZitiResource(authUrl, r.resourceConfig.apiToken)
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
		// Handle error
		resp.Diagnostics.AddError(
			"Error Reading edge router", fmt.Sprintf("Could not READ edge router, ERROR %v: ", err.Error()),
		)
		return
	}

	stringBody := string(cresp)
	fmt.Printf("**********************read response************************:\n %s\n", stringBody)

	data, ok := jsonBody["data"].(map[string]interface{})
	if !ok {
		resp.Diagnostics.AddError("Error: ", "'data' is either missing or not a map[string]interface{}")
		return
	}

	// Manually assign individual values from the map to the struct fields
	state.Name = types.StringValue(data["name"].(string))

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

// Update updates the resource and sets the updated Terraform state on success.
func (r *edgeRouterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan edgeRouterResourceModel
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state edgeRouterResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	cost_ := eplan.Cost.ValueInt64()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	appData := TagsFromAttributes(eplan.AppData.Elements())
	isTunnelerEnabled := eplan.IsTunnelerEnabled.ValueBool()
	noTraversal := eplan.NoTraversal.ValueBool()

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := rest_model.EdgeRouterUpdate{
		Name:              &name,
		RoleAttributes:    &roleAttributes,
		Cost:              &cost_,
		NoTraversal:       &noTraversal,
		IsTunnelerEnabled: isTunnelerEnabled,
		Tags:              tags,
		AppData:           appData,
	}
	fmt.Printf("**********************update resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/edge-routers/%s", r.resourceConfig.host, state.ID.ValueString())
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PUT Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating edge-routers", "Could not Update edge-routers, unexpected error: "+err.Error(),
		)
		return
	}

	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *edgeRouterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state edgeRouterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/edge-routers/%s", r.resourceConfig.host, state.ID.ValueString())

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting edge-routers", "Could not DELETE edge-routers, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *edgeRouterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
