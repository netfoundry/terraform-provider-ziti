package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &serviceEdgeRouterPolicyResource{}
	_ resource.ResourceWithConfigure   = &serviceEdgeRouterPolicyResource{}
	_ resource.ResourceWithImportState = &serviceEdgeRouterPolicyResource{}
)

// NewServiceEdgeRouterPolicyResource is a helper function to simplify the provider implementation.
func NewServiceEdgeRouterPolicyResource() resource.Resource {
	return &serviceEdgeRouterPolicyResource{}
}

// serviceEdgeRouterPolicyResource is the resource implementation.
type serviceEdgeRouterPolicyResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *serviceEdgeRouterPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *serviceEdgeRouterPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_edge_router_policy"
}

// serviceEdgeRouterPolicyResourceModel maps the resource schema data.
type serviceEdgeRouterPolicyResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Semantic        types.String `tfsdk:"semantic"`
	ServiceRoles    types.List   `tfsdk:"serviceroles"`
	EdgeRouterRoles types.List   `tfsdk:"edgerouterroles"`
	Tags            types.Map    `tfsdk:"tags"`
	LastUpdated     types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *serviceEdgeRouterPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Service Edge Router Policy Resource",
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
				MarkdownDescription: "Name of the service edge router policy",
			},
			"semantic": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("AllOf"),
				Validators: []validator.String{
					stringvalidator.OneOf("AnyOf", "AllOf"),
				},
				MarkdownDescription: "Semantic Value",
			},
			"serviceroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Service Roles",
			},
			"edgerouterroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Edge Router Roles",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Service Edge Router Policy Tags",
			},
		},
	}
}

// Create a new resource.
func (r *serviceEdgeRouterPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan serviceEdgeRouterPolicyResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	semantic := rest_model.Semantic(eplan.Semantic.ValueString())
	tags := TagsFromAttributes(eplan.Tags.Elements())

	var edgeRouterRoles rest_model.Roles
	for _, value := range eplan.EdgeRouterRoles.Elements() {
		if edgeRouterRole, ok := value.(types.String); ok {
			edgeRouterRoles = append(edgeRouterRoles, edgeRouterRole.ValueString())
		}
	}

	var serviceRoles rest_model.Roles
	for _, value := range eplan.ServiceRoles.Elements() {
		if serviceRole, ok := value.(types.String); ok {
			serviceRoles = append(serviceRoles, serviceRole.ValueString())
		}
	}

	payload := rest_model.ServiceEdgeRouterPolicyCreate{
		EdgeRouterRoles: edgeRouterRoles,
		Name:            &name,
		Semantic:        &semantic,
		ServiceRoles:    serviceRoles,
		Tags:            tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/service-edge-router-policies", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating service-edge-router-policies", "Could not Create service-edge-router-policies, unexpected error: "+err.Error(),
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
func (r *serviceEdgeRouterPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state serviceEdgeRouterPolicyResourceModel
	tflog.Debug(ctx, "Reading Service Edge Router Policy")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/service-edge-router-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := ReadZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		if cresp == "" || IsNotFoundError(err) {
			msg := fmt.Sprintf("Resource not found in backend; removing from state, id: %s", state.ID.ValueString())
			log.Info().Msg(msg)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading service-edge-router-policies", "Could not READ service-edge-router-policies, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		// Handle error
		resp.Diagnostics.AddError(
			"Error Reading service-edge-router-policies", fmt.Sprintf("Could not READ service-edge-router-policies, ERROR %v: ", err.Error()),
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

// Update updates the resource and sets the updated Terraform state on success.
func (r *serviceEdgeRouterPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan serviceEdgeRouterPolicyResourceModel
	tflog.Debug(ctx, "Updating Service Edge Router Policy")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state serviceEdgeRouterPolicyResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	semantic := rest_model.Semantic(eplan.Semantic.ValueString())
	tags := TagsFromAttributes(eplan.Tags.Elements())

	var edgeRouterRoles rest_model.Roles
	for _, value := range eplan.EdgeRouterRoles.Elements() {
		if edgeRouterRole, ok := value.(types.String); ok {
			edgeRouterRoles = append(edgeRouterRoles, edgeRouterRole.ValueString())
		}
	}

	var serviceRoles rest_model.Roles
	for _, value := range eplan.ServiceRoles.Elements() {
		if serviceRole, ok := value.(types.String); ok {
			serviceRoles = append(serviceRoles, serviceRole.ValueString())
		}
	}

	payload := rest_model.ServiceEdgeRouterPolicyUpdate{
		EdgeRouterRoles: edgeRouterRoles,
		Name:            &name,
		Semantic:        &semantic,
		ServiceRoles:    serviceRoles,
		Tags:            tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/service-edge-router-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PUT Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating service-edge-router-policies", "Could not Update service-edge-router-policies, unexpected error: "+err.Error(),
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
func (r *serviceEdgeRouterPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state serviceEdgeRouterPolicyResourceModel
	tflog.Debug(ctx, "Deleting Service Edge Router Policy")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/service-edge-router-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting service-edge-router-policies", "Could not DELETE service-edge-router-policies, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *serviceEdgeRouterPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
