package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ReferenceChecker defines the interface for validating resource references.
// The *client.Client struct satisfies this interface naturally.
type ReferenceChecker interface {
	GetLabel(ctx context.Context, id string) (*client.Label, error)
	GetLabelGroup(ctx context.Context, id string) (*client.LabelGroup, error)
	GetUserGroup(ctx context.Context, id string) (*client.UserGroup, error)
	GetAsset(ctx context.Context, id string) (*client.Asset, error)
	GetWorksite(ctx context.Context, id string) (*client.Worksite, error)
	GetPolicyGroup(ctx context.Context, id string) (*client.PolicyGroup, error)
}

// validateLabelExists checks if a label ID exists in Akamai Guardicore Segmentation.
func validateLabelExists(ctx context.Context, checker ReferenceChecker, labelID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if labelID == "" || containsUnknownSentinel(labelID) {
		return diags
	}

	label, err := checker.GetLabel(ctx, labelID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify label %q referenced in %s: %s", labelID, location, err),
		)
		return diags
	}

	if label == nil {
		diags.AddError(
			"Invalid Label Reference",
			fmt.Sprintf("Label %q referenced in %s does not exist in Akamai Guardicore Segmentation.", labelID, location),
		)
	}

	return diags
}

// validateLabelGroupExists checks if a label group ID exists in Akamai Guardicore Segmentation.
func validateLabelGroupExists(ctx context.Context, checker ReferenceChecker, labelGroupID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if labelGroupID == "" {
		return diags
	}

	group, err := checker.GetLabelGroup(ctx, labelGroupID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify label group %q referenced in %s: %s", labelGroupID, location, err),
		)
		return diags
	}

	if group == nil {
		diags.AddError(
			"Invalid Label Group Reference",
			fmt.Sprintf("Label group %q referenced in %s does not exist in Akamai Guardicore Segmentation.", labelGroupID, location),
		)
	}

	return diags
}

// validateUserGroupExists checks if a user group ID exists in Akamai Guardicore Segmentation.
func validateUserGroupExists(ctx context.Context, checker ReferenceChecker, userGroupID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if userGroupID == "" {
		return diags
	}

	group, err := checker.GetUserGroup(ctx, userGroupID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify user group %q referenced in %s: %s", userGroupID, location, err),
		)
		return diags
	}

	if group == nil {
		diags.AddError(
			"Invalid User Group Reference",
			fmt.Sprintf("User group %q referenced in %s does not exist in Akamai Guardicore Segmentation.", userGroupID, location),
		)
	}

	return diags
}

// validateAssetExists checks if an asset ID exists in Akamai Guardicore Segmentation.
func validateAssetExists(ctx context.Context, checker ReferenceChecker, assetID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if assetID == "" {
		return diags
	}

	asset, err := checker.GetAsset(ctx, assetID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify asset %q referenced in %s: %s", assetID, location, err),
		)
		return diags
	}

	if asset == nil {
		diags.AddError(
			"Invalid Asset Reference",
			fmt.Sprintf("Asset %q referenced in %s does not exist in Akamai Guardicore Segmentation.", assetID, location),
		)
	}

	return diags
}

// validateWorksiteExists checks if a worksite ID exists in Akamai Guardicore Segmentation.
func validateWorksiteExists(ctx context.Context, checker ReferenceChecker, worksiteID string) diag.Diagnostics {
	var diags diag.Diagnostics

	if worksiteID == "" {
		return diags
	}

	worksite, err := checker.GetWorksite(ctx, worksiteID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify worksite %q referenced in worksite_id: %s", worksiteID, err),
		)
		return diags
	}

	if worksite == nil {
		diags.AddError(
			"Invalid Worksite Reference",
			fmt.Sprintf("Worksite %q referenced in worksite_id does not exist in Akamai Guardicore Segmentation.", worksiteID),
		)
	}

	return diags
}

