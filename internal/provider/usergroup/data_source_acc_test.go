package usergroup_test

import (
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserGroupDataSource_byID(t *testing.T) {
	title := acctest.TestAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_user_group"),
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
	title := acctest.TestAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_user_group"),
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
`, title, acctest.TestConfigInstance.UserGroupOrchestrationID, acctest.TestConfigInstance.UserGroupGroupID)
}

func testAccUserGroupDataSourceConfigByTitle(title string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
`, title, acctest.TestConfigInstance.UserGroupOrchestrationID, acctest.TestConfigInstance.UserGroupGroupID)
}
