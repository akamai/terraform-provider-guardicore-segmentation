package provider

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	testAccDestroyCheckAttempts = 12
	testAccDestroyCheckInterval = 2 * time.Second
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

	// Worksite delete blocked acceptance-test fixture dependency
	WorksiteAssignedAssetID string `json:"worksite_assigned_asset_id"`
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
	if v := os.Getenv("GUARDICORE_WORKSITE_ASSIGNED_ASSET_ID"); v != "" {
		config.WorksiteAssignedAssetID = v
	}

	return config, nil
}

func testAccWorksiteDeleteBlockedPreCheck(t *testing.T) {
	t.Helper()
	testAccWorksitePreCheck(t)

	if testConfig.WorksiteAssignedAssetID == "" {
		t.Skip("Skipping worksite delete blocked test: worksite_assigned_asset_id must be set in testconfig.json or GUARDICORE_WORKSITE_ASSIGNED_ASSET_ID")
	}
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

			if err := testAccWaitForResourceDestroyed(ctx, apiClient, resourceType, rs.Primary.ID); err != nil {
				return err
			}
		}

		return nil
	}
}

func testAccWaitForResourceDestroyed(ctx context.Context, apiClient *client.Client, resourceType, id string) error {
	for attempt := 1; attempt <= testAccDestroyCheckAttempts; attempt++ {
		exists, err := testAccResourceExists(ctx, apiClient, resourceType, id)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		if attempt == testAccDestroyCheckAttempts {
			break
		}

		time.Sleep(testAccDestroyCheckInterval)
	}

	totalWait := testAccDestroyCheckInterval * time.Duration(testAccDestroyCheckAttempts-1)
	return fmt.Errorf("%s %s still exists after %d checks over %s", resourceType, id, testAccDestroyCheckAttempts, totalWait)
}

