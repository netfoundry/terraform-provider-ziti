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
	_ resource.Resource                = &postureCheckMultiProcessResource{}
	_ resource.ResourceWithConfigure   = &postureCheckMultiProcessResource{}
	_ resource.ResourceWithImportState = &postureCheckMultiProcessResource{}
)

// NewPostureCheckMultiProcessResource is a helper function to simplify the provider implementation.
func NewPostureCheckMultiProcessResource() resource.Resource {
	return &postureCheckMultiProcessResource{}
}

// postureCheckMultiProcessResource is the resource implementation.
type postureCheckMultiProcessResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *postureCheckMultiProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *postureCheckMultiProcessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check_multi_process"
}

var MultiProcessModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"path":                types.StringType,
		"os_type":             types.StringType,
		"hashes":              types.ListType{ElemType: types.StringType},
		"signer_fingerprints": types.ListType{ElemType: types.StringType},
	},
}

// postureCheckMultiProcessResourceModel maps the resource schema data.
type postureCheckMultiProcessResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	RoleAttributes types.List   `tfsdk:"role_attributes"`
	Semantic       types.String `tfsdk:"semantic"`
	Processes      types.Set    `tfsdk:"processes"`
	Tags           types.Map    `tfsdk:"tags"`
	LastUpdated    types.String `tfsdk:"last_updated"`
}

type postureCheckMultiProcessAddressPayload struct {
	Name           *string                     `json:"name"`
	RoleAttributes rest_model.Attributes       `json:"roleAttributes,omitempty"`
	TypeID         rest_model.PostureCheckType `json:"typeId"`
	Semantic       *rest_model.Semantic        `json:"semantic"`
	Processes      []MultiProcessDTO           `json:"processes"`
	Tags           *rest_model.Tags            `json:"tags,omitempty"`
}

type MultiProcessDTO struct {
	Path               string    `json:"path"`
	OsType             string    `json:"osType"`
	Hashes             *[]string `json:"hashes,omitempty"`
	SignerFingerprints *[]string `json:"signerFingerprints,omitempty"`
}

// Schema defines the schema for the resource.
func (r *postureCheckMultiProcessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Posture check Resource, Type: Multi Process check",
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
				MarkdownDescription: "Name of the Posture Check",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Role Attributes",
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
			"processes": schema.SetNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Path",
						},
						"os_type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("Windows", "WindowsServer", "Android", "iOS", "Linux", "macOS"),
							},
							MarkdownDescription: "Operating System type",
						},
						"hashes": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							MarkdownDescription: "File hashes list",
						},
						"signer_fingerprints": schema.ListAttribute{
							ElementType:         types.StringType,
							Optional:            true,
							MarkdownDescription: "Sign fingerprint",
						},
					},
				},
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Posture Check Tags",
			},
		},
	}
}

