package policyrule

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

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

// Unit tests for extractWorksiteIDFromRule

func TestExtractWorksiteIDFromRule_WithWorksite(t *testing.T) {
	rule := map[string]any{
		"action": "ALLOW",
		"worksite": map[string]any{
			"id":   "ws-123",
			"name": "Headquarters",
		},
	}

	result := extractWorksiteIDFromRule(rule)
	if result.IsNull() {
		t.Fatal("expected non-null worksite ID")
	}
	if result.ValueString() != "ws-123" {
		t.Errorf("expected 'ws-123', got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_NoWorksite(t *testing.T) {
	rule := map[string]any{
		"action": "ALLOW",
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null, got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_EmptyWorksiteID(t *testing.T) {
	rule := map[string]any{
		"worksite": map[string]any{
			"id":   "",
			"name": "",
		},
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null for empty worksite ID, got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_WorksiteNotMap(t *testing.T) {
	rule := map[string]any{
		"worksite": "not-a-map",
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null for non-map worksite, got '%s'", result.ValueString())
	}
}

func TestPolicyRuleCommentFromAPI(t *testing.T) {
	tests := []struct {
		name string
		rule map[string]interface{}
		want string
		ok   bool
	}{
		{
			name: "top-level comments",
			rule: map[string]interface{}{"comments": "top-level"},
			want: "top-level",
			ok:   true,
		},
		{
			name: "attributes comments",
			rule: map[string]interface{}{"attributes": map[string]interface{}{"comments": "nested"}},
			want: "nested",
			ok:   true,
		},
		{
			name: "missing comments",
			rule: map[string]interface{}{},
			want: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := policyRuleCommentFromAPI(tt.rule)
			if ok != tt.ok {
				t.Fatalf("expected ok %v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected comments %q, got %q", tt.want, got)
			}
		})
	}
}

func TestUpdatePolicyRuleModelFromAPI_preservesEmptyLists(t *testing.T) {
	ctx := context.Background()
	emptyIntList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{})
	emptyStringList, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	emptyRangeList, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}, []attr.Value{})

	tests := []struct {
		name  string
		field string
		model func() *PolicyRuleResourceModel
	}{
		{
			name:  "ports empty list preserved",
			field: "Ports",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{Ports: emptyIntList}
			},
		},
		{
			name:  "exclude_ports empty list preserved",
			field: "ExcludePorts",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{ExcludePorts: emptyIntList}
			},
		},
		{
			name:  "ip_protocols empty list preserved",
			field: "IPProtocols",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{IPProtocols: emptyStringList}
			},
		},
		{
			name:  "scope empty list preserved",
			field: "Scope",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{Scope: emptyStringList}
			},
		},
		{
			name:  "port_ranges empty list preserved",
			field: "PortRanges",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{PortRanges: emptyRangeList}
			},
		},
		{
			name:  "exclude_port_ranges empty list preserved",
			field: "ExcludePortRanges",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{ExcludePortRanges: emptyRangeList}
			},
		},
	}

	apiRule := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.model()
			diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, apiRule)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			var got types.List
			switch tt.field {
			case "Ports":
				got = data.Ports
			case "ExcludePorts":
				got = data.ExcludePorts
			case "IPProtocols":
				got = data.IPProtocols
			case "Scope":
				got = data.Scope
			case "PortRanges":
				got = data.PortRanges
			case "ExcludePortRanges":
				got = data.ExcludePortRanges
			}

			if got.IsNull() {
				t.Fatalf("expected empty list, got null")
			}
			if len(got.Elements()) != 0 {
				t.Fatalf("expected 0 elements, got %d", len(got.Elements()))
			}
		})
	}
}

func TestUpdatePolicyRuleModelFromAPI_preservesEmptyPortsWhenEffectiveSpecOmitsPorts(t *testing.T) {
	ctx := context.Background()
	emptyPorts, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{})

	data := &PolicyRuleResourceModel{Ports: emptyPorts}
	apiRule := map[string]interface{}{
		"action":           "BLOCK",
		"section_position": "BLOCK",
		"enabled":          true,
	}
	effectiveSpec := map[string]interface{}{
		"action":           "BLOCK",
		"section_position": "BLOCK",
		"enabled":          true,
		"source":           map[string]interface{}{"processes": []interface{}{"/usr/bin/curl"}},
		"destination":      map[string]interface{}{"address_classification": "Internet"},
	}

	diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, effectiveSpec)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Ports.IsNull() {
		t.Fatal("expected empty ports list, got null")
	}
	if len(data.Ports.Elements()) != 0 {
		t.Fatalf("expected 0 ports, got %d", len(data.Ports.Elements()))
	}
}

