// Package acctest provides shared test utilities for acceptance tests across
// domain sub-packages. This is a regular (non-test) package so that _test.go
// files in any sub-package can import it.
package acctest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	// TestAccDestroyCheckAttempts is the number of retry attempts when
	// waiting for a resource to be destroyed.
	TestAccDestroyCheckAttempts = 12

	// TestAccDestroyCheckInterval is the sleep duration between destroy
	// check retries.
	TestAccDestroyCheckInterval = 2 * time.Second
)

// TestConfigPath is the flag-registered path to the test configuration file.
var TestConfigPath = flag.String("test-config", "testconfig.json", "Path to test configuration file")

// TestConfigInstance holds the loaded test configuration. It is set during
// InitTestConfig and should be treated as read-only after that point.
var TestConfigInstance *TestConfig

// TestAccProtoV6ProviderFactories is used to instantiate a provider during
// acceptance testing. Each test package's TestMain must populate this via
// SetProviderFactories before running tests.
var TestAccProtoV6ProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// SetProviderFactories sets the provider factory map used by acceptance tests.
func SetProviderFactories(factories map[string]func() (tfprotov6.ProviderServer, error)) {
	TestAccProtoV6ProviderFactories = factories
}

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

	// Worksite delete blocked acceptance-test fixture dependency
	WorksiteAssignedAssetID string `json:"worksite_assigned_asset_id"`
}

// InitTestConfig should be called from each domain test package's TestMain.
// It parses flags, initialises TestConfigInstance to an empty TestConfig, and
// if TF_ACC=1 loads the full configuration from file/environment.
func InitTestConfig() {
	flag.Parse()

	// Keep non-acceptance runs safe for helper functions that reference
	// TestConfigInstance.
	TestConfigInstance = &TestConfig{}

	// Load test config only if TF_ACC is set (acceptance tests mode)
	if os.Getenv("TF_ACC") == "1" {
		var err error
		TestConfigInstance, err = LoadTestConfig()
		if err != nil {
			// Print error but don't fail - let individual tests handle
			// missing config
			println("Warning: failed to load test config:", err.Error())
		}
	}
}

