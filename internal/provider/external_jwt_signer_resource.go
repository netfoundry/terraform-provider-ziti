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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	_ resource.Resource                = &jwtSignerResource{}
	_ resource.ResourceWithConfigure   = &jwtSignerResource{}
	_ resource.ResourceWithImportState = &jwtSignerResource{}
)

// NewJwtSignerResource is a helper function to simplify the provider implementation.
func NewJwtSignerResource() resource.Resource {
	return &jwtSignerResource{}
}

// jwtSignerResource is the resource implementation.
type jwtSignerResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *jwtSignerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *jwtSignerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_jwt_signer"
}

// jwtSignerResourceModel maps the resource schema data.
type jwtSignerResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Issuer          types.String `tfsdk:"issuer"`
	Audience        types.String `tfsdk:"audience"`
	ClaimsProperty  types.String `tfsdk:"claims_property"`
	UseExternalId   types.Bool   `tfsdk:"use_external_id"`
	ClientID        types.String `tfsdk:"client_id"`
	ExternalAuthURL types.String `tfsdk:"external_auth_url"`
	Scopes          types.List   `tfsdk:"scopes"`
	TargetToken     types.String `tfsdk:"target_token"`
	JwksEndpoint    types.String `tfsdk:"jwks_endpoint"`
	CertPem         types.String `tfsdk:"cert_pem"`
	Kid             types.String `tfsdk:"kid"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Tags            types.Map    `tfsdk:"tags"`
	LastUpdated     types.String `tfsdk:"last_updated"`
}

type jwtSignerPayload struct {
	Name            *string          `json:"name"`
	Issuer          *string          `json:"issuer"`
	Audience        *string          `json:"audience"`
	ClaimsProperty  *string          `json:"claimsProperty,omitempty"`
	UseExternalId   *bool            `json:"useExternalId,omitempty"`
	ClientID        *string          `json:"clientId,omitempty"`
	ExternalAuthURL *string          `json:"externalAuthUrl,omitempty"`
	Scopes          []string         `json:"scopes"`
	TargetToken     *string          `json:"targetToken,omitempty"`
	JwksEndpoint    *string          `json:"jwksEndpoint,omitempty"`
	CertPem         *string          `json:"certPem,omitempty"`
	Kid             *string          `json:"kid,omitempty"`
	Enabled         *bool            `json:"enabled"`
	Tags            *rest_model.Tags `json:"tags,omitempty"`
}

// Schema defines the schema for the resource.
func (r *jwtSignerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti External jwt signer Resource",
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
				MarkdownDescription: "Name of the external jwt signer",
			},
			"issuer": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Issuer",
			},
			"audience": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Audience",
			},
			"claims_property": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("sub"),
				MarkdownDescription: "Claims Property",
			},
			"use_external_id": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Use External ID Flag",
			},
			"client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Client ID",
			},
			"external_auth_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "External Auth URL",
			},
			"scopes": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Scopes List",
			},
			"target_token": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("ACCESS"),
				Validators: []validator.String{
					stringvalidator.OneOf("ACCESS", "ID"),
				},
			},
			"jwks_endpoint": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.AtLeastOneOf(
						path.MatchRoot("jwks_endpoint"),
						path.MatchRoot("cert_pem"),
					),
					stringvalidator.ConflictsWith(
						path.MatchRoot("cert_pem"),
					),
				},
				MarkdownDescription: "Jwks Endpoint",
			},
			"cert_pem": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.AtLeastOneOf(
						path.MatchRoot("jwks_endpoint"),
						path.MatchRoot("cert_pem"),
					),
					stringvalidator.ConflictsWith(
						path.MatchRoot("jwks_endpoint"),
					),
				},
				MarkdownDescription: "Certificate PEM",
			},
			"kid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Kid",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enabled",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "External jwt signer Tags",
			},
		},
	}
}

// Create a new resource.
func (r *jwtSignerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan jwtSignerResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	issuer := eplan.Issuer.ValueString()
	audience := eplan.Audience.ValueString()
	claimsProperty := eplan.ClaimsProperty.ValueString()
	clientID := eplan.ClientID.ValueString()
	externalAuthURL := eplan.ExternalAuthURL.ValueString()
	targetToken := eplan.TargetToken.ValueString()
	kid := eplan.Kid.ValueString()
	useExternalId := eplan.UseExternalId.ValueBool()
	enabled := eplan.Enabled.ValueBool()

	var jwksEndpoint *string
	if !eplan.JwksEndpoint.IsNull() && !eplan.JwksEndpoint.IsUnknown() {
		v := eplan.JwksEndpoint.ValueString()
		jwksEndpoint = &v
	}

	var certPem *string
	if !eplan.CertPem.IsNull() && !eplan.CertPem.IsUnknown() {
		v := eplan.CertPem.ValueString()
		certPem = &v
	}

	var scopes []string
	for _, value := range eplan.Scopes.Elements() {
		if scope, ok := value.(types.String); ok {
			scopes = append(scopes, scope.ValueString())
		}
	}

	payload := jwtSignerPayload{
		Name:            &name,
		Issuer:          &issuer,
		Audience:        &audience,
		ClaimsProperty:  &claimsProperty,
		UseExternalId:   &useExternalId,
		ClientID:        &clientID,
		ExternalAuthURL: &externalAuthURL,
		Scopes:          scopes,
		TargetToken:     &targetToken,
		JwksEndpoint:    jwksEndpoint,
		CertPem:         certPem,
		Kid:             &kid,
		Enabled:         &enabled,
		Tags:            tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/external-jwt-signers", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating external jwt signer", "Could not Create external jwt signer, unexpected error: "+err.Error(),
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
func (r *jwtSignerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state jwtSignerResourceModel
	tflog.Debug(ctx, "Reading external jwt signer")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/external-jwt-signers/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
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
			"Error Reading external jwt signer", "Could not READ external jwt signer, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading external jwt signer", fmt.Sprintf("Could not READ external jwt signer, ERROR %v: ", err.Error()),
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

	if issuer, ok := data["issuer"].(string); ok {
		state.Issuer = types.StringValue(issuer)
	}

	if audience, ok := data["audience"].(string); ok {
		state.Audience = types.StringValue(audience)
	}

	if claimsProperty, ok := data["claimsProperty"].(string); ok {
		state.ClaimsProperty = types.StringValue(claimsProperty)
	}

	if clientID, ok := data["clientID"].(string); ok {
		state.ClientID = types.StringValue(clientID)
	}

	if externalAuthURL, ok := data["externalAuthUrl"].(string); ok {
		state.ExternalAuthURL = types.StringValue(externalAuthURL)
	}

	if targetToken, ok := data["targetToken"].(string); ok {
		state.TargetToken = types.StringValue(targetToken)
	}

	if jwksEndpoint, ok := data["jwksEndpoint"].(string); ok {
		state.JwksEndpoint = types.StringValue(jwksEndpoint)
	}

	if certPem, ok := data["certPem"].(string); ok {
		state.CertPem = types.StringValue(certPem)
	}

	if kid, ok := data["kid"].(string); ok {
		state.Kid = types.StringValue(kid)
	}

	if useExternalId, ok := data["useExternalId"].(bool); ok {
		state.UseExternalId = types.BoolValue(useExternalId)
	}

	if enabled, ok := data["enabled"].(bool); ok {
		state.Enabled = types.BoolValue(enabled)
	}

	if scopes, ok := data["scopes"].([]interface{}); ok {
		scopes, diag := types.ListValueFrom(ctx, types.StringType, scopes)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.Scopes = scopes
	} else {
		state.Scopes = types.ListNull(types.StringType)
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
func (r *jwtSignerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan jwtSignerResourceModel
	tflog.Debug(ctx, "Updating external jwt signer")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state jwtSignerResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	issuer := eplan.Issuer.ValueString()
	audience := eplan.Audience.ValueString()
	claimsProperty := eplan.ClaimsProperty.ValueString()
	clientID := eplan.ClientID.ValueString()
	externalAuthURL := eplan.ExternalAuthURL.ValueString()
	targetToken := eplan.TargetToken.ValueString()
	kid := eplan.Kid.ValueString()
	useExternalId := eplan.UseExternalId.ValueBool()
	enabled := eplan.Enabled.ValueBool()

	var jwksEndpoint *string
	if !eplan.JwksEndpoint.IsNull() && !eplan.JwksEndpoint.IsUnknown() {
		v := eplan.JwksEndpoint.ValueString()
		jwksEndpoint = &v
	}

	var certPem *string
	if !eplan.CertPem.IsNull() && !eplan.CertPem.IsUnknown() {
		v := eplan.CertPem.ValueString()
		certPem = &v
	}

	var scopes []string
	for _, value := range eplan.Scopes.Elements() {
		if scope, ok := value.(types.String); ok {
			scopes = append(scopes, scope.ValueString())
		}
	}

	payload := jwtSignerPayload{
		Name:            &name,
		Issuer:          &issuer,
		Audience:        &audience,
		ClaimsProperty:  &claimsProperty,
		UseExternalId:   &useExternalId,
		ClientID:        &clientID,
		ExternalAuthURL: &externalAuthURL,
		Scopes:          scopes,
		TargetToken:     &targetToken,
		JwksEndpoint:    jwksEndpoint,
		CertPem:         certPem,
		Kid:             &kid,
		Enabled:         &enabled,
		Tags:            tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/external-jwt-signers/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PATCH Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating external jwt signer", "Could not Update external jwt signer, unexpected error: "+err.Error(),
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
func (r *jwtSignerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state jwtSignerResourceModel
	tflog.Debug(ctx, "Deleting external jwt signer")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/external-jwt-signers/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting external jwt signer", "Could not DELETE external jwt signer, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *jwtSignerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
