package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestNormalizePolicyRuleLegacyAliases(t *testing.T) {
	spec := map[string]any{
		"any_port": true,
		"source": map[string]any{
			"label_groups": []any{"group-1"},
			"user_groups":  []any{"ug-1"},
			"assets":       []any{"asset-1"},
			"labels":       []any{"legacy-group"},
		},
	}

	result := normalizePolicyRuleLegacyAliases(spec)
	if _, ok := result["any_port"]; ok {
		t.Fatal("expected any_port to be removed")
	}

	source, ok := result["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source to be map[string]any, got %#v", result["source"])
	}
	if _, ok := source["label_group_ids"]; !ok {
		t.Fatal("expected label_group_ids to be present")
	}
	if _, ok := source["user_group_ids"]; !ok {
		t.Fatal("expected user_group_ids to be present")
	}
	if _, ok := source["asset_ids"]; !ok {
		t.Fatal("expected asset_ids to be present")
	}
}

func TestBuildPolicyRuleSpecFromModel_TypedOverridesRaw(t *testing.T) {
	ctx := context.Background()
	endpoint, diags := types.ObjectValueFrom(ctx, policyRuleEndpointAttrTypes(), PolicyRuleEndpointModel{
		AddressClassification: types.StringNull(),
		Subnets:               types.ListNull(types.StringType),
		Processes:             types.ListNull(types.StringType),
		WindowsServices:       types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}),
		Domains:               types.ListNull(types.StringType),
		LabelGroupIDs:         types.ListNull(types.StringType),
		UserGroupIDs:          types.ListNull(types.StringType),
		AssetIDs:              types.ListNull(types.StringType),
		PolicyGroupIDs:        types.ListNull(types.StringType),
		Labels:                types.ObjectNull(policyRuleLabelsAttrTypes()),
	})
	if diags.HasError() {
		t.Fatalf("unexpected endpoint diags: %v", diags)
	}

	ports, diags := types.ListValueFrom(ctx, types.Int64Type, []int64{443})
	if diags.HasError() {
		t.Fatalf("unexpected port diags: %v", diags)
	}
	protocols, diags := types.ListValueFrom(ctx, types.StringType, []string{"TCP"})
	if diags.HasError() {
		t.Fatalf("unexpected protocol diags: %v", diags)
	}

	model := PolicyRuleResourceModel{
		Action:          types.StringValue("ALLOW"),
		SectionPosition: types.StringValue("ALLOW"),
		Enabled:         types.BoolValue(true),
		Comments:        types.StringValue("typed comment"),
		Ports:           ports,
		IPProtocols:     protocols,
		Source:          endpoint,
		Destination:     endpoint,
		RawSpecJSON:     types.StringValue(`{"comments":"raw comment","ports":[22],"source":{"subnets":["10.0.0.0/24"]},"destination":{"domains":["raw.example.com"]}}`),
	}

	spec, specDiags := buildPolicyRuleSpecFromModel(ctx, &model)
	if specDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", specDiags)
	}

	if spec["comments"] != "typed comment" {
		t.Fatalf("expected typed comment to win, got %v", spec["comments"])
	}
	if ports, ok := spec["ports"].([]int64); !ok || len(ports) != 1 || ports[0] != 443 {
		t.Fatalf("expected typed ports [443], got %#v", spec["ports"])
	}
	source, ok := spec["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source to be map[string]any, got %#v", spec["source"])
	}
	if subnets, ok := source["subnets"].([]any); !ok || len(subnets) != 1 || subnets[0] != "10.0.0.0/24" {
		t.Fatalf("expected raw source subnet to remain when typed source omitted, got %#v", source)
	}
	destination, ok := spec["destination"].(map[string]any)
	if !ok {
		t.Fatalf("expected destination to be map[string]any, got %#v", spec["destination"])
	}
	if domains, ok := destination["domains"].([]any); !ok || len(domains) != 1 || domains[0] != "raw.example.com" {
		t.Fatalf("expected raw destination domain to remain when typed destination omitted, got %#v", destination)
	}
}

