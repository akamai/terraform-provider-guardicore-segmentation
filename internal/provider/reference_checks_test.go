package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mockReferenceChecker implements ReferenceChecker for testing.
type mockReferenceChecker struct {
	labels         map[string]*client.Label
	labelGroups    map[string]*client.LabelGroup
	policyGroups   map[string]*client.PolicyGroup
	userGroups     map[string]*client.UserGroup
	assets         map[string]*client.Asset
	worksites      map[string]*client.Worksite
	labelErr       error
	groupErr       error
	policyGroupErr error
	userGroupErr   error
	assetErr       error
	worksiteErr    error
}

func (m *mockReferenceChecker) GetLabel(_ context.Context, id string) (*client.Label, error) {
	if m.labelErr != nil {
		return nil, m.labelErr
	}
	label, ok := m.labels[id]
	if !ok {
		return nil, nil
	}
	return label, nil
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

	diags := validateLabelExists(context.Background(), checker, "label-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateLabelExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	diags := validateLabelExists(context.Background(), checker, "nonexistent-id", "test_field")
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

	diags := validateLabelExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateLabelExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		labelErr: fmt.Errorf("connection refused"),
	}

	diags := validateLabelExists(context.Background(), checker, "label-1", "test_field")
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

	diags := validateLabelGroupExists(context.Background(), checker, "group-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateLabelGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		labelGroups: map[string]*client.LabelGroup{},
	}

	diags := validateLabelGroupExists(context.Background(), checker, "nonexistent-group", "test_field")
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
	diags := validateLabelsInJSON(context.Background(), checker, jsonStr, "include_labels")
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
	diags := validateLabelsInJSON(context.Background(), checker, jsonStr, "include_labels")
	if !diags.HasError() {
		t.Fatal("expected error for missing label ID")
	}

	// Should have exactly 1 error (only "missing-id" is invalid)
	errorCount := 0
	for _, d := range diags {
		if d.Severity() == 1 { // Error severity
			errorCount++
		}
	}
	if errorCount != 1 {
		t.Fatalf("expected 1 error, got %d", errorCount)
	}
}

func TestValidateLabelsInJSON_EmptyString(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateLabelsInJSON(context.Background(), checker, "", "include_labels")
	if diags.HasError() {
		t.Fatal("expected no errors for empty string")
	}
}

func TestValidateLabelsInJSON_InvalidJSON(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateLabelsInJSON(context.Background(), checker, "not-json", "include_labels")
	// Invalid JSON is handled by the NonEmptyLabelsJSON validator, not this one
	if diags.HasError() {
		t.Fatal("expected no errors for invalid JSON (handled elsewhere)")
	}
}

func TestValidateLabelsInJSON_MultipleOrLabels(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"id-1": {ID: "id-1"},
		},
	}

	jsonStr := `{"or_labels":[{"and_labels":["id-1"]},{"and_labels":["missing-1","missing-2"]}]}`
	diags := validateLabelsInJSON(context.Background(), checker, jsonStr, "exclude_labels")
	if !diags.HasError() {
		t.Fatal("expected errors for missing label IDs")
	}

	// Should have 2 errors (missing-1 and missing-2)
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
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
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
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label group")
	}
}

func TestValidatePolicyRuleRefs_AnySource(t *testing.T) {
	checker := &mockReferenceChecker{}

	// Rules with "any: true" have no label references to validate
	specJSON := `{"source":{"any":true},"destination":{"any":true}}`
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatal("expected no errors for any:true source/destination")
	}
}

func TestValidatePolicyRuleRefs_EmptySpec(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validatePolicyRuleRefs(context.Background(), checker, "")
	if diags.HasError() {
		t.Fatal("expected no errors for empty spec")
	}
}

func TestValidatePolicyRuleRefs_LabelsExpression(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	specJSON := `{"source":{"labels":{"or_labels":[{"and_labels":["nonexistent-label"]}]}}}`
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label referenced in labels expression")
	}
}

