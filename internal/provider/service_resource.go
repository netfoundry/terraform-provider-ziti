package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/openziti/edge-api/rest_model"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &serviceResource{}
	_ resource.ResourceWithConfigure   = &serviceResource{}
	_ resource.ResourceWithImportState = &serviceResource{}
)

// NewServiceResource is a helper function to simplify the provider implementation.
func NewServiceResource() resource.Resource {
	return &serviceResource{}
}

// serviceResource is the resource implementation.
type serviceResource struct {
	resourceConfig *zitiData
}

// Configure adds the provider configured client to the resource.
func (r *serviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *serviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

// serviceResourceModel maps the resource schema data.
type serviceResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Configs                 types.List   `tfsdk:"configs"`
	EncryptionRequired      types.Bool   `tfsdk:"encryption_required"`
	MaxIdleTimeMilliseconds types.Int64  `tfsdk:"max_idle_milliseconds"`
	RoleAttributes          types.List   `tfsdk:"role_attributes"`
	TerminatorStrategy      types.String `tfsdk:"terminator_strategy"`
	Tags                    types.Map    `tfsdk:"tags"`
	LastUpdated             types.String `tfsdk:"last_updated"`
}

// Schema defines the schema for the resource.
func (r *serviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Service Resource",
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
				MarkdownDescription: "Name of the Service",
			},
			"role_attributes": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Role Attributes",
			},
			"terminator_strategy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("smartrouting"),
				Validators: []validator.String{
					stringvalidator.OneOf("smartrouting", "weighted", "random", "ha"),
				},
				MarkdownDescription: "Type of Terminator Strategy",
			},
			"max_idle_milliseconds": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
				MarkdownDescription: "Idle Timeout in milli seconds",
			},
			"encryption_required": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Flag which controls Encryption Required",
			},
			"configs": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListNull(types.StringType)),
				MarkdownDescription: "Service Configs",
			},
			"tags": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Optional:            true,
				Default:             mapdefault.StaticValue(types.MapNull(types.StringType)),
				MarkdownDescription: "Service Tags",
			},
		},
	}
}

// Create a new resource.
func (r *serviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var eplan serviceResourceModel

	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	encryptionRequired := eplan.EncryptionRequired.ValueBool()
	maxIdleMilliseconds := eplan.MaxIdleTimeMilliseconds.ValueInt64()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	terminatorStrategy := eplan.TerminatorStrategy.ValueString()

	var configs []string
	for _, value := range eplan.Configs.Elements() {
		if config, ok := value.(types.String); ok {
			configs = append(configs, config.ValueString())
		}
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := rest_model.ServiceCreate{
		Configs:            configs,
		EncryptionRequired: &encryptionRequired,
		MaxIdleTimeMillis:  maxIdleMilliseconds,
		Name:               &name,
		RoleAttributes:     roleAttributes,
		TerminatorStrategy: terminatorStrategy,
		Tags:               tags,
	}
	fmt.Printf("**********************create resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/services", r.resourceConfig.host)
	cresp, err := CreateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti POST Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating services", "Could not Create services, unexpected error: "+err.Error(),
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
func (r *serviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state serviceResourceModel
	tflog.Debug(ctx, "Reading Service")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/services/%s", r.resourceConfig.host, state.ID.ValueString())
	cresp, err := ReadZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti GET Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading services", "Could not READ services, unexpected error: "+err.Error(),
		)
		return
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal([]byte(cresp), &jsonBody)
	if err != nil {
		// Handle error
		resp.Diagnostics.AddError(
			"Error Reading services", fmt.Sprintf("Could not READ services, ERROR %v: ", err.Error()),
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

	if terminatorStrategy, ok := data["terminatorStrategy"].(string); ok {
		state.TerminatorStrategy = types.StringValue(terminatorStrategy)
	}

	if maxIdleTimeMilliseconds, ok := data["maxIdleTimeMillis"].(float64); ok {
		state.MaxIdleTimeMilliseconds = types.Int64Value(int64(maxIdleTimeMilliseconds))
	}

	if encryptionRequired, ok := data["encryptionRequired"].(bool); ok {
		state.EncryptionRequired = types.BoolValue(encryptionRequired)
	}

	if configs, ok := data["configs"].([]interface{}); ok {
		configs, diag := types.ListValueFrom(ctx, types.StringType, configs)
		resp.Diagnostics = append(resp.Diagnostics, diag...)
		state.Configs = configs
	} else {
		state.Configs = types.ListNull(types.StringType)
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
func (r *serviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var eplan serviceResourceModel
	tflog.Debug(ctx, "Updating Service")
	diags := req.Plan.Get(ctx, &eplan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state serviceResourceModel
	sdiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(sdiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := eplan.Name.ValueString()
	encryptionRequired := eplan.EncryptionRequired.ValueBool()
	maxIdleMilliseconds := eplan.MaxIdleTimeMilliseconds.ValueInt64()
	tags := TagsFromAttributes(eplan.Tags.Elements())
	terminatorStrategy := eplan.TerminatorStrategy.ValueString()

	var configs []string
	for _, value := range eplan.Configs.Elements() {
		if config, ok := value.(types.String); ok {
			configs = append(configs, config.ValueString())
		}
	}

	var roleAttributes rest_model.Attributes
	for _, value := range eplan.RoleAttributes.Elements() {
		if roleAttribute, ok := value.(types.String); ok {
			roleAttributes = append(roleAttributes, roleAttribute.ValueString())
		}
	}

	payload := rest_model.ServiceUpdate{
		Configs:            configs,
		EncryptionRequired: encryptionRequired,
		MaxIdleTimeMillis:  maxIdleMilliseconds,
		Name:               &name,
		RoleAttributes:     roleAttributes,
		TerminatorStrategy: terminatorStrategy,
		Tags:               tags,
	}
	fmt.Printf("**********************update resource payload***********************:\n %s\n", payload)

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/services/%s", r.resourceConfig.host, state.ID.ValueString())
	cresp, err := UpdateZitiResource(authUrl, r.resourceConfig.apiToken, jsonData)
	msg := fmt.Sprintf("Ziti PUT Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating services", "Could not Update services, unexpected error: "+err.Error(),
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
func (r *serviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state serviceResourceModel
	tflog.Debug(ctx, "Deleting Service")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	authUrl := fmt.Sprintf("%s/services/%s", r.resourceConfig.host, state.ID.ValueString())

	cresp, err := DeleteZitiResource(authUrl, r.resourceConfig.apiToken)
	msg := fmt.Sprintf("Ziti Delete Response: %s", cresp)
	log.Info().Msg(msg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting services", "Could not DELETE services, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *serviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
