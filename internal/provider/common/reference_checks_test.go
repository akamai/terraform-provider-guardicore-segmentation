package common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

// Verify that *client.Client satisfies the ReferenceChecker interface.
var _ ReferenceChecker = (*client.Client)(nil)

// mockReferenceChecker implements ReferenceChecker for testing.
type mockReferenceChecker struct {
	labels         map[string]*client.Label
	listLabels     []client.Label
	labelGroups    map[string]*client.LabelGroup
	policyGroups   map[string]*client.PolicyGroup
	userGroups     map[string]*client.UserGroup
	assets         map[string]*client.Asset
	worksites      map[string]*client.Worksite
	getLabelCalls  map[string]int
	listLabelCalls map[string]int
	labelErr       error
	groupErr       error
	policyGroupErr error
	userGroupErr   error
	assetErr       error
	worksiteErr    error
}

func (m *mockReferenceChecker) GetLabel(_ context.Context, id string) (*client.Label, error) {
	if m.getLabelCalls != nil {
		m.getLabelCalls[id]++
	}
	if m.labelErr != nil {
		return nil, m.labelErr
	}
	label, ok := m.labels[id]
	if !ok {
		return nil, nil
	}
	return label, nil
}

func (m *mockReferenceChecker) ListLabels(_ context.Context, key, value string) ([]client.Label, error) {
	if m.labelErr != nil {
		return nil, m.labelErr
	}
	if m.listLabelCalls != nil {
		m.listLabelCalls[key+"\x00"+value]++
	}
	source := m.listLabels
	if source == nil {
		source = make([]client.Label, 0, len(m.labels))
		for _, label := range m.labels {
			source = append(source, *label)
		}
	}

	result := make([]client.Label, 0, len(source))
	for _, label := range source {
		if key != "" && label.Key != key {
			continue
		}
		if value != "" && label.Value != value {
			continue
		}
		result = append(result, label)
	}
	return result, nil
}

func (m *mockReferenceChecker) GetLabelGroup(_ context.Context, id string) (*client.LabelGroup, error) {
	if m.groupErr != nil {
		return nil, m.groupErr
	}
	group, ok := m.labelGroups[id]
	if !ok {
		return nil, nil
	}
	return group, nil
}

func (m *mockReferenceChecker) GetPolicyGroup(_ context.Context, id string) (*client.PolicyGroup, error) {
	if m.policyGroupErr != nil {
		return nil, m.policyGroupErr
	}
	pg, ok := m.policyGroups[id]
	if !ok {
		return nil, nil
	}
	return pg, nil
}

func (m *mockReferenceChecker) GetUserGroup(_ context.Context, id string) (*client.UserGroup, error) {
	if m.userGroupErr != nil {
		return nil, m.userGroupErr
	}
	ug, ok := m.userGroups[id]
	if !ok {
		return nil, nil
	}
	return ug, nil
}

func (m *mockReferenceChecker) GetAsset(_ context.Context, id string) (*client.Asset, error) {
	if m.assetErr != nil {
		return nil, m.assetErr
	}
	asset, ok := m.assets[id]
	if !ok {
		return nil, nil
	}
	return asset, nil
}

func (m *mockReferenceChecker) GetWorksite(_ context.Context, id string) (*client.Worksite, error) {
	if m.worksiteErr != nil {
		return nil, m.worksiteErr
	}
	ws, ok := m.worksites[id]
	if !ok {
		return nil, nil
	}
	return ws, nil
}

func TestValidateLabelExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1", Key: "env", Value: "prod"},
		},
	}

	diags := ValidateLabelExists(context.Background(), checker, "label-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateLabelExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	diags := ValidateLabelExists(context.Background(), checker, "nonexistent-id", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label")
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}

	detail := diags[0].Detail()
	if detail == "" {
		t.Fatal("expected error detail to contain information")
	}
}

func TestValidateLabelExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateLabelExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateLabelExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		labelErr: fmt.Errorf("connection refused"),
	}

	diags := ValidateLabelExists(context.Background(), checker, "label-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

func TestValidateLabelGroupExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{
			"group-1": {ID: "group-1", Key: "env", Value: "production"},
		},
	}

	diags := ValidateLabelGroupExists(context.Background(), checker, "group-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateLabelGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{},
	}

	diags := ValidateLabelGroupExists(context.Background(), checker, "nonexistent-group", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label group")
	}
}

