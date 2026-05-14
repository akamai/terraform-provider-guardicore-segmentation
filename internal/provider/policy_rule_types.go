package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type PolicyRuleRangeModel struct {
	Start types.Int64 `tfsdk:"start"`
	End   types.Int64 `tfsdk:"end"`
}

type PolicyRuleICMPMatchModel struct {
	ICMPType  types.Int64  `tfsdk:"icmp_type"`
	ICMPCodes types.List   `tfsdk:"icmp_codes"`
	Version   types.String `tfsdk:"version"`
}

type PolicyRuleScheduleModel struct {
	Recurrence types.String `tfsdk:"recurrence"`
	EndTime    types.Int64  `tfsdk:"end_time"`
}

type PolicyRuleWindowsServiceModel struct {
	AllowedImageNames types.List   `tfsdk:"allowed_image_names"`
	DisplayName       types.String `tfsdk:"display_name"`
	ServiceName       types.String `tfsdk:"service_name"`
}

type PolicyRuleOrLabelsModel struct {
	AndLabels types.List `tfsdk:"and_labels"`
}

type PolicyRuleLabelsModel struct {
	OrLabels []PolicyRuleOrLabelsModel `tfsdk:"or_labels"`
}

type PolicyRuleEndpointModel struct {
	AddressClassification types.String `tfsdk:"address_classification"`
	Subnets               types.List   `tfsdk:"subnets"`
	Processes             types.List   `tfsdk:"processes"`
	WindowsServices       types.List   `tfsdk:"windows_services"`
	Domains               types.List   `tfsdk:"domains"`
	LabelGroupIDs         types.List   `tfsdk:"label_group_ids"`
	UserGroupIDs          types.List   `tfsdk:"user_group_ids"`
	AssetIDs              types.List   `tfsdk:"asset_ids"`
	PolicyGroupIDs        types.List   `tfsdk:"policy_group_ids"`
	Labels                types.Object `tfsdk:"labels"`
}

func policyRuleRangeSchemaAttribute() resourceschema.ListNestedAttribute {
	return resourceschema.ListNestedAttribute{
		Optional: true,
		NestedObject: resourceschema.NestedAttributeObject{
			Attributes: map[string]resourceschema.Attribute{
				"start": resourceschema.Int64Attribute{Optional: true},
				"end":   resourceschema.Int64Attribute{Optional: true},
			},
		},
	}
}

func policyRuleRangeDataSourceAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"start": schema.Int64Attribute{Computed: true},
				"end":   schema.Int64Attribute{Computed: true},
			},
		},
	}
}

func policyRuleICMPMatchSchemaAttribute() resourceschema.ListNestedAttribute {
	return resourceschema.ListNestedAttribute{
		Optional: true,
		NestedObject: resourceschema.NestedAttributeObject{
			Attributes: map[string]resourceschema.Attribute{
				"icmp_type":  resourceschema.Int64Attribute{Optional: true},
				"icmp_codes": resourceschema.ListAttribute{Optional: true, ElementType: types.Int64Type},
				"version":    resourceschema.StringAttribute{Optional: true},
			},
		},
	}
}

func policyRuleICMPMatchDataSourceAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"icmp_type":  schema.Int64Attribute{Computed: true},
				"icmp_codes": schema.ListAttribute{Computed: true, ElementType: types.Int64Type},
				"version":    schema.StringAttribute{Computed: true},
			},
		},
	}
}

func policyRuleLabelsSchemaAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]resourceschema.Attribute{
			"or_labels": resourceschema.ListNestedAttribute{
				Optional: true,
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"and_labels": resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
					},
				},
			},
		},
	}
}

func policyRuleLabelsDataSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"or_labels": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"and_labels": schema.ListAttribute{Computed: true, ElementType: types.StringType},
					},
				},
			},
		},
	}
}

func policyRuleEndpointSchemaAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]resourceschema.Attribute{
			"address_classification": resourceschema.StringAttribute{Optional: true},
			"subnets":                resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"processes":              resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"windows_services":       policyRuleWindowsServicesSchemaAttribute(),
			"domains":                resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"label_group_ids":        resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"user_group_ids":         resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"asset_ids":              resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"policy_group_ids":       resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
			"labels":                 policyRuleLabelsSchemaAttribute(),
		},
	}
}

func policyRuleEndpointDataSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"any_external":           schema.BoolAttribute{Computed: true},
			"address_classification": schema.StringAttribute{Computed: true},
			"subnets":                schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"processes":              schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"windows_services":       policyRuleWindowsServicesDataSourceAttribute(),
			"domains":                schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"label_group_ids":        schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"user_group_ids":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"asset_ids":              schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"policy_group_ids":       schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"labels":                 policyRuleLabelsDataSourceAttribute(),
		},
	}
}

func policyRuleWindowsServicesSchemaAttribute() resourceschema.ListNestedAttribute {
	return resourceschema.ListNestedAttribute{
		Optional: true,
		NestedObject: resourceschema.NestedAttributeObject{
			Attributes: map[string]resourceschema.Attribute{
				"allowed_image_names": resourceschema.ListAttribute{Optional: true, ElementType: types.StringType},
				"display_name":        resourceschema.StringAttribute{Optional: true},
				"service_name":        resourceschema.StringAttribute{Optional: true},
			},
		},
	}
}

func policyRuleWindowsServicesDataSourceAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"allowed_image_names": schema.ListAttribute{Computed: true, ElementType: types.StringType},
				"display_name":        schema.StringAttribute{Computed: true},
				"service_name":        schema.StringAttribute{Computed: true},
			},
		},
	}
}

func policyRuleScheduleSchemaAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]resourceschema.Attribute{
			"recurrence": resourceschema.StringAttribute{Optional: true},
			"end_time":   resourceschema.Int64Attribute{Optional: true},
		},
	}
}

func policyRuleScheduleDataSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"recurrence": schema.StringAttribute{Computed: true},
			"end_time":   schema.Int64Attribute{Computed: true},
		},
	}
}

func policyRuleScheduleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"recurrence": types.StringType,
		"end_time":   types.Int64Type,
	}
}

func policyRuleRangeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"start": types.Int64Type,
		"end":   types.Int64Type,
	}
}

func policyRuleICMPMatchAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"icmp_type":  types.Int64Type,
		"icmp_codes": types.ListType{ElemType: types.Int64Type},
		"version":    types.StringType,
	}
}

func policyRuleWindowsServiceAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"allowed_image_names": types.ListType{ElemType: types.StringType},
		"display_name":        types.StringType,
		"service_name":        types.StringType,
	}
}

func policyRuleLabelsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"or_labels": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"and_labels": types.ListType{ElemType: types.StringType},
		}}},
	}
}

func policyRuleEndpointAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"address_classification": types.StringType,
		"subnets":                types.ListType{ElemType: types.StringType},
		"processes":              types.ListType{ElemType: types.StringType},
		"windows_services":       types.ListType{ElemType: types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}},
		"domains":                types.ListType{ElemType: types.StringType},
		"label_group_ids":        types.ListType{ElemType: types.StringType},
		"user_group_ids":         types.ListType{ElemType: types.StringType},
		"asset_ids":              types.ListType{ElemType: types.StringType},
		"policy_group_ids":       types.ListType{ElemType: types.StringType},
		"labels":                 types.ObjectType{AttrTypes: policyRuleLabelsAttrTypes()},
	}
}

func policyRuleEndpointDataSourceAttrTypes() map[string]attr.Type {
	attrTypes := policyRuleEndpointAttrTypes()
	attrTypes["any_external"] = types.BoolType
	return attrTypes
}

func parsePolicyRuleJSONObject(value types.String, fieldName string) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return nil, diags
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(value.ValueString()), &parsed); err != nil {
		diags.AddError("Invalid JSON", fmt.Sprintf("Unable to parse %s: %s", fieldName, err))
		return nil, diags
	}

	return parsed, diags
}

func containsUnknownSentinel(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "74D93920-ED26-11E3-AC10-0800200C9A66" || typed == "<unknown>"
	case map[string]any:
		for _, nested := range typed {
			if containsUnknownSentinel(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsUnknownSentinel(nested) {
				return true
			}
		}
	}
	return false
}

func mergePolicyRuleMaps(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range overlay {
		if existing, ok := base[key]; ok {
			existingMap, existingOK := existing.(map[string]any)
			overlayMap, overlayOK := value.(map[string]any)
			if existingOK && overlayOK {
				base[key] = mergePolicyRuleMaps(existingMap, overlayMap)
				continue
			}
		}
		base[key] = value
	}
	return base
}

