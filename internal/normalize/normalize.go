package normalize

import (
	"encoding/json"
	"sort"
)

func normalizePolicyRuleReferenceIDs(value interface{}) interface{} {
	slice, ok := value.([]interface{})
	if !ok {
		return value
	}

	normalized := make([]interface{}, 0, len(slice))
	for _, raw := range slice {
		if mapped, ok := raw.(map[string]interface{}); ok {
			if id, ok := mapped["id"].(string); ok && id != "" {
				normalized = append(normalized, id)
				continue
			}
		}
		normalized = append(normalized, raw)
	}

	return normalized
}

func normalizePolicyRuleLabelsExpression(value interface{}) interface{} {
	labels, ok := value.(map[string]interface{})
	if !ok {
		return value
	}

	rawOrLabels, ok := labels["or_labels"].([]interface{})
	if !ok {
		return labels
	}

	for i, rawOrLabel := range rawOrLabels {
		orLabel, ok := rawOrLabel.(map[string]interface{})
		if !ok {
			continue
		}

		if andLabels, ok := orLabel["and_labels"]; ok {
			orLabel["and_labels"] = normalizePolicyRuleReferenceIDs(andLabels)
		}
		rawOrLabels[i] = orLabel
	}

	labels["or_labels"] = rawOrLabels
	return labels
}

// NormalizeJSON normalizes a JSON object by sorting keys to ensure consistent output.
func NormalizeJSON(data map[string]interface{}) (string, error) {
	normalized := NormalizeValue(data)
	bytes, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// NormalizeValue recursively normalizes a value, sorting map keys.
func NormalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeMap(val)
	case []interface{}:
		return normalizeSlice(val)
	default:
		return v
	}
}

// NormalizePolicyRuleSpec strips server-side fields and normalizes endpoints, ports, and protocols.
func NormalizePolicyRuleSpec(rule map[string]interface{}) map[string]interface{} {
	if rule == nil {
		return nil
	}

	if rawObjects, ok := rule["objects"]; ok {
		if objects, ok := rawObjects.([]interface{}); ok && len(objects) > 0 {
			if first, ok := objects[0].(map[string]interface{}); ok {
				rule = first
			}
		}
	}

	filtered := make(map[string]interface{})
	for key, value := range rule {
		switch key {
		case "id", "created_at", "updated_at", "author", "creation_time", "last_change_time", "hit_count", "hit_count_reset_time", "last_hit", "state", "read_only", "objects", "worksite", "attributes", "creation_origin":
			continue
		default:
			if key == "comments" {
				if comment, ok := value.(string); ok && comment == "" {
					continue
				}
			}
			if key == "ruleset_name" {
				if name, ok := value.(string); ok && name == "" {
					continue
				}
			}
			if key == "network_profile" {
				if profile, ok := value.(string); ok && profile == "CORPORATE" {
					continue
				}
			}
			if key == "schedule" && value == nil {
				continue
			}
			filtered[key] = value
		}
	}

	normalizedValue := NormalizeValue(filtered)
	normalized, ok := normalizedValue.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	if source, ok := normalized["source"]; ok {
		normalized["source"] = NormalizePolicyRuleEndpoint(source)
	}
	if destination, ok := normalized["destination"]; ok {
		normalized["destination"] = NormalizePolicyRuleEndpoint(destination)
	}

	if ports, ok := normalized["ports"]; ok {
		normalized["ports"] = NormalizeNumberSlice(ports)
		if items, ok := normalized["ports"].([]interface{}); ok && len(items) == 0 {
			delete(normalized, "ports")
		}
	}
	if ports, ok := normalized["exclude_ports"]; ok {
		normalized["exclude_ports"] = NormalizeNumberSlice(ports)
	}
	if protocols, ok := normalized["ip_protocols"]; ok {
		normalized["ip_protocols"] = NormalizeStringSlice(protocols)
		if items, ok := normalized["ip_protocols"].([]interface{}); ok && len(items) == 0 {
			delete(normalized, "ip_protocols")
		}
	}
	if scope, ok := normalized["scope"]; ok {
		normalizedScope := NormalizeStringSlice(scope)
		if items, ok := normalizedScope.([]interface{}); ok && len(items) == 0 {
			delete(normalized, "scope")
		} else {
			normalized["scope"] = normalizedScope
		}
	}
	if ranges, ok := normalized["port_ranges"]; ok {
		if items, ok := ranges.([]interface{}); ok && len(items) == 0 {
			delete(normalized, "port_ranges")
		}
	}
	if ranges, ok := normalized["exclude_port_ranges"]; ok {
		if items, ok := ranges.([]interface{}); ok && len(items) == 0 {
			delete(normalized, "exclude_port_ranges")
		}
	}
	if ports, ok := normalized["exclude_ports"]; ok {
		if items, ok := ports.([]interface{}); ok && len(items) == 0 {
			delete(normalized, "exclude_ports")
		}
	}
	if matches, ok := normalized["icmp_matches"]; ok {
		if items, ok := matches.([]interface{}); ok && len(items) == 0 {
			delete(normalized, "icmp_matches")
		}
	}

	return normalized
}

