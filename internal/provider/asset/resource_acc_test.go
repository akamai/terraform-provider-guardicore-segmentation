package asset_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAssetResource_basic(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-asset")
	name2 := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccAssetResourceConfig(name1, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", name1),
					resource.TestCheckResourceAttr("guardicore_asset.test", "orchestration_obj_id", orchObjID),
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.0", "10.0.0.1"),
				),
			},
			// ImportState testing.
			{
				ResourceName:            "guardicore_asset.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"orchestration_metadata", "orchestration_obj_id"},
			},
			// Update name and Read testing.
			{
				Config: testAccAssetResourceConfig(name2, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", name2),
				),
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccAssetResource_withOptionalFields(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config: testAccAssetResourceConfigFull(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_asset.test", "comments", "Test asset comment"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "status", "on"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.0", "10.0.0.1"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.1", "10.0.0.2"),
				),
			},
			// ImportState testing.
			{
				ResourceName:            "guardicore_asset.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"orchestration_metadata", "orchestration_obj_id"},
			},
		},
	})
}

func TestAccAssetResource_updateNics(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with one IP.
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.0", "10.0.0.1"),
				),
			},
			// Update NIC IP addresses.
			{
				Config: testAccAssetResourceConfigWithIP(name, orchObjID, "10.0.0.99"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.0", "10.0.0.99"),
				),
			},
		},
	})
}

func TestAccAssetResource_disappears(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset")
	orchObjID := acctest.TestAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
				),
			},
			// Delete out-of-band (deactivate)
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteAssetOutOfBand("guardicore_asset.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAssetResource_driftOnRemoteCommentsChange(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-drift")
	orchObjID := acctest.TestAccRandomName("orch-drift")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config: testAccAssetResourceConfigFull(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_asset.test", "comments", "Test asset comment"),
				),
			},
			{
				Config: testAccAssetResourceConfigFull(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccUpdateAssetCommentsOutOfBand("guardicore_asset.test", "remote-change-comment"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:             testAccAssetResourceConfigFull(name, orchObjID),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAssetResource_eventualConsistencyReadAfterCreate(t *testing.T) {
	restoreHook := common.SetReadAfterCreateVisibilityHookForTest(func(resourceName string, attempt int) bool {
		return resourceName == "asset" && attempt <= 2
	})
	t.Cleanup(restoreHook)

	name := acctest.TestAccRandomName("tf-acc-asset-ec")
	orchObjID := acctest.TestAccRandomName("orch-ec")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_asset.test", "orchestration_obj_id", orchObjID),
				),
			},
		},
	})
}

func TestAccAssetResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Empty name
			{
				Config:      testAccAssetResourceConfigWithName("", "orch-1"),
				ExpectError: regexp.MustCompile(`.*string length must be at least 1.*`),
			},
			// Invalid status
			{
				Config:      testAccAssetResourceConfigWithStatus("test-name", "orch-1", "invalid"),
				ExpectError: regexp.MustCompile(`.*value must be one of.*`),
			},
			// extra_json must be object
			{
				Config:      testAccAssetResourceConfigWithInvalidMetadataExtraType("test-name", "orch-1"),
				ExpectError: regexp.MustCompile(`(?s).*extra_json.*must be a JSON object.*`),
			},
			// extra_json cannot override typed keys
			{
				Config:      testAccAssetResourceConfigWithReservedMetadataExtraKey("test-name", "orch-1"),
				ExpectError: regexp.MustCompile(`(?s).*extra_json.*reserved key.*asset_type.*`),
			},
		},
	})
}

