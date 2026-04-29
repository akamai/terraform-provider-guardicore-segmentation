package provider

import (
	"flag"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
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
