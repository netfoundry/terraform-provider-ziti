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
	_ datasource.DataSource              = &jwtSignerDataSource{}
	_ datasource.DataSourceWithConfigure = &jwtSignerDataSource{}
)

// NewJwtSignerDataSource is a helper function to simplify the provider implementation.
func NewJwtSignerDataSource() datasource.DataSource {
	return &jwtSignerDataSource{}
}

// jwtSignerDataSource is the datasource implementation.
type jwtSignerDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *jwtSignerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *jwtSignerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_jwt_signer"
}

// jwtSignerDataSourceModel maps the datasource schema data.
type jwtSignerDataSourceModel struct {
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
}

// Schema defines the schema for the datasource.
func (r *jwtSignerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti External jwt signer Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the external jwt",
			},
			"issuer": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Issuer",
			},
			"audience": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Audience",
			},
			"claims_property": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Claims Property",
			},
			"use_external_id": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
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
				MarkdownDescription: "Scopes List",
			},
			"target_token": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"jwks_endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Jwks Endpoint",
			},
			"cert_pem": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Certificate PEM",
			},
			"kid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Kid",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enabled",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "External jwt signer Tags",
			},
		},
	}
}

// Read datasource information.
func (r *jwtSignerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state jwtSignerDataSourceModel
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

	authUrl := fmt.Sprintf("%s/external-jwt-signers?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading external jwt signer", "Could not READ external jwt signer, unexpected error: "+err.Error(),
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
