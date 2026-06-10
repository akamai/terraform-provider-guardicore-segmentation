package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ContainsUnknownSentinel checks if a value contains unknown sentinel strings
// used by the Terraform Plugin Framework to represent unknown values.
func ContainsUnknownSentinel(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "74D93920-ED26-11E3-AC10-0800200C9A66" || typed == "<unknown>"
	case map[string]any:
		for _, nested := range typed {
			if ContainsUnknownSentinel(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if ContainsUnknownSentinel(nested) {
				return true
			}
		}
	}
	return false
}

// StringSliceFromList extracts a string slice from a Terraform types.List attribute.
// Null/unknown lists return nil. Unknown individual elements are represented as "<unknown>".
func StringSliceFromList(ctx context.Context, list types.List, fieldName string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var values []types.String
	diags.Append(list.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s", fieldName))
		return nil, diags
	}

	decoded := make([]string, 0, len(values))
	for _, value := range values {
		if value.IsNull() {
			continue
		}
		if value.IsUnknown() {
			decoded = append(decoded, "<unknown>")
			continue
		}
		decoded = append(decoded, value.ValueString())
	}
	return decoded, diags
}

// ListStringsToValue converts a string slice to a Terraform types.List of StringType.
// An empty slice returns a null list.
func ListStringsToValue(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

// IsSet returns true when a Terraform string attribute is known and non-null.
func IsSet(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// IsSetInt64 returns true when a Terraform int64 attribute is known and non-null.
func IsSetInt64(v types.Int64) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// IsSetBool returns true when a Terraform bool attribute is known and non-null.
func IsSetBool(v types.Bool) bool {
	return !v.IsNull() && !v.IsUnknown()
}

// IsKnown returns true when a Terraform framework value is known and non-null.
func IsKnown[T interface {
	IsNull() bool
	IsUnknown() bool
}](v T) bool {
	return !v.IsNull() && !v.IsUnknown()
}