func testAccAssetResourceConfigWithInvalidMetadataExtraType(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  orchestration_metadata = {
    asset_type         = "F5"
    f5_device_hostname = "bigip-01.example.com"
    partition          = "/Common"
    vs_name            = "vs-main"
    extra_json         = jsonencode(["bad"])
  }

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithReservedMetadataExtraKey(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  orchestration_metadata = {
    asset_type         = "F5"
    f5_device_hostname = "bigip-01.example.com"
    partition          = "/Common"
    vs_name            = "vs-main"
    extra_json = jsonencode({
      asset_type = "OVERRIDE"
    })
  }

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfig(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigFull(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q
  status               = "on"
  comments             = "Test asset comment"
  nics = [{
    ip_addresses = ["10.0.0.1", "10.0.0.2"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithIP(name, orchObjID, ip string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = [%[3]q]
    mac_address  = "00:11:22:33:44:66"
  }]
}
`, name, orchObjID, ip)
}

func testAccAssetResourceConfigWithName(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithStatus(name, orchObjID, status string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q
  status               = %[3]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID, status)
}

// Label acceptance tests

func TestAccAssetResource_serverAssignedLabels(t *testing.T) {
	// Key regression test: creating an asset without labels should not cause
	// "Provider produced inconsistent result after apply" when the server
	// auto-assigns labels (e.g., "Agent: Not Installed").
	// Server-assigned labels are filtered out — only user-specified labels are tracked.
	name := acctest.TestAccRandomName("tf-acc-asset-sal")
	orchObjID := acctest.TestAccRandomName("orch-sal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create without labels — should succeed without "inconsistent result" error
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", name),
					// labels should not be in state (null) since user didn't configure them
					resource.TestCheckNoResourceAttr("guardicore_asset.test", "labels.#"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "guardicore_asset.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"orchestration_metadata", "orchestration_obj_id"},
			},
		},
	})
}

func TestAccAssetResource_withLabels(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-lbl")
	orchObjID := acctest.TestAccRandomName("orch-lbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with explicit label — only user's label should be in state
			// (server-assigned labels like "Agent: Not Installed" are filtered out)
			{
				Config: testAccAssetResourceConfigWithLabels(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("guardicore_asset.test", "labels.*", map[string]string{
						"key":   fmt.Sprintf("tf-acc-asset-label-%s", orchObjID),
						"value": "test-value",
					}),
					resource.TestCheckTypeSetElemAttrPair(
						"guardicore_asset.test", "labels.*.id",
						"guardicore_label.test_asset_label", "id",
					),
				),
			},
			// Remove user-specified labels (go back to no-labels config)
			{
				Config: testAccAssetResourceConfig(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckNoResourceAttr("guardicore_asset.test", "labels.#"),
				),
			},
		},
	})
}

func TestAccAssetResource_withExplicitEmptyLabels(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-emptylbl")
	orchObjID := acctest.TestAccRandomName("orch-emptylbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config: testAccAssetResourceConfigWithEmptyLabels(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "0"),
				),
			},
			{
				Config:             testAccAssetResourceConfigWithEmptyLabels(name, orchObjID),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccAssetResource_withDynamicLabelIgnored(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-dynlbl")
	orchObjID := acctest.TestAccRandomName("orch-dynlbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			{
				Config:      testAccAssetResourceConfigWithDynamicLabel(name, orchObjID),
				ExpectError: regexp.MustCompile(`(?i)dynamic criteria.*cannot be manually assigned`),
			},
		},
	})
}

func TestAccAssetResource_withReadOnlyLabelRejected(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-rolbl")
	orchObjID := acctest.TestAccRandomName("orch-rolbl")
	readOnly := acctest.TestAccReadOnlyLabel(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAssetResourceConfigWithReadOnlyLabel(name, orchObjID, readOnly.ID, readOnly.Key, readOnly.Value),
				ExpectError: regexp.MustCompile(`(?is)(read-only|system-managed).*(cannot be managed|cannot be assigned)`),
			},
		},
	})
}

func testAccAssetResourceConfigWithLabels(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test_asset_label" {
  key   = "tf-acc-asset-label-%[2]s"
  value = "test-value"
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = [{
    id    = guardicore_label.test_asset_label.id
    key   = guardicore_label.test_asset_label.key
    value = guardicore_label.test_asset_label.value
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithEmptyLabels(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = []
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithDynamicLabel(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test_dynamic_label" {
  key   = "tf-acc-asset-dyn-%[2]s"
  value = "test-value"

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "%[1]s"
    }
  ]
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = [{
    id    = guardicore_label.test_dynamic_label.id
    key   = guardicore_label.test_dynamic_label.key
    value = guardicore_label.test_dynamic_label.value
  }]
}
`, name, orchObjID)
}

func testAccAssetResourceConfigWithReadOnlyLabel(name, orchObjID, labelID, labelKey, labelValue string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = [{
    id    = %[3]q
    key   = %[4]q
    value = %[5]q
  }]
}
`, name, orchObjID, labelID, labelKey, labelValue)
}

func TestAccAssetResource_withLabelsAllFields(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-lbl2")
	orchObjID := acctest.TestAccRandomName("orch-lbl2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with all three label fields via Terraform references
			{
				Config: testAccAssetResourceConfigWithLabelsAllFields(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"guardicore_asset.test", "labels.*.id",
						"guardicore_label.test_asset_label", "id",
					),
					resource.TestCheckTypeSetElemAttrPair(
						"guardicore_asset.test", "labels.*.key",
						"guardicore_label.test_asset_label", "key",
					),
					resource.TestCheckTypeSetElemAttrPair(
						"guardicore_asset.test", "labels.*.value",
						"guardicore_label.test_asset_label", "value",
					),
				),
			},
		},
	})
}

func testAccAssetResourceConfigWithLabelsAllFields(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test_asset_label" {
  key   = "tf-acc-asset-label-%[2]s"
  value = "test-value"
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = [{
    id    = guardicore_label.test_asset_label.id
    key   = guardicore_label.test_asset_label.key
    value = guardicore_label.test_asset_label.value
  }]
}
`, name, orchObjID)
}

func TestAccAssetResource_withMultipleLabels(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-asset-mlbl")
	orchObjID := acctest.TestAccRandomName("orch-mlbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with 3 labels and verify set membership.
			{
				Config: testAccAssetResourceConfigWithMultipleLabels(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "3"),
					resource.TestCheckTypeSetElemNestedAttrs("guardicore_asset.test", "labels.*", map[string]string{
						"key":   fmt.Sprintf("tf-acc-lbl-a-%s", orchObjID),
						"value": "value-a",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("guardicore_asset.test", "labels.*", map[string]string{
						"key":   fmt.Sprintf("tf-acc-lbl-b-%s", orchObjID),
						"value": "value-b",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("guardicore_asset.test", "labels.*", map[string]string{
						"key":   fmt.Sprintf("tf-acc-lbl-c-%s", orchObjID),
						"value": "value-c",
					}),
				),
			},
		},
	})
}

func testAccAssetResourceConfigWithMultipleLabels(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "label_a" {
  key   = "tf-acc-lbl-a-%[2]s"
  value = "value-a"
}

resource "guardicore_label" "label_b" {
  key   = "tf-acc-lbl-b-%[2]s"
  value = "value-b"
}

resource "guardicore_label" "label_c" {
  key   = "tf-acc-lbl-c-%[2]s"
  value = "value-c"
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]

  labels = [
    {
      id    = guardicore_label.label_a.id
      key   = guardicore_label.label_a.key
      value = guardicore_label.label_a.value
    },
    {
      id    = guardicore_label.label_b.id
      key   = guardicore_label.label_b.key
      value = guardicore_label.label_b.value
    },
    {
      id    = guardicore_label.label_c.id
      key   = guardicore_label.label_c.key
      value = guardicore_label.label_c.value
    },
  ]
}
`, name, orchObjID)
}

// Worksite assignment acceptance tests

func TestAccAssetResource_withWorksite(t *testing.T) {
	assetName := acctest.TestAccRandomName("tf-acc-asset-ws")
	orchObjID := acctest.TestAccRandomName("orch-ws")
	worksiteName := acctest.TestAccRandomName("tf-acc-ws")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create asset with worksite
			{
				Config: testAccAssetResourceConfigWithWorksite(assetName, orchObjID, worksiteName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", assetName),
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "worksite_id"),
					resource.TestCheckResourceAttrPair("guardicore_asset.test", "worksite_id", "guardicore_worksite.test", "id"),
				),
			},
		},
	})
}

func TestAccAssetResource_worksiteUpdate(t *testing.T) {
	assetName := acctest.TestAccRandomName("tf-acc-asset-wsu")
	orchObjID := acctest.TestAccRandomName("orch-wsu")
	worksiteName1 := acctest.TestAccRandomName("tf-acc-ws1")
	worksiteName2 := acctest.TestAccRandomName("tf-acc-ws2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create asset without worksite
			{
				Config: testAccAssetResourceConfig(assetName, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
				),
			},
			// Add worksite
			{
				Config: testAccAssetResourceConfigWithWorksite(assetName, orchObjID, worksiteName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_asset.test", "worksite_id", "guardicore_worksite.test", "id"),
				),
			},
			// Change worksite
			{
				Config: testAccAssetResourceConfigWithWorksite2(assetName, orchObjID, worksiteName1, worksiteName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_asset.test", "worksite_id", "guardicore_worksite.test2", "id"),
				),
			},
		},
	})
}

func TestAccAssetResource_invalidWorksiteRef(t *testing.T) {
	assetName := acctest.TestAccRandomName("tf-acc-asset-iwsr")
	orchObjID := acctest.TestAccRandomName("orch-iwsr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q
  worksite_id          = "00000000-0000-0000-0000-000000000000"

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, assetName, orchObjID),
				ExpectError: regexp.MustCompile(`(?is)(does not exist|invalid worksite reference|unable to assign asset to worksite|failed to assign worksite|status\s*\d+)`),
			},
		},
	})
}

func testAccAssetResourceConfigWithWorksite(name, orchObjID, worksiteName string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[3]q
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q
  worksite_id          = guardicore_worksite.test.id

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID, worksiteName)
}

func testAccAssetResourceConfigWithWorksite2(name, orchObjID, worksiteName1, worksiteName2 string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[3]q
}

resource "guardicore_worksite" "test2" {
  name = %[4]q
}

resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q
  worksite_id          = guardicore_worksite.test2.id

  nics = [{
    ip_addresses = ["10.0.0.1"]
    mac_address  = "00:11:22:33:44:55"
  }]
}
`, name, orchObjID, worksiteName1, worksiteName2)
}

