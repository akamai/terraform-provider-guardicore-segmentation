package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelGroupResource_typedSelectors(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupResourceConfigTyped(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label_group.test", "key", groupKey),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "value", groupValue),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "comments", "All web servers"),
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "include_json"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.0.label_ids.#", "1"),
					testAccCheckLabelGroupPublished("guardicore_label_group.test"),
				),
			},
			{
				ResourceName:      "guardicore_label_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccLabelGroupResourceConfigTypedUpdated(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label_group.test", "comments", "Updated comment"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.0.label_ids.#", "2"),
				),
			},
		},
	})
}

func TestAccLabelGroupResource_typedIncludeAndExclude(t *testing.T) {
	key := testAccRandomName("tf-acc-Group")
	value := testAccRandomName("tf-acc-Value")
	labelKey := testAccRandomName("tf-acc-LabelKey")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupResourceConfigIncludeExclude(labelKey, key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "include_json"),
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "exclude_json"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "exclude.or_groups.#", "1"),
				),
			},
			{
				Config:   testAccLabelGroupResourceConfigIncludeExclude(labelKey, key, value),
				PlanOnly: true,
			},
		},
	})
}

func TestAccLabelGroupResource_rawOverlay(t *testing.T) {
	key := testAccRandomName("tf-acc-Raw")
	value := testAccRandomName("tf-acc-Overlay")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupResourceConfigRawOverlay(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "include_json"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label_group.test", "include.or_groups.0.label_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccLabelGroupResource_emptyOrGroups(t *testing.T) {
	key := testAccRandomName("tf-acc-Empty")
	value := testAccRandomName("tf-acc-OrGroups")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigEmptyOrGroups(key, value),
				ExpectError: regexp.MustCompile(`(?s).*or_groups.*at least 1.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_emptyLabelIDs(t *testing.T) {
	key := testAccRandomName("tf-acc-Empty")
	value := testAccRandomName("tf-acc-LabelIDs")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigEmptyLabelIDs(key, value),
				ExpectError: regexp.MustCompile(`(?s).*label_ids.*at least 1.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_missingSelectors(t *testing.T) {
	key := testAccRandomName("tf-acc-Missing")
	value := testAccRandomName("tf-acc-Selectors")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigMissingSelectors(key, value),
				ExpectError: regexp.MustCompile(`(?s).*Missing Label Group Selectors.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_invalidRawJSON(t *testing.T) {
	key := testAccRandomName("tf-acc-Invalid")
	value := testAccRandomName("tf-acc-JSON")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigInvalidJSON(key, value),
				ExpectError: regexp.MustCompile(`(?s).*Invalid JSON.*raw_include_json.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_deleteOrder(t *testing.T) {
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderWithRule(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelExpectFailure("guardicore_label.base"),
				),
			},
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderNoRule(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelGroupOutOfBand("guardicore_label_group.test"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderLabelOnly(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelOutOfBand("guardicore_label.base"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccLabelGroupResourceConfigTyped(labelKey, labelValue, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "test" {
  key      = %[3]q
  value    = %[4]q
  comments = "All web servers"

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.base.id]
      }
    ]
  }
}
`, labelKey, labelValue, groupKey, groupValue)
}

func testAccLabelGroupResourceConfigTypedUpdated(labelKey, labelValue, groupKey, groupValue string) string {
	secondLabelValue := testAccRandomName("tf-acc-Extra")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "extra" {
  key   = %[1]q
  value = %[5]q
}

resource "guardicore_label_group" "test" {
  key      = %[3]q
  value    = %[4]q
  comments = "Updated comment"

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.base.id, guardicore_label.extra.id]
      }
    ]
  }
}
`, labelKey, labelValue, groupKey, groupValue, secondLabelValue)
}

func testAccLabelGroupResourceConfigIncludeExclude(labelKey, key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "include" {
  key   = %[1]q
  value = "Include"
}

resource "guardicore_label" "exclude" {
  key   = %[1]q
  value = "Exclude"
}

resource "guardicore_label_group" "test" {
  key   = %[2]q
  value = %[3]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.include.id]
      }
    ]
  }

  exclude = {
    or_groups = [
      {
        label_ids = [guardicore_label.exclude.id]
      }
    ]
  }
}
`, labelKey, key, value)
}

func testAccLabelGroupResourceConfigRawOverlay(key, value string) string {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "typed" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "raw" {
  key   = %[1]q
  value = "raw-only"
}

resource "guardicore_label_group" "test" {
  key   = %[3]q
  value = %[4]q

  raw_include_json = jsonencode({
    or_labels = [
      {
        and_labels = [guardicore_label.raw.id]
      }
    ]
  })

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.typed.id]
      }
    ]
  }
}
`, labelKey, labelValue, key, value)
}

func testAccLabelGroupResourceConfigEmptyOrGroups(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %[1]q
  value = %[2]q

  include = {
    or_groups = []
  }
}
`, key, value)
}

func testAccLabelGroupResourceConfigEmptyLabelIDs(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %[1]q
  value = %[2]q

  include = {
    or_groups = [
      {
        label_ids = []
      }
    ]
  }
}
`, key, value)
}

func testAccLabelGroupResourceConfigMissingSelectors(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %[1]q
  value = %[2]q
}
`, key, value)
}

func testAccLabelGroupResourceConfigInvalidJSON(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key              = %[1]q
  value            = %[2]q
  raw_include_json = "{invalid json"
}
`, key, value)
}

func testAccLabelGroupResourceConfigDeleteOrderWithRule(labelKey, labelValue, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "test" {
  key   = %[3]q
  value = %[4]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.base.id]
      }
    ]
  }
}

resource "guardicore_policy_rule" "test" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Delete order test"

  source = {
    label_group_ids = [guardicore_label_group.test.id]
  }

  destination = {
    any = true
  }

  ports        = [443]
  ip_protocols = ["TCP"]
}
`, labelKey, labelValue, groupKey, groupValue)
}

func testAccLabelGroupResourceConfigDeleteOrderNoRule(labelKey, labelValue, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "test" {
  key   = %[3]q
  value = %[4]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.base.id]
      }
    ]
  }
}
`, labelKey, labelValue, groupKey, groupValue)
}

func testAccLabelGroupResourceConfigDeleteOrderLabelOnly(labelKey, labelValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}
`, labelKey, labelValue)
}

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