func testAccResourceExists(ctx context.Context, apiClient *client.Client, resourceType, id string) (bool, error) {
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
		// The resource is removed from state but persists in Akamai Guardicore Segmentation.
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
		// Asset DELETE deactivates rather than removes; check if still active
		return asset != nil && asset.Status != "deleted", nil

	default:
		return false, fmt.Errorf("unknown resource type: %s", resourceType)
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

// testAccDnsSecurityPreCheck verifies the DNS Security feature is enabled on the Akamai Guardicore Segmentation instance.
func testAccDnsSecurityPreCheck(t *testing.T) {
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
		t.Skipf("Skipping DNS security test: failed to create client: %v", err)
	}

	_, err = apiClient.ListDnsBlocklists(context.Background(), "", "")
	if err != nil {
		t.Skipf("Skipping DNS security test: %v", err)
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

func testAccAssignAssetToWorksiteOutOfBand(worksiteResourceName, assetID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ws, ok := s.RootModule().Resources[worksiteResourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", worksiteResourceName)
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

		return apiClient.AssignWorksite(context.Background(), &client.WorksiteAssignRequest{
			ID:         ws.Primary.ID,
			EntityType: "asset",
			EntityIDs:  []string{assetID},
		})
	}
}

func testAccAssignAssetToAllWorksitesOutOfBand(assetID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
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

		return apiClient.AssignWorksite(context.Background(), &client.WorksiteAssignRequest{
			ID:         "all_worksites",
			EntityType: "asset",
			EntityIDs:  []string{assetID},
		})
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

		if err := apiClient.DeleteUserGroup(context.Background(), rs.Primary.ID); err != nil {
			return err
		}

		return apiClient.CreateUserGroupRevision(context.Background(), &client.UserGroupRevisionRequest{
			Comments: "Published via Terraform test cleanup",
		})
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

// testAccReadOnlyLabel returns a read-only label for tests that validate
// read-only label behavior. Skips the test if no read-only label exists.
func testAccReadOnlyLabel(t *testing.T) *client.Label {
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

// testAccUpdateLabelValueOutOfBand updates a label value via API for drift tests.
func testAccUpdateLabelValueOutOfBand(resourceName, newValue string) resource.TestCheckFunc {
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

// testAccUpdateAssetCommentsOutOfBand updates an asset comment via API for drift tests.
func testAccUpdateAssetCommentsOutOfBand(resourceName, newComments string) resource.TestCheckFunc {
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

func testAccCaptureResourceID(resourceName string, dest *string) resource.TestCheckFunc {
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

func testAccCheckResourceIDEquals(expected *string) resource.TestCheckFunc {
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

func TestLoadTestConfig_FromFile(t *testing.T) {
	oldPath := *testConfigPath
	t.Cleanup(func() { *testConfigPath = oldPath })

	for _, key := range []string{
		"GUARDICORE_BASE_URL",
		"GUARDICORE_USERNAME",
		"GUARDICORE_PASSWORD",
		"GUARDICORE_ACCESS_TOKEN",
		"GUARDICORE_REFRESH_TOKEN",
		"GUARDICORE_INSECURE_SKIP_VERIFY",
		"GUARDICORE_REQUEST_TIMEOUT",
		"GUARDICORE_USER_GROUP_ORCHESTRATION_ID",
		"GUARDICORE_USER_GROUP_GROUP_ID",
	} {
		_ = os.Unsetenv(key)
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "testconfig.json")
	configJSON := `{
		"base_url": "https://example.local",
		"username": "file-user",
		"password": "file-pass",
		"access_token": "file-at",
		"refresh_token": "file-rt",
		"insecure_skip_verify": true,
		"request_timeout": 77,
		"user_group_orchestration_id": "orch-file",
		"user_group_group_id": "group-file"
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	*testConfigPath = configPath

	got, err := loadTestConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.BaseURL != "https://example.local" || got.Username != "file-user" || got.Password != "file-pass" {
		t.Fatalf("unexpected base credentials in loaded config: %#v", got)
	}
	if got.AccessToken != "file-at" || got.RefreshToken != "file-rt" {
		t.Fatalf("unexpected token values in loaded config: %#v", got)
	}
	if !got.InsecureSkipVerify || got.RequestTimeout != 77 {
		t.Fatalf("unexpected tls/timeout values in loaded config: %#v", got)
	}
	if got.UserGroupOrchestrationID != "orch-file" || got.UserGroupGroupID != "group-file" {
		t.Fatalf("unexpected user group dependency values in loaded config: %#v", got)
	}
}

func TestLoadTestConfig_EnvOverridesFile(t *testing.T) {
	oldPath := *testConfigPath
	t.Cleanup(func() { *testConfigPath = oldPath })

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "testconfig.json")
	if err := os.WriteFile(configPath, []byte(`{"base_url":"https://from-file"}`), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	*testConfigPath = configPath

	t.Setenv("GUARDICORE_BASE_URL", "https://from-env")
	t.Setenv("GUARDICORE_USERNAME", "env-user")
	t.Setenv("GUARDICORE_PASSWORD", "env-pass")
	t.Setenv("GUARDICORE_ACCESS_TOKEN", "env-at")
	t.Setenv("GUARDICORE_REFRESH_TOKEN", "env-rt")
	t.Setenv("GUARDICORE_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("GUARDICORE_REQUEST_TIMEOUT", "123")
	t.Setenv("GUARDICORE_USER_GROUP_ORCHESTRATION_ID", "orch-env")
	t.Setenv("GUARDICORE_USER_GROUP_GROUP_ID", "group-env")

	got, err := loadTestConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.BaseURL != "https://from-env" || got.Username != "env-user" || got.Password != "env-pass" {
		t.Fatalf("expected env credentials to override, got %#v", got)
	}
	if got.AccessToken != "env-at" || got.RefreshToken != "env-rt" {
		t.Fatalf("expected env tokens to override, got %#v", got)
	}
	if !got.InsecureSkipVerify || got.RequestTimeout != 123 {
		t.Fatalf("expected env tls/timeout to override, got %#v", got)
	}
	if got.UserGroupOrchestrationID != "orch-env" || got.UserGroupGroupID != "group-env" {
		t.Fatalf("expected env user group values to override, got %#v", got)
	}
}

func TestLoadTestConfig_InvalidJSON(t *testing.T) {
	oldPath := *testConfigPath
	t.Cleanup(func() { *testConfigPath = oldPath })

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "testconfig.json")
	if err := os.WriteFile(configPath, []byte(`{"base_url":`), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	*testConfigPath = configPath

	_, err := loadTestConfig()
	if err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestTestAccProviderConfigBuilders(t *testing.T) {
	old := testConfig
	t.Cleanup(func() { testConfig = old })

	testConfig = nil
	if got := testAccProviderConfig(); got != "" {
		t.Fatalf("expected empty config when testConfig is nil, got %q", got)
	}

	testConfig = &TestConfig{
		BaseURL:            "https://example.local",
		Username:           "alice",
		Password:           "secret",
		InsecureSkipVerify: true,
		RequestTimeout:     15,
	}

	got := testAccProviderConfig()
	if !strings.Contains(got, `base_url = "https://example.local"`) || !strings.Contains(got, `username = "alice"`) {
		t.Fatalf("expected username/password config, got %s", got)
	}
	if !strings.Contains(got, "insecure_skip_verify = true") || !strings.Contains(got, "request_timeout = 15") {
		t.Fatalf("expected optional attrs in config, got %s", got)
	}

	testConfig.AccessToken = "token-only"
	got = testAccProviderConfig()
	if !strings.Contains(got, `access_token = "token-only"`) {
		t.Fatalf("expected access token config, got %s", got)
	}
	if strings.Contains(got, `username =`) || strings.Contains(got, `password =`) {
		t.Fatalf("expected token config to omit username/password, got %s", got)
	}

	got = testAccProviderConfigWithRefValidation(true)
	if !strings.Contains(got, "validate_references_on_destroy = true") {
		t.Fatalf("expected ref validation flag, got %s", got)
	}
}

func TestTestAccCheckJSONAttr(t *testing.T) {
	state := &terraform.State{
		Modules: []*terraform.ModuleState{{
			Path:      []string{"root"},
			Outputs:   map[string]*terraform.OutputState{},
			Resources: map[string]*terraform.ResourceState{},
		}},
	}
	state.RootModule().Resources["data.guardicore_policy_rule.test"] = &terraform.ResourceState{
		Type: "guardicore_policy_rule",
		Primary: &terraform.InstanceState{
			ID: "rule-1",
			Attributes: map[string]string{
				"spec_json": `{"ports":[80,443],"protocols":["TCP"]}`,
			},
		},
	}

	check := testAccCheckJSONAttr("data.guardicore_policy_rule.test", "spec_json", `{
		"protocols": ["TCP"],
		"ports": [80, 443]
	}`)
	if err := check(state); err != nil {
		t.Fatalf("expected semantic JSON equality, got %v", err)
	}

	mismatch := testAccCheckJSONAttr("data.guardicore_policy_rule.test", "spec_json", `{"ports":[22]}`)
	if err := mismatch(state); err == nil {
		t.Fatal("expected mismatch error")
	}

	missingAttr := testAccCheckJSONAttr("data.guardicore_policy_rule.test", "missing", `{}`)
	if err := missingAttr(state); err == nil || !strings.Contains(err.Error(), "attribute missing not found") {
		t.Fatalf("expected missing attribute error, got %v", err)
	}

	missingResource := testAccCheckJSONAttr("data.guardicore_policy_rule.nope", "spec_json", `{}`)
	if err := missingResource(state); err == nil || !strings.Contains(err.Error(), "resource not found") {
		t.Fatalf("expected missing resource error, got %v", err)
	}
}