func normalizePolicyRuleLegacyAliases(spec map[string]any) map[string]any {
	if spec == nil {
		return nil
	}
	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := spec[endpointKey].(map[string]any)
		if !ok {
			continue
		}
		if value, ok := endpoint["label_groups"]; ok {
			endpoint["label_group_ids"] = value
			delete(endpoint, "label_groups")
		}
		if value, ok := endpoint["user_groups"]; ok {
			endpoint["user_group_ids"] = value
			delete(endpoint, "user_groups")
		}
		if value, ok := endpoint["assets"]; ok {
			endpoint["asset_ids"] = value
			delete(endpoint, "assets")
		}
		if value, ok := endpoint["policy_group_ids"]; ok {
			endpoint["policy_groups"] = value
			delete(endpoint, "policy_group_ids")
		}
		if value, ok := endpoint["labels"]; ok {
			if _, isList := value.([]any); isList {
				endpoint["label_group_ids"] = value
				delete(endpoint, "labels")
			}
		}
	}
	delete(spec, "any_port")
	return spec
}

func listStringsToValue(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

func listInt64ToValue(ctx context.Context, values []int64) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(types.Int64Type), nil
	}
	return types.ListValueFrom(ctx, types.Int64Type, values)
}

func intSliceFromList(ctx context.Context, list types.List, fieldName string) ([]int64, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var values []types.Int64
	diags.Append(list.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s", fieldName))
		return nil, diags
	}

	decoded := make([]int64, 0, len(values))
	for _, value := range values {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		decoded = append(decoded, value.ValueInt64())
	}
	return decoded, diags
}

func stringSliceFromList(ctx context.Context, list types.List, fieldName string) ([]string, diag.Diagnostics) {
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

func policyRuleRangeModelsFromList(ctx context.Context, list types.List, fieldName string) ([]PolicyRuleRangeModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var objects []types.Object
	diags.Append(list.ElementsAs(ctx, &objects, false)...)
	if diags.HasError() {
		diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s", fieldName))
		return nil, diags
	}

	models := make([]PolicyRuleRangeModel, 0, len(objects))
	for i, object := range objects {
		if object.IsNull() || object.IsUnknown() {
			continue
		}

		var model PolicyRuleRangeModel
		diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s[%d]", fieldName, i))
			return nil, diags
		}
		models = append(models, model)
	}

	return models, diags
}

func policyRuleICMPMatchModelsFromList(ctx context.Context, list types.List, fieldName string) ([]PolicyRuleICMPMatchModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var objects []types.Object
	diags.Append(list.ElementsAs(ctx, &objects, false)...)
	if diags.HasError() {
		diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s", fieldName))
		return nil, diags
	}

	models := make([]PolicyRuleICMPMatchModel, 0, len(objects))
	for i, object := range objects {
		if object.IsNull() || object.IsUnknown() {
			continue
		}

		var model PolicyRuleICMPMatchModel
		diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s[%d]", fieldName, i))
			return nil, diags
		}
		models = append(models, model)
	}

	return models, diags
}

