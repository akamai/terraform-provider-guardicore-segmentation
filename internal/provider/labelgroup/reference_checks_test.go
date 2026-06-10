package labelgroup

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type stubReferenceChecker struct {
	labels map[string]*client.Label
	err    error
}

func (s *stubReferenceChecker) GetLabel(_ context.Context, id string) (*client.Label, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.labels == nil {
		return nil, nil
	}
	return s.labels[id], nil
}

func (s *stubReferenceChecker) ListLabels(_ context.Context, _, _ string) ([]client.Label, error) {
	return nil, nil
}

func (s *stubReferenceChecker) GetLabelGroup(_ context.Context, _ string) (*client.LabelGroup, error) {
	return nil, nil
}

func (s *stubReferenceChecker) GetUserGroup(_ context.Context, _ string) (*client.UserGroup, error) {
	return nil, nil
}

func (s *stubReferenceChecker) GetAsset(_ context.Context, _ string) (*client.Asset, error) {
	return nil, nil
}

func (s *stubReferenceChecker) GetWorksite(_ context.Context, _ string) (*client.Worksite, error) {
	return nil, nil
}

func (s *stubReferenceChecker) GetPolicyGroup(_ context.Context, _ string) (*client.PolicyGroup, error) {
	return nil, nil
}

func testSelectorObject(t *testing.T, labels ...string) types.Object {
	t.Helper()
	ctx := context.Background()

	labelValues := make([]attr.Value, 0, len(labels))
	for _, id := range labels {
		labelValues = append(labelValues, types.StringValue(id))
	}

	labelList, diags := types.ListValue(types.StringType, labelValues)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating label list: %v", diags)
	}

	orGroup, diags := types.ObjectValueFrom(ctx, labelGroupOrGroupAttrTypes(), LabelGroupOrGroupModel{LabelIDs: labelList})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating or-group: %v", diags)
	}

	orGroups, diags := types.ListValue(types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}, []attr.Value{orGroup})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating or-groups: %v", diags)
	}

	obj, diags := types.ObjectValueFrom(ctx, labelGroupSelectorAttrTypes(), LabelGroupSelectorModel{ORGroups: orGroups})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating selector object: %v", diags)
	}

	return obj
}

func TestValidateLabelGroupSelectors_NoChecker(t *testing.T) {
	data := &LabelGroupResourceModel{}
	diags := validateLabelGroupSelectors(context.Background(), nil, data)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics with nil checker, got %v", diags)
	}
}

func TestValidateLabelGroupSelectors_RequiresAtLeastOneSelector(t *testing.T) {
	checker := &stubReferenceChecker{}
	data := &LabelGroupResourceModel{
		Include:        types.ObjectNull(labelGroupSelectorAttrTypes()),
		Exclude:        types.ObjectNull(labelGroupSelectorAttrTypes()),
		RawIncludeJSON: types.StringNull(),
		RawExcludeJSON: types.StringNull(),
	}

	diags := validateLabelGroupSelectors(context.Background(), checker, data)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for missing selectors")
	}
}

func TestValidateLabelGroupSelectors_TypedSelectorsValidated(t *testing.T) {
	checker := &stubReferenceChecker{labels: map[string]*client.Label{
		"l-include": {ID: "l-include"},
		"l-exclude": {ID: "l-exclude"},
	}}

	data := &LabelGroupResourceModel{
		Include:        testSelectorObject(t, "l-include"),
		Exclude:        testSelectorObject(t, "l-exclude"),
		RawIncludeJSON: types.StringNull(),
		RawExcludeJSON: types.StringNull(),
	}

	diags := validateLabelGroupSelectors(context.Background(), checker, data)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics for existing label IDs, got %v", diags)
	}
}

func TestValidateLabelGroupSelectors_RawJSONValidationErrorPropagates(t *testing.T) {
	checker := &stubReferenceChecker{err: errors.New("api unavailable")}
	data := &LabelGroupResourceModel{
		Include:        types.ObjectNull(labelGroupSelectorAttrTypes()),
		Exclude:        types.ObjectNull(labelGroupSelectorAttrTypes()),
		RawIncludeJSON: types.StringValue(`{"or_labels":[{"and_labels":["l-1"]}]}`),
		RawExcludeJSON: types.StringNull(),
	}

	diags := validateLabelGroupSelectors(context.Background(), checker, data)
	if !diags.HasError() {
		t.Fatal("expected diagnostics from raw json label validation")
	}
}