func TestValidateAssetLabelRefs_ValidLabels(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1"},
			"label-2": {ID: "label-2"},
		},
	}

	labels := []AssetLabelModel{
		{ID: types.StringValue("label-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		{ID: types.StringValue("label-2"), Key: types.StringValue("app"), Value: types.StringValue("web")},
	}

	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateAssetLabelRefs_InvalidLabel(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1"},
		},
	}

	labels := []AssetLabelModel{
		{ID: types.StringValue("label-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		{ID: types.StringValue("missing-label"), Key: types.StringValue("app"), Value: types.StringValue("web")},
	}

	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if !diags.HasError() {
		t.Fatal("expected error for missing label")
	}
}

func TestValidateAssetLabelRefs_NullID(t *testing.T) {
	checker := &mockReferenceChecker{}

	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}

	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if diags.HasError() {
		t.Fatal("expected no errors for null ID")
	}
}

func TestValidateAssetLabelRefs_UnknownID(t *testing.T) {
	checker := &mockReferenceChecker{}

	labels := []AssetLabelModel{
		{ID: types.StringUnknown(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}

	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if diags.HasError() {
		t.Fatal("expected no errors for unknown ID")
	}
}

func TestValidateAssetLabelRefs_EmptySlice(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateAssetLabelRefs(context.Background(), checker, []AssetLabelModel{})
	if diags.HasError() {
		t.Fatal("expected no errors for empty labels")
	}
}

// TestLabelGroupReferencesLabel tests the helper used in lifecycle protection.
func TestLabelGroupReferencesLabel_Found(t *testing.T) {
	group := &client.LabelGroup{
		ID: "group-1",
		IncludeLabels: &client.OrLabelsRead{
			OrLabels: []client.AndLabelsRead{
				{AndLabels: []client.LabelInGroup{
					{ID: "label-1", Key: "env", Value: "prod"},
					{ID: "label-2", Key: "app", Value: "web"},
				}},
			},
		},
	}

	if !labelGroupReferencesLabel(group, "label-1") {
		t.Fatal("expected to find label-1 in group")
	}
	if !labelGroupReferencesLabel(group, "label-2") {
		t.Fatal("expected to find label-2 in group")
	}
}

func TestLabelGroupReferencesLabel_NotFound(t *testing.T) {
	group := &client.LabelGroup{
		ID: "group-1",
		IncludeLabels: &client.OrLabelsRead{
			OrLabels: []client.AndLabelsRead{
				{AndLabels: []client.LabelInGroup{
					{ID: "label-1", Key: "env", Value: "prod"},
				}},
			},
		},
	}

	if labelGroupReferencesLabel(group, "nonexistent") {
		t.Fatal("should not find nonexistent label in group")
	}
}

func TestLabelGroupReferencesLabel_ExcludeLabels(t *testing.T) {
	group := &client.LabelGroup{
		ID: "group-1",
		ExcludeLabels: &client.OrLabelsRead{
			OrLabels: []client.AndLabelsRead{
				{AndLabels: []client.LabelInGroup{
					{ID: "excluded-label", Key: "env", Value: "dev"},
				}},
			},
		},
	}

	if !labelGroupReferencesLabel(group, "excluded-label") {
		t.Fatal("expected to find label in exclude_labels")
	}
}

func TestLabelGroupReferencesLabel_NilLabels(t *testing.T) {
	group := &client.LabelGroup{
		ID: "group-1",
	}

	if labelGroupReferencesLabel(group, "any-label") {
		t.Fatal("should not find any label in group with nil include/exclude")
	}
}

// TestPolicyRuleReferencesLabelGroup tests the helper used in lifecycle protection.
func TestPolicyRuleReferencesLabelGroup_Found(t *testing.T) {
	rule := map[string]any{
		"id": "rule-1",
		"source": map[string]any{
			"label_group_ids": []any{"group-1", "group-2"},
		},
		"destination": map[string]any{
			"label_group_ids": []any{"group-3"},
		},
	}

	if !policyRuleReferencesLabelGroup(rule, "group-1") {
		t.Fatal("expected to find group-1 in source")
	}
	if !policyRuleReferencesLabelGroup(rule, "group-3") {
		t.Fatal("expected to find group-3 in destination")
	}
}

func TestPolicyRuleReferencesLabelGroup_NotFound(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"label_group_ids": []any{"group-1"},
		},
	}

	if policyRuleReferencesLabelGroup(rule, "nonexistent") {
		t.Fatal("should not find nonexistent group in rule")
	}
}

func TestPolicyRuleReferencesLabelGroup_AnySource(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"any": true,
		},
	}

	if policyRuleReferencesLabelGroup(rule, "any-group") {
		t.Fatal("should not find group in any:true source")
	}
}

// Tests for user group validation

func TestValidateUserGroupExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{
			"ug-1": {ID: "ug-1", Title: "Dev Team"},
		},
	}

	diags := validateUserGroupExists(context.Background(), checker, "ug-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateUserGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{},
	}

	diags := validateUserGroupExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent user group")
	}
}

func TestValidateUserGroupExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateUserGroupExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateUserGroupExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroupErr: fmt.Errorf("connection refused"),
	}

	diags := validateUserGroupExists(context.Background(), checker, "ug-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

// Tests for asset validation

func TestValidateAssetExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{
			"asset-1": {ID: "asset-1", Name: "web-server"},
		},
	}

	diags := validateAssetExists(context.Background(), checker, "asset-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateAssetExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{},
	}

	diags := validateAssetExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent asset")
	}
}

func TestValidateAssetExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateAssetExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateAssetExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		assetErr: fmt.Errorf("connection refused"),
	}

	diags := validateAssetExists(context.Background(), checker, "asset-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

// Tests for policy rule user group and asset refs

func TestValidatePolicyRuleRefs_ValidUserGroups(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{
			"ug-1": {ID: "ug-1"},
		},
	}

	specJSON := `{"source":{"user_group_ids":["ug-1"]},"destination":{"any":true}}`
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidUserGroup(t *testing.T) {
	checker := &mockReferenceChecker{
		userGroups: map[string]*client.UserGroup{},
	}

	specJSON := `{"source":{"user_group_ids":["nonexistent"]}}`
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
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
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyRuleRefs_InvalidAsset(t *testing.T) {
	checker := &mockReferenceChecker{
		assets: map[string]*client.Asset{},
	}

	specJSON := `{"source":{"asset_ids":["nonexistent"]}}`
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
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
	diags := validatePolicyRuleRefs(context.Background(), checker, specJSON)
	if diags.HasError() {
		t.Fatalf("expected no errors for valid mixed refs, got: %v", diags)
	}
}

// Tests for policyRuleReferencesUserGroup helper

func TestPolicyRuleReferencesUserGroup_Found(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"user_group_ids": []any{"ug-1", "ug-2"},
		},
	}

	if !policyRuleReferencesUserGroup(rule, "ug-1") {
		t.Fatal("expected to find ug-1 in source")
	}
}

func TestPolicyRuleReferencesUserGroup_NotFound(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"user_group_ids": []any{"ug-1"},
		},
	}

	if policyRuleReferencesUserGroup(rule, "nonexistent") {
		t.Fatal("should not find nonexistent user group in rule")
	}
}

func TestPolicyRuleReferencesUserGroup_NoUserGroups(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"label_group_ids": []any{"group-1"},
		},
	}

	if policyRuleReferencesUserGroup(rule, "ug-1") {
		t.Fatal("should not find user group when none are referenced")
	}
}

// Tests for worksite validation

func TestValidateWorksiteExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		worksites: map[string]*client.Worksite{
			"ws-1": {ID: "ws-1", Name: "Headquarters"},
		},
	}

	diags := validateWorksiteExists(context.Background(), checker, "ws-1")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidateWorksiteExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		worksites: map[string]*client.Worksite{},
	}

	diags := validateWorksiteExists(context.Background(), checker, "nonexistent")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent worksite")
	}
}

func TestValidateWorksiteExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validateWorksiteExists(context.Background(), checker, "")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidateWorksiteExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		worksiteErr: fmt.Errorf("connection refused"),
	}

	diags := validateWorksiteExists(context.Background(), checker, "ws-1")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

// Tests for policy group validation

func TestValidatePolicyGroupExists_Found(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroups: map[string]*client.PolicyGroup{
			"pg-1": {ID: "pg-1", Name: "Production", Type: "LABEL"},
		},
	}

	diags := validatePolicyGroupExists(context.Background(), checker, "pg-1", "test_field")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyGroupExists_NotFound(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroups: map[string]*client.PolicyGroup{},
	}

	diags := validatePolicyGroupExists(context.Background(), checker, "nonexistent", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent policy group")
	}
}

func TestValidatePolicyGroupExists_EmptyID(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validatePolicyGroupExists(context.Background(), checker, "", "test_field")
	if diags.HasError() {
		t.Fatal("expected no errors for empty ID")
	}
}

