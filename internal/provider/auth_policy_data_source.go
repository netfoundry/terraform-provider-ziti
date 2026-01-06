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
	_ datasource.DataSource              = &authPolicyDataSource{}
	_ datasource.DataSourceWithConfigure = &authPolicyDataSource{}
)

// NewAuthPolicyDataSource is a helper function to simplify the provider implementation.
func NewAuthPolicyDataSource() datasource.DataSource {
	return &authPolicyDataSource{}
}

// authPolicyDataSource is the datasource implementation.
type authPolicyDataSource struct {
	datasourceConfig *zitiData
}

// Configure adds the provider configured client to the datasource.
func (r *authPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (r *authPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth_policy"
}

// authPolicyDataSourceModel maps the datasource schema data.
type authPolicyDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Primary   types.Object `tfsdk:"primary"`
	Secondary types.Object `tfsdk:"secondary"`
	Tags      types.Map    `tfsdk:"tags"`
}

// Schema defines the schema for the datasource.
func (r *authPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Auth Policy Data Source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Identifier",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Name of the auth policy",
			},
			"primary": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"cert": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow cert based authentication Flag",
							},
							"allow_expired_certs": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow expired cert to authentication  Flag",
							},
						},
					},
					"ext_jwt": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow external jwt bearer tokens Flag",
							},
							"allowed_signers": schema.ListAttribute{
								Optional:            true,
								Computed:            true,
								ElementType:         types.StringType,
								MarkdownDescription: "External JWT Signers List to be used for authentication",
							},
						},
					},
					"updb": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"lockout_duration_minutes": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"max_attempts": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"min_password_length": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"require_mixed_case": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Upper and lower case Flag",
							},
							"require_number_char": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Numerical charecters Flag",
							},
							"require_special_char": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Flag for special (non-alpha numeric) charachters. ie. !%$@*",
							},
						},
					},
				},
			},
			"secondary": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"require_totp": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "TOTP Flag",
					},
					"jwt_signer": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "JWT Signer ID",
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Auth Policy Tags",
			},
		},
	}
}

// Read datasource information.
func (r *authPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Get current state
	var state authPolicyDataSourceModel
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

	authUrl := fmt.Sprintf("%s/auth-policies?%s", r.datasourceConfig.host, filter)
	cresp, err := ReadZitiResource(authUrl, r.datasourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading auth policy", "Could not READ auth policy, unexpected error: "+err.Error(),
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

	if primaryData, ok := data["primary"].(map[string]interface{}); ok {
		attrTypes := PrimaryModel.AttrTypes
		values := make(map[string]attr.Value)

		if certData, ok := primaryData["cert"].(map[string]interface{}); ok {
			certValues := make(map[string]attr.Value)

			if v, ok := certData["allowExpiredCerts"].(bool); ok {
				certValues["allow_expired_certs"] = types.BoolValue(v)
			}
			if v, ok := certData["allowed"].(bool); ok {
				certValues["allowed"] = types.BoolValue(v)
			}

			certObj, _ := types.ObjectValue(authPolicyCertModel.AttrTypes, certValues)
			values["cert"] = certObj
		} else {
			values["cert"] = types.ObjectNull(authPolicyCertModel.AttrTypes)
		}

		if extJwtData, ok := primaryData["extJwt"].(map[string]interface{}); ok {
			extJwtValues := make(map[string]attr.Value)

			if v, ok := extJwtData["allowed"].(bool); ok {
				extJwtValues["allowed"] = types.BoolValue(v)
			}

			if signers, ok := extJwtData["allowedSigners"].([]interface{}); ok {
				signerVals := make([]attr.Value, 0, len(signers))
				for _, s := range signers {
					if str, ok := s.(string); ok {
						signerVals = append(signerVals, types.StringValue(str))
					}
				}
				extJwtValues["allowed_signers"] = types.ListValueMust(types.StringType, signerVals)
			} else {
				extJwtValues["allowed_signers"] = types.ListValueMust(types.StringType, []attr.Value{})
			}

			extJwtObj, _ := types.ObjectValue(authPolicyExtJWTModel.AttrTypes, extJwtValues)
			values["ext_jwt"] = extJwtObj
		} else {
			values["ext_jwt"] = types.ObjectNull(authPolicyExtJWTModel.AttrTypes)
		}

		if updbData, ok := primaryData["updb"].(map[string]interface{}); ok {
			updbValues := make(map[string]attr.Value)

			if v, ok := updbData["allowed"].(bool); ok {
				updbValues["allowed"] = types.BoolValue(v)
			}
			if v, ok := updbData["requireMixedCase"].(bool); ok {
				updbValues["require_mixed_case"] = types.BoolValue(v)
			}
			if v, ok := updbData["requireNumberChar"].(bool); ok {
				updbValues["require_number_char"] = types.BoolValue(v)
			}
			if v, ok := updbData["requireSpecialChar"].(bool); ok {
				updbValues["require_special_char"] = types.BoolValue(v)
			}

			if v, ok := updbData["lockoutDurationMinutes"].(float64); ok {
				updbValues["lockout_duration_minutes"] =
					types.Int64Value(int64(v))
			}
			if v, ok := updbData["maxAttempts"].(float64); ok {
				updbValues["max_attempts"] =
					types.Int64Value(int64(v))
			}
			if v, ok := updbData["minPasswordLength"].(float64); ok {
				updbValues["min_password_length"] =
					types.Int64Value(int64(v))
			}

			updbObj, _ := types.ObjectValue(authPolicyUPDBModel.AttrTypes, updbValues)
			values["updb"] = updbObj
		} else {
			values["updb"] = types.ObjectNull(authPolicyUPDBModel.AttrTypes)
		}

		primaryObj, _ := types.ObjectValue(attrTypes, values)
		state.Primary = primaryObj

	} else {
		state.Primary = types.ObjectNull(PrimaryModel.AttrTypes)
	}

	if secondaryData, ok := data["secondary"].(map[string]interface{}); ok {
		jwtSigner := types.StringNull()
		if v, ok := secondaryData["requireExtJwtSigner"].(string); ok && v != "" {
			jwtSigner = types.StringValue(v)
		}

		requireTotp := types.BoolValue(false)
		if v, ok := secondaryData["requireTotp"].(bool); ok {
			requireTotp = types.BoolValue(v)
		}

		values := map[string]attr.Value{
			"jwt_signer":   jwtSigner,
			"require_totp": requireTotp,
		}
		obj, _ := types.ObjectValue(SecondaryModel.AttrTypes, values)
		state.Secondary = obj

	} else {
		state.Secondary = types.ObjectNull(SecondaryModel.AttrTypes)
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
