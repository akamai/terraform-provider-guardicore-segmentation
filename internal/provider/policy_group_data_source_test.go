package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyGroupDataSource_byID(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-ds-id")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupDataSourceConfigByID(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "type", "FQDN"),
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "members_json"),
				),
			},
		},
	})
}

func testAccPolicyGroupDataSourceConfigByID(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "FQDN"

  members_json = jsonencode([
    "example.com",
    "*.test.com"
  ])
}

data "guardicore_policy_group" "test" {
  id = guardicore_policy_group.test.id
}
`, name)
}

func TestAccPolicyGroupDataSource_byNameAndType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-ds-name")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupDataSourceConfigByNameAndType(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "type", "IP_ADDRESS"),
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "members_json"),
				),
			},
		},
	})
}

func testAccPolicyGroupDataSourceConfigByNameAndType(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "IP_ADDRESS"

  members_json = jsonencode([
    { subnet = "10.0.0.0/8" }
  ])
}

data "guardicore_policy_group" "test" {
  name = guardicore_policy_group.test.name
  type = guardicore_policy_group.test.type
}
`, name)
}

func TestAccPolicyGroupDataSource_labelTypeWithExclude(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-ds-exclude")
	labelKey1 := testAccRandomName("tf-acc-label")
	labelValue1 := testAccRandomName("tf-acc-value")
	labelKey2 := testAccRandomName("tf-acc-label")
	labelValue2 := testAccRandomName("tf-acc-value")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupDataSourceConfigLabelTypeWithExclude(name, labelKey1, labelValue1, labelKey2, labelValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_policy_group.test", "type", "LABEL"),
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "members_json"),
					resource.TestCheckResourceAttrSet("data.guardicore_policy_group.test", "exclude_members_json"),
				),
			},
		},
	})
}

func testAccPolicyGroupDataSourceConfigLabelTypeWithExclude(name, labelKey1, labelValue1, labelKey2, labelValue2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "include" {
  key   = %[2]q
  value = %[3]q
}

resource "guardicore_label" "exclude" {
  key   = %[4]q
  value = %[5]q
}

resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "LABEL"

  members_json = jsonencode([
    [guardicore_label.include.id]
  ])

  exclude_members_json = jsonencode([
    [guardicore_label.exclude.id]
  ])
}

data "guardicore_policy_group" "test" {
  id = guardicore_policy_group.test.id
}
`, name, labelKey1, labelValue1, labelKey2, labelValue2)
}