func TestValidatePolicyGroupExists_APIError(t *testing.T) {
	checker := &mockReferenceChecker{
		policyGroupErr: fmt.Errorf("connection refused"),
	}

	diags := validatePolicyGroupExists(context.Background(), checker, "pg-1", "test_field")
	if !diags.HasError() {
		t.Fatal("expected error when API fails")
	}
}

// Tests for policy group label refs validation

func TestValidatePolicyGroupLabelRefs_ValidLabels(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {ID: "label-1"},
			"label-2": {ID: "label-2"},
		},
	}

	membersJSON := `[["label-1"], ["label-2"]]`
	diags := validatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if diags.HasError() {
		t.Fatalf("expected no errors, got: %v", diags)
	}
}

func TestValidatePolicyGroupLabelRefs_InvalidLabel(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}

	membersJSON := `[["nonexistent"]]`
	diags := validatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if !diags.HasError() {
		t.Fatal("expected error for non-existent label")
	}
}

func TestValidatePolicyGroupLabelRefs_EmptyString(t *testing.T) {
	checker := &mockReferenceChecker{}

	diags := validatePolicyGroupLabelRefs(context.Background(), checker, "", "members_json")
	if diags.HasError() {
		t.Fatal("expected no errors for empty string")
	}
}

func TestValidatePolicyGroupLabelRefs_InvalidJSON(t *testing.T) {
	checker := &mockReferenceChecker{}

	membersJSON := `{invalid json}`
	diags := validatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	// Should not error - JSON parsing errors are handled by validators
	if diags.HasError() {
		t.Fatal("expected no errors for invalid JSON (handled by validators)")
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
	diags := validatePolicyGroupLabelRefs(context.Background(), checker, membersJSON, "members_json")
	if diags.HasError() {
		t.Fatalf("expected no errors for multiple OR groups, got: %v", diags)
	}
}

// Tests for countPolicyGroupMembers

func TestCountPolicyGroupMembers_LABELType(t *testing.T) {
	membersJSON := `[["label-1", "label-2"], ["label-3"]]`
	count, err := countPolicyGroupMembers(membersJSON, "LABEL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_FQDNType(t *testing.T) {
	membersJSON := `["example.com", "*.test.com", "api.example.org"]`
	count, err := countPolicyGroupMembers(membersJSON, "FQDN")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_IPAddressType(t *testing.T) {
	membersJSON := `[{"subnet": "10.0.0.0/8"}, {"range": {"start": "192.168.1.1", "end": "192.168.1.254"}}]`
	count, err := countPolicyGroupMembers(membersJSON, "IP_ADDRESS")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_EmptyString(t *testing.T) {
	count, err := countPolicyGroupMembers("", "FQDN")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got: %d", count)
	}
}

func TestCountPolicyGroupMembers_InvalidJSON(t *testing.T) {
	_, err := countPolicyGroupMembers("{invalid", "FQDN")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCountPolicyGroupMembers_UnknownType(t *testing.T) {
	_, err := countPolicyGroupMembers(`["test"]`, "UNKNOWN")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

// Tests for policyRuleReferencesPolicyGroup helper

func TestPolicyRuleReferencesPolicyGroup_Found(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"policy_groups": []any{"pg-1", "pg-2"},
		},
	}

	if !policyRuleReferencesPolicyGroup(rule, "pg-1") {
		t.Fatal("expected to find pg-1 in source")
	}
}

func TestPolicyRuleReferencesPolicyGroup_FoundInDestination(t *testing.T) {
	rule := map[string]any{
		"destination": map[string]any{
			"policy_groups": []any{"pg-3"},
		},
	}

	if !policyRuleReferencesPolicyGroup(rule, "pg-3") {
		t.Fatal("expected to find pg-3 in destination")
	}
}

func TestPolicyRuleReferencesPolicyGroup_NotFound(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"policy_groups": []any{"pg-1"},
		},
	}

	if policyRuleReferencesPolicyGroup(rule, "nonexistent") {
		t.Fatal("should not find nonexistent policy group in rule")
	}
}

func TestPolicyRuleReferencesPolicyGroup_NoPolicyGroups(t *testing.T) {
	rule := map[string]any{
		"source": map[string]any{
			"label_groups": []any{"group-1"},
		},
	}

	if policyRuleReferencesPolicyGroup(rule, "pg-1") {
		t.Fatal("should not find policy group when none are referenced")
	}
}