func TestBuildPolicyRuleSpecFromModel_NormalizesLegacyAliasesFromRawSpecJSON(t *testing.T) {
	ctx := context.Background()
	model := PolicyRuleResourceModel{
		RawSpecJSON: types.StringValue(`{
			"action":"ALLOW",
			"section_position":"ALLOW",
			"enabled":true,
			"creation_origin":"AUTO",
			"source":{"label_groups":["group-1"]},
			"destination":{"labels":["group-2"]},
			"any_port":true,
			"ip_protocols":["TCP"]
		}`),
	}

	spec, diags := buildPolicyRuleSpecFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if _, ok := spec["any_port"]; ok {
		t.Fatal("expected any_port to be stripped")
	}
	source, ok := spec["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source to be map[string]any, got %#v", spec["source"])
	}
	if _, ok := source["label_group_ids"]; !ok {
		t.Fatalf("expected legacy label_groups to normalize, got %#v", source)
	}
	destination, ok := spec["destination"].(map[string]any)
	if !ok {
		t.Fatalf("expected destination to be map[string]any, got %#v", spec["destination"])
	}
	if _, ok := destination["label_group_ids"]; !ok {
		t.Fatalf("expected legacy labels list to normalize, got %#v", destination)
	}
	if _, ok := spec["creation_origin"]; ok {
		t.Fatalf("expected creation_origin to be stripped, got %#v", spec["creation_origin"])
	}
}

func TestBuildPolicyRuleSpecFromModel_TypedEndpointFieldsAndScope(t *testing.T) {
	ctx := context.Background()
	processes, diags := types.ListValueFrom(ctx, types.StringType, []string{"dns.exe"})
	if diags.HasError() {
		t.Fatalf("unexpected processes diags: %v", diags)
	}
	serviceNames, diags := types.ListValueFrom(ctx, types.StringType, []string{"Dnscache"})
	if diags.HasError() {
		t.Fatalf("unexpected service names diags: %v", diags)
	}
	service, diags := types.ObjectValue(policyRuleWindowsServiceAttrTypes(), map[string]attr.Value{
		"allowed_image_names": serviceNames,
		"display_name":        types.StringValue("DNS Client"),
		"service_name":        types.StringValue("Dnscache"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected service diags: %v", diags)
	}
	services, diags := types.ListValue(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}, []attr.Value{service})
	if diags.HasError() {
		t.Fatalf("unexpected services diags: %v", diags)
	}
	endpoint, diags := types.ObjectValueFrom(ctx, policyRuleEndpointAttrTypes(), PolicyRuleEndpointModel{
		Subnets:         types.ListNull(types.StringType),
		Processes:       processes,
		WindowsServices: services,
		Domains:         types.ListNull(types.StringType),
		LabelGroupIDs:   types.ListNull(types.StringType),
		UserGroupIDs:    types.ListNull(types.StringType),
		AssetIDs:        types.ListNull(types.StringType),
		PolicyGroupIDs:  types.ListNull(types.StringType),
		Labels:          types.ObjectNull(policyRuleLabelsAttrTypes()),
	})
	if diags.HasError() {
		t.Fatalf("unexpected endpoint diags: %v", diags)
	}
	scope, diags := types.ListValueFrom(ctx, types.StringType, []string{"label-1", "label-2"})
	if diags.HasError() {
		t.Fatalf("unexpected scope diags: %v", diags)
	}

	model := PolicyRuleResourceModel{
		Action:          types.StringValue("ALLOW"),
		SectionPosition: types.StringValue("ALLOW"),
		Enabled:         types.BoolValue(true),
		Source:          endpoint,
		Scope:           scope,
	}

	spec, specDiags := buildPolicyRuleSpecFromModel(ctx, &model)
	if specDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", specDiags)
	}
	source, ok := spec["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source map, got %#v", spec["source"])
	}
	if _, ok := source["processes"]; !ok {
		t.Fatalf("expected typed processes, got %#v", source)
	}
	if _, ok := source["windows_services"]; !ok {
		t.Fatalf("expected typed windows services, got %#v", source)
	}
	windowsServices, ok := source["windows_services"].([]any)
	if !ok || len(windowsServices) != 1 {
		t.Fatalf("expected one windows service object, got %#v", source["windows_services"])
	}
	serviceMap, ok := windowsServices[0].(map[string]any)
	if !ok {
		t.Fatalf("expected windows service map, got %#v", windowsServices[0])
	}
	if serviceMap["service_name"] != "Dnscache" {
		t.Fatalf("expected service_name Dnscache, got %#v", serviceMap["service_name"])
	}
	if _, ok := spec["scope"]; !ok {
		t.Fatalf("expected typed scope, got %#v", spec)
	}
}

