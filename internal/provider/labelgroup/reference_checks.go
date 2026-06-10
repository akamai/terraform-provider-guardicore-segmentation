package labelgroup

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// validateLabelGroupSelectors validates that all label IDs referenced in
// the typed include/exclude selectors and raw JSON overlays actually exist
// in Akamai Guardicore Segmentation.
func validateLabelGroupSelectors(ctx context.Context, checker common.ReferenceChecker, data *LabelGroupResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if checker == nil {
		return diags
	}

	// Validate labels in typed include selector
	if common.IsKnown(data.Include) {
		includeSelector, d := labelGroupSelectorObjectToCreate(ctx, data.Include, "include")
		diags.Append(d...)
		if includeSelector != nil {
			for i, orGroup := range includeSelector.OrLabels {
				for j, labelID := range orGroup.AndLabels {
					location := fmt.Sprintf("include.or_groups[%d].label_ids[%d]", i, j)
					diags.Append(common.ValidateLabelExists(ctx, checker, labelID, location)...)
				}
			}
		}
	}

	// Validate labels in typed exclude selector
	if common.IsKnown(data.Exclude) {
		excludeSelector, d := labelGroupSelectorObjectToCreate(ctx, data.Exclude, "exclude")
		diags.Append(d...)
		if excludeSelector != nil {
			for i, orGroup := range excludeSelector.OrLabels {
				for j, labelID := range orGroup.AndLabels {
					location := fmt.Sprintf("exclude.or_groups[%d].label_ids[%d]", i, j)
					diags.Append(common.ValidateLabelExists(ctx, checker, labelID, location)...)
				}
			}
		}
	}

	// Validate labels in raw include JSON overlay
	if common.IsSet(data.RawIncludeJSON) {
		diags.Append(common.ValidateLabelsInJSON(ctx, checker, data.RawIncludeJSON.ValueString(), "raw_include_json")...)
	}

	// Validate labels in raw exclude JSON overlay
	if common.IsSet(data.RawExcludeJSON) {
		diags.Append(common.ValidateLabelsInJSON(ctx, checker, data.RawExcludeJSON.ValueString(), "raw_exclude_json")...)
	}

	// Ensure at least one selector is provided
	if !labelGroupSelectorProvided(data.Include, data.RawIncludeJSON) && !labelGroupSelectorProvided(data.Exclude, data.RawExcludeJSON) {
		diags.AddError(
			"Missing Label Group Selectors",
			"At least one of include, exclude, raw_include_json, or raw_exclude_json must be specified.",
		)
	}

	return diags
}
