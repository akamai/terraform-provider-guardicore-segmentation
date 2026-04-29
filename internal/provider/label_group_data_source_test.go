package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelGroupDataSource_byID(t *testing.T) {
	key := testAccRandomName("tf-acc-DS")
	value := testAccRandomName("tf-acc-ByID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupDataSourceConfigByID(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "key", key),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "value", value),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "comments", "Test comment"),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "include.or_groups.#", "1"),
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "include_json"),
				),
			},
		},
	})
}

func TestAccLabelGroupDataSource_byKeyValue(t *testing.T) {
	key := testAccRandomName("tf-acc-DS")
	value := testAccRandomName("tf-acc-ByKV")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupDataSourceConfigByKeyValue(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "key", key),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "value", value),
				),
			},
		},
	})
}

func TestAccLabelGroupDataSource_withSelectors(t *testing.T) {
	key := testAccRandomName("tf-acc-DS")
	value := testAccRandomName("tf-acc-Selectors")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelGroupDataSourceConfigWithSelectors(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "include.or_groups.#", "1"),
					resource.TestCheckResourceAttr("data.guardicore_label_group.test", "exclude.or_groups.#", "1"),
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "include_json"),
					resource.TestCheckResourceAttrSet("data.guardicore_label_group.test", "exclude_json"),
				),
			},
		},
	})
}

func testAccLabelGroupDataSourceConfigByID(key, value string) string {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "minimal" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "test" {
  key      = %[3]q
  value    = %[4]q
  comments = "Test comment"

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.minimal.id]
      }
    ]
  }
}

data "guardicore_label_group" "test" {
  id = guardicore_label_group.test.id
}
`, labelKey, labelValue, key, value)
}

func testAccLabelGroupDataSourceConfigByKeyValue(key, value string) string {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "minimal" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "test" {
  key   = %[3]q
  value = %[4]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.minimal.id]
      }
    ]
  }
}

data "guardicore_label_group" "test" {
  key   = guardicore_label_group.test.key
  value = guardicore_label_group.test.value
}
`, labelKey, labelValue, key, value)
}

func testAccLabelGroupDataSourceConfigWithSelectors(key, value string) string {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "include" {
  key   = %[1]q
  value = "Yes"
}

resource "guardicore_label" "exclude" {
  key   = %[1]q
  value = "No"
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

data "guardicore_label_group" "test" {
  id = guardicore_label_group.test.id
}
`, labelKey, key, value)
}
