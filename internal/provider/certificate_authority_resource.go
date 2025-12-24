package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &certificateAuthorityResource{}
	_ resource.ResourceWithConfigure   = &certificateAuthorityResource{}
	_ resource.ResourceWithImportState = &certificateAuthorityResource{}
)

// NewCertificateAuthorityResource is a helper function to simplify the provider implementation.
func NewCertificateAuthorityResource() resource.Resource {
	return &certificateAuthorityResource{}
}

// certificateAuthorityResource is the resource implementation.
type certificateAuthorityResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *certificateAuthorityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *certificateAuthorityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate_authority"
}

// certificateAuthorityResourceModel maps the resource schema data.
type certificateAuthorityResourceModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	IdentityRoles             types.List   `tfsdk:"identityroles"`
	IsAutoCaEnrollmentEnabled types.Bool   `tfsdk:"is_autoca_enrollment_enabled"`
	IsOttCaEnrollmentEnabled  types.Bool   `tfsdk:"is_ottca_enrollment_enabled"`
	IsAuthEnabled             types.Bool   `tfsdk:"is_auth_enabled"`
	IdentityNameFormat        types.String `tfsdk:"identity_name_format"`
	CertPem                   types.String `tfsdk:"cert_pem"`
	ExternalIdClaim           types.Object `tfsdk:"external_id_claim"`
	Tags                      types.Map    `tfsdk:"tags"`
	LastUpdated               types.String `tfsdk:"last_updated"`
}

var ExternalIdClaimModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"location":        types.StringType,
		"matcher":         types.StringType,
		"parser":          types.StringType,
		"matchercriteria": types.StringType,
		"parsercriteria":  types.StringType,
		"index":           types.Int64Type,
	},
}

// Schema defines the schema for the resource.
func (r *certificateAuthorityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Certificate Authority",
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
				MarkdownDescription: "Name of the Certificate Authority",
			},
			"identityroles": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Identity Roles",
			},
			"is_autoca_enrollment_enabled": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Auto CA Enrollment Flag",
			},
			"is_ottca_enrollment_enabled": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "OTT CA Enrollment Flag",
			},
			"is_auth_enabled": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Auth Flag",
			},
			"identity_name_format": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("[caName]-[commonName]"),
				MarkdownDescription: "Identity Name Format",
			},
			"cert_pem": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Certificate PEM",
			},
			"external_id_claim": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"location": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("COMMON_NAME"),
						Validators: []validator.String{
							stringvalidator.OneOf("COMMON_NAME", "SAN_URI", "SAN_EMAIL"),
						},
						MarkdownDescription: "Location",
					},
					"matcher": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("ALL"),
						Validators: []validator.String{
							stringvalidator.OneOf("ALL", "PREFIX", "SUFFIX", "SCHEME"),
						},
						MarkdownDescription: "Matcher",
					},
					"parser": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString("NONE"),
						Validators: []validator.String{
							stringvalidator.OneOf("NONE", "SPLIT"),
						},
						MarkdownDescription: "Parser",
					},
					"matchercriteria": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Matcher Criteria",
					},
					"parsercriteria": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Parser Criteria",
					},
					"index": schema.Int64Attribute{
						Computed:            true,
						Optional:            true,
						Default:             int64default.StaticInt64(0),
						MarkdownDescription: "Index",
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Certificate Authority Tags",
			},
		},
	}
}

