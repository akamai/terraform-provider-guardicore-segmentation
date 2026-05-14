package provider

import (
	"context"
	"flag"
	"os"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"guardicore": providerserver.NewProtocol6WithError(New("test")()),
}

// TestMain handles test initialization including flag parsing and config loading.
func TestMain(m *testing.M) {
	// Parse flags (including -test-config flag defined in testconfig_test.go)
	flag.Parse()

	// Keep non-acceptance runs safe for helper functions that reference testConfig.
	testConfig = &TestConfig{}

	// Load test config only if TF_ACC is set (acceptance tests mode)
	if os.Getenv("TF_ACC") == "1" {
		var err error
		testConfig, err = loadTestConfig()
		if err != nil {
			// Print error but don't fail - let individual tests handle missing config
			println("Warning: failed to load test config:", err.Error())
		}
	}

	// Run tests
	os.Exit(m.Run())
}

func testAccPreCheck(t *testing.T) {
	// Verify config is loaded for acceptance tests
	if testConfig == nil {
		t.Skip("Test config not loaded. Set TF_ACC=1 and provide testconfig.json or environment variables")
	}

	// Verify required fields are set
	if testConfig.BaseURL == "" {
		t.Skip("GUARDICORE_BASE_URL must be set for acceptance tests")
	}

	// Verify at least one authentication method is configured
	hasAccessToken := testConfig.AccessToken != ""
	hasRefreshToken := testConfig.RefreshToken != ""
	hasCredentials := testConfig.Username != "" && testConfig.Password != ""

	if !hasAccessToken && !hasRefreshToken && !hasCredentials {
		t.Skip("Authentication must be configured: GUARDICORE_ACCESS_TOKEN, GUARDICORE_REFRESH_TOKEN, or GUARDICORE_USERNAME+GUARDICORE_PASSWORD")
	}
}

func TestGetStringValue(t *testing.T) {
	t.Setenv("TEST_STRING_ENV", "from-env")

	if got := getStringValue(types.StringValue("from-config"), "TEST_STRING_ENV"); got != "from-config" {
		t.Fatalf("expected config value, got %q", got)
	}

	if got := getStringValue(types.StringNull(), "TEST_STRING_ENV"); got != "from-env" {
		t.Fatalf("expected env fallback, got %q", got)
	}

	if got := getStringValue(types.StringUnknown(), "TEST_STRING_ENV"); got != "from-env" {
		t.Fatalf("expected env fallback for unknown value, got %q", got)
	}
}

func TestGetBoolValue(t *testing.T) {
	if got := getBoolValue(types.BoolValue(true), "TEST_BOOL_ENV"); !got {
		t.Fatalf("expected config bool to win")
	}

	t.Setenv("TEST_BOOL_ENV", "true")
	if got := getBoolValue(types.BoolNull(), "TEST_BOOL_ENV"); !got {
		t.Fatalf("expected true from env")
	}

	t.Setenv("TEST_BOOL_ENV", "not-a-bool")
	if got := getBoolValue(types.BoolUnknown(), "TEST_BOOL_ENV"); got {
		t.Fatalf("expected false when env bool parse fails")
	}
}

func TestGetInt64Value(t *testing.T) {
	if got := getInt64Value(types.Int64Value(42), "TEST_I64_ENV"); got != 42 {
		t.Fatalf("expected config int64, got %d", got)
	}

	t.Setenv("TEST_I64_ENV", "99")
	if got := getInt64Value(types.Int64Null(), "TEST_I64_ENV"); got != 99 {
		t.Fatalf("expected env int64, got %d", got)
	}

	t.Setenv("TEST_I64_ENV", "bad")
	if got := getInt64Value(types.Int64Unknown(), "TEST_I64_ENV"); got != 0 {
		t.Fatalf("expected zero when env int64 parse fails, got %d", got)
	}
}

func TestProviderMetadata(t *testing.T) {
	p := &GuardicoreProvider{version: "v-test"}
	resp := &frameworkprovider.MetadataResponse{}
	p.Metadata(context.Background(), frameworkprovider.MetadataRequest{}, resp)

	if resp.TypeName != "guardicore" {
		t.Fatalf("expected typename guardicore, got %q", resp.TypeName)
	}
	if resp.Version != "v-test" {
		t.Fatalf("expected version v-test, got %q", resp.Version)
	}
}

func TestProviderSchemaResourcesAndDataSources(t *testing.T) {
	p := &GuardicoreProvider{version: "v-test"}

	schemaResp := &frameworkprovider.SchemaResponse{}
	p.Schema(context.Background(), frameworkprovider.SchemaRequest{}, schemaResp)

	if len(schemaResp.Schema.Attributes) == 0 {
		t.Fatal("expected provider schema attributes")
	}
	if _, ok := schemaResp.Schema.Attributes["base_url"]; !ok {
		t.Fatal("expected base_url attribute in schema")
	}

	if got := len(p.Resources(context.Background())); got != 9 {
		t.Fatalf("expected 9 resources, got %d", got)
	}
	if got := len(p.DataSources(context.Background())); got != 10 {
		t.Fatalf("expected 10 data sources, got %d", got)
	}
}

func TestNewReturnsProviderWithVersion(t *testing.T) {
	factory := New("1.2.3")
	p := factory()

	gcp, ok := p.(*GuardicoreProvider)
	if !ok {
		t.Fatalf("expected *GuardicoreProvider, got %T", p)
	}
	if gcp.version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", gcp.version)
	}
}
