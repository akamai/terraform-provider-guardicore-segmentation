package labelgroup

import (
	"context"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMergeLabelGroupSelectors(t *testing.T) {
	overlay := &client.OrLabelsCreate{OrLabels: []client.AndLabelsCreate{{AndLabels: []string{"c"}}}}
	base := &client.OrLabelsCreate{OrLabels: []client.AndLabelsCreate{{AndLabels: []string{"a", "b"}}}}

	if got := mergeLabelGroupSelectors(nil, overlay); got != overlay {
		t.Fatal("expected overlay when base is nil")
	}
	if got := mergeLabelGroupSelectors(base, nil); got != base {
		t.Fatal("expected base when overlay is nil")
	}

	merged := mergeLabelGroupSelectors(base, overlay)
	if len(merged.OrLabels) != 1 || len(merged.OrLabels[0].AndLabels) != 1 || merged.OrLabels[0].AndLabels[0] != "c" {
		t.Fatalf("expected overlay to replace or_labels, got %#v", merged)
	}
}

func TestParseLabelGroupSelectorJSON(t *testing.T) {
	value, diags := parseLabelGroupSelectorJSON(types.StringValue(`{"or_labels":[{"and_labels":["l-1"]}]}`), "raw_include_json")
	if diags.HasError() {
		t.Fatalf("expected no error diagnostics, got %v", diags)
	}
	if value == nil || len(value.OrLabels) != 1 || len(value.OrLabels[0].AndLabels) != 1 {
		t.Fatalf("unexpected parsed selector %#v", value)
	}

	_, diags = parseLabelGroupSelectorJSON(types.StringValue("{"), "raw_include_json")
	if !diags.HasError() {
		t.Fatal("expected JSON parse diagnostics")
	}

	nilValue, diags := parseLabelGroupSelectorJSON(types.StringNull(), "raw_include_json")
	if diags.HasError() || nilValue != nil {
		t.Fatalf("expected nil/no diags for null, got value=%#v diags=%v", nilValue, diags)
	}
}

func TestLabelGroupSelectorProvided(t *testing.T) {
	nullObj := types.ObjectNull(labelGroupSelectorAttrTypes())
	if labelGroupSelectorProvided(nullObj, types.StringNull()) {
		t.Fatal("expected false when both typed and raw selectors are absent")
	}
	if !labelGroupSelectorProvided(nullObj, types.StringValue(`{"or_labels":[]}`)) {
		t.Fatal("expected true when raw selector string is provided")
	}
}

func TestConvertOrLabelsReadToCreateAndNormalize(t *testing.T) {
	if got := convertOrLabelsReadToCreate(nil); got != nil {
		t.Fatalf("expected nil conversion for nil input, got %#v", got)
	}

	read := &client.OrLabelsRead{OrLabels: []client.AndLabelsRead{{AndLabels: []client.LabelInGroup{{ID: "l-1"}, {ID: "l-2"}}}}}
	create := convertOrLabelsReadToCreate(read)
	if len(create.OrLabels) != 1 || len(create.OrLabels[0].AndLabels) != 2 || create.OrLabels[0].AndLabels[1] != "l-2" {
		t.Fatalf("unexpected converted selector %#v", create)
	}

	normalized, err := normalizeLabelGroupSelectorJSON(create)
	if err != nil {
		t.Fatalf("expected no normalize error, got %v", err)
	}
	if normalized.IsNull() || !strings.Contains(normalized.ValueString(), `"or_labels"`) {
		t.Fatalf("expected normalized JSON value, got %q", normalized.String())
	}

	nullNormalized, err := normalizeLabelGroupSelectorJSON(nil)
	if err != nil || !nullNormalized.IsNull() {
		t.Fatalf("expected null normalized value for nil input, got %q err=%v", nullNormalized.String(), err)
	}
}

func TestLabelGroupSelectorObjectRoundTrip(t *testing.T) {
	ctx := context.Background()
	create := &client.OrLabelsCreate{OrLabels: []client.AndLabelsCreate{{AndLabels: []string{"l-1", "l-2"}}}}
	obj, diags := labelGroupSelectorObjectFromCreate(ctx, create)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics converting from create: %v", diags)
	}

	back, diags := labelGroupSelectorObjectToCreate(ctx, obj, "include")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics converting back to create: %v", diags)
	}
	if back == nil || len(back.OrLabels) != 1 || len(back.OrLabels[0].AndLabels) != 2 {
		t.Fatalf("unexpected back conversion %#v", back)
	}

	read := &client.OrLabelsRead{OrLabels: []client.AndLabelsRead{{AndLabels: []client.LabelInGroup{{ID: "l-1"}}}}}
	objFromRead, diags := labelGroupSelectorObjectFromRead(ctx, read)
	if diags.HasError() || objFromRead.IsNull() {
		t.Fatalf("expected non-null object from read, got diags=%v null=%v", diags, objFromRead.IsNull())
	}
}

