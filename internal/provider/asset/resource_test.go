package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mockReferenceChecker implements common.ReferenceChecker for testing.
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

// TestAssetGoneAfterWrite covers F-01: the predicate used by Update and Read
// to decide whether a post-write GetAsset response indicates the server-side
// resource is gone. Returning true tells the caller to RemoveResource instead
// of writing stale state.
func TestAssetGoneAfterWrite(t *testing.T) {
	cases := []struct {
		name   string
		input  *client.Asset
		expect bool
	}{
		{"nil asset", nil, true},
		{"deleted status", &client.Asset{ID: "a-1", Status: "deleted"}, true},
		{"on status", &client.Asset{ID: "a-1", Status: "on"}, false},
		{"empty status", &client.Asset{ID: "a-1", Status: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := assetGoneAfterWrite(tc.input); got != tc.expect {
				t.Fatalf("assetGoneAfterWrite(%+v) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

// Unit tests for modelLabelsToAPI

func TestModelLabelsToAPI_Nil(t *testing.T) {
	r := &AssetResource{}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), nil, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestModelLabelsToAPI_Empty(t *testing.T) {
	r := &AssetResource{}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), []AssetLabelModel{}, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("expected non-nil for empty input")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestModelLabelsToAPI_Populated(t *testing.T) {
	r := &AssetResource{}
	labels := []AssetLabelModel{
		{
			ID:    types.StringValue("id-1"),
			Key:   types.StringValue("env"),
			Value: types.StringValue("prod"),
		},
		{
			ID:    types.StringValue("id-2"),
			Key:   types.StringValue("role"),
			Value: types.StringValue("web"),
		},
	}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != "id-1" || result[0].Key != "env" || result[0].Value != "prod" {
		t.Errorf("unexpected first label: %+v", result[0])
	}
	if result[1].ID != "id-2" || result[1].Key != "role" || result[1].Value != "web" {
		t.Errorf("unexpected second label: %+v", result[1])
	}
}

func TestModelLabelsToAPI_FiltersReadOnlyLabels(t *testing.T) {
	r := &AssetResource{}
	readOnly := true
	labels := []AssetLabelModel{
		{
			ID:       types.StringValue("user-label"),
			Key:      types.StringValue("Role"),
			Value:    types.StringValue("Server"),
			ReadOnly: types.BoolValue(false),
		},
		{
			ID:       types.StringValue("read-only-label"),
			Key:      types.StringValue("cloud_provider"),
			Value:    types.StringValue("AWS"),
			ReadOnly: types.BoolValue(true),
		},
		{
			ID:       types.StringValue("read-only-pointer-label"),
			Key:      types.StringValue("os_type"),
			Value:    types.StringValue("Linux"),
			ReadOnly: boolPointerToTerraform(&readOnly),
		},
	}

	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for read-only labels")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user-managed label, got %d", len(result))
	}
	if result[0].ID != "user-label" || result[0].Key != "Role" || result[0].Value != "Server" {
		t.Errorf("unexpected label: %+v", result[0])
	}
}

func TestModelLabelsToAPI_FiltersDynamicLabels(t *testing.T) {
	r := &AssetResource{
		referenceChecker: &mockReferenceChecker{
			labels: map[string]*client.Label{
				"dynamic-label": {
					ID:              "dynamic-label",
					DynamicCriteria: []client.LabelCriteria{{Field: "name", Op: "CONTAINS", Argument: "db"}},
				},
				"static-label": {
					ID: "static-label",
				},
			},
		},
	}
	labels := []AssetLabelModel{
		{
			ID:    types.StringValue("dynamic-label"),
			Key:   types.StringValue("role"),
			Value: types.StringValue("database"),
		},
		{
			ID:    types.StringValue("static-label"),
			Key:   types.StringValue("env"),
			Value: types.StringValue("prod"),
		},
	}

	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for dynamic labels")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user-managed label, got %d", len(result))
	}
	if result[0].ID != "static-label" {
		t.Fatalf("expected static label to remain in payload, got %+v", result[0])
	}
}

func TestModelLabelsToAPI_FiltersReadOnlyViaListFallback(t *testing.T) {
	r := &AssetResource{
		referenceChecker: &mockReferenceChecker{
			labels: map[string]*client.Label{
				"auto-label": {
					ID:    "auto-label",
					Key:   "os_type",
					Value: "Linux",
				},
				"static-label": {
					ID:    "static-label",
					Key:   "env",
					Value: "prod",
				},
			},
			listLabels: []client.Label{
				{
					ID:       "auto-label",
					Key:      "os_type",
					Value:    "Linux",
					ReadOnly: boolPtrAsset(true),
				},
				{
					ID:    "static-label",
					Key:   "env",
					Value: "prod",
				},
			},
		},
	}

	labels := []AssetLabelModel{
		{ID: types.StringValue("auto-label"), Key: types.StringValue("os_type"), Value: types.StringValue("Linux")},
		{ID: types.StringValue("static-label"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}

	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for read-only fallback labels")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user-managed label after read-only fallback filtering, got %d", len(result))
	}
	if result[0].ID != "static-label" {
		t.Fatalf("expected static label to remain in payload, got %+v", result[0])
	}
}

func TestModelLabelsToAPI_UsesIgnoreCacheAcrossCalls(t *testing.T) {
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{
			"label-1": {
				ID:    "label-1",
				Key:   "env",
				Value: "prod",
			},
		},
		getLabelCalls: map[string]int{},
	}

	r := &AssetResource{
		referenceChecker: checker,
		ignoreCache:      common.NewAssetLabelIgnoreCache(),
	}

	labels := []AssetLabelModel{
		{ID: types.StringValue("label-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}

	for i := 0; i < 3; i++ {
		var diags diag.Diagnostics
		result := r.modelLabelsToAPI(context.Background(), labels, &diags)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics on call %d: %v", i+1, diags)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 label on call %d, got %d", i+1, len(result))
		}
	}

	if checker.getLabelCalls["label-1"] != 1 {
		t.Fatalf("expected GetLabel to be called once due to cache, got %d", checker.getLabelCalls["label-1"])
	}
}

func TestModelLabelsToAPI_DoesNotCacheLookupErrors(t *testing.T) {
	checker := &mockReferenceChecker{
		labelErr:      fmt.Errorf("temporary label lookup failure"),
		getLabelCalls: map[string]int{},
	}

	r := &AssetResource{
		referenceChecker: checker,
		ignoreCache:      common.NewAssetLabelIgnoreCache(),
	}

	labels := []AssetLabelModel{
		{ID: types.StringValue("label-err"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}

	for i := 0; i < 2; i++ {
		var diags diag.Diagnostics
		result := r.modelLabelsToAPI(context.Background(), labels, &diags)
		if diags.HasError() {
			t.Fatalf("unexpected error diagnostics on call %d: %v", i+1, diags)
		}
		if len(result) != 1 {
			t.Fatalf("expected label to be kept on lookup failure (fail-open), got %d labels", len(result))
		}

		warnings := 0
		for _, d := range diags {
			if d.Severity() == diag.SeverityWarning {
				warnings++
			}
		}
		if warnings == 0 {
			t.Fatalf("expected warning diagnostics on call %d", i+1)
		}
	}

	if checker.getLabelCalls["label-err"] != 2 {
		t.Fatalf("expected GetLabel to be retried when lookup fails, got %d calls", checker.getLabelCalls["label-err"])
	}
}

func TestModelLabelsToAPI_InvalidIncompleteLabelFields(t *testing.T) {
	r := &AssetResource{}
	labels := []AssetLabelModel{
		{ID: types.StringValue("id-1"), Key: types.StringValue(""), Value: types.StringValue("prod")},
	}

	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for incomplete label fields")
	}
	if len(result) != 0 {
		t.Fatalf("expected no labels in payload when label fields are invalid, got %d", len(result))
	}
}

func TestModelLabelsToAPI_OptionalIDOmitted(t *testing.T) {
	// id is optional: a label with only key+value (id null) is valid and the
	// payload omits the id (the API resolves the label by key+value).
	r := &AssetResource{}
	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		{ID: types.StringUnknown(), Key: types.StringValue("role"), Value: types.StringValue("web")},
	}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(result))
	}
	for i, ref := range result {
		if ref.ID != "" {
			t.Errorf("labels[%d]: expected empty id when omitted, got %q", i, ref.ID)
		}
	}
	if result[0].Key != "env" || result[0].Value != "prod" {
		t.Errorf("unexpected first label: %+v", result[0])
	}
	if result[1].Key != "role" || result[1].Value != "web" {
		t.Errorf("unexpected second label: %+v", result[1])
	}
}