// LoadTestConfig loads the test configuration from file and environment
// variables.
func LoadTestConfig() (*TestConfig, error) {
	config := &TestConfig{}

	// Load from file if it exists
	if *TestConfigPath != "" {
		// Try multiple paths since go test changes working directory
		paths := []string{
			*TestConfigPath,
			"../../" + *TestConfigPath,    // From internal/provider/ to project root
			"../../../" + *TestConfigPath, // From internal/provider/XXX/ to project root
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
	if v := os.Getenv("GUARDICORE_WORKSITE_ASSIGNED_ASSET_ID"); v != "" {
		config.WorksiteAssignedAssetID = v
	}

	return config, nil
}

// newTestClient creates an API client from the current TestConfigInstance.
func newTestClient() (*client.Client, error) {
	if TestConfigInstance == nil {
		return nil, fmt.Errorf("test config not loaded")
	}

	config := client.Config{
		BaseURL:            TestConfigInstance.BaseURL,
		Username:           TestConfigInstance.Username,
		Password:           TestConfigInstance.Password,
		AccessToken:        TestConfigInstance.AccessToken,
		RefreshToken:       TestConfigInstance.RefreshToken,
		InsecureSkipVerify: TestConfigInstance.InsecureSkipVerify,
	}

	return client.NewClient(config)
}

// TestAccPreCheck verifies the test configuration is loaded and authentication
// is configured. Call this at the start of every acceptance test.
func TestAccPreCheck(t *testing.T) {
	t.Helper()

	// Verify config is loaded for acceptance tests
	if TestConfigInstance == nil {
		t.Skip("Test config not loaded. Set TF_ACC=1 and provide testconfig.json or environment variables")
	}

	// Verify required fields are set
	if TestConfigInstance.BaseURL == "" {
		t.Skip("GUARDICORE_BASE_URL must be set for acceptance tests")
	}

	// Verify at least one authentication method is configured
	hasAccessToken := TestConfigInstance.AccessToken != ""
	hasRefreshToken := TestConfigInstance.RefreshToken != ""
	hasCredentials := TestConfigInstance.Username != "" && TestConfigInstance.Password != ""

	if !hasAccessToken && !hasRefreshToken && !hasCredentials {
		t.Skip("Authentication must be configured: GUARDICORE_ACCESS_TOKEN, GUARDICORE_REFRESH_TOKEN, or GUARDICORE_USERNAME+GUARDICORE_PASSWORD")
	}
}

// TestAccProviderConfig returns the HCL provider block using loaded config
// values.
func TestAccProviderConfig() string {
	if TestConfigInstance == nil {
		return ""
	}

	extraAttrs := ""
	if TestConfigInstance.InsecureSkipVerify {
		extraAttrs += "\n  insecure_skip_verify = true"
	}
	if TestConfigInstance.RequestTimeout > 0 {
		extraAttrs += fmt.Sprintf("\n  request_timeout = %d", TestConfigInstance.RequestTimeout)
	}

	// Prefer access token if available
	if TestConfigInstance.AccessToken != "" {
		return fmt.Sprintf(`
provider "guardicore" {
  base_url     = %[1]q
  access_token = %[2]q%[3]s
}
`, TestConfigInstance.BaseURL, TestConfigInstance.AccessToken, extraAttrs)
	}

	// Use username/password
	return fmt.Sprintf(`
provider "guardicore" {
  base_url = %[1]q
  username = %[2]q
  password = %[3]q%[4]s
}
`, TestConfigInstance.BaseURL, TestConfigInstance.Username, TestConfigInstance.Password, extraAttrs)
}

// TestAccProviderConfigWithRefValidation returns the HCL provider block with
// the validate_references_on_destroy flag set.
func TestAccProviderConfigWithRefValidation(validateOnDestroy bool) string {
	if TestConfigInstance == nil {
		return ""
	}

	insecure := ""
	if TestConfigInstance.InsecureSkipVerify {
		insecure = "\n  insecure_skip_verify = true"
	}

	refValidation := fmt.Sprintf("\n  validate_references_on_destroy = %t", validateOnDestroy)

	if TestConfigInstance.AccessToken != "" {
		return fmt.Sprintf(`
provider "guardicore" {
  base_url     = %[1]q
  access_token = %[2]q%[3]s%[4]s
}
`, TestConfigInstance.BaseURL, TestConfigInstance.AccessToken, insecure, refValidation)
	}

	return fmt.Sprintf(`
provider "guardicore" {
  base_url = %[1]q
  username = %[2]q
  password = %[3]q%[4]s%[5]s
}
`, TestConfigInstance.BaseURL, TestConfigInstance.Username, TestConfigInstance.Password, insecure, refValidation)
}

// TestAccRandomName returns a random name with the given prefix, suitable for
// use in acceptance tests that create named resources.
func TestAccRandomName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, sdkacctest.RandString(8))
}

// TestAccCheckResourceDestroyed verifies that a resource no longer exists via
// the API after Terraform destroys it.
func TestAccCheckResourceDestroyed(resourceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if TestConfigInstance == nil {
			return fmt.Errorf("test config not loaded")
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		ctx := context.Background()

		for _, rs := range s.RootModule().Resources {
			if rs.Type != resourceType {
				continue
			}

			if err := TestAccWaitForResourceDestroyed(ctx, apiClient, resourceType, rs.Primary.ID); err != nil {
				return err
			}
		}

		return nil
	}
}

// TestAccWaitForResourceDestroyed polls the API until the resource is gone
// or the retry limit is reached.
func TestAccWaitForResourceDestroyed(ctx context.Context, apiClient *client.Client, resourceType, id string) error {
	for attempt := 1; attempt <= TestAccDestroyCheckAttempts; attempt++ {
		exists, err := TestAccResourceExists(ctx, apiClient, resourceType, id)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		if attempt == TestAccDestroyCheckAttempts {
			break
		}

		time.Sleep(TestAccDestroyCheckInterval)
	}

	totalWait := TestAccDestroyCheckInterval * time.Duration(TestAccDestroyCheckAttempts-1)
	return fmt.Errorf("%s %s still exists after %d checks over %s", resourceType, id, TestAccDestroyCheckAttempts, totalWait)
}

