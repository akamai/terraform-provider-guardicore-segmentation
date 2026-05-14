package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserGroupDataSource_byID(t *testing.T) {
	title := testAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_user_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupDataSourceConfigByID(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.guardicore_user_group.test", "id", "guardicore_user_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_user_group.test", "title", title),
					resource.TestCheckResourceAttr("data.guardicore_user_group.test", "orchestrations_groups.#", "1"),
					resource.TestCheckResourceAttr("data.guardicore_user_group.test", "system_managed", "false"),
					resource.TestCheckResourceAttr("data.guardicore_user_group.test", "managed_by", "terraform"),
				),
			},
		},
	})
}

func TestAccUserGroupDataSource_byTitle(t *testing.T) {
	title := testAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_user_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupDataSourceConfigByTitle(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.guardicore_user_group.test", "id", "guardicore_user_group.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_user_group.test", "title", title),
				),
			},
		},
	})
}

func testAccUserGroupDataSourceConfigByID(title string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_user_group" "test" {
  title = %[1]q

  orchestrations_groups {
    orchestration_id = %[2]q
    groups           = [%[3]q]
  }
}

data "guardicore_user_group" "test" {
  id = guardicore_user_group.test.id
}
`, title, testConfig.UserGroupOrchestrationID, testConfig.UserGroupGroupID)
}

func testAccUserGroupDataSourceConfigByTitle(title string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_user_group" "test" {
  title = %[1]q

  orchestrations_groups {
    orchestration_id = %[2]q
    groups           = [%[3]q]
  }
}

data "guardicore_user_group" "test" {
  title = guardicore_user_group.test.title
}
`, title, testConfig.UserGroupOrchestrationID, testConfig.UserGroupGroupID)
}