func TestModelLabelsToAPI_MissingKeyValueWithoutID(t *testing.T) {
	// Without an id, key and value are still required.
	r := &AssetResource{}
	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("env"), Value: types.StringValue("")},
	}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for missing value")
	}
	if len(result) != 0 {
		t.Fatalf("expected no labels in payload, got %d", len(result))
	}
}

func TestModelLabelsToAPI_FiltersReadOnlyByKeyValue(t *testing.T) {
	// A read-only label referenced without an id must be rejected, resolved via
	// key+value through ListLabels.
	r := &AssetResource{
		referenceChecker: &mockReferenceChecker{
			listLabels: []client.Label{
				{ID: "ro-1", Key: "cloud_provider", Value: "AWS", ReadOnly: boolPtrAsset(true)},
			},
		},
	}
	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("cloud_provider"), Value: types.StringValue("AWS")},
	}
	var diags diag.Diagnostics
	result := r.modelLabelsToAPI(context.Background(), labels, &diags)
	if !diags.HasError() {
		t.Fatalf("expected diagnostics for read-only label resolved by key+value")
	}
	if len(result) != 0 {
		t.Fatalf("expected no labels in payload, got %d", len(result))
	}
}

func boolPtrAsset(v bool) *bool {
	return &v
}

// Unit tests for apiToModel label filtering

