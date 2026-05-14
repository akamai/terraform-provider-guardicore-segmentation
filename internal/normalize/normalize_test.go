package normalize

import (
	"encoding/json"
	"testing"
)

func TestNormalizePolicyRuleSpec(t *testing.T) {
	rule := map[string]interface{}{
		"action":           "ALLOW",
		"enabled":          true,
		"id":               "should-be-removed",
		"creation_origin":  "AUTO",
		"created_at":       "2024-01-01",
		"updated_at":       "2024-01-02",
		"author":           "admin",
		"hit_count":        float64(42),
		"state":            "PUBLISHED",
		"source":           map[string]interface{}{"labels": map[string]interface{}{"or_labels": []interface{}{map[string]interface{}{"and_labels": []interface{}{"label-1"}}}}},
		"destination":      map[string]interface{}{"label_group_ids": []interface{}{"group-1"}},
		"ports":            []interface{}{float64(443), float64(80)},
		"ip_protocols":     []interface{}{"UDP", "TCP"},
		"exclude_ports":    []interface{}{float64(8443), float64(22)},
		"ruleset_name":     "ruleset-1",
		"network_profile":  "CORPORATE",
		"schedule":         map[string]interface{}{"recurrence": "RRULE:FREQ=DAILY"},
		"section_position": "ALLOW",
	}

	result := NormalizePolicyRuleSpec(rule)

	if _, ok := result["id"]; ok {
		t.Error("expected 'id' to be stripped")
	}
	if _, ok := result["created_at"]; ok {
		t.Error("expected 'created_at' to be stripped")
	}
	if _, ok := result["hit_count"]; ok {
		t.Error("expected 'hit_count' to be stripped")
	}
	if _, ok := result["state"]; ok {
		t.Error("expected 'state' to be stripped")
	}
	if _, ok := result["creation_origin"]; ok {
		t.Error("expected 'creation_origin' to be stripped")
	}

	if result["action"] != "ALLOW" {
		t.Errorf("expected action 'ALLOW', got %v", result["action"])
	}
	if result["ruleset_name"] != "ruleset-1" {
		t.Errorf("expected ruleset_name to be preserved, got %v", result["ruleset_name"])
	}
	if _, ok := result["network_profile"]; ok {
		t.Errorf("expected default network_profile to be stripped, got %v", result["network_profile"])
	}

	// Ports should be sorted
	ports, ok := result["ports"].([]interface{})
	if !ok {
		t.Fatal("expected ports to be []interface{}")
	}
	if ports[0] != 80 || ports[1] != 443 {
		t.Errorf("expected ports [80, 443], got %v", ports)
	}

	// Protocols should be sorted
	protocols, ok := result["ip_protocols"].([]interface{})
	if !ok {
		t.Fatal("expected ip_protocols to be []interface{}")
	}
	if protocols[0] != "TCP" || protocols[1] != "UDP" {
		t.Errorf("expected protocols [TCP, UDP], got %v", protocols)
	}

	excludePorts, ok := result["exclude_ports"].([]interface{})
	if !ok {
		t.Fatal("expected exclude_ports to be []interface{}")
	}
	if excludePorts[0] != 22 || excludePorts[1] != 8443 {
		t.Errorf("expected exclude_ports [22, 8443], got %v", excludePorts)
	}
}

func TestNormalizePolicyRuleSpec_UnwrapsObjects(t *testing.T) {
	rule := map[string]interface{}{
		"objects": []interface{}{
			map[string]interface{}{
				"action":  "BLOCK",
				"enabled": false,
			},
		},
	}

	result := NormalizePolicyRuleSpec(rule)

	if result["action"] != "BLOCK" {
		t.Errorf("expected action 'BLOCK', got %v", result["action"])
	}
}

func TestNormalizePolicyRuleSpec_EmptyComments(t *testing.T) {
	rule := map[string]interface{}{
		"action":   "ALLOW",
		"comments": "",
	}

	result := NormalizePolicyRuleSpec(rule)

	if _, ok := result["comments"]; ok {
		t.Error("expected empty comments to be stripped")
	}
}

