package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelDataSource_byID(t *testing.T) {
	key := testAccRandomName("tf-acc-LabelKey")
	value := testAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfigByID(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "value", value),
				),
			},
		},
	})
}

func testAccLabelDataSourceConfigByID(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q
}

data "guardicore_label" "test" {
  id = guardicore_label.test.id
}
`, key, value)
}

func TestAccLabelDataSource_byKeyValue(t *testing.T) {
	key := testAccRandomName("tf-acc-LabelKey")
	value := testAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfigByKeyValue(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_label.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "value", value),
				),
			},
		},
	})
}

func testAccLabelDataSourceConfigByKeyValue(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q
}

data "guardicore_label" "test" {
  key   = guardicore_label.test.key
  value = guardicore_label.test.value
}
`, key, value)
}

func TestAccLabelDataSource_withCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-LabelKey")
	value := testAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfigWithCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_label.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "value", value),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "criteria.0.op", "CONTAINS"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "criteria.0.argument", "redis"),
				),
			},
		},
	})
}

func testAccLabelDataSourceConfigWithCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "redis"
    }
  ]
}

data "guardicore_label" "test" {
  id = guardicore_label.test.id
}
`, key, value)
}