func TestAccAssetResource_multipleResources(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-asset-m1")
	name2 := acctest.TestAccRandomName("tf-acc-asset-m2")
	orch1 := acctest.TestAccRandomName("orch-m1")
	orch2 := acctest.TestAccRandomName("orch-m2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{{
			Config: testAccAssetResourceConfigMultiple(name1, orch1, name2, orch2),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("guardicore_asset.test1", "id"),
				resource.TestCheckResourceAttrSet("guardicore_asset.test2", "id"),
			),
		}},
	})
}

func testAccAssetResourceConfigMultiple(name1, orch1, name2, orch2 string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test1" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.0.1.1"]
    mac_address  = "00:11:22:33:44:71"
  }]
}

resource "guardicore_asset" "test2" {
  name                 = %[3]q
  orchestration_obj_id = %[4]q

  nics = [{
    ip_addresses = ["10.0.1.2"]
    mac_address  = "00:11:22:33:44:72"
  }]
}
`, name1, orch1, name2, orch2)
}

// Acceptance test: create an asset normally, then verify NIC filtering works
// during read. The unit tests cover empty-IP filtering directly, but this
// acceptance test exercises the full create-read-update-import cycle.

func TestAccAssetResource_importExternalAsset(t *testing.T) {
	acctest.TestAccAssetPreCheck(t)

	apiConfig := client.Config{
		BaseURL:            acctest.TestConfigInstance.BaseURL,
		Username:           acctest.TestConfigInstance.Username,
		Password:           acctest.TestConfigInstance.Password,
		AccessToken:        acctest.TestConfigInstance.AccessToken,
		RefreshToken:       acctest.TestConfigInstance.RefreshToken,
		InsecureSkipVerify: acctest.TestConfigInstance.InsecureSkipVerify,
	}
	apiClient, err := client.NewClient(apiConfig)
	if err != nil {
		t.Fatalf("failed to create API client: %s", err)
	}

	ctx := context.Background()
	orchObjID := acctest.TestAccRandomName("orch-ext")
	assetName := acctest.TestAccRandomName("tf-acc-asset-ext")

	// Create an asset via the API directly (simulating external creation).
	createResp, err := apiClient.BulkCreateAssets(ctx, []client.AssetCreate{
		{
			Name:               assetName,
			OrchestrationObjID: orchObjID,
			Nics: []client.AssetNIC{
				{MacAddress: "AA:BB:CC:11:22:01", IPAddresses: []string{"10.88.0.1"}},
				{MacAddress: "AA:BB:CC:11:22:02", IPAddresses: []string{"10.88.0.2"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test asset: %s", err)
	}

	assetID := createResp.CreatedAssetIDs[orchObjID]
	if assetID == "" {
		t.Fatal("failed to extract asset ID from create response")
	}

	t.Cleanup(func() {
		_ = apiClient.BulkDeactivateAssets(context.Background(), []string{assetID})
	})

	// Import the externally-created asset into Terraform.
	// This exercises the Read (apiToModel) path including NIC conversion.
	// After import, the orch_obj_id is not in state (API doesn't return it),
	// so we only verify import succeeds — a subsequent apply would require
	// replacement which we test separately in _basic.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccAssetResourceConfigForImport(assetName, orchObjID),
				ResourceName:      "guardicore_asset.test",
				ImportState:       true,
				ImportStateId:     assetID,
				ImportStateVerify: false, // orch_obj_id and orch_metadata not returned by API
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "name", assetName),
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.#", "2"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "nics.0.ip_addresses.#", "1"),
				),
			},
		},
	})
}

func testAccAssetResourceConfigForImport(name, orchObjID string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %[1]q
  orchestration_obj_id = %[2]q

  nics = [{
    ip_addresses = ["10.88.0.1"]
    mac_address  = "AA:BB:CC:11:22:01"
  },
  {
    ip_addresses = ["10.88.0.2"]
    mac_address  = "AA:BB:CC:11:22:02"
  }]
}
`, name, orchObjID)
}
