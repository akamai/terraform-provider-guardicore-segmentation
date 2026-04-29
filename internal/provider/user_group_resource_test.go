package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserGroupResource_basic(t *testing.T) {
	title1 := testAccRandomName("tf-acc-usergroup")
	title2 := testAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_user_group"),
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccUserGroupResourceConfig(title1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_user_group.test", "id"),
					resource.TestCheckResourceAttr("guardicore_user_group.test", "title", title1),
					resource.TestCheckResourceAttr("guardicore_user_group.test", "orchestrations_groups.#", "1"),
					resource.TestCheckResourceAttrSet("guardicore_user_group.test", "orchestrations_groups.0.orchestration_id"),
					resource.TestCheckResourceAttr("guardicore_user_group.test", "orchestrations_groups.0.groups.#", "1"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_user_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update title and Read testing.
			{
				Config: testAccUserGroupResourceConfig(title2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_user_group.test", "id"),
					resource.TestCheckResourceAttr("guardicore_user_group.test", "title", title2),
				),
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccUserGroupResource_disappears(t *testing.T) {
	title := testAccRandomName("tf-acc-usergroup")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_user_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupResourceConfig(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_user_group.test", "id"),
				),
			},
			// Delete out-of-band
			{
				Config: testAccUserGroupResourceConfig(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteUserGroupOutOfBand("guardicore_user_group.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccUserGroupResource_titleValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccUserGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserGroupResourceConfigWithTitle(""),
				ExpectError: regexp.MustCompile(`.*string length must be at least 1.*`),
			},
		},
	})
}

func testAccUserGroupResourceConfig(title string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_user_group" "test" {
  title = %[1]q

  orchestrations_groups {
    orchestration_id = %[2]q
    groups           = [%[3]q]
  }
}
`, title, testConfig.UserGroupOrchestrationID, testConfig.UserGroupGroupID)
}

func testAccUserGroupResourceConfigWithTitle(title string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_user_group" "test" {
  title = %[1]q

  orchestrations_groups {
    orchestration_id = %[2]q
    groups           = [%[3]q]
  }
}
`, title, testConfig.UserGroupOrchestrationID, testConfig.UserGroupGroupID)
}