func TestBuildPolicyRuleSpecFromModel_ICMPCodesPreservesEmptyList(t *testing.T) {
	ctx := context.Background()

	emptyCodes, diags := types.ListValueFrom(ctx, types.Int64Type, []int64{})
	if diags.HasError() {
		t.Fatalf("unexpected empty codes diags: %v", diags)
	}

	icmpMatch, diags := types.ObjectValue(policyRuleICMPMatchAttrTypes(), map[string]attr.Value{
		"icmp_type":  types.Int64Value(8),
		"icmp_codes": emptyCodes,
		"version":    types.StringValue("4"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected icmp match diags: %v", diags)
	}

	icmpMatches, diags := types.ListValue(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()}, []attr.Value{icmpMatch})
	if diags.HasError() {
		t.Fatalf("unexpected icmp matches diags: %v", diags)
	}

	model := PolicyRuleResourceModel{
		Action:          types.StringValue("ALLOW"),
		SectionPosition: types.StringValue("ALLOW"),
		Enabled:         types.BoolValue(true),
		ICMPMatches:     icmpMatches,
	}

	spec, specDiags := buildPolicyRuleSpecFromModel(ctx, &model)
	if specDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", specDiags)
	}

	rawMatches, ok := spec["icmp_matches"].([]any)
	if !ok || len(rawMatches) != 1 {
		t.Fatalf("expected one icmp_match, got %#v", spec["icmp_matches"])
	}

	matchMap, ok := rawMatches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first icmp_match to be map, got %#v", rawMatches[0])
	}

	codes, ok := matchMap["icmp_codes"].([]int64)
	if !ok {
		t.Fatalf("expected icmp_codes []int64, got %#v", matchMap["icmp_codes"])
	}
	if len(codes) != 0 {
		t.Fatalf("expected empty icmp_codes, got %#v", codes)
	}
}

func TestBuildPolicyRuleSpecFromModel_ICMPCodesDefaultsEmptyWhenOmitted(t *testing.T) {
	ctx := context.Background()

	icmpMatch, diags := types.ObjectValue(policyRuleICMPMatchAttrTypes(), map[string]attr.Value{
		"icmp_type":  types.Int64Value(8),
		"icmp_codes": types.ListNull(types.Int64Type),
		"version":    types.StringValue("4"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected icmp match diags: %v", diags)
	}

	icmpMatches, diags := types.ListValue(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()}, []attr.Value{icmpMatch})
	if diags.HasError() {
		t.Fatalf("unexpected icmp matches diags: %v", diags)
	}

	model := PolicyRuleResourceModel{
		Action:          types.StringValue("ALLOW"),
		SectionPosition: types.StringValue("ALLOW"),
		Enabled:         types.BoolValue(true),
		ICMPMatches:     icmpMatches,
	}

	spec, specDiags := buildPolicyRuleSpecFromModel(ctx, &model)
	if specDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", specDiags)
	}

	rawMatches, ok := spec["icmp_matches"].([]any)
	if !ok || len(rawMatches) != 1 {
		t.Fatalf("expected one icmp_match, got %#v", spec["icmp_matches"])
	}

	matchMap, ok := rawMatches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first icmp_match to be map, got %#v", rawMatches[0])
	}

	codes, ok := matchMap["icmp_codes"].([]int64)
	if !ok {
		t.Fatalf("expected icmp_codes []int64, got %#v", matchMap["icmp_codes"])
	}
	if len(codes) != 0 {
		t.Fatalf("expected empty icmp_codes, got %#v", codes)
	}
}

