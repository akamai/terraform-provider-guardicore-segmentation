package worksite_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorksiteResource_basic(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-worksite")
	name2 := acctest.TestAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_worksite"),
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccWorksiteResourceConfig(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "name", name1),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "system_managed", "false"),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "managed_by", "terraform"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_worksite.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing.
			{
				Config: testAccWorksiteResourceConfig(name2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "name", name2),
				),
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccWorksiteResource_withComment(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite")
	comment1 := "Initial comment"
	comment2 := "Updated comment"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_worksite"),
		Steps: []resource.TestStep{
			// Create with comment.
			{
				Config: testAccWorksiteResourceConfigWithComment(name, comment1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "comment", comment1),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_worksite.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update comment.
			{
				Config: testAccWorksiteResourceConfigWithComment(name, comment2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_worksite.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_worksite.test", "comment", comment2),
				),
			},
		},
	})
}

func TestAccWorksiteResource_disappears(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_worksite"),
		Steps: []resource.TestStep{
			{
				Config: testAccWorksiteResourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
				),
			},
			// Delete out-of-band
			{
				Config: testAccWorksiteResourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteWorksiteOutOfBand("guardicore_worksite.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccWorksiteResource_nameValidation(t *testing.T) {
	longName := strings.Repeat("a", 101)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccWorksiteResourceConfigWithName(""),
				ExpectError: regexp.MustCompile(`.*string length must be between 1 and 100.*`),
			},
			{
				Config:      testAccWorksiteResourceConfigWithName(longName),
				ExpectError: regexp.MustCompile(`.*string length must be between 1 and 100.*`),
			},
		},
	})
}

func TestAccWorksiteResource_commentValidation(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite")
	longComment := strings.Repeat("a", 2001)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccWorksiteResourceConfigWithComment(name, longComment),
				ExpectError: regexp.MustCompile(`.*string length must be at most 2000.*`),
			},
		},
	})
}

func testAccWorksiteResourceConfig(name string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}
`, name)
}

func testAccWorksiteResourceConfigWithComment(name, comment string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name    = %[1]q
  comment = %[2]q
}
`, name, comment)
}

func testAccWorksiteResourceConfigWithName(name string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}
`, name)
}

func TestAccWorksiteResource_multipleResources(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-worksite-m1")
	name2 := acctest.TestAccRandomName("tf-acc-worksite-m2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_worksite"),
		Steps: []resource.TestStep{{
			Config: testAccWorksiteResourceConfigMultiple(name1, name2),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("guardicore_worksite.test1", "id"),
				resource.TestCheckResourceAttrSet("guardicore_worksite.test2", "id"),
			),
		}},
	})
}

func TestAccWorksiteResource_deleteBlockedByAssignedAsset(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-worksite-blocked")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksiteDeleteBlockedPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorksiteResourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					acctest.TestAccAssignAssetToWorksiteOutOfBand("guardicore_worksite.test", acctest.TestConfigInstance.WorksiteAssignedAssetID),
				),
			},
			{
				Config:      acctest.TestAccProviderConfig(),
				ExpectError: regexp.MustCompile(`(?is)(Unable to delete worksite|failed to delete worksite).*(assigned|not deleted|skips=)`),
			},
			{
				Config: testAccWorksiteResourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					acctest.TestAccAssignAssetToAllWorksitesOutOfBand(acctest.TestConfigInstance.WorksiteAssignedAssetID),
				),
			},
		},
	})
}

func testAccWorksiteResourceConfigMultiple(name1, name2 string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test1" {
  name = %[1]q
}

resource "guardicore_worksite" "test2" {
  name = %[2]q
}
`, name1, name2)
}
