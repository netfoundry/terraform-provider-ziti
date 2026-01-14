package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &zitiProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &zitiProvider{
			version: version,
		}
	}
}

// zitiProvider is the provider implementation.
type zitiProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *zitiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ziti"
	resp.Version = p.version
}

type zitiData struct {
	apiToken string
	host     string
}

// Schema defines the provider-level schema for configuration data.
func (p *zitiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ziti Terraform Provider",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Ziti Host/Domain URL",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Ziti Session username",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Ziti Session password",
			},
		},
	}
}

// zitiProviderModel maps provider schema data to a Go type.
type zitiProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *zitiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring ziti client")
	// Retrieve provider data from configuration
	var config zitiProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown ziti API Host",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_HOST environment variable.",
		)
	}

	if config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown ziti API Username",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API username. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_USERNAME environment variable.",
		)
	}

	if config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown ziti API Password",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API password. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_PASSWORD environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("ZITI_API_HOST")
	username := os.Getenv("ZITI_API_USERNAME")
	password := os.Getenv("ZITI_API_PASSWORD")

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}

	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing ZITI API Host",
			"The provider cannot create the ziti API client as there is a missing or empty value for the ziti API host. "+
				"Set the host value in the configuration or use the ZITI_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if username == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing ziti API Username",
			"The provider cannot create the ziti API client as there is a missing or empty value for the zitiAPI username. "+
				"Set the username value in the configuration or use the ZITI_USERNAME environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing ziti API Password",
			"The provider cannot create the ziti API client as there is a missing or empty value for the ziti API password. "+
				"Set the password value in the configuration or use the ZITI_PASSWORD environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "ziti_host", host)
	ctx = tflog.SetField(ctx, "ziti_username", username)
	ctx = tflog.SetField(ctx, "ziti_password", password)

	tflog.Debug(ctx, "Creating ziti client")

	payload := map[string]interface{}{
		"username": username,
		"password": password,
	}

	// Convert the payload to JSON
	jsonData, _ := json.Marshal(payload)

	authUrl := fmt.Sprintf("%s/authenticate?method=password", host)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	creq, _ := http.NewRequest("POST", authUrl, bytes.NewBuffer(jsonData))
	creq.Header.Add("Content-Type", "application/json")
	cresp, err := httpClient.Do(creq)

	if err != nil {
		resp.Diagnostics.AddError("Error configuring the API client", err.Error())
		return
	}

	body, err := io.ReadAll(cresp.Body)
	defer cresp.Body.Close()
	if err != nil {
		log.Error().Msgf("Error Reading Ziti Resource Response: %v", err)
	}

	stringBody := string(body)
	if cresp.StatusCode != http.StatusOK {
		if cresp.StatusCode != http.StatusCreated {
			resp.Diagnostics.AddError("Unexpected error: %s", string(body))
			return
		}
	}

	var jsonBody map[string]interface{}
	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		log.Error().Msgf("Error unmarshalling JSON response from Ziti Resource Response: %v", err)
		return
	}

	zitiToken := gjson.Get(stringBody, "data.token").String()
	fmt.Printf("Using zitiToken: %s\n", zitiToken)

	// Make the HashiCups client available during DataSource and Resource
	// type Configure methods.
	resourceData := zitiData{
		apiToken: zitiToken,
		host:     host,
	}

	resp.DataSourceData = &resourceData
	resp.ResourceData = &resourceData

	tflog.Info(ctx, "Configured ziti client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *zitiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEdgeRouterPolicyDataSource,
		NewServicePolicyDataSource,
		NewServiceEdgeRouterPolicyDataSource,
		NewEdgeRouterDataSource,
		NewServiceDataSource,
		NewIdentityDataSource,
		NewInterceptV1ConfigDataSource,
		NewHostV1ConfigDataSource,
		NewHostV2ConfigDataSource,
		NewPostureCheckMacDataSource,
		NewPostureCheckDomainDataSource,
		NewPostureCheckMFADataSource,
		NewPostureCheckOSDataSource,
		NewPostureCheckProcessDataSource,
		NewPostureCheckMultiProcessDataSource,
		NewCertificateAuthorityDataSource,
		NewJwtSignerDataSource,
		NewAuthPolicyDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *zitiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEdgeRouterPolicyResource,
		NewIdentityResource,
		NewIdentityUpdbResource,
		NewIdentityCaResource,
		NewServicePolicyResource,
		NewServiceEdgeRouterPolicyResource,
		NewEdgeRouterResource,
		NewInterceptV1ConfigResource,
		NewHostV1ConfigResource,
		NewHostV2ConfigResource,
		NewServiceResource,
		NewPostureCheckMacResource,
		NewPostureCheckDomainResource,
		NewPostureCheckMFAResource,
		NewPostureCheckOSResource,
		NewPostureCheckProcessResource,
		NewPostureCheckMultiProcessResource,
		NewCertificateAuthorityResource,
		NewJwtSignerResource,
		NewAuthPolicyResource,
	}
}