func policyRuleLabelsObjectToMap(ctx context.Context, object types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if object.IsNull() || object.IsUnknown() {
		return nil, diags
	}

	var labels PolicyRuleLabelsModel
	diags.Append(object.As(ctx, &labels, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	orLabels := make([]any, 0, len(labels.OrLabels))
	for i, orLabel := range labels.OrLabels {
		andLabels, andDiags := stringSliceFromList(ctx, orLabel.AndLabels, fmt.Sprintf("labels.or_labels[%d].and_labels", i))
		diags.Append(andDiags...)
		if diags.HasError() {
			return nil, diags
		}

		if len(andLabels) == 0 {
			continue
		}

		values := make([]any, len(andLabels))
		for idx, value := range andLabels {
			values[idx] = value
		}
		orLabels = append(orLabels, map[string]any{"and_labels": values})
	}

	if len(orLabels) == 0 {
		return nil, diags
	}

	return map[string]any{"or_labels": orLabels}, diags
}

func policyRuleEndpointObjectToMap(ctx context.Context, object types.Object, fieldName string) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if object.IsNull() || object.IsUnknown() {
		return nil, diags
	}
	attrs := object.Attributes()
	addressClassificationAttr, _ := attrs["address_classification"].(types.String)
	subnetsAttr, _ := attrs["subnets"].(types.List)
	processesAttr, _ := attrs["processes"].(types.List)
	windowsServicesAttr, _ := attrs["windows_services"].(types.List)
	domainsAttr, _ := attrs["domains"].(types.List)
	labelGroupIDsAttr, _ := attrs["label_group_ids"].(types.List)
	userGroupIDsAttr, _ := attrs["user_group_ids"].(types.List)
	assetIDsAttr, _ := attrs["asset_ids"].(types.List)
	policyGroupIDsAttr, _ := attrs["policy_group_ids"].(types.List)
	labelsAttr, _ := attrs["labels"].(types.Object)

	result := make(map[string]any)
	if !addressClassificationAttr.IsNull() && !addressClassificationAttr.IsUnknown() && addressClassificationAttr.ValueString() != "" {
		result["address_classification"] = addressClassificationAttr.ValueString()
	}

	if values, d := stringSliceFromList(ctx, subnetsAttr, fieldName+".subnets"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["subnets"] = values
	}
	if values, d := stringSliceFromList(ctx, processesAttr, fieldName+".processes"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["processes"] = values
	}
	if values, d := policyRuleWindowsServicesListToAny(ctx, windowsServicesAttr, fieldName+".windows_services"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["windows_services"] = values
	}
	if values, d := stringSliceFromList(ctx, domainsAttr, fieldName+".domains"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["domains"] = values
	}
	if values, d := stringSliceFromList(ctx, labelGroupIDsAttr, fieldName+".label_group_ids"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["label_group_ids"] = values
	}
	if values, d := stringSliceFromList(ctx, userGroupIDsAttr, fieldName+".user_group_ids"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["user_group_ids"] = values
	}
	if values, d := stringSliceFromList(ctx, assetIDsAttr, fieldName+".asset_ids"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["asset_ids"] = values
	}
	if values, d := stringSliceFromList(ctx, policyGroupIDsAttr, fieldName+".policy_group_ids"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		result["policy_groups"] = values
	}
	if labels, d := policyRuleLabelsObjectToMap(ctx, labelsAttr); labels != nil || d.HasError() {
		diags.Append(d...)
		if labels != nil {
			result["labels"] = labels
		}
	}

	if _, hasAC := result["address_classification"]; hasAC {
		otherFields := []string{"subnets", "processes", "windows_services", "domains",
			"label_group_ids", "user_group_ids", "asset_ids", "policy_groups", "labels"}
		for _, f := range otherFields {
			if _, ok := result[f]; ok {
				diags.AddError(
					"Invalid Endpoint Configuration",
					fmt.Sprintf("%s.address_classification cannot be combined with %s.%s; "+
						"when address_classification is set, no other endpoint filter fields are allowed",
						fieldName, fieldName, f),
				)
			}
		}
	}

	if len(result) == 0 {
		return nil, diags
	}

	return result, diags
}

func policyRuleWindowsServicesModelsFromList(ctx context.Context, list types.List, fieldName string) ([]PolicyRuleWindowsServiceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var objects []types.Object
	diags.Append(list.ElementsAs(ctx, &objects, false)...)
	if diags.HasError() {
		diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s", fieldName))
		return nil, diags
	}

	models := make([]PolicyRuleWindowsServiceModel, 0, len(objects))
	for i, object := range objects {
		if object.IsNull() || object.IsUnknown() {
			continue
		}

		var model PolicyRuleWindowsServiceModel
		diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			diags.AddError("Invalid Attribute", fmt.Sprintf("Unable to decode %s[%d]", fieldName, i))
			return nil, diags
		}
		models = append(models, model)
	}

	return models, diags
}

func policyRuleWindowsServicesListToAny(ctx context.Context, list types.List, fieldName string) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	models, d := policyRuleWindowsServicesModelsFromList(ctx, list, fieldName)
	diags.Append(d...)
	if diags.HasError() || len(models) == 0 {
		return nil, diags
	}

	services := make([]any, 0, len(models))
	for i, service := range models {
		serviceMap := make(map[string]any)

		allowedImageNames, d := stringSliceFromList(ctx, service.AllowedImageNames, fmt.Sprintf("%s[%d].allowed_image_names", fieldName, i))
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if len(allowedImageNames) > 0 {
			serviceMap["allowed_image_names"] = allowedImageNames
		}

		if !service.DisplayName.IsNull() && !service.DisplayName.IsUnknown() && service.DisplayName.ValueString() != "" {
			serviceMap["display_name"] = service.DisplayName.ValueString()
		}

		if !service.ServiceName.IsNull() && !service.ServiceName.IsUnknown() && service.ServiceName.ValueString() != "" {
			serviceMap["service_name"] = service.ServiceName.ValueString()
		}

		if len(serviceMap) > 0 {
			services = append(services, serviceMap)
		}
	}

	if len(services) == 0 {
		return nil, diags
	}

	return services, diags
}