func TestListInt64ToValue(t *testing.T) {
	ctx := context.Background()

	// nil slice -> null list
	result, diags := listInt64ToValue(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if !result.IsNull() {
		t.Error("expected null list for nil slice")
	}

	// empty slice -> null list
	result, diags = listInt64ToValue(ctx, []int64{})
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if !result.IsNull() {
		t.Error("expected null list for empty slice")
	}

	// single value
	result, diags = listInt64ToValue(ctx, []int64{42})
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if result.IsNull() {
		t.Fatal("expected non-null list for single value")
	}
	var elems []types.Int64
	diags.Append(result.ElementsAs(ctx, &elems, false)...)
	if len(elems) != 1 || elems[0].ValueInt64() != 42 {
		t.Errorf("expected [42], got %v", elems)
	}

	// multiple values
	result, diags = listInt64ToValue(ctx, []int64{1, 2, 3})
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	diags.Append(result.ElementsAs(ctx, &elems, false)...)
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	for i, expected := range []int64{1, 2, 3} {
		if elems[i].ValueInt64() != expected {
			t.Errorf("element %d: got %d, want %d", i, elems[i].ValueInt64(), expected)
		}
	}
}

func TestPolicyRuleScheduleObjectToMap(t *testing.T) {
	ctx := context.Background()

	// null object -> nil
	nullObj := types.ObjectNull(policyRuleScheduleAttrTypes())
	result, diags := policyRuleScheduleObjectToMap(ctx, nullObj)
	if diags.HasError() {
		t.Fatalf("unexpected diags for null: %v", diags)
	}
	if result != nil {
		t.Error("expected nil for null object")
	}

	// unknown object -> nil
	unknownObj := types.ObjectUnknown(policyRuleScheduleAttrTypes())
	result, diags = policyRuleScheduleObjectToMap(ctx, unknownObj)
	if diags.HasError() {
		t.Fatalf("unexpected diags for unknown: %v", diags)
	}
	if result != nil {
		t.Error("expected nil for unknown object")
	}

	// recurrence only
	obj, d := types.ObjectValueFrom(ctx, policyRuleScheduleAttrTypes(), PolicyRuleScheduleModel{
		Recurrence: types.StringValue("RRULE:FREQ=DAILY"),
		EndTime:    types.Int64Null(),
	})
	if d.HasError() {
		t.Fatalf("build object: %v", d)
	}
	result, diags = policyRuleScheduleObjectToMap(ctx, obj)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if result["recurrence"] != "RRULE:FREQ=DAILY" {
		t.Errorf("expected recurrence, got %v", result)
	}
	if _, ok := result["end_time"]; ok {
		t.Error("expected no end_time")
	}

	// end_time only
	obj, d = types.ObjectValueFrom(ctx, policyRuleScheduleAttrTypes(), PolicyRuleScheduleModel{
		Recurrence: types.StringNull(),
		EndTime:    types.Int64Value(1700000000),
	})
	if d.HasError() {
		t.Fatalf("build object: %v", d)
	}
	result, diags = policyRuleScheduleObjectToMap(ctx, obj)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if _, ok := result["recurrence"]; ok {
		t.Error("expected no recurrence")
	}
	if result["end_time"] != int64(1700000000) {
		t.Errorf("expected end_time 1700000000, got %v", result["end_time"])
	}

	// both fields
	obj, d = types.ObjectValueFrom(ctx, policyRuleScheduleAttrTypes(), PolicyRuleScheduleModel{
		Recurrence: types.StringValue("RRULE:FREQ=WEEKLY"),
		EndTime:    types.Int64Value(1700000000),
	})
	if d.HasError() {
		t.Fatalf("build object: %v", d)
	}
	result, diags = policyRuleScheduleObjectToMap(ctx, obj)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if result["recurrence"] != "RRULE:FREQ=WEEKLY" {
		t.Errorf("expected recurrence, got %v", result["recurrence"])
	}
	if result["end_time"] != int64(1700000000) {
		t.Errorf("expected end_time, got %v", result["end_time"])
	}

	// both null -> nil
	obj, d = types.ObjectValueFrom(ctx, policyRuleScheduleAttrTypes(), PolicyRuleScheduleModel{
		Recurrence: types.StringNull(),
		EndTime:    types.Int64Null(),
	})
	if d.HasError() {
		t.Fatalf("build object: %v", d)
	}
	result, diags = policyRuleScheduleObjectToMap(ctx, obj)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if result != nil {
		t.Error("expected nil for empty result")
	}
}

func TestBuildPolicyRuleSpecFromModel_AddressClassificationExclusivity(t *testing.T) {
	ctx := context.Background()
	processes, _ := types.ListValueFrom(ctx, types.StringType, []string{"dns.exe"})

	endpoint, diags := types.ObjectValueFrom(ctx, policyRuleEndpointAttrTypes(), PolicyRuleEndpointModel{
		AddressClassification: types.StringValue("Private"),
		Subnets:               types.ListNull(types.StringType),
		Processes:             processes,
		WindowsServices:       types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}),
		Domains:               types.ListNull(types.StringType),
		LabelGroupIDs:         types.ListNull(types.StringType),
		UserGroupIDs:          types.ListNull(types.StringType),
		AssetIDs:              types.ListNull(types.StringType),
		PolicyGroupIDs:        types.ListNull(types.StringType),
		Labels:                types.ObjectNull(policyRuleLabelsAttrTypes()),
	})
	if diags.HasError() {
		t.Fatalf("unexpected endpoint diags: %v", diags)
	}

	model := PolicyRuleResourceModel{
		Action:          types.StringValue("ALLOW"),
		SectionPosition: types.StringValue("ALLOW"),
		Enabled:         types.BoolValue(true),
		Source:          endpoint,
	}

	_, specDiags := buildPolicyRuleSpecFromModel(ctx, &model)
	if !specDiags.HasError() {
		t.Fatal("expected diagnostics error for address_classification combined with processes")
	}

	found := false
	for _, d := range specDiags {
		if strings.Contains(d.Detail(), "address_classification cannot be combined") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected address_classification exclusivity error, got: %v", specDiags)
	}
}