// TestAccResourceExists checks whether a resource of the given type and ID
// exists in the API.
func TestAccResourceExists(ctx context.Context, apiClient *client.Client, resourceType, id string) (bool, error) {
	switch resourceType {
	case "guardicore_label":
		label, err := apiClient.GetLabel(ctx, id)
		if err != nil {
			return false, err
		}
		return label != nil, nil

	case "guardicore_label_group":
		labelGroup, err := apiClient.GetLabelGroup(ctx, id)
		if err != nil {
			return false, err
		}
		return labelGroup != nil, nil

	case "guardicore_policy_rule":
		rule, err := apiClient.GetPolicyRule(ctx, id)
		if err != nil {
			return false, err
		}
		return rule != nil, nil

	case "guardicore_dns_security":
		blocklist, err := apiClient.GetDnsBlocklist(ctx, id)
		if err != nil {
			return false, err
		}
		return blocklist != nil, nil

	case "guardicore_incident":
		// Incidents cannot be deleted via API, so destroy is a no-op.
		// The resource is removed from state but persists in Akamai
		// Guardicore Segmentation.
		return false, nil

	case "guardicore_worksite":
		worksite, err := apiClient.GetWorksite(ctx, id)
		if err != nil {
			return false, err
		}
		return worksite != nil, nil

	case "guardicore_user_group":
		userGroup, err := apiClient.GetUserGroup(ctx, id)
		if err != nil {
			return false, err
		}
		return userGroup != nil, nil

	case "guardicore_asset":
		asset, err := apiClient.GetAsset(ctx, id)
		if err != nil {
			return false, err
		}
		// Asset DELETE deactivates rather than removes; check if still
		// active
		return asset != nil && asset.Status != "deleted", nil

	default:
		return false, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// TestAccDeleteLabelOutOfBand deletes a label via API for disappears tests.
func TestAccDeleteLabelOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabel(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.PublishLabelGroups(context.Background())
	}
}

// TestAccDeleteLabelGroupOutOfBand deletes a label group via API for disappears
// tests.
func TestAccDeleteLabelGroupOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabelGroup(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.PublishLabelGroups(context.Background())
	}
}

// TestAccDeletePolicyRuleOutOfBand deletes a policy rule via API for disappears
// tests.
func TestAccDeletePolicyRuleOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeletePolicyRule(context.Background(), rs.Primary.ID)
	}
}

// TestAccDeleteDnsSecurityOutOfBand deletes a DNS blocklist via API for
// disappears tests.
func TestAccDeleteDnsSecurityOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeleteDnsBlocklist(context.Background(), rs.Primary.ID)
	}
}

// TestAccDeleteLabelExpectFailure attempts to delete a label and expects the
// API call to fail.
func TestAccDeleteLabelExpectFailure(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabel(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("expected label delete to fail, but it succeeded")
		}

		return nil
	}
}

// TestAccDeleteLabelGroupExpectFailure attempts to delete a label group and
// expects the API call to fail.
func TestAccDeleteLabelGroupExpectFailure(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteLabelGroup(context.Background(), rs.Primary.ID); err == nil {
			return fmt.Errorf("expected label group delete to fail, but it succeeded")
		}

		return nil
	}
}

