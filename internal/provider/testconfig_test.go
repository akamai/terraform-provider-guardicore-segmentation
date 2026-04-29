package provider

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testConfigPath is the path to the test configuration file.
var testConfigPath = flag.String("test-config", "testconfig.json", "Path to test configuration file")

// testConfig holds the loaded test configuration.
var testConfig *TestConfig

// TestConfig represents the test configuration structure.
type TestConfig struct {
	BaseURL            string `json:"base_url"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	RequestTimeout     int64  `json:"request_timeout"`

	// User Group test dependencies
	UserGroupOrchestrationID string `json:"user_group_orchestration_id"`
	UserGroupGroupID         string `json:"user_group_group_id"`
}

// loadTestConfig loads the test configuration from file and environment variables.
func loadTestConfig() (*TestConfig, error) {
	config := &TestConfig{}

	// Load from file if it exists
	if *testConfigPath != "" {
		// Try multiple paths since go test changes working directory
		paths := []string{
			*testConfigPath,
			"../../" + *testConfigPath, // From internal/provider/ to project root
		}

		var data []byte
		var err error
		loaded := false

		for _, path := range paths {
			data, err = os.ReadFile(path)
			if err == nil {
				loaded = true
				break
			}
		}

		if loaded {
			if err := json.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse test config: %w", err)
			}
		}
		// If file doesn't exist, that's ok - we'll use env vars
	}

	// Environment variables override file values
	if v := os.Getenv("GUARDICORE_BASE_URL"); v != "" {
		config.BaseURL = v
	}
	if v := os.Getenv("GUARDICORE_USERNAME"); v != "" {
		config.Username = v
	}
	if v := os.Getenv("GUARDICORE_PASSWORD"); v != "" {
		config.Password = v
	}
	if v := os.Getenv("GUARDICORE_ACCESS_TOKEN"); v != "" {
		config.AccessToken = v
	}
	if v := os.Getenv("GUARDICORE_REFRESH_TOKEN"); v != "" {
		config.RefreshToken = v
	}
	if v := os.Getenv("GUARDICORE_INSECURE_SKIP_VERIFY"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			config.InsecureSkipVerify = parsed
		}
	}
	if v := os.Getenv("GUARDICORE_REQUEST_TIMEOUT"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			config.RequestTimeout = parsed
		}
	}
	if v := os.Getenv("GUARDICORE_USER_GROUP_ORCHESTRATION_ID"); v != "" {
		config.UserGroupOrchestrationID = v
	}
	if v := os.Getenv("GUARDICORE_USER_GROUP_GROUP_ID"); v != "" {
		config.UserGroupGroupID = v
	}

	return config, nil
}

// testAccProviderConfig returns the HCL provider block using loaded config values.
// nolint:unused // Used by acceptance tests when TF_ACC=1
func testAccProviderConfig() string {
	if testConfig == nil {
		return ""
	}

	extraAttrs := ""
	if testConfig.InsecureSkipVerify {
		extraAttrs += "\n  insecure_skip_verify = true"
	}
	if testConfig.RequestTimeout > 0 {
		extraAttrs += fmt.Sprintf("\n  request_timeout = %d", testConfig.RequestTimeout)
	}

	// Prefer access token if available
	if testConfig.AccessToken != "" {
		return fmt.Sprintf(`
provider "guardicore" {
  base_url     = %[1]q
  access_token = %[2]q%[3]s
}
`, testConfig.BaseURL, testConfig.AccessToken, extraAttrs)
	}

	// Use username/password
	return fmt.Sprintf(`
provider "guardicore" {
  base_url = %[1]q
  username = %[2]q
  password = %[3]q%[4]s
}
`, testConfig.BaseURL, testConfig.Username, testConfig.Password, extraAttrs)
}

// nolint:unused // Used by acceptance tests when TF_ACC=1
func testAccProviderConfigWithRefValidation(validateOnDestroy bool) string {
	if testConfig == nil {
		return ""
	}

	insecure := ""
	if testConfig.InsecureSkipVerify {
		insecure = "\n  insecure_skip_verify = true"
	}

	refValidation := fmt.Sprintf("\n  validate_references_on_destroy = %t", validateOnDestroy)

	if testConfig.AccessToken != "" {
		return fmt.Sprintf(`
provider "guardicore" {
  base_url     = %[1]q
  access_token = %[2]q%[3]s%[4]s
}
`, testConfig.BaseURL, testConfig.AccessToken, insecure, refValidation)
	}

	return fmt.Sprintf(`
provider "guardicore" {
  base_url = %[1]q
  username = %[2]q
  password = %[3]q%[4]s%[5]s
}
`, testConfig.BaseURL, testConfig.Username, testConfig.Password, insecure, refValidation)
}

// testAccRandomName returns a random name with the given prefix.
func testAccRandomName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, acctest.RandString(8))
}

// testAccCheckResourceDestroyed verifies that a resource no longer exists via API.
func testAccCheckResourceDestroyed(resourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		// Create API client
		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		ctx := context.Background()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}

			var exists bool
			switch resourceType {
			case "guardicore_label":
				label, err := apiClient.GetLabel(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = label != nil

			case "guardicore_label_group":
				labelGroup, err := apiClient.GetLabelGroup(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = labelGroup != nil

			case "guardicore_policy_rule":
				rule, err := apiClient.GetPolicyRule(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = rule != nil

			case "guardicore_dns_security":
				blocklist, err := apiClient.GetDnsBlocklist(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = blocklist != nil

			case "guardicore_incident":
				// Incidents cannot be deleted via API, so destroy is a no-op.
				// The resource is removed from state but persists in Akamai Guardicore Segmentation.
				exists = false

			case "guardicore_worksite":
				worksite, err := apiClient.GetWorksite(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = worksite != nil

			case "guardicore_user_group":
				userGroup, err := apiClient.GetUserGroup(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				exists = userGroup != nil

			case "guardicore_asset":
				asset, err := apiClient.GetAsset(ctx, rs.Primary.ID)
				if err != nil {
					return err
				}
				// Asset DELETE deactivates rather than removes; check if still active
				exists = asset != nil && asset.Status != "deleted"

			default:
				return fmt.Errorf("unknown resource type: %s", resourceType)
			}

			if exists {
				return fmt.Errorf("%s %s still exists", resourceType, rs.Primary.ID)
			}
		}

		return nil
	}
}

// testAccDeleteLabelOutOfBand deletes a label via API for disappears tests.
func testAccDeleteLabelOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabel(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.PublishLabelGroups(context.Background())
	}
}

// testAccDeleteLabelGroupOutOfBand deletes a label group via API for disappears tests.
func testAccDeleteLabelGroupOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabelGroup(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.PublishLabelGroups(context.Background())
	}
}

// testAccDeletePolicyRuleOutOfBand deletes a policy rule via API for disappears tests.
func testAccDeletePolicyRuleOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeletePolicyRule(context.Background(), rs.Primary.ID)
	}
}

// testAccDeleteDnsSecurityOutOfBand deletes a DNS blocklist via API for disappears tests.
func testAccDeleteDnsSecurityOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeleteDnsBlocklist(context.Background(), rs.Primary.ID)
	}
}

// testAccDeleteLabelExpectFailure attempts delete and expects API failure.
func testAccDeleteLabelExpectFailure(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabel(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("expected label delete to fail, but it succeeded")
		}

		return nil
	}
}

// testAccDeleteLabelGroupExpectFailure attempts delete and expects API failure.
//
//nolint:unused // test helper kept for future use
func testAccDeleteLabelGroupExpectFailure(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabelGroup(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("expected label group delete to fail, but it succeeded")
		}

		return nil
	}
}

// testAccCheckLabelGroupPublished ensures label groups are published after create.
func testAccCheckLabelGroupPublished(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		labelGroup, err := apiClient.GetLabelGroup(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if labelGroup == nil {
			return fmt.Errorf("label group not found: %s", rs.Primary.ID)
		}

		if err := apiClient.PublishLabelGroups(context.Background()); err != nil {
			return err
		}

		return nil
	}
}

// testAccCheckPolicyRevisionPublished ensures policy rules are published after create.
func testAccCheckPolicyRevisionPublished(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		rule, err := apiClient.GetPolicyRule(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if rule == nil {
			return fmt.Errorf("policy rule not found: %s", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckPolicyRuleRemoteComments verifies the remote policy rule comments value.
func testAccCheckPolicyRuleRemoteComments(resourceName, expectedComments string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		rule, err := apiClient.GetPolicyRule(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if rule == nil {
			return fmt.Errorf("policy rule not found: %s", rs.Primary.ID)
		}

		comments, ok := policyRuleCommentFromAPI(rule)
		if !ok {
			return fmt.Errorf("policy rule comments missing or not a string: %#v", rule)
		}
		if comments != expectedComments {
			return fmt.Errorf("expected remote policy rule comments %q, got %q", expectedComments, comments)
		}

		return nil
	}
}

func policyRuleCommentFromAPI(rule map[string]interface{}) (string, bool) {
	if comments, ok := rule["comments"].(string); ok {
		return comments, true
	}

	attributes, ok := rule["attributes"].(map[string]interface{})
	if !ok {
		return "", false
	}

	comments, ok := attributes["comments"].(string)
	return comments, ok
}

// testAccWorksitePreCheck verifies the worksites feature is enabled on the Akamai Guardicore Segmentation instance.
func testAccWorksitePreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	config := client.Config{
		BaseURL:            testConfig.BaseURL,
		Username:           testConfig.Username,
		Password:           testConfig.Password,
		AccessToken:        testConfig.AccessToken,
		RefreshToken:       testConfig.RefreshToken,
		InsecureSkipVerify: testConfig.InsecureSkipVerify,
	}

	apiClient, err := client.NewClient(config)
	if err != nil {
		t.Skipf("Skipping worksite test: failed to create client: %v", err)
	}

	_, err = apiClient.ListWorksites(context.Background(), "")
	if err != nil {
		t.Skipf("Skipping worksite test: %v", err)
	}
}

// testAccPolicyGroupPreCheck verifies the policy groups feature is enabled on the Akamai Guardicore Segmentation instance.
func testAccPolicyGroupPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	config := client.Config{
		BaseURL:            testConfig.BaseURL,
		Username:           testConfig.Username,
		Password:           testConfig.Password,
		AccessToken:        testConfig.AccessToken,
		RefreshToken:       testConfig.RefreshToken,
		InsecureSkipVerify: testConfig.InsecureSkipVerify,
	}

	apiClient, err := client.NewClient(config)
	if err != nil {
		t.Skipf("Skipping policy group test: failed to create client: %v", err)
	}

	_, err = apiClient.ListPolicyGroups(context.Background(), "", "")
	if err != nil {
		t.Skipf("Skipping policy group test: %v", err)
	}
}

// testAccDeleteWorksiteOutOfBand deletes a worksite via API for disappears tests.
func testAccDeleteWorksiteOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeleteWorksite(context.Background(), rs.Primary.ID)
	}
}

// testAccUserGroupPreCheck verifies orchestration/group IDs are configured for user group tests.
func testAccUserGroupPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if testConfig.UserGroupOrchestrationID == "" || testConfig.UserGroupGroupID == "" {
		t.Skip("Skipping user group test: user_group_orchestration_id and user_group_group_id must be set in testconfig.json or environment variables (GUARDICORE_USER_GROUP_ORCHESTRATION_ID, GUARDICORE_USER_GROUP_GROUP_ID)")
	}
}

// testAccDeleteUserGroupOutOfBand deletes a user group via API for disappears tests.
func testAccDeleteUserGroupOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeleteUserGroup(context.Background(), rs.Primary.ID)
	}
}

// testAccAssetPreCheck verifies the assets endpoint is accessible.
func testAccAssetPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	config := client.Config{
		BaseURL:            testConfig.BaseURL,
		Username:           testConfig.Username,
		Password:           testConfig.Password,
		AccessToken:        testConfig.AccessToken,
		RefreshToken:       testConfig.RefreshToken,
		InsecureSkipVerify: testConfig.InsecureSkipVerify,
	}

	apiClient, err := client.NewClient(config)
	if err != nil {
		t.Skipf("Skipping asset test: failed to create client: %v", err)
	}

	_, err = apiClient.ListAssets(context.Background(), "")
	if err != nil {
		t.Skipf("Skipping asset test: %v", err)
	}
}

// testAccDeleteAssetOutOfBand deletes (deactivates) an asset via API for disappears tests.
func testAccDeleteAssetOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if testConfig == nil {
			return fmt.Errorf("test config not loaded")
		}

		config := client.Config{
			BaseURL:            testConfig.BaseURL,
			Username:           testConfig.Username,
			Password:           testConfig.Password,
			AccessToken:        testConfig.AccessToken,
			RefreshToken:       testConfig.RefreshToken,
			InsecureSkipVerify: testConfig.InsecureSkipVerify,
		}

		apiClient, err := client.NewClient(config)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.BulkDeactivateAssets(context.Background(), []string{rs.Primary.ID})
	}
}

// testAccCheckJSONAttr performs semantic JSON comparison for attributes.
func testAccCheckJSONAttr(resourceName, attrName string, expectedJSON string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		actualJSON, ok := rs.Primary.Attributes[attrName]
		if !ok {
			return fmt.Errorf("attribute %s not found", attrName)
		}

		// Unmarshal both to compare semantically
		var actual, expected interface{}
		if err := json.Unmarshal([]byte(actualJSON), &actual); err != nil {
			return fmt.Errorf("failed to unmarshal actual JSON: %w", err)
		}
		if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
			return fmt.Errorf("failed to unmarshal expected JSON: %w", err)
		}

		// Marshal both back to get normalized JSON for comparison
		actualNorm, _ := json.Marshal(actual)
		expectedNorm, _ := json.Marshal(expected)

		if string(actualNorm) != string(expectedNorm) {
			return fmt.Errorf("JSON mismatch:\nActual:   %s\nExpected: %s", actualNorm, expectedNorm)
		}

		return nil
	}
}