func policyRuleScheduleObjectToMap(ctx context.Context, object types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if object.IsNull() || object.IsUnknown() {
		return nil, diags
	}

	var schedule PolicyRuleScheduleModel
	diags.Append(object.As(ctx, &schedule, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	result := make(map[string]any)
	if !schedule.Recurrence.IsNull() && !schedule.Recurrence.IsUnknown() && schedule.Recurrence.ValueString() != "" {
		result["recurrence"] = schedule.Recurrence.ValueString()
	}
	if !schedule.EndTime.IsNull() && !schedule.EndTime.IsUnknown() {
		result["end_time"] = schedule.EndTime.ValueInt64()
	}
	if len(result) == 0 {
		return nil, diags
	}
	return result, diags
}

func buildPolicyRuleSpecFromModel(ctx context.Context, data *PolicyRuleResourceModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := map[string]any{}
	if parsed, d := parsePolicyRuleJSONObject(data.RawSpecJSON, "raw_spec_json"); parsed != nil || d.HasError() {
		diags.Append(d...)
		spec = mergePolicyRuleMaps(spec, parsed)
	}
	if diags.HasError() {
		return nil, diags
	}

	spec = normalizePolicyRuleLegacyAliases(spec)
	delete(spec, "creation_origin")

	if !data.Action.IsNull() && !data.Action.IsUnknown() && data.Action.ValueString() != "" {
		spec["action"] = data.Action.ValueString()
	}
	if !data.SectionPosition.IsNull() && !data.SectionPosition.IsUnknown() && data.SectionPosition.ValueString() != "" {
		spec["section_position"] = data.SectionPosition.ValueString()
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		spec["enabled"] = data.Enabled.ValueBool()
	}
	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		spec["comments"] = data.Comments.ValueString()
	}
	if !data.RulesetName.IsNull() && !data.RulesetName.IsUnknown() && data.RulesetName.ValueString() != "" {
		spec["ruleset_name"] = data.RulesetName.ValueString()
	}
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		spec["priority"] = data.Priority.ValueInt64()
	}
	if values, d := stringSliceFromList(ctx, data.IPProtocols, "ip_protocols"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		spec["ip_protocols"] = values
	}
	if values, d := intSliceFromList(ctx, data.Ports, "ports"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		spec["ports"] = values
	}
	if values, d := intSliceFromList(ctx, data.ExcludePorts, "exclude_ports"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		spec["exclude_ports"] = values
	}

	if models, d := policyRuleRangeModelsFromList(ctx, data.PortRanges, "port_ranges"); len(models) > 0 || d.HasError() {
		diags.Append(d...)
		ranges := make([]any, 0, len(models))
		for _, r := range models {
			if r.Start.IsNull() || r.Start.IsUnknown() || r.End.IsNull() || r.End.IsUnknown() {
				continue
			}
			ranges = append(ranges, map[string]any{"start": r.Start.ValueInt64(), "end": r.End.ValueInt64()})
		}
		if len(ranges) > 0 {
			spec["port_ranges"] = ranges
		}
	}
	if models, d := policyRuleRangeModelsFromList(ctx, data.ExcludePortRanges, "exclude_port_ranges"); len(models) > 0 || d.HasError() {
		diags.Append(d...)
		ranges := make([]any, 0, len(models))
		for _, r := range models {
			if r.Start.IsNull() || r.Start.IsUnknown() || r.End.IsNull() || r.End.IsUnknown() {
				continue
			}
			ranges = append(ranges, map[string]any{"start": r.Start.ValueInt64(), "end": r.End.ValueInt64()})
		}
		if len(ranges) > 0 {
			spec["exclude_port_ranges"] = ranges
		}
	}
	if models, d := policyRuleICMPMatchModelsFromList(ctx, data.ICMPMatches, "icmp_matches"); len(models) > 0 || d.HasError() {
		diags.Append(d...)
		matches := make([]any, 0, len(models))
		for i, match := range models {
			matchMap := make(map[string]any)
			if !match.ICMPType.IsNull() && !match.ICMPType.IsUnknown() {
				matchMap["icmp_type"] = match.ICMPType.ValueInt64()
			}
			codes, d := intSliceFromList(ctx, match.ICMPCodes, fmt.Sprintf("icmp_matches[%d].icmp_codes", i))
			diags.Append(d...)
			if codes == nil {
				codes = []int64{}
			}
			matchMap["icmp_codes"] = codes
			if !match.Version.IsNull() && !match.Version.IsUnknown() && match.Version.ValueString() != "" {
				matchMap["version"] = match.Version.ValueString()
			}
			if len(matchMap) > 0 {
				matches = append(matches, matchMap)
			}
		}
		if len(matches) > 0 {
			spec["icmp_matches"] = matches
		}
	}
	if !data.NetworkProfile.IsNull() && !data.NetworkProfile.IsUnknown() && data.NetworkProfile.ValueString() != "" {
		spec["network_profile"] = data.NetworkProfile.ValueString()
	}
	if values, d := stringSliceFromList(ctx, data.Scope, "scope"); len(values) > 0 || d.HasError() {
		diags.Append(d...)
		spec["scope"] = values
	}
	if schedule, d := policyRuleScheduleObjectToMap(ctx, data.Schedule); schedule != nil || d.HasError() {
		diags.Append(d...)
		if schedule != nil {
			spec["schedule"] = schedule
		}
	}
	if source, d := policyRuleEndpointObjectToMap(ctx, data.Source, "source"); source != nil || d.HasError() {
		diags.Append(d...)
		if source != nil {
			existing, _ := spec["source"].(map[string]any)
			spec["source"] = mergePolicyRuleMaps(existing, source)
		}
	}
	if destination, d := policyRuleEndpointObjectToMap(ctx, data.Destination, "destination"); destination != nil || d.HasError() {
		diags.Append(d...)
		if destination != nil {
			existing, _ := spec["destination"].(map[string]any)
			spec["destination"] = mergePolicyRuleMaps(existing, destination)
		}
	}

	if diags.HasError() {
		return nil, diags
	}
	if _, ok := spec["source"]; !ok {
		spec["source"] = map[string]any{}
	}
	if _, ok := spec["destination"]; !ok {
		spec["destination"] = map[string]any{}
	}

	delete(spec, "creation_origin")

	if len(spec) == 0 {
		diags.AddError("Missing Policy Rule Configuration", "Set typed policy rule attributes or provide raw_spec_json.")
		return nil, diags
	}

	if _, ok := spec["action"]; !ok {
		diags.AddError("Missing Required Policy Rule Field", "Policy rule configuration must define action.")
	}
	if _, ok := spec["section_position"]; !ok {
		diags.AddError("Missing Required Policy Rule Field", "Policy rule configuration must define section_position.")
	}
	if _, ok := spec["enabled"]; !ok {
		diags.AddError("Missing Required Policy Rule Field", "Policy rule configuration must define enabled.")
	}

	return spec, diags
}