func TestApiToModel_ServerAssignedLabelsFiltered(t *testing.T) {
	// When API returns server-assigned labels but user didn't configure any,
	// labels should stay nil (server-assigned labels are not tracked).
	// API list endpoint typically returns labels with only the ID.
	// data.Name is set to simulate a normal Read (not import).
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
	} // Labels is nil — user didn't configure labels
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "auto-1", Key: "", Value: ""},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Errorf("expected nil labels (user didn't configure), got %d labels", len(data.Labels))
	}
}

func TestApiToModel_ImportPopulatesLabels(t *testing.T) {
	// During import, data.Labels is nil and data.Name is null (ImportState only sets ID).
	// Non-read-only labels from the API should be populated so they appear in state after import.
	r := &AssetResource{}
	readOnly := true
	data := &AssetResourceModel{} // Name is null (zero value) — simulates import
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "env", Value: "prod"},
			{ID: "lbl-2", Key: "os_type", Value: "Linux", ReadOnly: &readOnly},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels == nil {
		t.Fatal("expected labels to be populated during import, got nil")
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (auto labels skipped), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected first label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[0].ReadOnly.ValueBool() {
		t.Error("expected imported user label read_only=false")
	}
}

func TestApiToModel_ImportSkipsIncompleteLabelsWithWarning(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{} // Name is null (zero value) — simulates import
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-incomplete", Key: "", Value: ""},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Fatalf("expected skipped imported labels to normalize to nil, got %d", len(data.Labels))
	}

	warningCount := 0
	for _, d := range diags {
		if d.Severity() != 1 {
			warningCount++
		}
	}
	if warningCount == 0 {
		t.Fatal("expected warning for skipped incomplete imported label")
	}
}

func TestApiToModel_ImportDoesNotHydrateIDOnlyLabels(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{} // Name is null (zero value) — simulates import
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "", Value: ""},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Fatalf("expected id-only imported label to normalize to nil, got %d", len(data.Labels))
	}

	warningCount := 0
	for _, d := range diags {
		if d.Severity() != 1 {
			warningCount++
		}
	}
	if warningCount == 0 {
		t.Fatal("expected warning for skipped incomplete imported label")
	}
}

