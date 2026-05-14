package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const (
	assetOrchestrationMetadataFieldAssetType        = "asset_type"
	assetOrchestrationMetadataFieldF5DeviceHostname = "f5_device_hostname"
	assetOrchestrationMetadataFieldPartition        = "partition"
	assetOrchestrationMetadataFieldVSName           = "vs_name"
	assetOrchestrationMetadataFieldExtraJSON        = "extra_json"
)

var assetOrchestrationMetadataKnownFields = map[string]struct{}{
	assetOrchestrationMetadataFieldAssetType:        {},
	assetOrchestrationMetadataFieldF5DeviceHostname: {},
	assetOrchestrationMetadataFieldPartition:        {},
	assetOrchestrationMetadataFieldVSName:           {},
}

type assetOrchestrationMetadataModel struct {
	AssetType        types.String `tfsdk:"asset_type"`
	F5DeviceHostname types.String `tfsdk:"f5_device_hostname"`
	Partition        types.String `tfsdk:"partition"`
	VSName           types.String `tfsdk:"vs_name"`
	ExtraJSON        types.String `tfsdk:"extra_json"`
}

func assetOrchestrationMetadataAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		assetOrchestrationMetadataFieldAssetType:        types.StringType,
		assetOrchestrationMetadataFieldF5DeviceHostname: types.StringType,
		assetOrchestrationMetadataFieldPartition:        types.StringType,
		assetOrchestrationMetadataFieldVSName:           types.StringType,
		assetOrchestrationMetadataFieldExtraJSON:        types.StringType,
	}
}

func assetOrchestrationMetadataResourceSchemaAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]resourceschema.Attribute{
			assetOrchestrationMetadataFieldAssetType: resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Asset orchestration metadata type. Must be `F5`.",
				Validators: []validator.String{
					stringvalidator.OneOf("F5"),
				},
			},
			assetOrchestrationMetadataFieldF5DeviceHostname: resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname of the F5 BigIP device (case-sensitive).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			assetOrchestrationMetadataFieldPartition: resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Partition where the F5 virtual server is created (case-sensitive).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			assetOrchestrationMetadataFieldVSName: resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the F5 virtual server (case-sensitive).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			assetOrchestrationMetadataFieldExtraJSON: resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Additional metadata as a JSON object string. Use `jsonencode({ ... })`. Keys must not overlap with typed fields (`asset_type`, `f5_device_hostname`, `partition`, `vs_name`).",
			},
		},
	}
}

func assetOrchestrationMetadataDataSourceSchemaAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]schema.Attribute{
			assetOrchestrationMetadataFieldAssetType:        schema.StringAttribute{Computed: true},
			assetOrchestrationMetadataFieldF5DeviceHostname: schema.StringAttribute{Computed: true},
			assetOrchestrationMetadataFieldPartition:        schema.StringAttribute{Computed: true},
			assetOrchestrationMetadataFieldVSName:           schema.StringAttribute{Computed: true},
			assetOrchestrationMetadataFieldExtraJSON:        schema.StringAttribute{Computed: true},
		},
	}
}

func buildAssetOrchestrationMetadataJSON(ctx context.Context, obj types.Object) (json.RawMessage, diag.Diagnostics) {
	var diags diag.Diagnostics
	const fieldName = "orchestration_metadata"
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var model assetOrchestrationMetadataModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	if model.AssetType.IsNull() || model.AssetType.IsUnknown() || model.AssetType.ValueString() == "" {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("%s.asset_type is required", fieldName))
		return nil, diags
	}
	if model.AssetType.ValueString() != "F5" {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("%s.asset_type must be \"F5\"", fieldName))
		return nil, diags
	}

	if model.F5DeviceHostname.IsNull() || model.F5DeviceHostname.IsUnknown() || model.F5DeviceHostname.ValueString() == "" {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("%s.f5_device_hostname is required for asset_type \"F5\"", fieldName))
	}
	if model.Partition.IsNull() || model.Partition.IsUnknown() || model.Partition.ValueString() == "" {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("%s.partition is required for asset_type \"F5\"", fieldName))
	}
	if model.VSName.IsNull() || model.VSName.IsUnknown() || model.VSName.ValueString() == "" {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("%s.vs_name is required for asset_type \"F5\"", fieldName))
	}
	if diags.HasError() {
		return nil, diags
	}

	result := map[string]any{
		assetOrchestrationMetadataFieldAssetType:        model.AssetType.ValueString(),
		assetOrchestrationMetadataFieldF5DeviceHostname: model.F5DeviceHostname.ValueString(),
		assetOrchestrationMetadataFieldPartition:        model.Partition.ValueString(),
		assetOrchestrationMetadataFieldVSName:           model.VSName.ValueString(),
	}

	extra, extraDiags := parseAssetOrchestrationMetadataExtraJSON(model.ExtraJSON, fieldName+"."+assetOrchestrationMetadataFieldExtraJSON)
	diags.Append(extraDiags...)
	if diags.HasError() {
		return nil, diags
	}

	for k, v := range extra {
		if _, exists := assetOrchestrationMetadataKnownFields[k]; exists {
			diags.AddError(
				"Invalid Orchestration Metadata",
				fmt.Sprintf("%s.%s contains reserved key %q; use typed attributes instead", fieldName, assetOrchestrationMetadataFieldExtraJSON, k),
			)
			continue
		}
		result[k] = v
	}
	if diags.HasError() {
		return nil, diags
	}

	raw, err := json.Marshal(result)
	if err != nil {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("Unable to encode %s: %s", fieldName, err))
		return nil, diags
	}

	return json.RawMessage(raw), diags
}