func listStringsFromAny(ctx context.Context, value any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []string:
		if len(typed) == 0 {
			return types.ListNull(types.StringType), diags
		}
		return listStringsToValue(ctx, typed)
	default:
		return types.ListNull(types.StringType), diags
	}
	if len(raw) == 0 {
		return types.ListNull(types.StringType), diags
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			values = append(values, text)
		}
	}
	return listStringsToValue(ctx, values)
}

func listIntsFromAny(ctx context.Context, value any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []int64:
		if len(typed) == 0 {
			return types.ListNull(types.Int64Type), diags
		}
		return listInt64ToValue(ctx, typed)
	case []int:
		if len(typed) == 0 {
			return types.ListNull(types.Int64Type), diags
		}
		values := make([]int64, 0, len(typed))
		for _, item := range typed {
			values = append(values, int64(item))
		}
		return listInt64ToValue(ctx, values)
	default:
		return types.ListNull(types.Int64Type), diags
	}
	if len(raw) == 0 {
		return types.ListNull(types.Int64Type), diags
	}
	values := make([]int64, 0, len(raw))
	for _, item := range raw {
		switch num := item.(type) {
		case float64:
			values = append(values, int64(num))
		case int64:
			values = append(values, num)
		case int:
			values = append(values, int64(num))
		}
	}
	return listInt64ToValue(ctx, values)
}

