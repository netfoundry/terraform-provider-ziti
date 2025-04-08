package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &identityResource{}
	_ resource.ResourceWithConfigure   = &identityResource{}
	_ resource.ResourceWithImportState = &identityResource{}
)

// NewIdentityResource is a helper function to simplify the provider implementation.
func NewIdentityResource() resource.Resource {
	return &identityResource{}
}

// identityResource is the resource implementation.
type identityResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *identityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *identityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity"
}

// identityResourceModel maps the resource schema data.
type identityResourceModel struct {
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
	LastUpdated              types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *identityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Identity Resource",
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
				MarkdownDescription: "Name of the Identity",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Role Attributes",
			},
			"auth_policy_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("default"),
				MarkdownDescription: "Auth Policy ID",
			},
			"external_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "External id of the identity.",
			},
			"is_admin": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Flag to controls whether an identity has admin rights",
			},
			"default_hosting_cost": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				Default:  int64default.StaticInt64(0),
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
				MarkdownDescription: "Cost of the service identity",
			},
			"default_hosting_precedence": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Default:  stringdefault.StaticString("default"),
				Validators: []validator.String{
					stringvalidator.OneOf("default", "required", "failed"),
				},
				MarkdownDescription: "Precedence of the service identity",
			},
			"service_hosting_costs": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.Int64Type)),
				MarkdownDescription: "Service Hosting Costs",
			},
			"service_hosting_precedence": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Service Hosting Precedence",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Identity Tags",
			},
			"app_data": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "App Data of Identity",
			},
			"type": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Default:  stringdefault.StaticString("Default"),
				Validators: []validator.String{
					stringvalidator.OneOf("User", "Device", "Service", "Router", "Default"),
				},
				MarkdownDescription: "Type of the identity.",
			},
		},
	}
}

// Create a new resource.
func (r *identityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan identityResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	appData := TagsFromAttributes(eplan.AppData.Elements())
	tags := TagsFromAttributes(eplan.Tags.Elements())
	name := eplan.Name.ValueString()
	authPolicyId := eplan.AuthPolicyID.ValueString()
	defaultHostingCost := rest_model.TerminatorCost(eplan.DefaultHostingCost.ValueInt64())
	defaultHostingPrecedence := rest_model.TerminatorPrecedence(eplan.DefaultHostingPrecedence.ValueString())
	externalId := eplan.ExternalID.ValueString()
	isAdmin := eplan.IsAdmin.ValueBool()

	serviceHostingCosts := make(rest_model.TerminatorCostMap)
	for key, value := range AttributesToNativeTypes(ctx, eplan.ServiceHostingCosts.Elements()) {
		if val, ok := value.(int64); ok {
			cost := rest_model.TerminatorCost(val)
			serviceHostingCosts[key] = &cost
		}
	}
	serviceHostingPrecedences := make(rest_model.TerminatorPrecedenceMap)
	for key, value := range AttributesToNativeTypes(ctx, eplan.ServiceHostingPrecedence.Elements()) {
		if val, ok := value.(string); ok {
			serviceHostingPrecedences[key] = rest_model.TerminatorPrecedence(val)
		}
	}
	type_ := rest_model.IdentityType(eplan.Type.ValueString())

	payload := rest_model.IdentityCreate{
		AppData:                   appData,
		AuthPolicyID:              &authPolicyId,
		DefaultHostingCost:        &defaultHostingCost,
		DefaultHostingPrecedence:  defaultHostingPrecedence,
		ExternalID:                &externalId,
		IsAdmin:                   &isAdmin,
		Name:                      &name,
		RoleAttributes:            &roleAttributes,
		ServiceHostingCosts:       serviceHostingCosts,
		ServiceHostingPrecedences: serviceHostingPrecedences,
		Tags:                      tags,
		Type:                      &type_,
	}

	fmt.Printf("**********************create resource payload***********************:\n %s\n", payload)
	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/identities", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Identity", "Could not Create Identity, unexpected error: "+err.Error(),
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
func (r *identityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state identityResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/identities/%s", r.resourceConfig.host, state.ID.ValueString())
	cresp, err := ReadZitiResource(authUrl, r.resourceConfig.apiToken)
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
		// Handle error
		resp.Diagnostics.AddError(
			"Error Reading Identity", fmt.Sprintf("Could not READ Identity, ERROR %v: ", err.Error()),
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

// Update updates the resource and sets the updated Terraform state on success.
func (r *identityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan identityResourceModel
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	appData := TagsFromAttributes(eplan.AppData.Elements())
	tags := TagsFromAttributes(eplan.Tags.Elements())
	name := eplan.Name.ValueString()
	authPolicyId := eplan.AuthPolicyID.ValueString()
	defaultHostingCost := rest_model.TerminatorCost(eplan.DefaultHostingCost.ValueInt64())
	defaultHostingPrecedence := rest_model.TerminatorPrecedence(eplan.DefaultHostingPrecedence.ValueString())
	externalId := eplan.ExternalID.ValueString()
	isAdmin := eplan.IsAdmin.ValueBool()

	serviceHostingCosts := make(rest_model.TerminatorCostMap)
	for key, value := range AttributesToNativeTypes(ctx, eplan.ServiceHostingCosts.Elements()) {
		if val, ok := value.(int64); ok {
			cost := rest_model.TerminatorCost(val)
			serviceHostingCosts[key] = &cost
		}
	}
	serviceHostingPrecedences := make(rest_model.TerminatorPrecedenceMap)
	for key, value := range AttributesToNativeTypes(ctx, eplan.ServiceHostingPrecedence.Elements()) {
		if val, ok := value.(string); ok {
			serviceHostingPrecedences[key] = rest_model.TerminatorPrecedence(val)
		}
	}
	type_ := rest_model.IdentityType(eplan.Type.ValueString())

	payload := rest_model.IdentityCreate{
		AppData:                   appData,
		AuthPolicyID:              &authPolicyId,
		DefaultHostingCost:        &defaultHostingCost,
		DefaultHostingPrecedence:  defaultHostingPrecedence,
		ExternalID:                &externalId,
		IsAdmin:                   &isAdmin,
		Name:                      &name,
		RoleAttributes:            &roleAttributes,
		ServiceHostingCosts:       serviceHostingCosts,
		ServiceHostingPrecedences: serviceHostingPrecedences,
		Tags:                      tags,
		Type:                      &type_,
	}

	fmt.Printf("**********************update resource payload***********************:\n %s\n", payload)
	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/identities/%s", r.resourceConfig.host, eplan.ID.ValueString())
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PUT Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Identity", "Could not Update Identity, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	eplan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *identityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state identityResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/identities/%s", r.resourceConfig.host, state.ID.ValueString())

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti DELETE Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Identity", "Could not DELETE Identity, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *identityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
