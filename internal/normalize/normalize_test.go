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
