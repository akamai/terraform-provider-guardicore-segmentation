package labelgroup_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelGroupResource_typedSelectors(t *testing.T) {
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
					acctest.TestAccCheckLabelGroupPublished("guardicore_label_group.test"),
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
	key := acctest.TestAccRandomName("tf-acc-Group")
	value := acctest.TestAccRandomName("tf-acc-Value")
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_label_group"),
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
	key := acctest.TestAccRandomName("tf-acc-Raw")
	value := acctest.TestAccRandomName("tf-acc-Overlay")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_label_group"),
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
	key := acctest.TestAccRandomName("tf-acc-Empty")
	value := acctest.TestAccRandomName("tf-acc-OrGroups")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigEmptyOrGroups(key, value),
				ExpectError: regexp.MustCompile(`(?s).*or_groups.*at least 1.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_emptyLabelIDs(t *testing.T) {
	key := acctest.TestAccRandomName("tf-acc-Empty")
	value := acctest.TestAccRandomName("tf-acc-LabelIDs")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigEmptyLabelIDs(key, value),
				ExpectError: regexp.MustCompile(`(?s).*label_ids.*at least 1.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_missingSelectors(t *testing.T) {
	key := acctest.TestAccRandomName("tf-acc-Missing")
	value := acctest.TestAccRandomName("tf-acc-Selectors")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigMissingSelectors(key, value),
				ExpectError: regexp.MustCompile(`(?s).*Missing Label Group Selectors.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_invalidRawJSON(t *testing.T) {
	key := acctest.TestAccRandomName("tf-acc-Invalid")
	value := acctest.TestAccRandomName("tf-acc-JSON")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelGroupResourceConfigInvalidJSON(key, value),
				ExpectError: regexp.MustCompile(`(?s).*Invalid JSON.*raw_include_json.*`),
			},
		},
	})
}

func TestAccLabelGroupResource_deleteOrder(t *testing.T) {
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderWithRule(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelExpectFailure("guardicore_label.base"),
				),
			},
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderNoRule(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelGroupOutOfBand("guardicore_label_group.test"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccLabelGroupResourceConfigDeleteOrderLabelOnly(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelOutOfBand("guardicore_label.base"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccLabelGroupResourceConfigTyped(labelKey, labelValue, groupKey, groupValue string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	secondLabelValue := acctest.TestAccRandomName("tf-acc-Extra")
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %[1]q
  value = %[2]q
}
`, key, value)
}

func testAccLabelGroupResourceConfigInvalidJSON(key, value string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key              = %[1]q
  value            = %[2]q
  raw_include_json = "{invalid json"
}
`, key, value)
}

func testAccLabelGroupResourceConfigDeleteOrderWithRule(labelKey, labelValue, groupKey, groupValue string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}
`, labelKey, labelValue)
}
