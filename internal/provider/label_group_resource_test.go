package provider

import (
	"fmt"
	"regexp"
	"testing"

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
