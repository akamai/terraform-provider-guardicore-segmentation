package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func mergeLabelGroupSelectors(base, overlay *client.OrLabelsCreate) *client.OrLabelsCreate {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	merged := &client.OrLabelsCreate{OrLabels: base.OrLabels}
	if overlay.OrLabels != nil {
		merged.OrLabels = overlay.OrLabels
	}

	return merged
}

type LabelGroupOrGroupModel struct {
	LabelIDs types.List `tfsdk:"label_ids"`
}

type LabelGroupSelectorModel struct {
	ORGroups types.List `tfsdk:"or_groups"`
}

func labelGroupSelectorSchemaAttribute(description string) resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: description,
		Attributes: map[string]resourceschema.Attribute{
			"or_groups": resourceschema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"label_ids": resourceschema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
						},
					},
				},
			},
		},
	}
}

func labelGroupSelectorDataSourceAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"or_groups": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label_ids": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func labelGroupOrGroupAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"label_ids": types.ListType{ElemType: types.StringType},
	}
}

func labelGroupSelectorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"or_groups": types.ListType{ElemType: types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}},
	}
}

func labelGroupSelectorObjectToCreate(ctx context.Context, object types.Object, fieldName string) (*client.OrLabelsCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	if object.IsNull() || object.IsUnknown() {
		return nil, diags
	}

	var selector LabelGroupSelectorModel
	diags.Append(object.As(ctx, &selector, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	orGroups, d := labelGroupOrGroupModelsFromList(ctx, selector.ORGroups, fieldName+".or_groups")
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	if len(orGroups) == 0 {
		return nil, diags
	}

	result := &client.OrLabelsCreate{OrLabels: make([]client.AndLabelsCreate, 0, len(orGroups))}
	for i, orGroup := range orGroups {
		labelIDs, d := stringSliceFromList(ctx, orGroup.LabelIDs, fmt.Sprintf("%s.or_groups[%d].label_ids", fieldName, i))
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if len(labelIDs) == 0 {
			continue
		}
		result.OrLabels = append(result.OrLabels, client.AndLabelsCreate{AndLabels: labelIDs})
	}

	if len(result.OrLabels) == 0 {
		return nil, diags
	}

	return result, diags
}

func labelGroupOrGroupModelsFromList(ctx context.Context, list types.List, fieldName string) ([]LabelGroupOrGroupModel, diag.Diagnostics) {
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

	models := make([]LabelGroupOrGroupModel, 0, len(objects))
	for _, object := range objects {
		var model LabelGroupOrGroupModel
		diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		models = append(models, model)
	}
	return models, diags
}

func parseLabelGroupSelectorJSON(value types.String, fieldName string) (*client.OrLabelsCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return nil, diags
	}

	var parsed client.OrLabelsCreate
	if err := json.Unmarshal([]byte(value.ValueString()), &parsed); err != nil {
		diags.AddError("Invalid JSON", fmt.Sprintf("Unable to parse %s: %s", fieldName, err))
		return nil, diags
	}

	return &parsed, diags
}

func resolveLabelGroupSelector(ctx context.Context, typed types.Object, raw types.String, typedFieldName, rawFieldName string) (*client.OrLabelsCreate, diag.Diagnostics) {
	var diags diag.Diagnostics

	result, d := parseLabelGroupSelectorJSON(raw, rawFieldName)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	typedValue, d := labelGroupSelectorObjectToCreate(ctx, typed, typedFieldName)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	if typedValue != nil {
		result = mergeLabelGroupSelectors(result, typedValue)
	}

	return result, diags
}

func labelGroupSelectorProvided(object types.Object, raw types.String) bool {
	if !object.IsNull() {
		return true
	}
	return !raw.IsNull() && (raw.IsUnknown() || raw.ValueString() != "")
}

func labelGroupSelectorObjectFromCreate(ctx context.Context, create *client.OrLabelsCreate) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if create == nil || len(create.OrLabels) == 0 {
		return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
	}

	orGroups := make([]LabelGroupOrGroupModel, 0, len(create.OrLabels))
	for _, orGroup := range create.OrLabels {
		labelIDs, d := listStringsToValue(ctx, orGroup.AndLabels)
		diags.Append(d...)
		if diags.HasError() {
			return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
		}
		orGroups = append(orGroups, LabelGroupOrGroupModel{LabelIDs: labelIDs})
	}

	orGroupsList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}, orGroups)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
	}

	return types.ObjectValueFrom(ctx, labelGroupSelectorAttrTypes(), LabelGroupSelectorModel{ORGroups: orGroupsList})
}

func labelGroupSelectorObjectFromRead(ctx context.Context, read *client.OrLabelsRead) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if read == nil || len(read.OrLabels) == 0 {
		return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
	}

	orGroups := make([]LabelGroupOrGroupModel, 0, len(read.OrLabels))
	for _, orGroup := range read.OrLabels {
		labelIDs := make([]string, 0, len(orGroup.AndLabels))
		for _, label := range orGroup.AndLabels {
			labelIDs = append(labelIDs, label.ID)
		}
		labelIDsValue, d := listStringsToValue(ctx, labelIDs)
		diags.Append(d...)
		if diags.HasError() {
			return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
		}
		orGroups = append(orGroups, LabelGroupOrGroupModel{LabelIDs: labelIDsValue})
	}

	orGroupsList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}, orGroups)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(labelGroupSelectorAttrTypes()), diags
	}

	return types.ObjectValueFrom(ctx, labelGroupSelectorAttrTypes(), LabelGroupSelectorModel{ORGroups: orGroupsList})
}

func convertOrLabelsReadToCreate(read *client.OrLabelsRead) *client.OrLabelsCreate {
	if read == nil {
		return nil
	}

	create := &client.OrLabelsCreate{
		OrLabels: make([]client.AndLabelsCreate, len(read.OrLabels)),
	}

	for i, orLabel := range read.OrLabels {
		andLabels := make([]string, len(orLabel.AndLabels))
		for j, label := range orLabel.AndLabels {
			andLabels[j] = label.ID
		}
		create.OrLabels[i] = client.AndLabelsCreate{AndLabels: andLabels}
	}

	return create
}

func normalizeLabelGroupSelectorJSON(value *client.OrLabelsCreate) (types.String, error) {
	if value == nil || len(value.OrLabels) == 0 {
		return types.StringNull(), nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return types.StringNull(), err
	}

	normalized, err := normalize.NormalizeJSONString(string(encoded))
	if err != nil {
		return types.StringNull(), err
	}

	return types.StringValue(normalized), nil
}