func TestValidateLabelsInJSON_ValidIDs(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"id-1": {ID: "id-1"},
			"id-2": {ID: "id-2"},
		},
	}

	jsonStr := `{"or_labels":[{"and_labels":["id-1","id-2"]}]}`
	diags := ValidateLabelsInJSON(context.Background(), checker, jsonStr, "include_labels")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateLabelsInJSON_InvalidID(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"id-1": {ID: "id-1"},
		},
	}

	jsonStr := `{"or_labels":[{"and_labels":["id-1","missing-id"]}]}`
	diags := ValidateLabelsInJSON(context.Background(), checker, jsonStr, "include_labels")
	if !diags.HasError() {
		t.Fatal("expected error for missing label ID")
	}

	errorCount := 0
	for _, d := range diags {
		if d.Severity() == 1 {
			errorCount++
		}
	}
	if errorCount != 1 {
		t.Fatalf("expected 1 error, got %d", errorCount)
	}
}

func TestValidateLabelsInJSON_EmptyString(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateLabelsInJSON(context.Background(), checker, "", "include_labels")
	if diags.HasError() {
		t.Fatal("expected no errors for empty string")
	}
}

func TestValidateLabelsInJSON_InvalidJSON(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateLabelsInJSON(context.Background(), checker, "not-json", "include_labels")
	if !diags.HasError() {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidatePolicyRuleRefs_InvalidJSON(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidatePolicyRuleRefs(context.Background(), checker, "not-json")
	if !diags.HasError() {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateLabelsInJSON_MultipleOrLabels(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"id-1": {ID: "id-1"},
		},
	}

	jsonStr := `{"or_labels":[{"and_labels":["id-1"]},{"and_labels":["missing-1","missing-2"]}]}`
	diags := ValidateLabelsInJSON(context.Background(), checker, jsonStr, "exclude_labels")
	if !diags.HasError() {
		t.Fatal("expected errors for missing label IDs")
	}

	errorCount := 0
	for _, d := range diags {
		if d.Severity() == 1 {
			errorCount++
		}
	}
	if errorCount != 2 {
		t.Fatalf("expected 2 errors, got %d", errorCount)
	}
}

func TestValidatePolicyRuleRefs_ValidGroups(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{
			"group-1": {ID: "group-1"},
			"group-2": {ID: "group-2"},
		},
	}

	specJSON := `{"source":{"label_group_ids":["group-1"]},"destination":{"label_group_ids":["group-2"]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidGroup(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{
			"group-1": {ID: "group-1"},
		},
	}

	specJSON := `{"source":{"label_group_ids":["group-1"]},"destination":{"label_group_ids":["nonexistent"]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label group")
	}
}

func TestValidatePolicyRuleRefs_AnySource(t *testing.T) {
	checker := &mockReferenceChecker{}

	specJSON := `{"source":{"any":true},"destination":{"any":true}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatal("expected no errors for any:true source/destination")
	}
}

func TestValidatePolicyRuleRefs_EmptySpec(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidatePolicyRuleRefs(context.Background(), checker, "")
	if diags.HasError() {
		t.Fatal("expected no errors for empty spec")
	}
}

func TestValidatePolicyRuleRefs_LabelsExpression(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	specJSON := `{"source":{"labels":{"or_labels":[{"and_labels":["nonexistent-label"]}]}}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label referenced in labels expression")
	}
}

func TestValidateUserGroupExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{
			"ug-1": {ID: "ug-1", Title: "Dev Team"},
		},
	}

	diags := ValidateUserGroupExists(context.Background(), checker, "ug-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateUserGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{},
	}

	diags := ValidateUserGroupExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent user group")
	}
}

func TestValidateUserGroupExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateUserGroupExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateUserGroupExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroupErr: fmt.Errorf("connection refused"),
	}

	diags := ValidateUserGroupExists(context.Background(), checker, "ug-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

func TestValidateAssetExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{
			"asset-1": {ID: "asset-1", Name: "web-server"},
		},
	}

	diags := ValidateAssetExists(context.Background(), checker, "asset-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateAssetExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{},
	}

	diags := ValidateAssetExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent asset")
	}
}

func TestValidateAssetExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateAssetExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateAssetExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		assetErr: fmt.Errorf("connection refused"),
	}

	diags := ValidateAssetExists(context.Background(), checker, "asset-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

func TestValidatePolicyRuleRefs_ValidUserGroups(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{
			"ug-1": {ID: "ug-1"},
		},
	}

	specJSON := `{"source":{"user_group_ids":["ug-1"]},"destination":{"any":true}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidUserGroup(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{},
	}

	specJSON := `{"source":{"user_group_ids":["nonexistent"]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent user group")
	}
}

func TestValidatePolicyRuleRefs_ValidAssets(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{
			"asset-1": {ID: "asset-1"},
		},
	}

	specJSON := `{"source":{"asset_ids":["asset-1"]},"destination":{"any":true}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidAsset(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{},
	}

	specJSON := `{"source":{"asset_ids":["nonexistent"]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent asset")
	}
}

func TestValidatePolicyRuleRefs_MixedRefs(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{
			"group-1": {ID: "group-1"},
		},
		userGroups: map[string]*client.UserGroup{
			"ug-1": {ID: "ug-1"},
		},
		assets: map[string]*client.Asset{
			"asset-1": {ID: "asset-1"},
		},
	}

	specJSON := `{"source":{"label_group_ids":["group-1"],"user_group_ids":["ug-1"]},"destination":{"asset_ids":["asset-1"]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors for valid mixed refs, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidEndpointReferenceShape(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{
			"group-1": {ID: "group-1"},
		},
	}

	specJSON := `{"source":{"label_group_ids":["group-1"],"user_group_ids":[{"id":"ug-1","name":"Local Administrators"}]}}`
	diags := ValidatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-string user_group_ids entry")
	}

	found := false
	for _, d := range diags {
		if strings.Contains(d.Detail(), "policy_rule.source.user_group_ids[0]") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic to include exact path for invalid ref shape, got: %v", diags)
	}
}

func TestValidateWorksiteExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		worksites: map[string]*client.Worksite{
			"ws-1": {ID: "ws-1", Name: "Headquarters"},
		},
	}

	diags := ValidateWorksiteExists(context.Background(), checker, "ws-1")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateWorksiteExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		worksites: map[string]*client.Worksite{},
	}

	diags := ValidateWorksiteExists(context.Background(), checker, "nonexistent")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent worksite")
	}
}

func TestValidateWorksiteExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidateWorksiteExists(context.Background(), checker, "")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateWorksiteExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		worksiteErr: fmt.Errorf("connection refused"),
	}

	diags := ValidateWorksiteExists(context.Background(), checker, "ws-1")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

func TestValidatePolicyGroupExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroups: map[string]*client.PolicyGroup{
			"pg-1": {ID: "pg-1", Name: "Production", Type: "LABEL"},
		},
	}

	diags := ValidatePolicyGroupExists(context.Background(), checker, "pg-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroups: map[string]*client.PolicyGroup{},
	}

	diags := ValidatePolicyGroupExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent policy group")
	}
}

func TestValidatePolicyGroupExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidatePolicyGroupExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidatePolicyGroupExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroupErr: fmt.Errorf("connection refused"),
	}

	diags := ValidatePolicyGroupExists(context.Background(), checker, "pg-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

func TestValidatePolicyGroupLabelRefs_ValidLabels(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1"},
			"label-2": {ID: "label-2"},
		},
	}

	membersJSON := `[["label-1"], ["label-2"]]`
	diags := ValidatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyGroupLabelRefs_InvalidLabel(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	membersJSON := `[["nonexistent"]]`
	diags := ValidatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label")
	}
}

func TestValidatePolicyGroupLabelRefs_EmptyString(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := ValidatePolicyGroupLabelRefs(context.Background(), checker, "", "members_json")
	if diags.HasError() {
		t.Fatal("expected no errors for empty string")
	}
}

func TestValidatePolicyGroupLabelRefs_InvalidJSON(t *testing.T) {
	checker := &mockReferenceChecker{}

	membersJSON := `{invalid json}`
	diags := ValidatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if !diags.HasError() {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidatePolicyGroupLabelRefs_MultipleORGroups(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1"},
			"label-2": {ID: "label-2"},
			"label-3": {ID: "label-3"},
		},
	}

	membersJSON := `[["label-1", "label-2"], ["label-3"]]`
	diags := ValidatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if diags.HasError() {
		t.Fatalf("expected no errors for multiple OR groups, got: %v", diags)
	}
}

func TestCountPolicyGroupMembers_LABELType(t *testing.T) {
	membersJSON := `[["label-1", "label-2"], ["label-3"]]`
	count, err := CountPolicyGroupMembers(membersJSON, "LABEL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_FQDNType(t *testing.T) {
	membersJSON := `["example.com", "*.test.com", "api.example.org"]`
	count, err := CountPolicyGroupMembers(membersJSON, "FQDN")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_IPAddressType(t *testing.T) {
	membersJSON := `[{"subnet": "10.0.0.0/8"}, {"range": {"start": "192.168.1.1", "end": "192.168.1.254"}}]`
	count, err := CountPolicyGroupMembers(membersJSON, "IP_ADDRESS")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_EmptyString(t *testing.T) {
	count, err := CountPolicyGroupMembers("", "FQDN")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_InvalidJSON(t *testing.T) {
	_, err := CountPolicyGroupMembers("{invalid", "FQDN")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCountPolicyGroupMembers_UnknownType(t *testing.T) {
	_, err := CountPolicyGroupMembers(`["test"]`, "UNKNOWN")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