func TestApiToModel_ImportReadOnlyLabelsNormalizeToNil(t *testing.T) {
	r := &AssetResource{}
	readOnly := true
	data := &AssetResourceModel{} // Name null simulates import
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-readonly", Key: "os_type", Value: "Linux", ReadOnly: &readOnly},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Fatalf("expected read-only imported labels to normalize to nil, got %d", len(data.Labels))
	}
}

func TestApiToModel_ImportDoesNotHydrateOrchestrationMetadataWhenUnset(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{}
	asset := &client.Asset{
		ID:                    "asset-1",
		Name:                  "test",
		Status:                "on",
		OrchestrationMetadata: []byte(`{"asset_type":"F5","f5_device_hostname":"f5-host","partition":"Common","vs_name":"vs-main","Agent":{}}`),
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if !data.OrchestrationMetadata.IsNull() {
		t.Fatal("expected orchestration_metadata to remain null during import when unset in config")
	}
}

func TestApiToModel_UpdatesOrchestrationMetadataWhenConfigured(t *testing.T) {
	r := &AssetResource{}
	configuredObj, d := types.ObjectValue(assetOrchestrationMetadataAttrTypes(), map[string]attr.Value{
		"asset_type":         types.StringValue("F5"),
		"f5_device_hostname": types.StringValue("configured-host"),
		"partition":          types.StringValue("Common"),
		"vs_name":            types.StringValue("configured-vs"),
		"extra_json":         types.StringNull(),
	})
	if d.HasError() {
		t.Fatalf("unexpected setup diagnostics: %v", d)
	}

	data := &AssetResourceModel{OrchestrationMetadata: configuredObj}
	asset := &client.Asset{
		ID:                    "asset-1",
		Name:                  "test",
		Status:                "on",
		OrchestrationMetadata: json.RawMessage(`{"asset_type":"F5","f5_device_hostname":"api-host","partition":"Common","vs_name":"api-vs","Agent":{}}`),
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.OrchestrationMetadata.IsNull() {
		t.Fatal("expected orchestration_metadata to be populated when configured")
	}
	attrs := data.OrchestrationMetadata.Attributes()
	assetType, _ := attrs["asset_type"].(types.String)
	if assetType.ValueString() != "F5" {
		t.Fatalf("expected asset_type F5, got %q", assetType.ValueString())
	}
	extraJSON, _ := attrs["extra_json"].(types.String)
	if extraJSON.IsNull() || extraJSON.ValueString() == "" {
		t.Fatal("expected extra_json to include API-only metadata")
	}
}

func TestApiToModel_NoLabelsNilModel(t *testing.T) {
	// When API returns no labels and user didn't configure any, labels stay nil.
	r := &AssetResource{}
	data := &AssetResourceModel{}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: nil,
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Fatal("expected nil labels, got non-nil")
	}
}

func TestApiToModel_UserLabelsKept(t *testing.T) {
	// When user configured labels, only those labels should be kept from API response.
	// Matching is by ID. The API list endpoint returns labels with only the ID
	// (key and value are empty strings), so the user's planned key+value are preserved.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "", Value: ""},  // API returns only ID
			{ID: "auto-1", Key: "", Value: ""}, // server-assigned label
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (user-specified only), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[0].Key.ValueString() != "env" {
		t.Errorf("expected label key 'env' (preserved from plan), got %q", data.Labels[0].Key.ValueString())
	}
	if data.Labels[0].Value.ValueString() != "prod" {
		t.Errorf("expected label value 'prod' (preserved from plan), got %q", data.Labels[0].Value.ValueString())
	}
}

func TestApiToModel_UserLabelsMatchedByKeyValueWithoutID(t *testing.T) {
	// id is optional: when a configured label omits id (unknown in plan), the
	// read-back matches it to the API label by key+value and writes back the
	// resolved id so the computed attribute becomes known.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringUnknown(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "env", Value: "prod"},
			{ID: "auto-1", Key: "Agent", Value: "Not Installed"},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (matched by key+value), got %d", len(data.Labels))
	}
	if got := data.Labels[0].ID.ValueString(); got != "lbl-1" {
		t.Errorf("expected resolved id 'lbl-1', got %q", got)
	}
	if data.Labels[0].Key.ValueString() != "env" || data.Labels[0].Value.ValueString() != "prod" {
		t.Errorf("unexpected key/value: %+v", data.Labels[0])
	}
}

