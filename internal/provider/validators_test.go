package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNonEmptyLabelsJSON_ValidInput(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(`{"or_labels":[{"and_labels":["label-1","label-2"]}]}`),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors, got %v", resp.Diagnostics.Errors())
	}
}

func TestNonEmptyLabelsJSON_NullValue(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringNull(),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for null, got %v", resp.Diagnostics.Errors())
	}
}

func TestNonEmptyLabelsJSON_UnknownValue(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringUnknown(),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown, got %v", resp.Diagnostics.Errors())
	}
}

func TestNonEmptyLabelsJSON_EmptyString(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(""),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for empty string, got %v", resp.Diagnostics.Errors())
	}
}

func TestNonEmptyLabelsJSON_InvalidJSON(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue("{invalid json"),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for invalid JSON")
	}
}

func TestNonEmptyLabelsJSON_EmptyOrLabels(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(`{"or_labels":[]}`),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for empty or_labels")
	}
}

func TestNonEmptyLabelsJSON_EmptyAndLabels(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(`{"or_labels":[{"and_labels":[]}]}`),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for empty and_labels")
	}
}

func TestNonEmptyLabelsJSON_MultipleOrLabels(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(`{"or_labels":[{"and_labels":["label-1"]},{"and_labels":["label-2"]}]}`),
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for multiple valid or_labels, got %v", resp.Diagnostics.Errors())
	}
}

func TestNonEmptyLabelsJSON_MixedValidAndEmpty(t *testing.T) {
	v := NonEmptyLabelsJSON()

	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("test"),
		ConfigValue: types.StringValue(`{"or_labels":[{"and_labels":["label-1"]},{"and_labels":[]}]}`),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error when one and_labels is empty")
	}
}

func TestNonEmptyLabelsJSON_Description(t *testing.T) {
	v := NonEmptyLabelsJSON()

	validator, ok := v.(nonEmptyLabelsJSONValidator)
	if !ok {
		t.Fatal("expected NonEmptyLabelsJSON to return nonEmptyLabelsJSONValidator")
	}

	desc := validator.Description(context.Background())
	if desc == "" {
		t.Error("expected non-empty description")
	}

	mdDesc := validator.MarkdownDescription(context.Background())
	if mdDesc == "" {
		t.Error("expected non-empty markdown description")
	}
}

// Policy Group validator tests
// Note: Full validation tests that require framework context (Config.GetAttribute)
// are covered by acceptance tests. Unit tests here cover basic validation logic.

func TestPolicyGroupMembersValidator_Description(t *testing.T) {
	v := PolicyGroupMembersValidator("type")

	validator, ok := v.(policyGroupMembersValidator)
	if !ok {
		t.Fatal("expected PolicyGroupMembersValidator to return policyGroupMembersValidator")
	}

	desc := validator.Description(context.Background())
	if desc == "" {
		t.Error("expected non-empty description")
	}

	mdDesc := validator.MarkdownDescription(context.Background())
	if mdDesc == "" {
		t.Error("expected non-empty markdown description")
	}
}

func TestPolicyGroupMembersValidator_NullValue(t *testing.T) {
	v := PolicyGroupMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("members_json"),
		ConfigValue: types.StringNull(),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for null, got %v", resp.Diagnostics.Errors())
	}
}

func TestPolicyGroupMembersValidator_UnknownValue(t *testing.T) {
	v := PolicyGroupMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("members_json"),
		ConfigValue: types.StringUnknown(),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown, got %v", resp.Diagnostics.Errors())
	}
}

func TestPolicyGroupMembersValidator_EmptyString(t *testing.T) {
	v := PolicyGroupMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("members_json"),
		ConfigValue: types.StringValue(""),
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for empty string")
	}
}

func TestPolicyGroupExcludeMembersValidator_NullValue(t *testing.T) {
	v := PolicyGroupExcludeMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("exclude_members_json"),
		ConfigValue: types.StringNull(),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for null, got %v", resp.Diagnostics.Errors())
	}
}

func TestPolicyGroupExcludeMembersValidator_UnknownValue(t *testing.T) {
	v := PolicyGroupExcludeMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("exclude_members_json"),
		ConfigValue: types.StringUnknown(),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown, got %v", resp.Diagnostics.Errors())
	}
}

func TestPolicyGroupExcludeMembersValidator_EmptyString(t *testing.T) {
	v := PolicyGroupExcludeMembersValidator("type")
	resp := &validator.StringResponse{}
	v.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("exclude_members_json"),
		ConfigValue: types.StringValue(""),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for empty string, got %v", resp.Diagnostics.Errors())
	}
}

func TestPolicyGroupExcludeMembersValidator_Description(t *testing.T) {
	v := PolicyGroupExcludeMembersValidator("type")

	validator, ok := v.(policyGroupExcludeMembersValidator)
	if !ok {
		t.Fatal("expected PolicyGroupExcludeMembersValidator to return policyGroupExcludeMembersValidator")
	}

	desc := validator.Description(context.Background())
	if desc == "" {
		t.Error("expected non-empty description")
	}

	mdDesc := validator.MarkdownDescription(context.Background())
	if mdDesc == "" {
		t.Error("expected non-empty markdown description")
	}
}
