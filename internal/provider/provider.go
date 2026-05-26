package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

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
				MarkdownDescription: "Ziti controller Host/Domain URL. Use `hosts` to configure multiple controllers for HA failover.",
			},
			"hosts": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of Ziti controller Host/Domain URLs for HA failover. First successful authentication wins. Finds and prefers the leader",
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
	Hosts    types.List   `tfsdk:"hosts"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

// tryAuthenticate authenticates against a single Ziti controller and returns the session token.
func tryAuthenticate(host, username, password string) (string, error) {
	payload := map[string]interface{}{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(payload)
	authUrl := fmt.Sprintf("%s/authenticate?method=password", host)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	creq, err := http.NewRequest("POST", authUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to build auth request: %w", err)
	}
	creq.Header.Add("Content-Type", "application/json")
	cresp, err := httpClient.Do(creq)
	if err != nil {
		return "", fmt.Errorf("auth request failed: %w", err)
	}
	body, err := io.ReadAll(cresp.Body)
	defer cresp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("error reading auth response: %w", err)
	}
	if cresp.StatusCode != http.StatusOK && cresp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("status %d: %s", cresp.StatusCode, string(body))
	}
	token := gjson.GetBytes(body, "data.token").String()
	if token == "" {
		return "", fmt.Errorf("no token returned in auth response")
	}
	return token, nil
}

type clusterMember struct {
	Address   string `json:"address"`
	Connected bool   `json:"connected"`
	ID        string `json:"id"`
	Leader    bool   `json:"leader"`
	ReadOnly  bool   `json:"readOnly"`
	Version   string `json:"version"`
	Voter     bool   `json:"voter"`
}

// fetchClusterMembers calls /fabric/v1/cluster/list-members and returns the member list.
func fetchClusterMembers(activeHost, token string) ([]clusterMember, error) {
	u, err := url.Parse(activeHost)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}
	clusterURL := fmt.Sprintf("%s://%s/fabric/v1/cluster/list-members", u.Scheme, u.Host)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", clusterURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build cluster request: %w", err)
	}
	req.Header.Set("zt-session", token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cluster request failed: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading cluster response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cluster list-members status %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Data []clusterMember `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error parsing cluster response: %w", err)
	}
	return result.Data, nil
}

// memberToHost converts a cluster member tls address (e.g. "tls:HOST:PORT") to
// an https URL preserving the base path of the original host.
func memberToHost(address, originalHost string) (string, error) {
	addr := strings.TrimPrefix(address, "tls:")
	u, err := url.Parse(originalHost)
	if err != nil {
		return "", fmt.Errorf("invalid original host: %w", err)
	}
	return fmt.Sprintf("https://%s%s", addr, u.Path), nil
}

func (p *zitiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring ziti client")
	var config zitiProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown ziti API Host",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_API_HOST environment variable.",
		)
	}

	if config.Hosts.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("hosts"),
			"Unknown ziti API Hosts",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API hosts list. "+
				"Either target apply the source of the value first or set the values statically in the configuration.",
		)
	}

	if config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown ziti API Username",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API username. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_API_USERNAME environment variable.",
		)
	}

	if config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown ziti API Password",
			"The provider cannot create the ziti API client as there is an unknown configuration value for the API password. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ZITI_API_PASSWORD environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	username := os.Getenv("ZITI_API_USERNAME")
	password := os.Getenv("ZITI_API_PASSWORD")

	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}

	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}

	if username == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing ziti API Username",
			"The provider cannot create the ziti API client as there is a missing or empty value for the ziti API username. "+
				"Set the username value in the configuration or use the ZITI_API_USERNAME environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing ziti API Password",
			"The provider cannot create the ziti API client as there is a missing or empty value for the ziti API password. "+
				"Set the password value in the configuration or use the ZITI_API_PASSWORD environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Build an ordered, deduplicated list of controllers to try.
	// 'host' is prepended first so existing single-host configs are unaffected.
	// 'hosts' elements follow for HA failover.
	// Falls back to ZITI_API_HOST only when neither attribute provides any value.
	var allHosts []string
	multiHost := false
	seen := make(map[string]bool)
	addHost := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			allHosts = append(allHosts, h)
		}
	}

	if !config.Host.IsNull() {
		addHost(config.Host.ValueString())
	}

	if !config.Hosts.IsNull() {
		var hostList []string
		diags = config.Hosts.ElementsAs(ctx, &hostList, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, h := range hostList {
			addHost(h)
		}
		multiHost = len(hostList) > 0
	}

	if len(allHosts) == 0 {
		addHost(os.Getenv("ZITI_API_HOST"))
	}

	if len(allHosts) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing ZITI API Host",
			"The provider cannot create the ziti API client as there is a missing or empty value for the ziti API host. "+
				"Set the 'host' value, the 'hosts' list in the configuration, or use the ZITI_API_HOST environment variable.",
		)
		return
	}

	ctx = tflog.SetField(ctx, "ziti_username", username)
	tflog.Debug(ctx, "Creating ziti client")

	// Try each controller in order; use the first one that authenticates.
	var activeHost, zitiToken string
	var authErrs []string
	for _, h := range allHosts {
		token, err := tryAuthenticate(h, username, password)
		if err != nil {
			log.Warn().Msgf("Failed to authenticate with Ziti controller %s: %v", h, err)
			authErrs = append(authErrs, fmt.Sprintf("%s: %v", h, err))
			continue
		}
		activeHost = h
		zitiToken = token
		break
	}

	if activeHost == "" {
		resp.Diagnostics.AddError(
			"Failed to authenticate with any configured Ziti controller",
			strings.Join(authErrs, "; "),
		)
		return
	}

	// When multiple hosts are configured, discover cluster members and re-authenticate
	// against each one, preferring the leader.
	if multiHost {
		members, err := fetchClusterMembers(activeHost, zitiToken)
		if err != nil {
			log.Warn().Msgf("Could not fetch cluster members from %s: %v", activeHost, err)
		} else {
			// Sort so the leader is tried first.
			leaderFirst := make([]clusterMember, 0, len(members))
			for _, m := range members {
				if m.Leader {
					leaderFirst = append([]clusterMember{m}, leaderFirst...)
				} else {
					leaderFirst = append(leaderFirst, m)
				}
			}
			for _, m := range leaderFirst {
				if !m.Connected {
					continue
				}
				memberHost, err := memberToHost(m.Address, activeHost)
				if err != nil {
					log.Warn().Msgf("Skipping cluster member %s (bad address %q): %v", m.ID, m.Address, err)
					continue
				}
				token, err := tryAuthenticate(memberHost, username, password)
				if err != nil {
					log.Warn().Msgf("Auth failed for cluster member %s (%s): %v", m.ID, memberHost, err)
					continue
				}
				log.Info().Msgf("Using cluster member %s (leader=%v) at %s", m.ID, m.Leader, memberHost)
				activeHost = memberHost
				zitiToken = token
				break
			}
		}
	}

	fmt.Printf("Using zitiToken: %s\n", zitiToken)

	resourceData := zitiData{
		apiToken: zitiToken,
		host:     activeHost,
	}

	resp.DataSourceData = &resourceData
	resp.ResourceData = &resourceData

	tflog.Info(ctx, "Configured ziti client", map[string]any{"success": true, "host": activeHost})
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
		NewIdentityNoneResource,
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