func TestEndpointObjectFromAny_LabelsAndLabelsIDs(t *testing.T) {
	ctx := context.Background()

	value := map[string]any{
		"labels": map[string]any{
			"or_labels": []any{
				map[string]any{
					"and_labels": []any{"label-1", "label-2"},
				},
			},
		},
	}

	endpoint, diags := endpointObjectFromAny(ctx, value, policyRuleEndpointAttrTypes())
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if endpoint.IsNull() {
		t.Fatal("expected non-null endpoint")
	}

	var model PolicyRuleEndpointModel
	if d := endpoint.As(ctx, &model, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("unexpected endpoint decode diagnostics: %v", d)
	}
	if model.Labels.IsNull() {
		t.Fatal("expected labels object to be set")
	}

	var labels PolicyRuleLabelsModel
	if d := model.Labels.As(ctx, &labels, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("unexpected labels decode diagnostics: %v", d)
	}

	if len(labels.OrLabels) != 1 {
		t.Fatalf("expected one or_labels entry, got %d", len(labels.OrLabels))
	}

	var andLabels []types.String
	if d := labels.OrLabels[0].AndLabels.ElementsAs(ctx, &andLabels, false); d.HasError() {
		t.Fatalf("unexpected and_labels decode diagnostics: %v", d)
	}
	if len(andLabels) != 2 {
		t.Fatalf("expected two and_labels entries, got %d", len(andLabels))
	}
	if andLabels[0].ValueString() != "label-1" {
		t.Fatalf("expected first and_label label-1, got %q", andLabels[0].ValueString())
	}
	if andLabels[1].ValueString() != "label-2" {
		t.Fatalf("expected second and_label label-2, got %q", andLabels[1].ValueString())
	}
}