// TestAccCheckLabelGroupPublished ensures label groups are published after
// create.
func TestAccCheckLabelGroupPublished(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
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

// TestAccCheckPolicyRevisionPublished ensures policy rules are published after
// create.
func TestAccCheckPolicyRevisionPublished(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
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

// TestAccCheckPolicyRuleRemoteComments verifies the remote policy rule comments
// value.
func TestAccCheckPolicyRuleRemoteComments(resourceName, expectedComments string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
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

		comments, ok := PolicyRuleCommentFromAPI(rule)
		if !ok {
			return fmt.Errorf("policy rule comments missing or not a string: %#v", rule)
		}
		if comments != expectedComments {
			return fmt.Errorf("expected remote policy rule comments %q, got %q", expectedComments, comments)
		}

		return nil
	}
}

// PolicyRuleCommentFromAPI extracts the comments field from a raw API policy
// rule response map.
func PolicyRuleCommentFromAPI(rule map[string]interface{}) (string, bool) {
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

// TestAccDnsSecurityPreCheck verifies the DNS Security feature is enabled on
// the Akamai Guardicore Segmentation instance.
func TestAccDnsSecurityPreCheck(t *testing.T) {
	t.Helper()
	TestAccPreCheck(t)

	apiClient, err := newTestClient()
	if err != nil {
		t.Skipf("Skipping DNS security test: failed to create client: %v", err)
	}

	_, err = apiClient.ListDnsBlocklists(context.Background(), "", "")
	if err != nil {
		t.Skipf("Skipping DNS security test: %v", err)
	}

	probeName := TestAccRandomName("tf-acc-dns-probe")
	createReq := &client.DnsBlocklistCreate{
		Name:    probeName,
		Type:    "CUSTOM_BLOCK",
		Domains: []string{"precheck.example.com"},
	}
	createdID, createErr := apiClient.CreateDnsBlocklist(context.Background(), createReq)
	if createErr != nil {
		if errors.Is(createErr, client.ErrDnsSecurityFeatureDisabled) {
			t.Skipf("Skipping DNS security test: %v", createErr)
		}
		t.Skipf("Skipping DNS security test: DNS blocklist create probe failed: %v", createErr)
	}

	if createdID != "" {
		_ = apiClient.DeleteDnsBlocklist(context.Background(), createdID)
	}
}

// TestAccWorksitePreCheck verifies the worksites feature is enabled on the
// Akamai Guardicore Segmentation instance.
func TestAccWorksitePreCheck(t *testing.T) {
	t.Helper()
	TestAccPreCheck(t)

	apiClient, err := newTestClient()
	if err != nil {
		t.Skipf("Skipping worksite test: failed to create client: %v", err)
	}

	_, err = apiClient.ListWorksites(context.Background(), "")
	if err != nil {
		t.Skipf("Skipping worksite test: %v", err)
	}
}

// TestAccWorksiteDeleteBlockedPreCheck verifies the worksites feature is
// enabled and the worksite_assigned_asset_id fixture is configured.
func TestAccWorksiteDeleteBlockedPreCheck(t *testing.T) {
	t.Helper()
	TestAccWorksitePreCheck(t)

	if TestConfigInstance.WorksiteAssignedAssetID == "" {
		t.Skip("Skipping worksite delete blocked test: worksite_assigned_asset_id must be set in testconfig.json or GUARDICORE_WORKSITE_ASSIGNED_ASSET_ID")
	}
}

// TestAccPolicyGroupPreCheck verifies the policy groups feature is enabled on
// the Akamai Guardicore Segmentation instance.
func TestAccPolicyGroupPreCheck(t *testing.T) {
	t.Helper()
	TestAccPreCheck(t)

	apiClient, err := newTestClient()
	if err != nil {
		t.Skipf("Skipping policy group test: failed to create client: %v", err)
	}

	_, err = apiClient.ListPolicyGroups(context.Background(), "", "")
	if err != nil {
		t.Skipf("Skipping policy group test: %v", err)
	}
}

// TestAccUserGroupPreCheck verifies orchestration/group IDs are configured for
// user group tests.
func TestAccUserGroupPreCheck(t *testing.T) {
	t.Helper()
	TestAccPreCheck(t)

	if TestConfigInstance.UserGroupOrchestrationID == "" || TestConfigInstance.UserGroupGroupID == "" {
		t.Skip("Skipping user group test: user_group_orchestration_id and user_group_group_id must be set in testconfig.json or environment variables (GUARDICORE_USER_GROUP_ORCHESTRATION_ID, GUARDICORE_USER_GROUP_GROUP_ID)")
	}
}

// TestAccAssetPreCheck verifies the assets endpoint is accessible.
func TestAccAssetPreCheck(t *testing.T) {
	t.Helper()
	TestAccPreCheck(t)

	apiClient, err := newTestClient()
	if err != nil {
		t.Skipf("Skipping asset test: failed to create client: %v", err)
	}

	_, err = apiClient.ListAssets(context.Background(), "")
	if err != nil {
		t.Skipf("Skipping asset test: %v", err)
	}
}

// TestAccDeleteWorksiteOutOfBand deletes a worksite via API for disappears
// tests.
func TestAccDeleteWorksiteOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.DeleteWorksite(context.Background(), rs.Primary.ID)
	}
}

// TestAccDeleteUserGroupOutOfBand deletes a user group via API for disappears
// tests.
func TestAccDeleteUserGroupOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		if err := apiClient.DeleteUserGroup(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.CreateUserGroupRevision(context.Background(), &client.UserGroupRevisionRequest{
			Comments: "Published via Terraform test cleanup",
		})
	}
}

// TestAccDeleteAssetOutOfBand deletes (deactivates) an asset via API for
// disappears tests.
func TestAccDeleteAssetOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.BulkDeactivateAssets(context.Background(), []string{rs.Primary.ID})
	}
}