func TestUpdatePolicyRuleModelFromAPI_nullListsStayNull(t *testing.T) {
	ctx := context.Background()

	data := &PolicyRuleResourceModel{
		Ports:             types.ListNull(types.Int64Type),
		ExcludePorts:      types.ListNull(types.Int64Type),
		IPProtocols:       types.ListNull(types.StringType),
		Scope:             types.ListNull(types.StringType),
		PortRanges:        types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}),
		ExcludePortRanges: types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}),
	}

	apiRule := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
	}

	diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, apiRule)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	for _, tc := range []struct {
		name string
		got  types.List
	}{
		{"Ports", data.Ports},
		{"ExcludePorts", data.ExcludePorts},
		{"IPProtocols", data.IPProtocols},
		{"Scope", data.Scope},
		{"PortRanges", data.PortRanges},
		{"ExcludePortRanges", data.ExcludePortRanges},
	} {
		if !tc.got.IsNull() {
			t.Errorf("%s: expected null, got non-null list with %d elements", tc.name, len(tc.got.Elements()))
		}
	}
}

// TestUpdatePolicyRuleModelFromAPI_MirroredPolicyGroupsIgnored guards against
// the API mirroring label_group_ids into the policy_groups response field.
// When the API returns both fields with overlapping UUIDs, the provider must
// populate label_group_ids in state and leave policy_group_ids null so that
// subsequent plans are idempotent.
func TestUpdatePolicyRuleModelFromAPI_MirroredPolicyGroupsIgnored(t *testing.T) {
	ctx := context.Background()

	data := &PolicyRuleResourceModel{}
	apiRule := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
		"source": map[string]interface{}{
			"label_group_ids": []interface{}{"lg-src-1", "lg-src-2"},
			"policy_groups":   []interface{}{"lg-src-1", "lg-src-2"},
		},
		"destination": map[string]interface{}{
			"label_group_ids": []interface{}{"lg-dst-1"},
			"policy_groups":   []interface{}{"lg-dst-1"},
		},
	}

	diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	assertEndpoint := func(side string, endpoint types.Object, wantLGs []string) {
		t.Helper()
		var model PolicyRuleEndpointModel
		if d := endpoint.As(ctx, &model, basetypes.ObjectAsOptions{}); d.HasError() {
			t.Fatalf("%s: decode diagnostics: %v", side, d)
		}
		if !model.PolicyGroupIDs.IsNull() {
			t.Errorf("%s.policy_group_ids: expected null (mirrored values must be filtered), got %v", side, model.PolicyGroupIDs)
		}
		if model.LabelGroupIDs.IsNull() {
			t.Fatalf("%s.label_group_ids: expected non-null", side)
		}
		var got []types.String
		if d := model.LabelGroupIDs.ElementsAs(ctx, &got, false); d.HasError() {
			t.Fatalf("%s.label_group_ids: decode diagnostics: %v", side, d)
		}
		if len(got) != len(wantLGs) {
			t.Fatalf("%s.label_group_ids: expected %d entries, got %d", side, len(wantLGs), len(got))
		}
		for i, want := range wantLGs {
			if got[i].ValueString() != want {
				t.Errorf("%s.label_group_ids[%d]: expected %q, got %q", side, i, want, got[i].ValueString())
			}
		}
	}

	assertEndpoint("source", data.Source, []string{"lg-src-1", "lg-src-2"})
	assertEndpoint("destination", data.Destination, []string{"lg-dst-1"})

	if data.RawJSON.IsNull() {
		t.Fatal("expected raw_json to be set")
	}
	if strings.Contains(data.RawJSON.ValueString(), "policy_groups") {
		t.Errorf("raw_json must not contain mirrored policy_groups; got: %s", data.RawJSON.ValueString())
	}
}
