package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorksiteDataSource_byID(t *testing.T) {
	name := testAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorksiteDataSourceConfigByID(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_worksite.test", "name", name),
				),
			},
		},
	})
}

func testAccWorksiteDataSourceConfigByID(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

data "guardicore_worksite" "test" {
  id = guardicore_worksite.test.id
}
`, name)
}

func TestAccWorksiteDataSource_byName(t *testing.T) {
	name := testAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorksiteDataSourceConfigByName(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_worksite.test", "name", name),
				),
			},
		},
	})
}

func testAccWorksiteDataSourceConfigByName(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

data "guardicore_worksite" "test" {
  name = guardicore_worksite.test.name
}
`, name)
}
