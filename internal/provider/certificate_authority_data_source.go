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
	_ datasource.DataSource              = &certificateAuthorityDataSource{}
	_ datasource.DataSourceWithConfigure = &certificateAuthorityDataSource{}
)

// NewCertificateAuthorityDataSource is a helper function to simplify the provider implementation.
func NewCertificateAuthorityDataSource() datasource.DataSource {
	return &certificateAuthorityDataSource{}
}

// certificateAuthorityDataSource is the datasource implementation.
type certificateAuthorityDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *certificateAuthorityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *certificateAuthorityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate_authority"
}

// certificateAuthorityDataSourceModel maps the datasource schema data.
type certificateAuthorityDataSourceModel struct {
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
}

// Schema defines the schema for the datasource.
func (r *certificateAuthorityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Certificate Authority",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the Certificate Authority",
			},
			"identityroles": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Identity Roles",
			},
			"is_autoca_enrollment_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Auto CA Enrollment Flag",
			},
			"is_ottca_enrollment_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "OTT CA Enrollment Flag",
			},
			"is_auth_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Auth Flag",
			},
			"identity_name_format": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity Name Format",
			},
			"cert_pem": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Certificate PEM",
			},
			"external_id_claim": schema.SingleNestedAttribute{
				Computed: true,
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"location": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Location",
					},
					"matcher": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Matcher",
					},
					"parser": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Parser",
					},
					"matchercriteria": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Matcher Criteria",
					},
					"parsercriteria": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Parser Criteria",
					},
					"index": schema.Int64Attribute{
						Computed:            true,
						MarkdownDescription: "Index",
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Certificate Authority Tags",
			},
		},
	}
}

// Read datasource information.
func (r *certificateAuthorityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state certificateAuthorityDataSourceModel
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

	authUrl := fmt.Sprintf("%s/cas?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Certificate Authority", "Could not READ CA, unexpected error: "+err.Error(),
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