func TestNormalizeJSON(t *testing.T) {
	data := map[string]interface{}{
		"zebra": "z",
		"alpha": "a",
		"nested": map[string]interface{}{
			"beta":  "b",
			"alpha": "a",
		},
	}

	result, err := NormalizeJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify keys are sorted by unmarshaling and checking order in raw string
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if parsed["alpha"] != "a" || parsed["zebra"] != "z" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestNormalizePolicyRuleEndpoint(t *testing.T) {
	// Empty endpoint remains empty for catch-all selectors
	empty := map[string]interface{}{}
	result := NormalizePolicyRuleEndpoint(empty)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if len(mapped) != 0 {
		t.Errorf("expected empty endpoint to remain empty, got %v", result)
	}

	// Endpoint with labels expression preserved
	withLabels := map[string]interface{}{"labels": map[string]interface{}{"or_labels": []interface{}{map[string]interface{}{"and_labels": []interface{}{"label-1"}}}}}
	result = NormalizePolicyRuleEndpoint(withLabels)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if _, ok := mapped["labels"]; !ok {
		t.Error("expected labels to be preserved")
	}

	// Endpoint with subnets preserved
	withSubnets := map[string]interface{}{"subnets": []interface{}{"10.0.0.0/8"}}
	result = NormalizePolicyRuleEndpoint(withSubnets)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if _, ok := mapped["subnets"]; !ok {
		t.Error("expected subnets to be preserved")
	}

	withProcesses := map[string]interface{}{
		"processes": []interface{}{"dns.exe"},
		"windows_services": []interface{}{
			map[string]interface{}{
				"service_name":        "Dnscache",
				"display_name":        "DNS Client",
				"allowed_image_names": []interface{}{"svchost.exe"},
			},
		},
	}
	result = NormalizePolicyRuleEndpoint(withProcesses)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if _, ok := mapped["processes"]; !ok {
		t.Error("expected processes to be preserved")
	}
	if _, ok := mapped["windows_services"]; !ok {
		t.Error("expected windows_services to be preserved")
	}

	// Non-map returns as-is
	result = NormalizePolicyRuleEndpoint("not-a-map")
	if result != "not-a-map" {
		t.Errorf("expected non-map to pass through, got %v", result)
	}
}

func TestNormalizePolicyRuleEndpoint_LabelGroups(t *testing.T) {
	endpoint := map[string]interface{}{"label_group_ids": []interface{}{"group-1"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if _, ok := mapped["label_group_ids"]; !ok {
		t.Error("expected label_group_ids to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_AdvancedTypedKeys(t *testing.T) {
	endpoint := map[string]interface{}{
		"user_group_ids":         []interface{}{"ug-1"},
		"asset_ids":              []interface{}{"asset-1"},
		"policy_groups":          []interface{}{"pg-1"},
		"domains":                []interface{}{"example.com"},
		"address_classification": "Private",
	}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	for _, key := range []string{"user_group_ids", "asset_ids", "policy_groups", "domains", "address_classification"} {
		if _, ok := mapped[key]; !ok {
			t.Fatalf("expected %s to be preserved", key)
		}
	}
}

func TestNormalizePolicyRuleEndpoint_AnyExternal(t *testing.T) {
	endpoint := map[string]interface{}{"any_external": true}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be map[string]interface{}")
	}
	if _, ok := mapped["any_external"]; !ok {
		t.Error("expected any_external to be preserved")
	}
}

func TestNormalizePolicyRuleSpec_NilInput(t *testing.T) {
	result := NormalizePolicyRuleSpec(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestNormalizePolicyRuleSpec_WithDestination(t *testing.T) {
	rule := map[string]interface{}{
		"action":      "ALLOW",
		"source":      map[string]interface{}{},
		"destination": map[string]interface{}{"labels": []interface{}{"label-1"}},
	}

	result := NormalizePolicyRuleSpec(rule)

	// Source should remain an empty catch-all object
	source, ok := result["source"].(map[string]interface{})
	if !ok {
		t.Fatal("expected source to be map[string]interface{}")
	}
	if len(source) != 0 {
		t.Errorf("expected source to remain empty, got %v", source)
	}

	// Legacy destination labels list should normalize to label_group_ids
	dest, ok := result["destination"].(map[string]interface{})
	if !ok {
		t.Fatal("expected destination to be map[string]interface{}")
	}
	if _, ok := dest["label_group_ids"]; !ok {
		t.Errorf("expected destination labels list to normalize to label_group_ids, got %v", dest)
	}
}

func TestNormalizeValue_Primitives(t *testing.T) {
	// String passes through
	if NormalizeValue("hello") != "hello" {
		t.Error("expected string to pass through")
	}

	// Number passes through
	if NormalizeValue(42.0) != 42.0 {
		t.Error("expected number to pass through")
	}

	// Bool passes through
	if NormalizeValue(true) != true {
		t.Error("expected bool to pass through")
	}

	// Nil passes through
	if NormalizeValue(nil) != nil {
		t.Error("expected nil to pass through")
	}
}

func TestNormalizeNumberSlice(t *testing.T) {
	input := []interface{}{float64(443), float64(80), float64(8080)}
	result, ok := NormalizeNumberSlice(input).([]interface{})
	if !ok {
		t.Fatal("expected result to be []interface{}")
	}

	expected := []int{80, 443, 8080}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("index %d: expected %d, got %v", i, expected[i], v)
		}
	}

	// Non-slice returns as-is
	nonSlice := "not-a-slice"
	if NormalizeNumberSlice(nonSlice) != nonSlice {
		t.Error("expected non-slice to pass through")
	}
}

func TestNormalizeStringSlice(t *testing.T) {
	input := []interface{}{"UDP", "TCP", "ICMP"}
	result, ok := NormalizeStringSlice(input).([]interface{})
	if !ok {
		t.Fatal("expected result to be []interface{}")
	}

	expected := []string{"ICMP", "TCP", "UDP"}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("index %d: expected %s, got %v", i, expected[i], v)
		}
	}

	// Non-slice returns as-is
	nonSlice := 42
	if NormalizeStringSlice(nonSlice) != nonSlice {
		t.Error("expected non-slice to pass through")
	}
}

func TestNormalizePolicyRuleSpec_StripsWorksite(t *testing.T) {
	rule := map[string]interface{}{
		"action":  "ALLOW",
		"enabled": true,
		"worksite": map[string]interface{}{
			"id":   "ws-1",
			"name": "Headquarters",
		},
	}

	result := NormalizePolicyRuleSpec(rule)

	if _, ok := result["worksite"]; ok {
		t.Error("expected 'worksite' to be stripped from spec")
	}
	if result["action"] != "ALLOW" {
		t.Errorf("expected action 'ALLOW', got %v", result["action"])
	}
}

func TestNormalizePolicyRuleSpec_EmptyPortsDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":  "ALLOW",
		"enabled": true,
		"ports":   []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["ports"]; ok {
		t.Error("expected empty ports to be deleted from normalized spec")
	}
}

func TestNormalizeJSONString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		{
			name:     "object keys sorted",
			input:    `{"b":2,"a":1}`,
			expected: `{"a":1,"b":2}`,
		},
		{
			name:     "array order preserved",
			input:    `[3,1,2]`,
			expected: `[3,1,2]`,
		},
		{
			name:     "nested keys sorted",
			input:    `{"z":{"b":2,"a":1},"a":0}`,
			expected: `{"a":0,"z":{"a":1,"b":2}}`,
		},
		{
			name:     "string value",
			input:    `"hello"`,
			expected: `"hello"`,
		},
		{
			name:      "invalid JSON",
			input:     `{invalid`,
			wantError: true,
		},
		{
			name:      "empty string",
			input:     ``,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeJSONString(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizePolicyRuleEndpoint_LegacyAliases(t *testing.T) {
	// label_groups -> label_group_ids
	endpoint := map[string]interface{}{"label_groups": []interface{}{"group-1"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["label_groups"]; ok {
		t.Error("expected label_groups to be removed")
	}
	if _, ok := mapped["label_group_ids"]; !ok {
		t.Error("expected label_group_ids to be present")
	}

	// user_groups -> user_group_ids
	endpoint = map[string]interface{}{"user_groups": []interface{}{"ug-1"}}
	result = NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["user_groups"]; ok {
		t.Error("expected user_groups to be removed")
	}
	if _, ok := mapped["user_group_ids"]; !ok {
		t.Error("expected user_group_ids to be present")
	}

	// assets -> asset_ids
	endpoint = map[string]interface{}{"assets": []interface{}{"asset-1"}}
	result = NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["assets"]; ok {
		t.Error("expected assets to be removed")
	}
	if _, ok := mapped["asset_ids"]; !ok {
		t.Error("expected asset_ids to be present")
	}

	// policy_group_ids -> policy_groups
	endpoint = map[string]interface{}{"policy_group_ids": []interface{}{"pg-1"}}
	result = NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["policy_group_ids"]; ok {
		t.Error("expected policy_group_ids to be removed")
	}
	if _, ok := mapped["policy_groups"]; !ok {
		t.Error("expected policy_groups to be present")
	}
}

func TestNormalizePolicyRuleEndpoint_LabelsListAlias(t *testing.T) {
	// labels as list (legacy) -> label_group_ids
	endpoint := map[string]interface{}{"labels": []interface{}{"group-1"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["labels"]; ok {
		t.Error("expected labels list to be removed")
	}
	if _, ok := mapped["label_group_ids"]; !ok {
		t.Error("expected label_group_ids to be present after labels list conversion")
	}
}

func TestNormalizePolicyRuleEndpoint_WindowsServicesAlone(t *testing.T) {
	endpoint := map[string]interface{}{
		"windows_services": []interface{}{
			map[string]interface{}{"service_name": "Dnscache"},
		},
	}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["windows_services"]; !ok {
		t.Error("expected windows_services to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_AssetIdsAlone(t *testing.T) {
	endpoint := map[string]interface{}{"asset_ids": []interface{}{"asset-1"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["asset_ids"]; !ok {
		t.Error("expected asset_ids to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_PolicyGroupsAlone(t *testing.T) {
	endpoint := map[string]interface{}{"policy_groups": []interface{}{"pg-1"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["policy_groups"]; !ok {
		t.Error("expected policy_groups to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_DomainsAlone(t *testing.T) {
	endpoint := map[string]interface{}{"domains": []interface{}{"example.com"}}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["domains"]; !ok {
		t.Error("expected domains to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_AddressClassificationAlone(t *testing.T) {
	endpoint := map[string]interface{}{"address_classification": "Private"}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := mapped["address_classification"]; !ok {
		t.Error("expected address_classification to be preserved")
	}
}

func TestNormalizePolicyRuleEndpoint_ReferenceIDObjects(t *testing.T) {
	// label_group_ids with object references
	endpoint := map[string]interface{}{
		"label_group_ids": []interface{}{
			map[string]interface{}{"id": "group-1"},
			"group-2",
		},
		"user_group_ids": []interface{}{
			map[string]interface{}{"id": "ug-1", "name": "Local Administrators"},
			"ug-2",
		},
		"asset_ids": []interface{}{
			map[string]interface{}{"id": "asset-1"},
			"asset-2",
		},
		"policy_groups": []interface{}{
			map[string]interface{}{"id": "pg-1"},
			"pg-2",
		},
	}
	result := NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	ids, ok := mapped["label_group_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected label_group_ids to be slice, got %T", mapped["label_group_ids"])
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] != "group-1" {
		t.Errorf("expected object reference extracted to 'group-1', got %v", ids[0])
	}
	if ids[1] != "group-2" {
		t.Errorf("expected string reference preserved as 'group-2', got %v", ids[1])
	}

	userGroupIDs, ok := mapped["user_group_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected user_group_ids to be slice, got %T", mapped["user_group_ids"])
	}
	if len(userGroupIDs) != 2 {
		t.Fatalf("expected 2 user_group_ids, got %d", len(userGroupIDs))
	}
	if userGroupIDs[0] != "ug-1" {
		t.Errorf("expected object reference extracted to 'ug-1', got %v", userGroupIDs[0])
	}

	assetIDs, ok := mapped["asset_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected asset_ids to be slice, got %T", mapped["asset_ids"])
	}
	if len(assetIDs) != 2 {
		t.Fatalf("expected 2 asset_ids, got %d", len(assetIDs))
	}
	if assetIDs[0] != "asset-1" {
		t.Errorf("expected object reference extracted to 'asset-1', got %v", assetIDs[0])
	}

	policyGroupIDs, ok := mapped["policy_groups"].([]interface{})
	if !ok {
		t.Fatalf("expected policy_groups to be slice, got %T", mapped["policy_groups"])
	}
	if len(policyGroupIDs) != 2 {
		t.Fatalf("expected 2 policy_groups, got %d", len(policyGroupIDs))
	}
	if policyGroupIDs[0] != "pg-1" {
		t.Errorf("expected object reference extracted to 'pg-1', got %v", policyGroupIDs[0])
	}

	// labels.or_labels[].and_labels[] with object references
	endpoint = map[string]interface{}{
		"labels": map[string]interface{}{
			"or_labels": []interface{}{
				map[string]interface{}{
					"and_labels": []interface{}{
						map[string]interface{}{"id": "label-1", "key": "Environment", "value": "Prod"},
						"label-2",
					},
				},
			},
		},
	}

	result = NormalizePolicyRuleEndpoint(endpoint)
	mapped, ok = result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result for labels normalization, got %T", result)
	}

	labels, ok := mapped["labels"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected labels map, got %T", mapped["labels"])
	}
	orLabels, ok := labels["or_labels"].([]interface{})
	if !ok || len(orLabels) != 1 {
		t.Fatalf("expected one or_labels entry, got %#v", labels["or_labels"])
	}
	orLabel, ok := orLabels[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected or_label map, got %T", orLabels[0])
	}
	andLabels, ok := orLabel["and_labels"].([]interface{})
	if !ok || len(andLabels) != 2 {
		t.Fatalf("expected two and_labels entries, got %#v", orLabel["and_labels"])
	}
	if andLabels[0] != "label-1" {
		t.Errorf("expected nested object reference extracted to 'label-1', got %v", andLabels[0])
	}
	if andLabels[1] != "label-2" {
		t.Errorf("expected nested string reference preserved as 'label-2', got %v", andLabels[1])
	}
}

func TestNormalizePolicyRuleReferenceIDs_NonSlice(t *testing.T) {
	result := normalizePolicyRuleReferenceIDs("not-a-slice")
	if result != "not-a-slice" {
		t.Errorf("expected non-slice to pass through, got %v", result)
	}
}

func TestNormalizePolicyRuleReferenceIDs_EmptyIDSkipped(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"id": ""},
		"plain-id",
	}
	result := normalizePolicyRuleReferenceIDs(input)
	ids, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} result, got %T", result)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 items, got %d", len(ids))
	}
	// Empty id object passes through as-is
	if _, ok := ids[0].(map[string]interface{}); !ok {
		t.Errorf("expected empty-id object to pass through, got %T", ids[0])
	}
	if ids[1] != "plain-id" {
		t.Errorf("expected 'plain-id', got %v", ids[1])
	}
}

func TestNormalizePolicyRuleSpec_EmptyRulesetNameStripped(t *testing.T) {
	rule := map[string]interface{}{
		"action":       "ALLOW",
		"ruleset_name": "",
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["ruleset_name"]; ok {
		t.Error("expected empty ruleset_name to be stripped")
	}
}

func TestNormalizePolicyRuleSpec_NilScheduleDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":   "ALLOW",
		"schedule": nil,
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["schedule"]; ok {
		t.Error("expected nil schedule to be deleted")
	}
}

func TestNormalizePolicyRuleSpec_EmptyIPProtocolsDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":       "ALLOW",
		"ip_protocols": []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["ip_protocols"]; ok {
		t.Error("expected empty ip_protocols to be deleted")
	}
}

func TestNormalizePolicyRuleSpec_EmptyScopeDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action": "ALLOW",
		"scope":  []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["scope"]; ok {
		t.Error("expected empty scope to be deleted")
	}
}

func TestNormalizePolicyRuleSpec_EmptyExcludePortRangesDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":              "ALLOW",
		"exclude_port_ranges": []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["exclude_port_ranges"]; ok {
		t.Error("expected empty exclude_port_ranges to be deleted")
	}
}

func TestNormalizePolicyRuleSpec_EmptyPortRangesDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":      "ALLOW",
		"port_ranges": []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["port_ranges"]; ok {
		t.Error("expected empty port_ranges to be deleted")
	}
}

func TestNormalizePolicyRuleSpec_EmptyICMPMatchesDeleted(t *testing.T) {
	rule := map[string]interface{}{
		"action":       "ALLOW",
		"icmp_matches": []interface{}{},
	}
	result := NormalizePolicyRuleSpec(rule)
	if _, ok := result["icmp_matches"]; ok {
		t.Error("expected empty icmp_matches to be deleted")
	}
}
