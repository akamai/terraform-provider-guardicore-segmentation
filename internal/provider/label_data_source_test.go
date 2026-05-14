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
					resource.TestCheckResourceAttr("data.guardicore_label.test", "system_managed", "false"),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "managed_by", "terraform"),
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
					testCheckLabelCriteriaContains("data.guardicore_label.test", "CONTAINS", "redis"),
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

func TestAccLabelDataSource_systemManagedByID(t *testing.T) {
	label := testAccReadOnlyLabel(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelDataSourceConfigReadOnlyByID(label.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.guardicore_label.test", "id", label.ID),
					resource.TestCheckResourceAttr("data.guardicore_label.test", "system_managed", "true"),
					resource.TestCheckResourceAttrSet("data.guardicore_label.test", "managed_by"),
				),
			},
		},
	})
}

func testAccLabelDataSourceConfigReadOnlyByID(id string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
data "guardicore_label" "test" {
  id = %[1]q
}
`, id)
}
