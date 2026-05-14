package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure GuardicoreProvider satisfies provider interface.
var _ provider.Provider = &GuardicoreProvider{}

// GuardicoreProvider defines the provider implementation.
type GuardicoreProvider struct {
	version string
}

// GuardicoreProviderModel describes the provider data model.
type GuardicoreProviderModel struct {
	BaseURL               types.String `tfsdk:"base_url"`
	Username              types.String `tfsdk:"username"`
	Password              types.String `tfsdk:"password"`
	AccessToken           types.String `tfsdk:"access_token"`
	RefreshToken          types.String `tfsdk:"refresh_token"`
	InsecureSkipVerify    types.Bool   `tfsdk:"insecure_skip_verify"`
	RequestTimeout        types.Int64  `tfsdk:"request_timeout"`
	ValidateRefsOnDestroy types.Bool   `tfsdk:"validate_references_on_destroy"`
	StrictRefsOnDestroy   types.Bool   `tfsdk:"strict_references_on_destroy"`
}

func (p *GuardicoreProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "guardicore"
	resp.Version = p.version
}

func (p *GuardicoreProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use the Akamai Guardicore Segmentation provider to manage labels, label groups, policy rules, policy groups, DNS security blocklists, incidents, worksites, user groups, and assets through the Akamai Guardicore Segmentation API.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				MarkdownDescription: "The base URL of the Akamai Guardicore Segmentation API (e.g., https://guardicore.example.com). Can also be set via GUARDICORE_BASE_URL environment variable.",
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username for Akamai Guardicore Segmentation authentication. Can also be set via GUARDICORE_USERNAME environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for Akamai Guardicore Segmentation authentication. Can also be set via GUARDICORE_PASSWORD environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "Access token for Akamai Guardicore Segmentation authentication. Can also be set via GUARDICORE_ACCESS_TOKEN environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"refresh_token": schema.StringAttribute{
				MarkdownDescription: "Refresh token for Akamai Guardicore Segmentation authentication. Can also be set via GUARDICORE_REFRESH_TOKEN environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"insecure_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification. Can also be set via GUARDICORE_INSECURE_SKIP_VERIFY environment variable. " +
					"**Warning:** Disabling verification exposes connections to man-in-the-middle attacks. " +
					"Only use this for development or testing environments with self-signed certificates.",
				Optional: true,
			},
			"request_timeout": schema.Int64Attribute{
				MarkdownDescription: fmt.Sprintf("Timeout in seconds for individual HTTP requests to the Akamai Guardicore Segmentation API. "+
					"This applies per-request, not per-operation (paginated operations make multiple requests, "+
					"each with this timeout). Can also be set via GUARDICORE_REQUEST_TIMEOUT environment variable. "+
					"Default: %d.", client.DefaultRequestTimeout),
				Optional: true,
			},
			"validate_references_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "When true, check for external references before destroying labels or label groups. " +
					"If references are found, a warning diagnostic is emitted. Default: false.",
				Optional: true,
			},
			"strict_references_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "When true, block destruction of resources that are still referenced by other resources. " +
					"Implies validate_references_on_destroy. Default: false.",
				Optional: true,
			},
		},
	}
}