// Create a new resource.
func (r *postureCheckMultiProcessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan postureCheckMultiProcessResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	semantic := rest_model.Semantic(eplan.Semantic.ValueString())

	var processes []MultiProcessDTO
	if !eplan.Processes.IsNull() && !eplan.Processes.IsUnknown() {
		for _, v := range eplan.Processes.Elements() {
			if obj, ok := v.(types.Object); ok {
				attrs := obj.Attributes()
				dto := MultiProcessDTO{}

				if v, ok := attrs["path"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
					dto.Path = v.ValueString()
				}

				if v, ok := attrs["os_type"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
					dto.OsType = v.ValueString()
				}

				if v, ok := attrs["hashes"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
					var hashes []string
					for _, hv := range v.Elements() {
						if s, ok := hv.(types.String); ok && !s.IsNull() {
							hashes = append(hashes, s.ValueString())
						}
					}
					dto.Hashes = &hashes
				}

				if v, ok := attrs["signer_fingerprints"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
					var fps []string
					for _, fv := range v.Elements() {
						if s, ok := fv.(types.String); ok && !s.IsNull() {
							fps = append(fps, s.ValueString())
						}
					}
					dto.SignerFingerprints = &fps
				}
				processes = append(processes, dto)
			}
		}
	}

	for i := range processes {
		if processes[i].Hashes == nil {
			empty := []string{}
			processes[i].Hashes = &empty
		}
		if processes[i].SignerFingerprints == nil {
			signerEmpty := []string{}
			processes[i].SignerFingerprints = &signerEmpty
		}
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := postureCheckMultiProcessAddressPayload{
		Processes:      processes,
		Name:           &name,
		TypeID:         "PROCESS_MULTI",
		RoleAttributes: roleAttributes,
		Semantic:       &semantic,
		Tags:           tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %+v\n", jsonData)

	authUrl := fmt.Sprintf("%s/posture-checks", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating posture check", "Could not Create posture check, unexpected error: "+err.Error(),
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
func (r *postureCheckMultiProcessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state postureCheckMultiProcessResourceModel
	tflog.Debug(ctx, "Reading Posture check")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/posture-checks/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
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
			"Error Reading posture check", "Could not READ posture check, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading posture check", fmt.Sprintf("Could not READ posture check, ERROR %v: ", err.Error()),
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

	if procs, ok := data["processes"].([]interface{}); ok && len(procs) > 0 {
		var processValues []attr.Value

		for _, p := range procs {
			proc, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			attrTypes := MultiProcessModel.AttrTypes
			values := make(map[string]attr.Value)

			if v, ok := proc["path"].(string); ok {
				values["path"] = types.StringValue(v)
			}

			if v, ok := proc["osType"].(string); ok {
				values["os_type"] = types.StringValue(v)
			}

			if fps, ok := proc["signerFingerprints"].([]interface{}); ok && len(fps) > 0 {
				list, diag := types.ListValueFrom(ctx, types.StringType, fps)
				resp.Diagnostics.Append(diag...)
				values["signer_fingerprints"] = list
			} else {
				values["signer_fingerprints"] = types.ListNull(types.StringType)
			}

			if hashes, ok := proc["hashes"].([]interface{}); ok && len(hashes) > 0 {
				list, diag := types.ListValueFrom(ctx, types.StringType, hashes)
				resp.Diagnostics.Append(diag...)
				values["hashes"] = list
			} else {
				values["hashes"] = types.ListNull(types.StringType)
			}

			obj, diag := types.ObjectValue(attrTypes, values)
			resp.Diagnostics.Append(diag...)
			processValues = append(processValues, obj)
		}

		list, diag := types.SetValue(types.ObjectType{AttrTypes: MultiProcessModel.AttrTypes}, processValues)
		resp.Diagnostics.Append(diag...)

		state.Processes = list
	} else {
		state.Processes = types.SetNull(types.ObjectType{AttrTypes: MultiProcessModel.AttrTypes})
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

// Update updates the resource and sets the updated Terraform state on success.
func (r *postureCheckMultiProcessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan postureCheckMultiProcessResourceModel
	tflog.Debug(ctx, "Updating Posture check")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state postureCheckMultiProcessResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	semantic := rest_model.Semantic(eplan.Semantic.ValueString())

	var processes []MultiProcessDTO
	if !eplan.Processes.IsNull() && !eplan.Processes.IsUnknown() {
		for _, v := range eplan.Processes.Elements() {
			if obj, ok := v.(types.Object); ok {
				attrs := obj.Attributes()
				dto := MultiProcessDTO{}

				if v, ok := attrs["path"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
					dto.Path = v.ValueString()
				}

				if v, ok := attrs["os_type"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
					dto.OsType = v.ValueString()
				}

				if v, ok := attrs["hashes"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
					var hashes []string
					for _, hv := range v.Elements() {
						if s, ok := hv.(types.String); ok && !s.IsNull() {
							hashes = append(hashes, s.ValueString())
						}
					}
					dto.Hashes = &hashes
				}

				if v, ok := attrs["signer_fingerprints"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
					var fps []string
					for _, fv := range v.Elements() {
						if s, ok := fv.(types.String); ok && !s.IsNull() {
							fps = append(fps, s.ValueString())
						}
					}
					dto.SignerFingerprints = &fps
				}
				processes = append(processes, dto)
			}
		}
	}

	for i := range processes {
		if processes[i].Hashes == nil {
			empty := []string{}
			processes[i].Hashes = &empty
		}
		if processes[i].SignerFingerprints == nil {
			signerEmpty := []string{}
			processes[i].SignerFingerprints = &signerEmpty
		}
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := postureCheckMultiProcessAddressPayload{
		Processes:      processes,
		Name:           &name,
		TypeID:         "PROCESS_MULTI",
		RoleAttributes: roleAttributes,
		Semantic:       &semantic,
		Tags:           tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %+v\n", jsonData)

	authUrl := fmt.Sprintf("%s/posture-checks/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))
	cresp, err := PatchZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PATCH Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating posture check", "Could not Update posture check, unexpected error: "+err.Error(),
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
func (r *postureCheckMultiProcessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state postureCheckMultiProcessResourceModel
	tflog.Debug(ctx, "Deleting Posture check")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/posture-checks/%s", r.resourceConfig.host, url.QueryEscape(state.ID.ValueString()))

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting posture check", "Could not DELETE posture check, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *postureCheckMultiProcessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