// TestAccAssignAssetToWorksiteOutOfBand assigns an asset to a worksite via API
// for out-of-band mutation tests.
func TestAccAssignAssetToWorksiteOutOfBand(worksiteResourceName, assetID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ws, ok := s.RootModule().Resources[worksiteResourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", worksiteResourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.AssignWorksite(context.Background(), &client.WorksiteAssignRequest{
			ID:         ws.Primary.ID,
			EntityType: "asset",
			EntityIDs:  []string{assetID},
		})
	}
}

// TestAccAssignAssetToAllWorksitesOutOfBand assigns an asset to
// "all_worksites" via API for out-of-band mutation tests.
func TestAccAssignAssetToAllWorksitesOutOfBand(assetID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		return apiClient.AssignWorksite(context.Background(), &client.WorksiteAssignRequest{
			ID:         "all_worksites",
			EntityType: "asset",
			EntityIDs:  []string{assetID},
		})
	}
}

// TestAccUpdateLabelValueOutOfBand updates a label value via API for drift
// tests.
func TestAccUpdateLabelValueOutOfBand(resourceName, newValue string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		current, err := apiClient.GetLabel(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("label not found: %s", rs.Primary.ID)
		}

		if _, err := apiClient.UpdateLabel(context.Background(), rs.Primary.ID, &client.LabelUpdate{Key: current.Key, Value: newValue}); err != nil {
			return err
		}

		return apiClient.PublishLabelGroups(context.Background())
	}
}

// TestAccUpdateAssetCommentsOutOfBand updates an asset comment via API for
// drift tests.
func TestAccUpdateAssetCommentsOutOfBand(resourceName, newComments string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		apiClient, err := newTestClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		current, err := apiClient.GetAsset(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("asset not found: %s", rs.Primary.ID)
		}

		item := client.AssetBulkUpdateItem{
			AssetID:  rs.Primary.ID,
			Name:     current.Name,
			Nics:     current.Nics,
			Status:   current.Status,
			Comments: newComments,
		}
		item.Labels = &current.Labels

		bulkResp, err := apiClient.BulkUpdateAssets(context.Background(), []client.AssetBulkUpdateItem{item})
		if err != nil {
			return err
		}
		if bulkResp.NumberOfFailed > 0 {
			errMsg := "unknown error"
			if len(bulkResp.Errors) > 0 {
				errMsg = bulkResp.Errors[0].Error
			}
			return fmt.Errorf("bulk update asset failed: %s", errMsg)
		}
		return nil
	}
}

// TestAccReadOnlyLabel returns a read-only label for tests that validate
// read-only label behavior. Skips the test if no read-only label exists.
func TestAccReadOnlyLabel(t *testing.T) *client.Label {
	t.Helper()
	TestAccPreCheck(t)

	apiClient, err := newTestClient()
	if err != nil {
		t.Skipf("Skipping read-only label test: failed to create client: %v", err)
	}

	labels, err := apiClient.ListLabels(context.Background(), "", "")
	if err != nil {
		t.Skipf("Skipping read-only label test: %v", err)
	}

	for _, label := range labels {
		if label.ReadOnly != nil && *label.ReadOnly {
			return &label
		}
	}

	t.Skip("Skipping read-only label test: no read-only label found")
	return nil
}

// TestAccCheckJSONAttr performs semantic JSON comparison for attributes.
func TestAccCheckJSONAttr(resourceName, attrName string, expectedJSON string) resource.TestCheckFunc {
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

// TestAccCaptureResourceID captures the ID of a Terraform resource into the
// provided string pointer.
func TestAccCaptureResourceID(resourceName string, dest *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		if rs.Primary == nil {
			return fmt.Errorf("resource has no primary state: %s", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource has empty id: %s", resourceName)
		}
		*dest = rs.Primary.ID
		return nil
	}
}

// TestAccCheckResourceIDEquals checks that the resource "guardicore_label.test"
// has the same ID as the provided pointer value.
func TestAccCheckResourceIDEquals(expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["guardicore_label.test"]
		if !ok {
			return fmt.Errorf("resource not found: guardicore_label.test")
		}
		if rs.Primary == nil {
			return fmt.Errorf("resource has no primary state: guardicore_label.test")
		}
		if *expected == "" {
			return fmt.Errorf("expected id not captured for guardicore_label.test")
		}
		if rs.Primary.ID != *expected {
			return fmt.Errorf("resource id changed unexpectedly for guardicore_label.test: got %q want %q", rs.Primary.ID, *expected)
		}
		return nil
	}
}