func policyRuleRangeListFromAny(value any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	rawRanges, ok := value.([]any)
	if !ok || len(rawRanges) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}), diags
	}

	elements := make([]attr.Value, 0, len(rawRanges))
	for _, rawRange := range rawRanges {
		rangeMap, ok := rawRange.(map[string]any)
		if !ok {
			continue
		}

		start := types.Int64Null()
		end := types.Int64Null()
		if value, ok := rangeMap["start"].(float64); ok {
			start = types.Int64Value(int64(value))
		} else if value, ok := rangeMap["start"].(int64); ok {
			start = types.Int64Value(value)
		} else if value, ok := rangeMap["start"].(int); ok {
			start = types.Int64Value(int64(value))
		}
		if value, ok := rangeMap["end"].(float64); ok {
			end = types.Int64Value(int64(value))
		} else if value, ok := rangeMap["end"].(int64); ok {
			end = types.Int64Value(value)
		} else if value, ok := rangeMap["end"].(int); ok {
			end = types.Int64Value(int64(value))
		}

		object, d := types.ObjectValue(policyRuleRangeAttrTypes(), map[string]attr.Value{
			"start": start,
			"end":   end,
		})
		diags.Append(d...)
		elements = append(elements, object)
	}

	if len(elements) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}), diags
	}

	list, d := types.ListValue(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}, elements)
	diags.Append(d...)
	return list, diags
}

func policyRuleICMPMatchListFromAny(ctx context.Context, value any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	rawMatches, ok := value.([]any)
	if !ok || len(rawMatches) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()}), diags
	}

	elements := make([]attr.Value, 0, len(rawMatches))
	for _, rawMatch := range rawMatches {
		matchMap, ok := rawMatch.(map[string]any)
		if !ok {
			continue
		}

		icmpType := types.Int64Null()
		if value, ok := matchMap["icmp_type"].(float64); ok {
			icmpType = types.Int64Value(int64(value))
		} else if value, ok := matchMap["icmp_type"].(int64); ok {
			icmpType = types.Int64Value(value)
		}

		emptyCodes, d := types.ListValueFrom(ctx, types.Int64Type, []int64{})
		diags.Append(d...)
		icmpCodes := emptyCodes
		if value, d := listIntsFromAny(ctx, matchMap["icmp_codes"]); !value.IsNull() || d.HasError() {
			diags.Append(d...)
			icmpCodes = value
		}

		version := types.StringNull()
		if value, ok := matchMap["version"].(string); ok {
			version = types.StringValue(value)
		}

		object, d := types.ObjectValue(policyRuleICMPMatchAttrTypes(), map[string]attr.Value{
			"icmp_type":  icmpType,
			"icmp_codes": icmpCodes,
			"version":    version,
		})
		diags.Append(d...)
		elements = append(elements, object)
	}

	if len(elements) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()}), diags
	}

	list, d := types.ListValue(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()}, elements)
	diags.Append(d...)
	return list, diags
}

func labelsObjectFromAny(ctx context.Context, value any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return types.ObjectNull(policyRuleLabelsAttrTypes()), diags
	}

	rawOrLabels, _ := mapped["or_labels"].([]any)
	orLabels := make([]PolicyRuleOrLabelsModel, 0, len(rawOrLabels))
	for _, rawOrLabel := range rawOrLabels {
		orLabelMap, ok := rawOrLabel.(map[string]any)
		if !ok {
			continue
		}
		andValues, d := listStringsFromAny(ctx, orLabelMap["and_labels"])
		diags.Append(d...)
		orLabels = append(orLabels, PolicyRuleOrLabelsModel{AndLabels: andValues})
	}

	return types.ObjectValueFrom(ctx, policyRuleLabelsAttrTypes(), PolicyRuleLabelsModel{OrLabels: orLabels})
}

