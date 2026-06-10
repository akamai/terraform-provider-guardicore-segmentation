package worksite_test

import (
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorksiteDataSource_byID(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorksiteDataSourceConfigByID(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_worksite.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_worksite.test", "system_managed", "false"),
					resource.TestCheckResourceAttr("data.guardicore_worksite.test", "managed_by", "terraform"),
				),
			},
		},
	})
}

func testAccWorksiteDataSourceConfigByID(name string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

data "guardicore_worksite" "test" {
  id = guardicore_worksite.test.id
}
`, name)
}

func TestAccWorksiteDataSource_byName(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

data "guardicore_worksite" "test" {
  name = guardicore_worksite.test.name
}
`, name)
}