func (p *GuardicoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data GuardicoreProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get values from config or environment variables
	baseURL := getStringValue(data.BaseURL, "GUARDICORE_BASE_URL")
	username := getStringValue(data.Username, "GUARDICORE_USERNAME")
	password := getStringValue(data.Password, "GUARDICORE_PASSWORD")
	accessToken := getStringValue(data.AccessToken, "GUARDICORE_ACCESS_TOKEN")
	refreshToken := getStringValue(data.RefreshToken, "GUARDICORE_REFRESH_TOKEN")
	insecureSkipVerify := getBoolValue(data.InsecureSkipVerify, "GUARDICORE_INSECURE_SKIP_VERIFY")

	if baseURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Missing Base URL",
			"The provider requires base_url to be set, either in the provider configuration or via the GUARDICORE_BASE_URL environment variable.",
		)
		return
	}

	if insecureSkipVerify {
		resp.Diagnostics.AddWarning(
			"TLS Certificate Verification Disabled",
			"The provider is configured with insecure_skip_verify = true. "+
				"This disables TLS certificate verification, making connections susceptible to "+
				"man-in-the-middle attacks. Only use this for development or testing "+
				"with self-signed certificates.",
		)
		tflog.Warn(ctx, "TLS certificate verification is disabled", map[string]any{
			"insecure_skip_verify": true,
		})
	}

	requestTimeout := getInt64Value(data.RequestTimeout, "GUARDICORE_REQUEST_TIMEOUT")

	if requestTimeout < 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("request_timeout"),
			"Invalid Request Timeout",
			"request_timeout must be a positive integer (seconds). Set to 0 or omit to use the default.",
		)
		return
	}

	// Validate that at least one authentication method is provided
	hasAccessToken := accessToken != ""
	hasRefreshToken := refreshToken != ""
	hasCredentials := username != "" && password != ""

	if !hasAccessToken && !hasRefreshToken && !hasCredentials {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing Authentication Configuration",
			"The provider requires one of: access_token, refresh_token, or username+password. "+
				"These can be set via provider configuration or environment variables "+
				"(GUARDICORE_ACCESS_TOKEN, GUARDICORE_REFRESH_TOKEN, GUARDICORE_USERNAME, GUARDICORE_PASSWORD).",
		)
		return
	}

	// Create the client configuration
	config := client.Config{
		BaseURL:            baseURL,
		Username:           username,
		Password:           password,
		AccessToken:        accessToken,
		RefreshToken:       refreshToken,
		InsecureSkipVerify: insecureSkipVerify,
		RequestTimeout:     requestTimeout,
		RuntimeSettings:    nil,
	}
	runtime := client.ResolveRuntimeSettings(config.RuntimeSettings)
	assetLabelIgnoreCache := newAssetLabelIgnoreCache()

	// Create the API client
	apiClient, err := client.NewClient(config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Akamai Guardicore Segmentation API Client",
			"An unexpected error occurred when creating the Akamai Guardicore Segmentation API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Error: "+err.Error(),
		)
		return
	}

	// Wrap the client with provider-level configuration
	strictRefs := data.StrictRefsOnDestroy.ValueBool()
	providerData := &ProviderData{
		Client:                   apiClient,
		RuntimeSettings:          runtime,
		AssetLabelIgnoreCache:    assetLabelIgnoreCache,
		LabelCreateBatcher:       NewLabelCreateBatcher(apiClient, runtime.Batchers.Label.Create),
		LabelUpdateBatcher:       NewLabelUpdateBatcher(apiClient, runtime.Batchers.Label.Update),
		LabelDeleteBatcher:       NewLabelDeleteBatcher(apiClient, runtime.Batchers.Label.Delete),
		PolicyRuleCreateBatcher:  NewPolicyRuleCreateBatcher(apiClient, runtime.Batchers.PolicyRule.Create),
		PolicyRuleUpdateBatcher:  NewPolicyRuleUpdateBatcher(apiClient, runtime.Batchers.PolicyRule.Update),
		PolicyRuleDeleteBatcher:  NewPolicyRuleDeleteBatcher(apiClient, runtime.Batchers.PolicyRule.Delete),
		LabelGroupCreateBatcher:  NewLabelGroupCreateBatcher(apiClient, runtime.Batchers.LabelGroup.Create),
		LabelGroupUpdateBatcher:  NewLabelGroupUpdateBatcher(apiClient, runtime.Batchers.LabelGroup.Update),
		LabelGroupDeleteBatcher:  NewLabelGroupDeleteBatcher(apiClient, runtime.Batchers.LabelGroup.Delete),
		UserGroupCreateBatcher:   NewUserGroupCreateBatcher(apiClient, runtime.Batchers.UserGroup.Create),
		UserGroupUpdateBatcher:   NewUserGroupUpdateBatcher(apiClient, runtime.Batchers.UserGroup.Update),
		UserGroupDeleteBatcher:   NewUserGroupDeleteBatcher(apiClient, runtime.Batchers.UserGroup.Delete),
		AssetCreateBatcher:       NewAssetCreateBatcher(apiClient, runtime.Batchers.Asset.Create),
		AssetUpdateBatcher:       NewAssetUpdateBatcher(apiClient, runtime.Batchers.Asset.Update),
		AssetDeleteBatcher:       NewAssetDeleteBatcher(apiClient, runtime.Batchers.Asset.Delete),
		DnsSecurityCreateBatcher: NewDnsSecurityCreateBatcher(apiClient, runtime.Batchers.DnsSecurity.Create),
		DnsSecurityUpdateBatcher: NewDnsSecurityUpdateBatcher(apiClient, runtime.Batchers.DnsSecurity.Update),
		DnsSecurityDeleteBatcher: NewDnsSecurityDeleteBatcher(apiClient, runtime.Batchers.DnsSecurity.Delete),
		IncidentCreateBatcher:    NewIncidentCreateBatcher(apiClient, runtime.Batchers.Incident.Create),
		WorksiteDeleteBatcher:    NewWorksiteDeleteBatcher(apiClient, runtime.Batchers.Worksite.Delete),
		ValidateRefsOnDestroy:    data.ValidateRefsOnDestroy.ValueBool() || strictRefs,
		StrictRefsOnDestroy:      strictRefs,
	}

	// Make the provider data available to resources and data sources
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *GuardicoreProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewLabelResource,
		NewLabelGroupResource,
		NewPolicyRuleResource,
		NewPolicyGroupResource,
		NewDnsSecurityResource,
		NewIncidentResource,
		NewWorksiteResource,
		NewUserGroupResource,
		NewAssetResource,
	}
}

func (p *GuardicoreProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLabelDataSource,
		NewLabelGroupDataSource,
		NewPolicyRuleDataSource,
		NewPolicyGroupDataSource,
		NewDnsSecurityDataSource,
		NewIncidentDataSource,
		NewWorksiteDataSource,
		NewUserGroupDataSource,
		NewAssetDataSource,
		NewAgentAggregatorDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GuardicoreProvider{
			version: version,
		}
	}
}

// getStringValue returns the value from the Terraform config, or falls back to environment variable.
func getStringValue(tfValue types.String, envVar string) string {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueString()
	}
	return os.Getenv(envVar)
}

// getBoolValue returns the value from the Terraform config, or falls back to environment variable.
func getBoolValue(tfValue types.Bool, envVar string) bool {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueBool()
	}
	if v := os.Getenv(envVar); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return false
}

// getInt64Value returns the value from the Terraform config, or falls back to environment variable.
// Returns 0 if neither is set.
func getInt64Value(tfValue types.Int64, envVar string) int64 {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueInt64()
	}
	if v := os.Getenv(envVar); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}
