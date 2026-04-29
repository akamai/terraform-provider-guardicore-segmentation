package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		AddressClassification: types.StringValue("Private"),
		Subnets:               types.ListNull(types.StringType),
		Processes:             processes,
		WindowsServices:       services,
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
	if source["address_classification"] != "Private" {
		t.Fatalf("expected typed address classification, got %#v", source)
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