func TestApiToModel_UserLabelsWithoutIDDistinctMatches(t *testing.T) {
	// Two distinct id-less labels must each match a distinct API label and not
	// collapse onto the same one.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringUnknown(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringUnknown(), Key: types.StringValue("role"), Value: types.StringValue("web")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-role", Key: "role", Value: "web"},
			{ID: "lbl-env", Key: "env", Value: "prod"},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(data.Labels))
	}
	// Plan order preserved: env first, role second.
	if data.Labels[0].ID.ValueString() != "lbl-env" {
		t.Errorf("labels[0]: expected 'lbl-env', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[1].ID.ValueString() != "lbl-role" {
		t.Errorf("labels[1]: expected 'lbl-role', got %q", data.Labels[1].ID.ValueString())
	}
}

func TestApiToModel_MultipleUserLabels(t *testing.T) {
	// Multiple user labels are kept; server-assigned ones are filtered out.
	// Matching is by ID. API returns labels in different order than plan.
	// Result must preserve plan order.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-2"), Key: types.StringValue("role"), Value: types.StringValue("web")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-2", Key: "", Value: ""},  // API returns lbl-2 first
			{ID: "auto-1", Key: "", Value: ""}, // server-assigned
			{ID: "lbl-1", Key: "", Value: ""},  // API returns lbl-1 last
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(data.Labels))
	}
	// Verify plan order is preserved (lbl-1 first, lbl-2 second)
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected labels[0] ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[1].ID.ValueString() != "lbl-2" {
		t.Errorf("expected labels[1] ID 'lbl-2', got %q", data.Labels[1].ID.ValueString())
	}
}

func TestApiToModel_LabelOrderPreserved(t *testing.T) {
	// Verify that plan order is preserved when API returns 3 labels in a
	// completely different order, with a server-assigned label mixed in.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-2"), Key: types.StringValue("role"), Value: types.StringValue("web")},
			{ID: types.StringValue("lbl-3"), Key: types.StringValue("tier"), Value: types.StringValue("frontend")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-2", Key: "", Value: ""},
			{ID: "lbl-3", Key: "", Value: ""},
			{ID: "auto-1", Key: "", Value: ""},
			{ID: "lbl-1", Key: "", Value: ""},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(data.Labels))
	}
	// Must match plan order: lbl-1, lbl-2, lbl-3
	expectedIDs := []string{"lbl-1", "lbl-2", "lbl-3"}
	for i, exp := range expectedIDs {
		if got := data.Labels[i].ID.ValueString(); got != exp {
			t.Errorf("labels[%d]: expected ID %q, got %q", i, exp, got)
		}
	}
	// Verify key+value preserved from plan
	if data.Labels[0].Key.ValueString() != "env" {
		t.Errorf("labels[0]: expected key 'env', got %q", data.Labels[0].Key.ValueString())
	}
	if data.Labels[2].Value.ValueString() != "frontend" {
		t.Errorf("labels[2]: expected value 'frontend', got %q", data.Labels[2].Value.ValueString())
	}
}

func TestApiToModel_UserLabelRemovedServerSide(t *testing.T) {
	// When a user-specified label is removed server-side, it disappears from state.
	// Matching is by ID.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-deleted"), Key: types.StringValue("old"), Value: types.StringValue("val")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "", Value: ""},
			// lbl-deleted is no longer in API response
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (remaining after server-side removal), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
}