func assetOrchestrationMetadataObjectFromAPI(raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(raw) == 0 || string(raw) == "null" {
		return types.ObjectNull(assetOrchestrationMetadataAttrTypes()), diags
	}

	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("Unable to parse orchestration metadata from API: %s", err))
		return types.ObjectNull(assetOrchestrationMetadataAttrTypes()), diags
	}

	values := map[string]attr.Value{
		assetOrchestrationMetadataFieldAssetType:        types.StringNull(),
		assetOrchestrationMetadataFieldF5DeviceHostname: types.StringNull(),
		assetOrchestrationMetadataFieldPartition:        types.StringNull(),
		assetOrchestrationMetadataFieldVSName:           types.StringNull(),
		assetOrchestrationMetadataFieldExtraJSON:        types.StringNull(),
	}

	for _, key := range []string{
		assetOrchestrationMetadataFieldAssetType,
		assetOrchestrationMetadataFieldF5DeviceHostname,
		assetOrchestrationMetadataFieldPartition,
		assetOrchestrationMetadataFieldVSName,
	} {
		if value, ok := metadata[key]; ok {
			strValue, ok := value.(string)
			if !ok {
				diags.AddError(
					"Invalid Orchestration Metadata",
					fmt.Sprintf("API returned orchestration metadata field %q as non-string value", key),
				)
				return types.ObjectNull(assetOrchestrationMetadataAttrTypes()), diags
			}
			values[key] = types.StringValue(strValue)
			delete(metadata, key)
		}
	}

	if len(metadata) > 0 {
		extraRaw, err := json.Marshal(metadata)
		if err != nil {
			diags.AddError("Invalid Orchestration Metadata", fmt.Sprintf("Unable to encode API orchestration metadata extra fields: %s", err))
			return types.ObjectNull(assetOrchestrationMetadataAttrTypes()), diags
		}
		values[assetOrchestrationMetadataFieldExtraJSON] = types.StringValue(string(extraRaw))
	}

	obj, d := types.ObjectValue(assetOrchestrationMetadataAttrTypes(), values)
	diags.Append(d...)
	return obj, diags
}

func validateAssetOrchestrationMetadataConfig(obj types.Object, basePath path.Path, diags *diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return
	}

	var model assetOrchestrationMetadataModel
	if d := obj.As(context.Background(), &model, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return
	}

	if model.AssetType.IsNull() || model.AssetType.IsUnknown() || model.AssetType.ValueString() == "" {
		diags.AddAttributeError(
			basePath.AtName(assetOrchestrationMetadataFieldAssetType),
			"Missing Orchestration Metadata Field",
			"asset_type is required when orchestration_metadata is set.",
		)
		return
	}
	if model.AssetType.ValueString() != "F5" {
		diags.AddAttributeError(
			basePath.AtName(assetOrchestrationMetadataFieldAssetType),
			"Invalid Orchestration Metadata Field",
			"asset_type must be \"F5\".",
		)
		return
	}

	if model.F5DeviceHostname.IsNull() || model.F5DeviceHostname.IsUnknown() || model.F5DeviceHostname.ValueString() == "" {
		diags.AddAttributeError(
			basePath.AtName(assetOrchestrationMetadataFieldF5DeviceHostname),
			"Missing Orchestration Metadata Field",
			"f5_device_hostname is required for asset_type \"F5\".",
		)
	}
	if model.Partition.IsNull() || model.Partition.IsUnknown() || model.Partition.ValueString() == "" {
		diags.AddAttributeError(
			basePath.AtName(assetOrchestrationMetadataFieldPartition),
			"Missing Orchestration Metadata Field",
			"partition is required for asset_type \"F5\".",
		)
	}
	if model.VSName.IsNull() || model.VSName.IsUnknown() || model.VSName.ValueString() == "" {
		diags.AddAttributeError(
			basePath.AtName(assetOrchestrationMetadataFieldVSName),
			"Missing Orchestration Metadata Field",
			"vs_name is required for asset_type \"F5\".",
		)
	}

	if model.ExtraJSON.IsNull() || model.ExtraJSON.IsUnknown() || model.ExtraJSON.ValueString() == "" {
		return
	}

	extra, extraDiags := parseAssetOrchestrationMetadataExtraJSON(model.ExtraJSON, "orchestration_metadata."+assetOrchestrationMetadataFieldExtraJSON)
	if extraDiags.HasError() {
		for _, d := range extraDiags {
			diags.AddAttributeError(
				basePath.AtName(assetOrchestrationMetadataFieldExtraJSON),
				d.Summary(),
				d.Detail(),
			)
		}
		return
	}

	for key := range extra {
		if _, exists := assetOrchestrationMetadataKnownFields[key]; exists {
			diags.AddAttributeError(
				basePath.AtName(assetOrchestrationMetadataFieldExtraJSON),
				"Invalid Orchestration Metadata Field",
				fmt.Sprintf("extra_json contains reserved key %q; use typed attributes instead", key),
			)
		}
	}
}

func parseAssetOrchestrationMetadataExtraJSON(value types.String, fieldName string) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return map[string]any{}, diags
	}

	var parsed any
	if err := json.Unmarshal([]byte(value.ValueString()), &parsed); err != nil {
		diags.AddError("Invalid JSON", fmt.Sprintf("Unable to parse %s: %s", fieldName, err))
		return nil, diags
	}

	obj, ok := parsed.(map[string]any)
	if !ok {
		diags.AddError("Invalid JSON", fmt.Sprintf("%s must be a JSON object", fieldName))
		return nil, diags
	}

	return obj, diags
}