// Create a new resource.
func (r *certificateAuthorityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan certificateAuthorityResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	isAuthEnabled := eplan.IsAuthEnabled.ValueBool()
	isAutoCaEnrollmentEnabled := eplan.IsAutoCaEnrollmentEnabled.ValueBool()
	isOttCaEnrollmentEnabled := eplan.IsOttCaEnrollmentEnabled.ValueBool()
	identityNameFormat := eplan.IdentityNameFormat.ValueString()
	certPem := eplan.CertPem.ValueString()

	var identityRoles rest_model.Roles
	for _, value := range eplan.IdentityRoles.Elements() {
		if identityRole, ok := value.(types.String); ok {
			identityRoles = append(identityRoles, identityRole.ValueString())
		}
	}

	var externalIDClaimPtr *rest_model.ExternalIDClaim
	if !eplan.ExternalIdClaim.IsNull() && !eplan.ExternalIdClaim.IsUnknown() {
		var externalIDClaim rest_model.ExternalIDClaim
		GenericFromObject(convertKeysToCamel(AttributesToNativeTypes(ctx, eplan.ExternalIdClaim.Attributes())), &externalIDClaim)
		externalIDClaimPtr = &externalIDClaim
	}

	payload := rest_model.CaCreate{
		Name:                      &name,
		IdentityNameFormat:        identityNameFormat,
		IdentityRoles:             identityRoles,
		IsAuthEnabled:             &isAuthEnabled,
		IsAutoCaEnrollmentEnabled: &isAutoCaEnrollmentEnabled,
		IsOttCaEnrollmentEnabled:  &isOttCaEnrollmentEnabled,
		CertPem:                   &certPem,
		ExternalIDClaim:           externalIDClaimPtr,
		Tags:                      tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %+v\n", jsonData)

	authUrl := fmt.Sprintf("%s/cas", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating certificate authority", "Could not Create CA, unexpected error: "+err.Error(),
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
func (r *certificateAuthorityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state certificateAuthorityResourceModel
	tflog.Debug(ctx, "Reading Certificate Authority")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/cas/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
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
			"Error Reading Certificate Authority", "Could not READ CA, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Certificate Authority", fmt.Sprintf("Could not READ CA, ERROR %v: ", err.Error()),
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

	if identityNameFormat, ok := data["identityNameFormat"].(string); ok {
		state.IdentityNameFormat = types.StringValue(identityNameFormat)
	}

	if certPem, ok := data["certPem"].(string); ok {
		state.CertPem = types.StringValue(certPem)
	}

	if isAuthEnabled, ok := data["isAuthEnabled"].(bool); ok {
		state.IsAuthEnabled = types.BoolValue(isAuthEnabled)
	}

	if isAutoCaEnrollmentEnabled, ok := data["isAutoCaEnrollmentEnabled"].(bool); ok {
		state.IsAutoCaEnrollmentEnabled = types.BoolValue(isAutoCaEnrollmentEnabled)
	}

	if isOttCaEnrollmentEnabled, ok := data["isOttCaEnrollmentEnabled"].(bool); ok {
		state.IsOttCaEnrollmentEnabled = types.BoolValue(isOttCaEnrollmentEnabled)
	}

	if extIdClaim, ok := data["externalIdClaim"].(map[string]interface{}); ok {
		attrTypes := ExternalIdClaimModel.AttrTypes
		values := make(map[string]attr.Value)

		if v, ok := extIdClaim["location"].(string); ok {
			values["location"] = types.StringValue(v)
		}

		if v, ok := extIdClaim["matcher"].(string); ok {
			values["matcher"] = types.StringValue(v)
		}

		if v, ok := extIdClaim["parser"].(string); ok && v != "" {
			values["parser"] = types.StringValue(v)
		}

		if v, ok := extIdClaim["matcherCriteria"].(string); ok && v != "" {
			values["matchercriteria"] = types.StringValue(v)
		}

		if v, ok := extIdClaim["parserCriteria"].(string); ok && v != "" {
			values["parsercriteria"] = types.StringValue(v)
		}

		if v, ok := extIdClaim["index"].(float64); ok {
			values["index"] = types.Int64Value(int64(v))
		}

		obj, _ := types.ObjectValue(attrTypes, values)
		state.ExternalIdClaim = obj

	} else {
		state.ExternalIdClaim = types.ObjectNull(ExternalIdClaimModel.AttrTypes)
	}

	if identityRoles, ok := data["identityRoles"].([]interface{}); ok {
		identityRoles, diag := types.ListValueFrom(ctx, types.StringType, identityRoles)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.IdentityRoles = identityRoles
	} else {
		state.IdentityRoles = types.ListNull(types.StringType)
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
func (r *certificateAuthorityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan certificateAuthorityResourceModel
	tflog.Debug(ctx, "Updating Certificate Authority")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state certificateAuthorityResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	isAuthEnabled := eplan.IsAuthEnabled.ValueBool()
	isAutoCaEnrollmentEnabled := eplan.IsAutoCaEnrollmentEnabled.ValueBool()
	isOttCaEnrollmentEnabled := eplan.IsOttCaEnrollmentEnabled.ValueBool()
	identityNameFormat := eplan.IdentityNameFormat.ValueString()

	var identityRoles rest_model.Roles
	for _, value := range eplan.IdentityRoles.Elements() {
		if identityRole, ok := value.(types.String); ok {
			identityRoles = append(identityRoles, identityRole.ValueString())
		}
	}

	var externalIDClaimPtr *rest_model.ExternalIDClaim
	if !eplan.ExternalIdClaim.IsNull() && !eplan.ExternalIdClaim.IsUnknown() {
		var externalIDClaim rest_model.ExternalIDClaim
		GenericFromObject(convertKeysToCamel(AttributesToNativeTypes(ctx, eplan.ExternalIdClaim.Attributes())), &externalIDClaim)
		externalIDClaimPtr = &externalIDClaim
	}

	payload := rest_model.CaUpdate{
		Name:                      &name,
		IdentityNameFormat:        &identityNameFormat,
		IdentityRoles:             identityRoles,
		IsAuthEnabled:             &isAuthEnabled,
		IsAutoCaEnrollmentEnabled: &isAutoCaEnrollmentEnabled,
		IsOttCaEnrollmentEnabled:  &isOttCaEnrollmentEnabled,
		ExternalIDClaim:           externalIDClaimPtr,
		Tags:                      tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %+v\n", jsonData)

	authUrl := fmt.Sprintf("%s/cas/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PATCH Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Certificate Authority", "Could not Update CA, unexpected error: "+err.Error(),
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
func (r *certificateAuthorityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state certificateAuthorityResourceModel
	tflog.Debug(ctx, "Deleting Certificate Authority")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/cas/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Certificate Authority", "Could not DELETE Certificate Authority, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *certificateAuthorityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