func TestApiToModel_DynamicLabelDroppedFromManagedState(t *testing.T) {
	// Explicitly configured dynamic labels are not assignable and should not be
	// retained as managed labels when reconciling state from API labels.
	r := &AssetResource{
		referenceChecker: &mockReferenceChecker{
			labels: map[string]*client.Label{
				"dynamic-label": {
					ID:              "dynamic-label",
					DynamicCriteria: []client.LabelCriteria{{Field: "name", Op: "CONTAINS", Argument: "db"}},
				},
				"static-label": {ID: "static-label"},
			},
		},
	}

	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("dynamic-label"), Key: types.StringValue("role"), Value: types.StringValue("database")},
			{ID: types.StringValue("static-label"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "static-label", Key: "", Value: ""},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (static only), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "static-label" {
		t.Fatalf("expected static label retained, got %q", data.Labels[0].ID.ValueString())
	}
}

func TestApiToModel_RemovedStaticLabelStillRemoved(t *testing.T) {
	// Non-ignored labels still follow API truth and disappear from state when removed.
	r := &AssetResource{
		referenceChecker: &mockReferenceChecker{
			labels: map[string]*client.Label{
				"static-label": {ID: "static-label"},
			},
		},
	}

	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("static-label"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 0 {
		t.Fatalf("expected removed static label to be dropped, got %d", len(data.Labels))
	}
}

func TestApiToModel_UserLabelsEmptyAPIResponse(t *testing.T) {
	// When user configured labels but API returns none, preserve explicit
	// empty labels to avoid losing ownership semantics.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: nil,
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels == nil {
		t.Fatal("expected explicit empty labels, got nil")
	}
	if len(data.Labels) != 0 {
		t.Fatalf("expected 0 labels, got %d", len(data.Labels))
	}
}

func TestApiToModel_EmptyConfiguredLabelsRemainNil(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{Labels: []AssetLabelModel{}}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: nil,
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels == nil {
		t.Fatal("expected explicit empty labels to remain empty, got nil")
	}
	if len(data.Labels) != 0 {
		t.Fatalf("expected explicit empty labels to remain empty, got %d", len(data.Labels))
	}
}

// Unit tests for NIC filtering in apiToModel

func TestApiToModel_FiltersEmptyIPNics(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
		Nics: []NICModel{},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Nics: []client.AssetNIC{
			{MacAddress: "AA:BB:CC:DD:EE:01", IPAddresses: []string{"10.0.0.1"}},
			{MacAddress: "AA:BB:CC:DD:EE:02", IPAddresses: []string{}},
			{MacAddress: "AA:BB:CC:DD:EE:03", IPAddresses: []string{"10.0.0.3", "10.0.0.4"}},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}

	if len(data.Nics) != 2 {
		t.Fatalf("expected 2 valid NICs, got %d", len(data.Nics))
	}
	if data.Nics[0].MacAddress.ValueString() != "AA:BB:CC:DD:EE:01" {
		t.Errorf("expected first NIC MAC AA:BB:CC:DD:EE:01, got %q", data.Nics[0].MacAddress.ValueString())
	}
	if data.Nics[1].MacAddress.ValueString() != "AA:BB:CC:DD:EE:03" {
		t.Errorf("expected second NIC MAC AA:BB:CC:DD:EE:03, got %q", data.Nics[1].MacAddress.ValueString())
	}

	warningCount := 0
	for _, d := range diags {
		if d.Severity() == diag.SeverityWarning {
			warningCount++
		}
	}
	if warningCount != 1 {
		t.Errorf("expected 1 warning for filtered NIC, got %d", warningCount)
	}
}

func TestApiToModel_AllNicsEmptyIP(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
		Nics: []NICModel{},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Nics: []client.AssetNIC{
			{MacAddress: "AA:BB:CC:DD:EE:01", IPAddresses: []string{}},
			{MacAddress: "AA:BB:CC:DD:EE:02", IPAddresses: nil},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}

	if len(data.Nics) != 0 {
		t.Fatalf("expected 0 valid NICs, got %d", len(data.Nics))
	}

	warningCount := 0
	for _, d := range diags {
		if d.Severity() == diag.SeverityWarning {
			warningCount++
		}
	}
	// 2 per-NIC warnings + 1 "all NICs invalid" warning
	if warningCount != 3 {
		t.Errorf("expected 3 warnings (2 per-NIC + 1 all-invalid), got %d", warningCount)
	}
}

func TestApiToModel_NilIPAddressesFiltered(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
		Nics: []NICModel{},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Nics: []client.AssetNIC{
			{MacAddress: "AA:BB:CC:DD:EE:01", IPAddresses: nil},
			{MacAddress: "AA:BB:CC:DD:EE:02", IPAddresses: []string{"10.0.0.1"}},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}

	if len(data.Nics) != 1 {
		t.Fatalf("expected 1 valid NIC, got %d", len(data.Nics))
	}
	if data.Nics[0].MacAddress.ValueString() != "AA:BB:CC:DD:EE:02" {
		t.Errorf("expected NIC MAC AA:BB:CC:DD:EE:02, got %q", data.Nics[0].MacAddress.ValueString())
	}
}

func TestApiToModel_AllNicsValidNoWarnings(t *testing.T) {
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
		Nics: []NICModel{},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Nics: []client.AssetNIC{
			{MacAddress: "AA:BB:CC:DD:EE:01", IPAddresses: []string{"10.0.0.1"}},
			{MacAddress: "AA:BB:CC:DD:EE:02", IPAddresses: []string{"10.0.0.2"}},
		},
	}

	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}

	if len(data.Nics) != 2 {
		t.Fatalf("expected 2 NICs, got %d", len(data.Nics))
	}

	warningCount := 0
	for _, d := range diags {
		if d.Severity() == diag.SeverityWarning {
			warningCount++
		}
	}
	if warningCount != 0 {
		t.Errorf("expected no warnings for valid NICs, got %d", warningCount)
	}
}

