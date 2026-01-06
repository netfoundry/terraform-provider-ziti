package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &authPolicyResource{}
	_ resource.ResourceWithConfigure   = &authPolicyResource{}
	_ resource.ResourceWithImportState = &authPolicyResource{}
)

// NewAuthPolicyResource is a helper function to simplify the provider implementation.
func NewAuthPolicyResource() resource.Resource {
	return &authPolicyResource{}
}

// authPolicyResource is the resource implementation.
type authPolicyResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *authPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *authPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth_policy"
}

// authPolicyResourceModel maps the resource schema data.
type authPolicyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Primary     types.Object `tfsdk:"primary"`
	Secondary   types.Object `tfsdk:"secondary"`
	Tags        types.Map    `tfsdk:"tags"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *authPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Auth Policy Resource",
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
				MarkdownDescription: "Name of the auth policy",
			},
			"primary": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"cert": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Allow cert based authentication Flag",
							},
							"allow_expired_certs": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Allow expired cert to authentication  Flag",
							},
						},
					},
					"ext_jwt": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Allow external jwt bearer tokens Flag",
							},
							"allowed_signers": schema.ListAttribute{
								Optional:    true,
								Computed:    true,
								ElementType: types.StringType,
								Default: listdefault.StaticValue(
									types.ListValueMust(types.StringType, []attr.Value{}),
								),
								MarkdownDescription: "External JWT Signers List to be used for authentication",
							},
						},
					},
					"updb": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"allowed": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"lockout_duration_minutes": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								Default:             int64default.StaticInt64(0),
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"max_attempts": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								Default:             int64default.StaticInt64(5),
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"min_password_length": schema.Int64Attribute{
								Optional:            true,
								Computed:            true,
								Default:             int64default.StaticInt64(5),
								MarkdownDescription: "Allow authentication via username and password Flag",
							},
							"require_mixed_case": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Upper and lower case Flag",
							},
							"require_number_char": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Numerical charecters Flag",
							},
							"require_special_char": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Flag for special (non-alpha numeric) charachters. ie. !%$@*",
							},
						},
					},
				},
			},
			"secondary": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"require_totp": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "TOTP Flag",
					},
					"jwt_signer": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "JWT Signer ID",
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Auth Policy Tags",
			},
		},
	}
}

var PrimaryModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"cert":    authPolicyCertModel,
		"ext_jwt": authPolicyExtJWTModel,
		"updb":    authPolicyUPDBModel,
	},
}

var SecondaryModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"require_totp": types.BoolType,
		"jwt_signer":   types.StringType,
	},
}

var authPolicyCertModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"allowed":             types.BoolType,
		"allow_expired_certs": types.BoolType,
	},
}
var authPolicyExtJWTModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"allowed":         types.BoolType,
		"allowed_signers": types.ListType{ElemType: types.StringType},
	},
}
var authPolicyUPDBModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"allowed":                  types.BoolType,
		"require_mixed_case":       types.BoolType,
		"require_number_char":      types.BoolType,
		"require_special_char":     types.BoolType,
		"lockout_duration_minutes": types.Int64Type,
		"max_attempts":             types.Int64Type,
		"min_password_length":      types.Int64Type,
	},
}

// Create a new resource.
func (r *authPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan authPolicyResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	jwtSigner := ""
	requireTotp := false
	if !eplan.Secondary.IsNull() && !eplan.Secondary.IsUnknown() {
		secondaryAttrs := eplan.Secondary.Attributes()
		if v, ok := secondaryAttrs["require_totp"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
			requireTotp = v.ValueBool()
		}
		if v, ok := secondaryAttrs["jwt_signer"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			jwtSigner = v.ValueString()
		}
	}
	authPolicySecondary := rest_model.AuthPolicySecondary{
		RequireTotp: &requireTotp,
	}
	if jwtSigner != "" {
		authPolicySecondary.RequireExtJWTSigner = &jwtSigner
	}

	authPolicyPrimary := rest_model.AuthPolicyPrimary{}
	if !eplan.Primary.IsNull() && !eplan.Primary.IsUnknown() {
		primaryAttrs := eplan.Primary.Attributes()

		if certAttr, ok := primaryAttrs["cert"].(types.Object); ok {
			certMap := certAttr.Attributes()

			var allowExpired, allowed bool
			if allowAttr, ok := certMap["allow_expired_certs"].(types.Bool); ok && !allowAttr.IsNull() && !allowAttr.IsUnknown() {
				allowExpired = allowAttr.ValueBool()
			}
			if allowedAttr, ok := certMap["allowed"].(types.Bool); ok && !allowedAttr.IsNull() && !allowedAttr.IsUnknown() {
				allowed = allowedAttr.ValueBool()
			}
			authPolicyPrimary.Cert = &rest_model.AuthPolicyPrimaryCert{
				AllowExpiredCerts: &allowExpired,
				Allowed:           &allowed,
			}
		}

		if extJwtAttr, ok := primaryAttrs["ext_jwt"].(types.Object); ok && !extJwtAttr.IsNull() && !extJwtAttr.IsUnknown() {
			extJwtMap := extJwtAttr.Attributes()

			var allowed bool
			allowedSigners := []string{}
			if v, ok := extJwtMap["allowed"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				allowed = v.ValueBool()
			}
			if signersAttr, ok := extJwtMap["allowed_signers"].(types.List); ok && !signersAttr.IsNull() && !signersAttr.IsUnknown() {
				for _, s := range signersAttr.Elements() {
					if signerStr, ok := s.(types.String); ok && !signerStr.IsNull() && !signerStr.IsUnknown() {
						allowedSigners = append(allowedSigners, signerStr.ValueString())
					}
				}
			}
			authPolicyPrimary.ExtJWT = &rest_model.AuthPolicyPrimaryExtJWT{
				Allowed:        &allowed,
				AllowedSigners: allowedSigners,
			}
		}
		if updbAttr, ok := primaryAttrs["updb"].(types.Object); ok && !updbAttr.IsNull() && !updbAttr.IsUnknown() {
			updbMap := updbAttr.Attributes()

			var allowed, requireMixedCase, requireNumberChar, requireSpecialChar bool
			var lockoutDurationMinutes, maxAttempts, minPasswordLength int64

			if v, ok := updbMap["allowed"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				allowed = v.ValueBool()
			}
			if v, ok := updbMap["require_mixed_case"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireMixedCase = v.ValueBool()
			}
			if v, ok := updbMap["require_number_char"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireNumberChar = v.ValueBool()
			}
			if v, ok := updbMap["require_special_char"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireSpecialChar = v.ValueBool()
			}
			if v, ok := updbMap["lockout_duration_minutes"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				lockoutDurationMinutes = v.ValueInt64()
			}
			if v, ok := updbMap["max_attempts"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				maxAttempts = v.ValueInt64()
			}
			if v, ok := updbMap["min_password_length"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				minPasswordLength = v.ValueInt64()
			}

			authPolicyPrimary.Updb = &rest_model.AuthPolicyPrimaryUpdb{
				Allowed:                &allowed,
				LockoutDurationMinutes: &lockoutDurationMinutes,
				MaxAttempts:            &maxAttempts,
				MinPasswordLength:      &minPasswordLength,
				RequireMixedCase:       &requireMixedCase,
				RequireNumberChar:      &requireNumberChar,
				RequireSpecialChar:     &requireSpecialChar,
			}
		}
	}

	payload := rest_model.AuthPolicyCreate{
		Name:      &name,
		Primary:   &authPolicyPrimary,
		Secondary: &authPolicySecondary,
		Tags:      tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/auth-policies", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating auth policy", "Could not Create auth policy, unexpected error: "+err.Error(),
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
func (r *authPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state authPolicyResourceModel
	tflog.Debug(ctx, "Reading auth policy")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/auth-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
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
			"Error Reading auth policy", "Could not READ auth policy, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading auth policy", fmt.Sprintf("Could not READ auth policy, ERROR %v: ", err.Error()),
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

// Update updates the resource and sets the updated Terraform state on success.
func (r *authPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan authPolicyResourceModel
	tflog.Debug(ctx, "Updating auth policy")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state authPolicyResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	jwtSigner := ""
	requireTotp := false
	if !eplan.Secondary.IsNull() && !eplan.Secondary.IsUnknown() {
		secondaryAttrs := eplan.Secondary.Attributes()
		if v, ok := secondaryAttrs["require_totp"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
			requireTotp = v.ValueBool()
		}
		if v, ok := secondaryAttrs["jwt_signer"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			jwtSigner = v.ValueString()
		}
	}
	authPolicySecondary := rest_model.AuthPolicySecondary{
		RequireTotp: &requireTotp,
	}
	if jwtSigner != "" {
		authPolicySecondary.RequireExtJWTSigner = &jwtSigner
	}

	authPolicyPrimary := rest_model.AuthPolicyPrimary{}
	if !eplan.Primary.IsNull() && !eplan.Primary.IsUnknown() {
		primaryAttrs := eplan.Primary.Attributes()

		if certAttr, ok := primaryAttrs["cert"].(types.Object); ok {
			certMap := certAttr.Attributes()

			var allowExpired, allowed bool
			if allowAttr, ok := certMap["allow_expired_certs"].(types.Bool); ok && !allowAttr.IsNull() && !allowAttr.IsUnknown() {
				allowExpired = allowAttr.ValueBool()
			}
			if allowedAttr, ok := certMap["allowed"].(types.Bool); ok && !allowedAttr.IsNull() && !allowedAttr.IsUnknown() {
				allowed = allowedAttr.ValueBool()
			}
			authPolicyPrimary.Cert = &rest_model.AuthPolicyPrimaryCert{
				AllowExpiredCerts: &allowExpired,
				Allowed:           &allowed,
			}
		}

		if extJwtAttr, ok := primaryAttrs["ext_jwt"].(types.Object); ok && !extJwtAttr.IsNull() && !extJwtAttr.IsUnknown() {
			extJwtMap := extJwtAttr.Attributes()

			var allowed bool
			allowedSigners := []string{}
			if v, ok := extJwtMap["allowed"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				allowed = v.ValueBool()
			}
			if signersAttr, ok := extJwtMap["allowed_signers"].(types.List); ok && !signersAttr.IsNull() && !signersAttr.IsUnknown() {
				for _, s := range signersAttr.Elements() {
					if signerStr, ok := s.(types.String); ok && !signerStr.IsNull() && !signerStr.IsUnknown() {
						allowedSigners = append(allowedSigners, signerStr.ValueString())
					}
				}
			}
			authPolicyPrimary.ExtJWT = &rest_model.AuthPolicyPrimaryExtJWT{
				Allowed:        &allowed,
				AllowedSigners: allowedSigners,
			}
		}
		if updbAttr, ok := primaryAttrs["updb"].(types.Object); ok && !updbAttr.IsNull() && !updbAttr.IsUnknown() {
			updbMap := updbAttr.Attributes()

			var allowed, requireMixedCase, requireNumberChar, requireSpecialChar bool
			var lockoutDurationMinutes, maxAttempts, minPasswordLength int64

			if v, ok := updbMap["allowed"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				allowed = v.ValueBool()
			}
			if v, ok := updbMap["require_mixed_case"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireMixedCase = v.ValueBool()
			}
			if v, ok := updbMap["require_number_char"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireNumberChar = v.ValueBool()
			}
			if v, ok := updbMap["require_special_char"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
				requireSpecialChar = v.ValueBool()
			}
			if v, ok := updbMap["lockout_duration_minutes"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				lockoutDurationMinutes = v.ValueInt64()
			}
			if v, ok := updbMap["max_attempts"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				maxAttempts = v.ValueInt64()
			}
			if v, ok := updbMap["min_password_length"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
				minPasswordLength = v.ValueInt64()
			}

			authPolicyPrimary.Updb = &rest_model.AuthPolicyPrimaryUpdb{
				Allowed:                &allowed,
				LockoutDurationMinutes: &lockoutDurationMinutes,
				MaxAttempts:            &maxAttempts,
				MinPasswordLength:      &minPasswordLength,
				RequireMixedCase:       &requireMixedCase,
				RequireNumberChar:      &requireNumberChar,
				RequireSpecialChar:     &requireSpecialChar,
			}
		}
	}

	payload := rest_model.AuthPolicyCreate{
		Name:      &name,
		Primary:   &authPolicyPrimary,
		Secondary: &authPolicySecondary,
		Tags:      tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %s\n", jsonData)

	authUrl := fmt.Sprintf("%s/auth-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PATCH Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating auth policy", "Could not Update auth policy, unexpected error: "+err.Error(),
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
func (r *authPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state authPolicyResourceModel
	tflog.Debug(ctx, "Deleting auth policy")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/auth-policies/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting auth policy", "Could not DELETE auth policy, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *authPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