// NormalizeNumberSlice converts float64 values to sorted ints.
func NormalizeNumberSlice(value interface{}) interface{} {
	slice, ok := value.([]interface{})
	if !ok {
		return value
	}

	items := make([]int, 0, len(slice))
	for _, raw := range slice {
		if number, ok := raw.(float64); ok {
			items = append(items, int(number))
		}
	}

	sort.Ints(items)

	normalized := make([]interface{}, len(items))
	for i, number := range items {
		normalized[i] = number
	}

	return normalized
}

// NormalizeStringSlice sorts string values.
func NormalizeStringSlice(value interface{}) interface{} {
	slice, ok := value.([]interface{})
	if !ok {
		return value
	}

	items := make([]string, 0, len(slice))
	for _, raw := range slice {
		if text, ok := raw.(string); ok {
			items = append(items, text)
		}
	}

	sort.Strings(items)

	normalized := make([]interface{}, len(items))
	for i, text := range items {
		normalized[i] = text
	}

	return normalized
}

// NormalizePolicyRuleEndpoint normalizes an endpoint while preserving empty objects for catch-all selectors.
func NormalizePolicyRuleEndpoint(endpoint interface{}) interface{} {
	mapped, ok := endpoint.(map[string]interface{})
	if !ok {
		return endpoint
	}

	if value, ok := mapped["label_groups"]; ok {
		mapped["label_group_ids"] = value
		delete(mapped, "label_groups")
	}
	if value, ok := mapped["user_groups"]; ok {
		mapped["user_group_ids"] = value
		delete(mapped, "user_groups")
	}
	if value, ok := mapped["assets"]; ok {
		mapped["asset_ids"] = value
		delete(mapped, "assets")
	}
	if value, ok := mapped["policy_group_ids"]; ok {
		mapped["policy_groups"] = value
		delete(mapped, "policy_group_ids")
	}
	if value, ok := mapped["labels"]; ok {
		if _, isList := value.([]interface{}); isList {
			mapped["label_group_ids"] = value
			delete(mapped, "labels")
		}
	}

	if _, ok := mapped["label_group_ids"]; ok {
		mapped["label_group_ids"] = normalizePolicyRuleReferenceIDs(mapped["label_group_ids"])
	}

	if _, ok := mapped["user_group_ids"]; ok {
		mapped["user_group_ids"] = normalizePolicyRuleReferenceIDs(mapped["user_group_ids"])
	}

	if _, ok := mapped["asset_ids"]; ok {
		mapped["asset_ids"] = normalizePolicyRuleReferenceIDs(mapped["asset_ids"])
	}

	if _, ok := mapped["policy_groups"]; ok {
		mapped["policy_groups"] = normalizePolicyRuleReferenceIDs(mapped["policy_groups"])
	}

	if _, ok := mapped["labels"]; ok {
		mapped["labels"] = normalizePolicyRuleLabelsExpression(mapped["labels"])
		return mapped
	}

	if _, ok := mapped["label_group_ids"]; ok {
		return mapped
	}

	if _, ok := mapped["subnets"]; ok {
		return mapped
	}

	if _, ok := mapped["processes"]; ok {
		return mapped
	}

	if _, ok := mapped["windows_services"]; ok {
		return mapped
	}

	if _, ok := mapped["user_group_ids"]; ok {
		return mapped
	}

	if _, ok := mapped["asset_ids"]; ok {
		return mapped
	}

	if _, ok := mapped["policy_groups"]; ok {
		return mapped
	}

	if _, ok := mapped["domains"]; ok {
		return mapped
	}

	if _, ok := mapped["address_classification"]; ok {
		return mapped
	}

	if _, ok := mapped["any_external"]; ok {
		return mapped
	}

	return map[string]interface{}{}
}

// normalizeMap normalizes a map by sorting its keys.
func normalizeMap(m map[string]interface{}) map[string]interface{} {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make(map[string]interface{})
	for _, k := range keys {
		result[k] = NormalizeValue(m[k])
	}
	return result
}

// normalizeSlice normalizes a slice by normalizing each element.
func normalizeSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = NormalizeValue(v)
	}
	return result
}

// NormalizeJSONString normalizes a JSON string by unmarshaling and remarshaling with sorted keys.
func NormalizeJSONString(jsonStr string) (string, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}