func endpointObjectFromAny(ctx context.Context, value any, attrTypes map[string]attr.Type) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return types.ObjectNull(attrTypes), diags
	}

	endpoint := PolicyRuleEndpointModel{
		AddressClassification: types.StringNull(),
		Subnets:               types.ListNull(types.StringType),
		Processes:             types.ListNull(types.StringType),
		WindowsServices:       types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}),
		Domains:               types.ListNull(types.StringType),
		LabelGroupIDs:         types.ListNull(types.StringType),
		UserGroupIDs:          types.ListNull(types.StringType),
		AssetIDs:              types.ListNull(types.StringType),
		PolicyGroupIDs:        types.ListNull(types.StringType),
		Labels:                types.ObjectNull(policyRuleLabelsAttrTypes()),
	}

	if value, ok := mapped["address_classification"].(string); ok && value != "" {
		endpoint.AddressClassification = types.StringValue(value)
	}
	if value, d := listStringsFromAny(ctx, mapped["subnets"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.Subnets = value
	}
	if value, d := listStringsFromAny(ctx, mapped["processes"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.Processes = value
	}
	if value, d := policyRuleWindowsServicesFromAny(ctx, mapped["windows_services"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.WindowsServices = value
	}
	if value, d := listStringsFromAny(ctx, mapped["domains"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.Domains = value
	}
	if value, d := listStringsFromAny(ctx, mapped["label_group_ids"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.LabelGroupIDs = value
	}
	if value, d := listStringsFromAny(ctx, mapped["user_group_ids"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.UserGroupIDs = value
	}
	if value, d := listStringsFromAny(ctx, mapped["asset_ids"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.AssetIDs = value
	}
	if value, d := listStringsFromAny(ctx, mapped["policy_groups"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.PolicyGroupIDs = value
	}
	if labels, d := labelsObjectFromAny(ctx, mapped["labels"]); !labels.IsNull() || d.HasError() {
		diags.Append(d...)
		endpoint.Labels = labels
	}

	values := map[string]attr.Value{
		"address_classification": endpoint.AddressClassification,
		"subnets":                endpoint.Subnets,
		"processes":              endpoint.Processes,
		"windows_services":       endpoint.WindowsServices,
		"domains":                endpoint.Domains,
		"label_group_ids":        endpoint.LabelGroupIDs,
		"user_group_ids":         endpoint.UserGroupIDs,
		"asset_ids":              endpoint.AssetIDs,
		"policy_group_ids":       endpoint.PolicyGroupIDs,
		"labels":                 endpoint.Labels,
	}
	if _, ok := attrTypes["any_external"]; ok {
		if value, ok := mapped["any_external"].(bool); ok {
			values["any_external"] = types.BoolValue(value)
		} else {
			values["any_external"] = types.BoolNull()
		}
	}

	return types.ObjectValue(attrTypes, values)
}

func policyRuleWindowsServicesFromAny(ctx context.Context, value any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	rawServices, ok := value.([]any)
	if !ok || len(rawServices) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}), diags
	}

	elements := make([]attr.Value, 0, len(rawServices))
	for _, rawService := range rawServices {
		allowedImageNamesValue := types.ListNull(types.StringType)
		displayNameValue := types.StringNull()
		serviceNameValue := types.StringNull()

		if serviceMap, ok := rawService.(map[string]any); ok {
			allowedImageNamesRaw := serviceMap["allowed_image_names"]
			if allowedImageNamesRaw == nil {
				allowedImageNamesRaw = serviceMap["names"]
			}
			if value, d := listStringsFromAny(ctx, allowedImageNamesRaw); !value.IsNull() || d.HasError() {
				diags.Append(d...)
				allowedImageNamesValue = value
			}

			if displayName, ok := serviceMap["display_name"].(string); ok && displayName != "" {
				displayNameValue = types.StringValue(displayName)
			}

			if serviceName, ok := serviceMap["service_name"].(string); ok && serviceName != "" {
				serviceNameValue = types.StringValue(serviceName)
			}
		} else {
			if value, d := listStringsFromAny(ctx, rawService); !value.IsNull() || d.HasError() {
				diags.Append(d...)
				allowedImageNamesValue = value
			}
		}

		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}), diags
		}

		if allowedImageNamesValue.IsNull() && displayNameValue.IsNull() && serviceNameValue.IsNull() {
			continue
		}

		service, d := types.ObjectValue(policyRuleWindowsServiceAttrTypes(), map[string]attr.Value{
			"allowed_image_names": allowedImageNamesValue,
			"display_name":        displayNameValue,
			"service_name":        serviceNameValue,
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}), diags
		}
		elements = append(elements, service)
	}

	if len(elements) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}), diags
	}

	list, d := types.ListValue(types.ObjectType{AttrTypes: policyRuleWindowsServiceAttrTypes()}, elements)
	diags.Append(d...)
	return list, diags
}

func scheduleObjectFromAny(ctx context.Context, value any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return types.ObjectNull(policyRuleScheduleAttrTypes()), diags
	}

	schedule := PolicyRuleScheduleModel{Recurrence: types.StringNull(), EndTime: types.Int64Null()}
	if value, ok := mapped["recurrence"].(string); ok && value != "" {
		schedule.Recurrence = types.StringValue(value)
	}
	if value, ok := mapped["end_time"].(float64); ok {
		schedule.EndTime = types.Int64Value(int64(value))
	} else if value, ok := mapped["end_time"].(int64); ok {
		schedule.EndTime = types.Int64Value(value)
	}

	return types.ObjectValueFrom(ctx, policyRuleScheduleAttrTypes(), schedule)
}