// validateLabelsInJSON parses a labels JSON string (OrLabelsCreate format)
// and validates that every label ID exists in Akamai Guardicore Segmentation.
func validateLabelsInJSON(ctx context.Context, checker ReferenceChecker, labelsJSON, fieldName string) diag.Diagnostics {
	var diags diag.Diagnostics

	if labelsJSON == "" {
		return diags
	}

	var labels client.OrLabelsCreate
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		// JSON parsing errors are handled by the existing NonEmptyLabelsJSON validator.
		return diags
	}

	for i, orLabel := range labels.OrLabels {
		for j, labelID := range orLabel.AndLabels {
			location := fmt.Sprintf("%s.or_labels[%d].and_labels[%d]", fieldName, i, j)
			diags.Append(validateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

func validateLabelGroupSelector(ctx context.Context, checker ReferenceChecker, object types.Object, raw types.String, typedFieldName, rawFieldName string) diag.Diagnostics {
	var diags diag.Diagnostics

	resolved, d := resolveLabelGroupSelector(ctx, object, raw, typedFieldName, rawFieldName)
	diags.Append(d...)
	if diags.HasError() || resolved == nil {
		return diags
	}
	if containsUnknownSentinel(resolved) {
		return diags
	}

	for i, orLabel := range resolved.OrLabels {
		for j, labelID := range orLabel.AndLabels {
			location := fmt.Sprintf("%s.or_groups[%d].label_ids[%d]", typedFieldName, i, j)
			if object.IsNull() || object.IsUnknown() {
				location = fmt.Sprintf("%s.or_labels[%d].and_labels[%d]", rawFieldName, i, j)
			}
			diags.Append(validateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

func validateLabelGroupSelectors(ctx context.Context, checker ReferenceChecker, data *LabelGroupResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if !labelGroupSelectorProvided(data.Include, data.RawIncludeJSON) && !labelGroupSelectorProvided(data.Exclude, data.RawExcludeJSON) {
		diags.AddError(
			"Missing Label Group Selectors",
			"Set at least one of `include`, `exclude`, `raw_include_json`, or `raw_exclude_json`.",
		)
		return diags
	}

	if checker == nil {
		return diags
	}

	diags.Append(validateLabelGroupSelector(ctx, checker, data.Include, data.RawIncludeJSON, "include", "raw_include_json")...)
	diags.Append(validateLabelGroupSelector(ctx, checker, data.Exclude, data.RawExcludeJSON, "exclude", "raw_exclude_json")...)
	return diags
}

// validatePolicyRuleRefs parses a policy rule spec JSON and validates all typed
// references in source and destination endpoints.
func validatePolicyRuleRefs(ctx context.Context, checker ReferenceChecker, specJSON string) diag.Diagnostics {
	var diags diag.Diagnostics

	if specJSON == "" {
		return diags
	}

	var spec map[string]any
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		// JSON parsing errors are handled elsewhere.
		return diags
	}

	return validatePolicyRuleRefsMap(ctx, checker, spec)
}

func validatePolicyRuleRefsMap(ctx context.Context, checker ReferenceChecker, spec map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	if spec == nil {
		return diags
	}

	// refKeyValidators maps endpoint keys to their validation functions.
	type refValidator func(ctx context.Context, checker ReferenceChecker, id, location string) diag.Diagnostics
	refKeyValidators := map[string]refValidator{
		"label_group_ids": validateLabelGroupExists,
		"user_group_ids":  validateUserGroupExists,
		"asset_ids":       validateAssetExists,
		"policy_groups":   validatePolicyGroupExists,
	}

	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := spec[endpointKey]
		if !ok {
			continue
		}

		endpointMap, ok := endpoint.(map[string]any)
		if !ok {
			continue
		}

		for refKey, validateFn := range refKeyValidators {
			refs, ok := endpointMap[refKey]
			if !ok {
				continue
			}

			var refIDs []string
			switch typed := refs.(type) {
			case []any:
				for _, ref := range typed {
					if refID, ok := ref.(string); ok {
						refIDs = append(refIDs, refID)
					}
				}
			case []string:
				refIDs = append(refIDs, typed...)
			default:
				continue
			}

			for i, refID := range refIDs {
				if refID == "" || refID == "<unknown>" {
					continue
				}
				location := fmt.Sprintf("policy_rule.%s.%s[%d]", endpointKey, refKey, i)
				diags.Append(validateFn(ctx, checker, refID, location)...)
			}
		}

		if labels, ok := endpointMap["labels"]; ok {
			labelsMap, ok := labels.(map[string]any)
			if !ok {
				continue
			}
			rawOrLabels, ok := labelsMap["or_labels"].([]any)
			if !ok {
				continue
			}
			for i, rawOrLabel := range rawOrLabels {
				orLabelMap, ok := rawOrLabel.(map[string]any)
				if !ok {
					continue
				}
				rawAndLabels, ok := orLabelMap["and_labels"].([]any)
				if !ok {
					continue
				}
				for j, rawLabelID := range rawAndLabels {
					labelID, ok := rawLabelID.(string)
					if !ok {
						continue
					}
					location := fmt.Sprintf("policy_rule.%s.labels.or_labels[%d].and_labels[%d]", endpointKey, i, j)
					diags.Append(validateLabelExists(ctx, checker, labelID, location)...)
				}
			}
		}
	}

	if scope, ok := spec["scope"]; ok {
		var scopeIDs []string
		switch typed := scope.(type) {
		case []any:
			for _, ref := range typed {
				if refID, ok := ref.(string); ok {
					scopeIDs = append(scopeIDs, refID)
				}
			}
		case []string:
			scopeIDs = append(scopeIDs, typed...)
		}

		for i, labelID := range scopeIDs {
			if labelID == "" || labelID == "<unknown>" {
				continue
			}
			location := fmt.Sprintf("policy_rule.scope[%d]", i)
			diags.Append(validateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

// validateAssetLabelRefs validates that all label references in an asset's
// labels block exist in Akamai Guardicore Segmentation (by ID).
func validateAssetLabelRefs(ctx context.Context, checker ReferenceChecker, labels []AssetLabelModel) diag.Diagnostics {
	var diags diag.Diagnostics

	for i, label := range labels {
		if label.ID.IsNull() || label.ID.IsUnknown() || label.ID.ValueString() == "" {
			continue
		}
		location := fmt.Sprintf("labels[%d]", i)
		diags.Append(validateLabelExists(ctx, checker, label.ID.ValueString(), location)...)
	}

	return diags
}

// validatePolicyGroupExists checks if a policy group ID exists in Akamai Guardicore Segmentation.
func validatePolicyGroupExists(ctx context.Context, checker ReferenceChecker, policyGroupID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if policyGroupID == "" {
		return diags
	}

	group, err := checker.GetPolicyGroup(ctx, policyGroupID)
	if err != nil {
		diags.AddError(
			"Reference Validation Error",
			fmt.Sprintf("Unable to verify policy group %q referenced in %s: %s", policyGroupID, location, err),
		)
		return diags
	}

	if group == nil {
		diags.AddError(
			"Invalid Policy Group Reference",
			fmt.Sprintf("Policy group %q referenced in %s does not exist in Akamai Guardicore Segmentation.", policyGroupID, location),
		)
	}

	return diags
}

// validatePolicyGroupLabelRefs validates label IDs in LABEL type policy group members.
// membersJSON should be the include_members or exclude_members JSON string.
func validatePolicyGroupLabelRefs(ctx context.Context, checker ReferenceChecker, membersJSON, fieldName string) diag.Diagnostics {
	var diags diag.Diagnostics

	if membersJSON == "" {
		return diags
	}

	// Parse as nested array of label IDs: [[id1, id2], [id3]]
	var memberArrays [][]string
	if err := json.Unmarshal([]byte(membersJSON), &memberArrays); err != nil {
		// JSON parsing errors are handled by validators
		return diags
	}

	for i, orGroup := range memberArrays {
		for j, labelID := range orGroup {
			location := fmt.Sprintf("%s[%d][%d]", fieldName, i, j)
			diags.Append(validateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

// countPolicyGroupMembers counts the total number of members in policy group JSON.
// For LABEL type: counts total label IDs across all OR groups.
// For FQDN type: counts string array elements.
// For IP_ADDRESS type: counts object array elements.
func countPolicyGroupMembers(membersJSON, groupType string) (int, error) {
	if membersJSON == "" {
		return 0, nil
	}

	switch groupType {
	case "LABEL":
		var memberArrays [][]string
		if err := json.Unmarshal([]byte(membersJSON), &memberArrays); err != nil {
			return 0, err
		}
		count := 0
		for _, orGroup := range memberArrays {
			count += len(orGroup)
		}
		return count, nil

	case "FQDN":
		var members []string
		if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
			return 0, err
		}
		return len(members), nil

	case "IP_ADDRESS":
		var members []map[string]interface{}
		if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
			return 0, err
		}
		return len(members), nil

	default:
		return 0, fmt.Errorf("unknown policy group type: %s", groupType)
	}
}
