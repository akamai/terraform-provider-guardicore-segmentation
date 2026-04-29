package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// nonEmptyLabelsJSONValidator validates that a JSON labels string has meaningful content.
type nonEmptyLabelsJSONValidator struct{}

func (v nonEmptyLabelsJSONValidator) Description(_ context.Context) string {
	return "JSON must contain non-empty or_labels with non-empty and_labels entries"
}

func (v nonEmptyLabelsJSONValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v nonEmptyLabelsJSONValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if value == "" {
		return
	}

	var labels client.OrLabelsCreate
	if err := json.Unmarshal([]byte(value), &labels); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON",
			fmt.Sprintf("Failed to parse labels JSON: %s", err),
		)
		return
	}

	if len(labels.OrLabels) == 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Empty Labels",
			"or_labels must contain at least one entry",
		)
		return
	}

	for i, orLabel := range labels.OrLabels {
		if len(orLabel.AndLabels) == 0 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Empty Labels",
				fmt.Sprintf("or_labels[%d].and_labels must contain at least one label ID", i),
			)
		}
	}
}

// NonEmptyLabelsJSON returns a validator that checks raw label selector JSON content is meaningful.
func NonEmptyLabelsJSON() validator.String {
	return nonEmptyLabelsJSONValidator{}
}

// policyGroupMembersValidator validates that policy group members JSON structure matches the group type.
type policyGroupMembersValidator struct {
	typeAttribute string
}

func (v policyGroupMembersValidator) Description(_ context.Context) string {
	return "Policy group members JSON must match the group type (LABEL: nested arrays, FQDN: string array, IP_ADDRESS: object array)"
}

func (v policyGroupMembersValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v policyGroupMembersValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	membersJSON := req.ConfigValue.ValueString()
	if membersJSON == "" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Empty Members",
			"members_json cannot be empty",
		)
		return
	}

	// Get the type attribute value
	var groupType string
	diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName(v.typeAttribute), &groupType)
	if diags.HasError() {
		// Type is unknown or null; skip validation (will be validated at apply-time)
		return
	}

	// Validate JSON structure based on type
	switch groupType {
	case "LABEL":
		var members [][]string
		if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid LABEL Members JSON",
				fmt.Sprintf("For LABEL type, members_json must be a nested array of label IDs (e.g., [[\"id1\", \"id2\"], [\"id3\"]]): %s", err),
			)
			return
		}
		if len(members) == 0 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Empty LABEL Members",
				"For LABEL type, members_json must contain at least one OR group",
			)
			return
		}
		for i, orGroup := range members {
			if len(orGroup) == 0 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Empty AND Group",
					fmt.Sprintf("members_json[%d] (OR group) must contain at least one label ID", i),
				)
			}
		}

	case "FQDN":
		var members []string
		if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid FQDN Members JSON",
				fmt.Sprintf("For FQDN type, members_json must be a string array (e.g., [\"example.com\", \"*.example.com\"]): %s", err),
			)
			return
		}
		if len(members) == 0 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Empty FQDN Members",
				"For FQDN type, members_json must contain at least one domain",
			)
		}

	case "IP_ADDRESS":
		var members []map[string]interface{}
		if err := json.Unmarshal([]byte(membersJSON), &members); err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid IP_ADDRESS Members JSON",
				fmt.Sprintf("For IP_ADDRESS type, members_json must be an object array with 'subnet' or 'range' fields: %s", err),
			)
			return
		}
		if len(members) == 0 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Empty IP_ADDRESS Members",
				"For IP_ADDRESS type, members_json must contain at least one IP address entry",
			)
		}

	default:
		// Unknown type; skip validation
		return
	}
}

// PolicyGroupMembersValidator returns a validator that checks policy group members JSON matches the type.
func PolicyGroupMembersValidator(typeAttribute string) validator.String {
	return policyGroupMembersValidator{typeAttribute: typeAttribute}
}

// policyGroupExcludeMembersValidator validates that exclude_members is only used with LABEL type.
type policyGroupExcludeMembersValidator struct {
	typeAttribute string
}

func (v policyGroupExcludeMembersValidator) Description(_ context.Context) string {
	return "exclude_members_json can only be used with LABEL type policy groups"
}

func (v policyGroupExcludeMembersValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v policyGroupExcludeMembersValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	excludeMembersJSON := req.ConfigValue.ValueString()
	if excludeMembersJSON == "" {
		return
	}

	// Get the type attribute value
	var groupType string
	diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName(v.typeAttribute), &groupType)
	if diags.HasError() {
		// Type is unknown or null; skip validation
		return
	}

	if groupType != "LABEL" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute for Type",
			fmt.Sprintf("exclude_members_json can only be used with LABEL type policy groups, but type is %q", groupType),
		)
	}
}

// PolicyGroupExcludeMembersValidator returns a validator that checks exclude_members is only used with LABEL type.
func PolicyGroupExcludeMembersValidator(typeAttribute string) validator.String {
	return policyGroupExcludeMembersValidator{typeAttribute: typeAttribute}
}
