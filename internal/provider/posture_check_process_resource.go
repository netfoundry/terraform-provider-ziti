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
	_ resource.Resource                = &postureCheckProcessResource{}
	_ resource.ResourceWithConfigure   = &postureCheckProcessResource{}
	_ resource.ResourceWithImportState = &postureCheckProcessResource{}
)

// NewPostureCheckProcessResource is a helper function to simplify the provider implementation.
func NewPostureCheckProcessResource() resource.Resource {
	return &postureCheckProcessResource{}
}

// postureCheckProcessResource is the resource implementation.
type postureCheckProcessResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *postureCheckProcessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *postureCheckProcessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check_process"
}

var ProcessModel = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"path":               types.StringType,
		"os_type":            types.StringType,
		"hashes":             types.ListType{ElemType: types.StringType},
		"signer_fingerprint": types.StringType,
	},
}

// postureCheckProcessResourceModel maps the resource schema data.
type postureCheckProcessResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	RoleAttributes types.List   `tfsdk:"role_attributes"`
	Process        types.Object `tfsdk:"process"`
	Tags           types.Map    `tfsdk:"tags"`
	LastUpdated    types.String `tfsdk:"last_updated"`
}

type postureCheckProcessAddressPayload struct {
	Name           *string                     `json:"name"`
	RoleAttributes rest_model.Attributes       `json:"roleAttributes,omitempty"`
	TypeID         rest_model.PostureCheckType `json:"typeId"`
	Process        *ProcessDTO                 `json:"process"`
	Tags           *rest_model.Tags            `json:"tags,omitempty"`
}

type ProcessDTO struct {
	Path              string    `json:"path"`
	OsType            string    `json:"osType"`
	Hashes            *[]string `json:"hashes,omitempty"`
	SignerFingerprint *string   `json:"signerFingerprint,omitempty"`
}

// Schema defines the schema for the resource.
func (r *postureCheckProcessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Posture check Resource, Type: Process check",
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
			"process": schema.SingleNestedAttribute{
				Required: true,
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
					"signer_fingerprint": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Signer fingerprint",
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
func (r *postureCheckProcessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan postureCheckProcessResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	var process ProcessDTO
	GenericFromObject(convertKeysToCamel(AttributesToNativeTypes(ctx, eplan.Process.Attributes())), &process)

	if process.Hashes == nil {
		emptyHashes := []string{}
		process.Hashes = &emptyHashes
	}
	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := postureCheckProcessAddressPayload{
		Process:        &process,
		Name:           &name,
		TypeID:         "PROCESS",
		RoleAttributes: roleAttributes,
		Tags:           tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************create resource payload***********************:\n %s\n", jsonData)

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
func (r *postureCheckProcessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state postureCheckProcessResourceModel
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

	if proc, ok := data["process"].(map[string]interface{}); ok {
		attrTypes := ProcessModel.AttrTypes
		values := make(map[string]attr.Value)

		if v, ok := proc["path"].(string); ok {
			values["path"] = types.StringValue(v)
		}

		if v, ok := proc["osType"].(string); ok {
			values["os_type"] = types.StringValue(v)
		}

		if v, ok := proc["signerFingerprint"].(string); ok && v != "" {
			values["signer_fingerprint"] = types.StringValue(v)
		} else {
			values["signer_fingerprint"] = types.StringNull()
		}

		if hashes, ok := proc["hashes"].([]interface{}); ok && len(hashes) > 0 {
			list, diag := types.ListValueFrom(ctx, types.StringType, hashes)
			resp.Diagnostics.Append(diag...)
			values["hashes"] = list
		} else {
			values["hashes"] = types.ListNull(types.StringType)
		}

		obj, _ := types.ObjectValue(attrTypes, values)
		state.Process = obj

	} else {
		state.Process = types.ObjectNull(ProcessModel.AttrTypes)
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
func (r *postureCheckProcessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan postureCheckProcessResourceModel
	tflog.Debug(ctx, "Updating Posture check")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state postureCheckProcessResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	tags := TagsFromAttributes(eplan.Tags.Elements())

	var process ProcessDTO
	GenericFromObject(convertKeysToCamel(AttributesToNativeTypes(ctx, eplan.Process.Attributes())), &process)
	if process.Hashes == nil {
		emptyHashes := []string{}
		process.Hashes = &emptyHashes
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := postureCheckProcessAddressPayload{
		Process:        &process,
		Name:           &name,
		TypeID:         "PROCESS",
		RoleAttributes: roleAttributes,
		Tags:           tags,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)
	fmt.Printf("**********************update resource payload***********************:\n %s\n", jsonData)

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
func (r *postureCheckProcessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state postureCheckProcessResourceModel
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

func (r *postureCheckProcessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