func TestResolveLabelGroupSelectorPrefersTypedOverlay(t *testing.T) {
	ctx := context.Background()
	labelIDs, diags := types.ListValueFrom(ctx, types.StringType, []string{"typed"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating label_ids list: %v", diags)
	}

	orGroupObj, diags := types.ObjectValueFrom(ctx, labelGroupOrGroupAttrTypes(), LabelGroupOrGroupModel{LabelIDs: labelIDs})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating or-group object: %v", diags)
	}

	orGroupsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}, []types.Object{orGroupObj})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating or_groups list: %v", diags)
	}

	typedObj, diags := types.ObjectValueFrom(ctx, labelGroupSelectorAttrTypes(), LabelGroupSelectorModel{ORGroups: orGroupsList})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating typed object: %v", diags)
	}

	resolved, diags := resolveLabelGroupSelector(
		ctx,
		typedObj,
		types.StringValue(`{"or_labels":[{"and_labels":["raw"]}]}`),
		"include",
		"raw_include_json",
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics resolving selector: %v", diags)
	}
	if resolved == nil || len(resolved.OrLabels) != 1 || len(resolved.OrLabels[0].AndLabels) != 1 || resolved.OrLabels[0].AndLabels[0] != "typed" {
		t.Fatalf("expected typed selector to overlay raw selector, got %#v", resolved)
	}
}

func TestResolveLabelGroupSelectorVariants(t *testing.T) {
	ctx := context.Background()

	t.Run("raw only", func(t *testing.T) {
		resolved, diags := resolveLabelGroupSelector(
			ctx,
			types.ObjectNull(labelGroupSelectorAttrTypes()),
			types.StringValue(`{"or_labels":[{"and_labels":["raw-1"]}]}`),
			"include",
			"raw_include_json",
		)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if resolved == nil || len(resolved.OrLabels) != 1 || resolved.OrLabels[0].AndLabels[0] != "raw-1" {
			t.Fatalf("unexpected resolved selector %#v", resolved)
		}
	})

	t.Run("typed only", func(t *testing.T) {
		labelIDs, diags := types.ListValueFrom(ctx, types.StringType, []string{"typed-1", "typed-2"})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating label list: %v", diags)
		}

		orGroupObj, diags := types.ObjectValueFrom(ctx, labelGroupOrGroupAttrTypes(), LabelGroupOrGroupModel{LabelIDs: labelIDs})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating or-group object: %v", diags)
		}

		orGroupsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}, []types.Object{orGroupObj})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating or-groups list: %v", diags)
		}

		typedObj, diags := types.ObjectValueFrom(ctx, labelGroupSelectorAttrTypes(), LabelGroupSelectorModel{ORGroups: orGroupsList})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating selector object: %v", diags)
		}

		resolved, diags := resolveLabelGroupSelector(ctx, typedObj, types.StringNull(), "include", "raw_include_json")
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if resolved == nil || len(resolved.OrLabels) != 1 || len(resolved.OrLabels[0].AndLabels) != 2 {
			t.Fatalf("unexpected resolved typed selector %#v", resolved)
		}
	})

	t.Run("invalid raw json", func(t *testing.T) {
		resolved, diags := resolveLabelGroupSelector(
			ctx,
			types.ObjectNull(labelGroupSelectorAttrTypes()),
			types.StringValue("{"),
			"include",
			"raw_include_json",
		)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for invalid raw json")
		}
		if resolved != nil {
			t.Fatalf("expected nil selector on parse failure, got %#v", resolved)
		}
	})
}

func TestLabelGroupOrGroupModelsFromList_NullAndUnknown(t *testing.T) {
	ctx := context.Background()

	models, diags := labelGroupOrGroupModelsFromList(ctx, types.ListNull(types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}), "include.or_groups")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for null list: %v", diags)
	}
	if models != nil {
		t.Fatalf("expected nil models for null list, got %#v", models)
	}

	models, diags = labelGroupOrGroupModelsFromList(ctx, types.ListUnknown(types.ObjectType{AttrTypes: labelGroupOrGroupAttrTypes()}), "include.or_groups")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for unknown list: %v", diags)
	}
	if models != nil {
		t.Fatalf("expected nil models for unknown list, got %#v", models)
	}
}

func TestLabelGroupSelectorObjectFromCreateAndRead_NullInputs(t *testing.T) {
	ctx := context.Background()

	obj, diags := labelGroupSelectorObjectFromCreate(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for nil create selector: %v", diags)
	}
	if !obj.IsNull() {
		t.Fatalf("expected null object for nil create selector, got %#v", obj)
	}

	obj, diags = labelGroupSelectorObjectFromRead(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for nil read selector: %v", diags)
	}
	if !obj.IsNull() {
		t.Fatalf("expected null object for nil read selector, got %#v", obj)
	}
}
