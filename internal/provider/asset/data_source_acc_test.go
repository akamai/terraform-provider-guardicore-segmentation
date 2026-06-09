package asset_test

import (
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAssetDataSource_byID(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetDataSourceConfigByID(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_asset.test", "name", name),
				),
			},
		},
	})
}

func TestAccAssetDataSource_byName(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAssetDataSourceConfigByName(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_asset.test", "name", name),
				),
			},
		},
	})
}

func testAccAssetDataSourceConfigByID(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}

data "guardicore_asset" "test" {
  id = guardicore_asset.test.id
}
`, name, orchObjID)
}

func testAccAssetDataSourceConfigByName(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}

data "guardicore_asset" "test" {
  name = guardicore_asset.test.name
}
`, name, orchObjID)
}