func TestValidateAssetLabelRefs_KeyValueReadOnlyRejected(t *testing.T) {
	// A label referenced without an id but pointing at a read-only label
	// (resolved by key+value) must be rejected at validation time.
	checker := &mockReferenceChecker{
		listLabels: []client.Label{
			{ID: "ro-1", Key: "cloud_provider", Value: "AWS", ReadOnly: boolPtrAsset(true)},
		},
	}
	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("cloud_provider"), Value: types.StringValue("AWS")},
	}
	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if !diags.HasError() {
		t.Fatalf("expected error for read-only label resolved by key+value")
	}
}

func TestValidateAssetLabelRefs_KeyValueAssignableAllowed(t *testing.T) {
	// A label referenced without an id pointing at an assignable label is fine.
	checker := &mockReferenceChecker{
		listLabels: []client.Label{
			{ID: "ok-1", Key: "env", Value: "prod"},
		},
	}
	labels := []AssetLabelModel{
		{ID: types.StringNull(), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}
	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if diags.HasError() {
		t.Fatalf("unexpected error for assignable label: %s", diags)
	}
}

func TestValidateAssetLabelRefs_KeyValueUnknownSkipped(t *testing.T) {
	// When key or value is unknown (e.g. references a not-yet-created label),
	// validation is skipped rather than erroring.
	checker := &mockReferenceChecker{}
	labels := []AssetLabelModel{
		{ID: types.StringUnknown(), Key: types.StringUnknown(), Value: types.StringValue("prod")},
	}
	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if diags.HasError() {
		t.Fatalf("unexpected error when key is unknown: %s", diags)
	}
}

func TestValidateAssetLabelRefs_IDStillValidated(t *testing.T) {
	// When an id is supplied, existence is still validated by id.
	checker := &mockReferenceChecker{
		labels: map[string]*client.Label{},
	}
	labels := []AssetLabelModel{
		{ID: types.StringValue("does-not-exist"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
	}
	diags := validateAssetLabelRefs(context.Background(), checker, labels)
	if !diags.HasError() {
		t.Fatalf("expected error for non-existent label id")
	}
}
