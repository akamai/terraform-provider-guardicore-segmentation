package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func testAssetOrchestrationMetadataObject(t *testing.T, values map[string]attr.Value) types.Object {
	t.Helper()

	obj, diags := types.ObjectValue(assetOrchestrationMetadataAttrTypes(), values)
	if diags.HasError() {
		t.Fatalf("unexpected object diagnostics: %v", diags)
	}

	return obj
}

func TestBuildAssetOrchestrationMetadataJSON_WithExtra(t *testing.T) {
	ctx := context.Background()
	obj := testAssetOrchestrationMetadataObject(t, map[string]attr.Value{
		"asset_type":         types.StringValue("F5"),
		"f5_device_hostname": types.StringValue("bigip-1.example.com"),
		"partition":          types.StringValue("Common"),
		"vs_name":            types.StringValue("vs-main"),
		"extra_json":         types.StringValue(`{"tenant":"prod","enabled":true}`),
	})

	raw, diags := buildAssetOrchestrationMetadataJSON(ctx, obj)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unable to parse output JSON: %v", err)
	}

	if parsed["asset_type"] != "F5" {
		t.Fatalf("expected asset_type F5, got %#v", parsed["asset_type"])
	}
	if parsed["tenant"] != "prod" {
		t.Fatalf("expected tenant extra field, got %#v", parsed["tenant"])
	}
}

func TestBuildAssetOrchestrationMetadataJSON_RejectsNonObjectExtra(t *testing.T) {
	ctx := context.Background()
	obj := testAssetOrchestrationMetadataObject(t, map[string]attr.Value{
		"asset_type":         types.StringValue("F5"),
		"f5_device_hostname": types.StringValue("bigip-1.example.com"),
		"partition":          types.StringValue("Common"),
		"vs_name":            types.StringValue("vs-main"),
		"extra_json":         types.StringValue(`["not","an","object"]`),
	})

	_, diags := buildAssetOrchestrationMetadataJSON(ctx, obj)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for non-object extra_json")
	}
}

func TestBuildAssetOrchestrationMetadataJSON_RejectsReservedExtraKey(t *testing.T) {
	ctx := context.Background()
	obj := testAssetOrchestrationMetadataObject(t, map[string]attr.Value{
		"asset_type":         types.StringValue("F5"),
		"f5_device_hostname": types.StringValue("bigip-1.example.com"),
		"partition":          types.StringValue("Common"),
		"vs_name":            types.StringValue("vs-main"),
		"extra_json":         types.StringValue(`{"asset_type":"BAD"}`),
	})

	_, diags := buildAssetOrchestrationMetadataJSON(ctx, obj)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for reserved extra_json key")
	}
}

func TestAssetOrchestrationMetadataObjectFromAPI_SplitsKnownAndExtra(t *testing.T) {
	raw := json.RawMessage(`{"asset_type":"F5","f5_device_hostname":"f5-host","partition":"Common","vs_name":"vs-api","color":"blue"}`)

	obj, diags := assetOrchestrationMetadataObjectFromAPI(raw)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var model assetOrchestrationMetadataModel
	diags = obj.As(context.Background(), &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected conversion diagnostics: %v", diags)
	}

	if model.AssetType.ValueString() != "F5" {
		t.Fatalf("expected asset_type F5, got %q", model.AssetType.ValueString())
	}
	if model.ExtraJSON.IsNull() || model.ExtraJSON.ValueString() == "" {
		t.Fatal("expected extra_json to be populated")
	}

	var extra map[string]any
	if err := json.Unmarshal([]byte(model.ExtraJSON.ValueString()), &extra); err != nil {
		t.Fatalf("unable to parse extra_json: %v", err)
	}
	if extra["color"] != "blue" {
		t.Fatalf("expected color extra field, got %#v", extra["color"])
	}
}

func TestValidateAssetOrchestrationMetadataConfig_MissingF5Field(t *testing.T) {
	obj := testAssetOrchestrationMetadataObject(t, map[string]attr.Value{
		"asset_type":         types.StringValue("F5"),
		"f5_device_hostname": types.StringValue("bigip-1.example.com"),
		"partition":          types.StringValue("Common"),
		"vs_name":            types.StringNull(),
		"extra_json":         types.StringNull(),
	})

	var diags diag.Diagnostics
	validateAssetOrchestrationMetadataConfig(obj, path.Root("orchestration_metadata"), &diags)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for missing vs_name")
	}

	found := false
	for _, d := range diags {
		if strings.Contains(d.Detail(), "vs_name") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic mentioning vs_name, got: %v", diags)
	}
}
