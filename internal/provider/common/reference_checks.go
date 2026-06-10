package common

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// ReferenceChecker defines the interface for validating resource references.
// The *client.Client struct satisfies this interface naturally.
type ReferenceChecker interface {
	GetLabel(ctx context.Context, id string) (*client.Label, error)
	ListLabels(ctx context.Context, key, value string) ([]client.Label, error)
	GetLabelGroup(ctx context.Context, id string) (*client.LabelGroup, error)
	GetUserGroup(ctx context.Context, id string) (*client.UserGroup, error)
	GetAsset(ctx context.Context, id string) (*client.Asset, error)
	GetWorksite(ctx context.Context, id string) (*client.Worksite, error)
	GetPolicyGroup(ctx context.Context, id string) (*client.PolicyGroup, error)
}

func ValidateLabelExists(ctx context.Context, checker ReferenceChecker, labelID, location string) diag.Diagnostics {
	var diags diag.Diagnostics

	if labelID == "" || ContainsUnknownSentinel(labelID) {
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

func ValidateLabelGroupExists(ctx context.Context, checker ReferenceChecker, labelGroupID, location string) diag.Diagnostics {
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

func ValidateUserGroupExists(ctx context.Context, checker ReferenceChecker, userGroupID, location string) diag.Diagnostics {
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

func ValidateAssetExists(ctx context.Context, checker ReferenceChecker, assetID, location string) diag.Diagnostics {
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

func ValidateWorksiteExists(ctx context.Context, checker ReferenceChecker, worksiteID string) diag.Diagnostics {
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

func ValidateLabelsInJSON(ctx context.Context, checker ReferenceChecker, labelsJSON, fieldName string) diag.Diagnostics {
	var diags diag.Diagnostics

	if labelsJSON == "" {
		return diags
	}

	var labels client.OrLabelsCreate
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		diags.AddAttributeError(
			path.Root(fieldName),
			fmt.Sprintf("Invalid JSON in %s", fieldName),
			err.Error(),
		)
		return diags
	}

	for i, orLabel := range labels.OrLabels {
		for j, labelID := range orLabel.AndLabels {
			location := fmt.Sprintf("%s.or_labels[%d].and_labels[%d]", fieldName, i, j)
			diags.Append(ValidateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

func ValidatePolicyRuleRefs(ctx context.Context, checker ReferenceChecker, specJSON string) diag.Diagnostics {
	var diags diag.Diagnostics

	if specJSON == "" {
		return diags
	}

	var spec map[string]any
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		diags.AddAttributeError(
			path.Root("raw_spec_json"),
			"Invalid JSON in raw_spec_json",
			err.Error(),
		)
		return diags
	}

	return ValidatePolicyRuleRefsMap(ctx, checker, spec)
}

func ValidatePolicyRuleRefsMap(ctx context.Context, checker ReferenceChecker, spec map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	if spec == nil {
		return diags
	}

	type refValidator func(ctx context.Context, checker ReferenceChecker, id, location string) diag.Diagnostics
	refKeyValidators := map[string]refValidator{
		"label_group_ids": ValidateLabelGroupExists,
		"user_group_ids":  ValidateUserGroupExists,
		"asset_ids":       ValidateAssetExists,
		"policy_groups":   ValidatePolicyGroupExists,
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

			if shapeDiags := ValidatePolicyRuleEndpointReferenceShape(endpointKey, refKey, refs); shapeDiags.HasError() {
				diags.Append(shapeDiags...)
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
					diags.Append(ValidateLabelExists(ctx, checker, labelID, location)...)
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
			diags.Append(ValidateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

func ValidatePolicyRuleEndpointReferenceShape(endpointKey, refKey string, refs any) diag.Diagnostics {
	var diags diag.Diagnostics

	var refSlice []any
	switch typed := refs.(type) {
	case []any:
		refSlice = typed
	case []string:
		return diags
	default:
		return diags
	}

	for i, ref := range refSlice {
		if _, ok := ref.(string); ok {
			continue
		}
		location := fmt.Sprintf("policy_rule.%s.%s[%d]", endpointKey, refKey, i)
		diags.AddError(
			"Invalid Policy Rule Endpoint Reference",
			fmt.Sprintf("Expected %s to be a string ID, got %T. Use a list of IDs in %s (for example: [\"id-1\", \"id-2\"]).", location, ref, location),
		)
	}

	return diags
}

func ValidatePolicyGroupExists(ctx context.Context, checker ReferenceChecker, policyGroupID, location string) diag.Diagnostics {
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

func ValidatePolicyGroupLabelRefs(ctx context.Context, checker ReferenceChecker, membersJSON, fieldName string) diag.Diagnostics {
	var diags diag.Diagnostics

	if membersJSON == "" {
		return diags
	}

	var memberArrays [][]string
	if err := json.Unmarshal([]byte(membersJSON), &memberArrays); err != nil {
		diags.AddAttributeError(
			path.Root(fieldName),
			fmt.Sprintf("Invalid JSON in %s", fieldName),
			err.Error(),
		)
		return diags
	}

	for i, orGroup := range memberArrays {
		for j, labelID := range orGroup {
			location := fmt.Sprintf("%s[%d][%d]", fieldName, i, j)
			diags.Append(ValidateLabelExists(ctx, checker, labelID, location)...)
		}
	}

	return diags
}

func CountPolicyGroupMembers(membersJSON, groupType string) (int, error) {
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
