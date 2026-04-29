package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAssetResource_basic(t *testing.T) {
	name1 := testAccRandomName("tf-acc-asset")
	name2 := testAccRandomName("tf-acc-asset")
	orchObjID := testAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
				ResourceName:      "guardicore_asset.test",
				ImportState:       true,
				ImportStateVerify: true,
				// orchestration_metadata_json may not round-trip exactly
				ImportStateVerifyIgnore: []string{"orchestration_metadata_json", "orchestration_obj_id"},
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
	name := testAccRandomName("tf-acc-asset")
	orchObjID := testAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
				ImportStateVerifyIgnore: []string{"orchestration_metadata_json", "orchestration_obj_id"},
			},
		},
	})
}

func TestAccAssetResource_updateNics(t *testing.T) {
	name := testAccRandomName("tf-acc-asset")
	orchObjID := testAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
	name := testAccRandomName("tf-acc-asset")
	orchObjID := testAccRandomName("orch")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
					testAccDeleteAssetOutOfBand("guardicore_asset.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAssetResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		},
	})
}

func testAccAssetResourceConfig(name, orchObjID string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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
	return testAccProviderConfig() + fmt.Sprintf(`
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
	return testAccProviderConfig() + fmt.Sprintf(`
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
	return testAccProviderConfig() + fmt.Sprintf(`
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
	return testAccProviderConfig() + fmt.Sprintf(`
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
	name := testAccRandomName("tf-acc-asset-sal")
	orchObjID := testAccRandomName("orch-sal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
				ImportStateVerifyIgnore: []string{"orchestration_metadata_json", "orchestration_obj_id"},
			},
		},
	})
}

func TestAccAssetResource_withLabels(t *testing.T) {
	name := testAccRandomName("tf-acc-asset-lbl")
	orchObjID := testAccRandomName("orch-lbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with explicit label — only user's label should be in state
			// (server-assigned labels like "Agent: Not Installed" are filtered out)
			{
				Config: testAccAssetResourceConfigWithLabels(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "1"),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.0.id",
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

func testAccAssetResourceConfigWithLabels(name, orchObjID string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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

func TestAccAssetResource_withLabelsAllFields(t *testing.T) {
	name := testAccRandomName("tf-acc-asset-lbl2")
	orchObjID := testAccRandomName("orch-lbl2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with all three label fields via Terraform references
			{
				Config: testAccAssetResourceConfigWithLabelsAllFields(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "1"),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.0.id",
						"guardicore_label.test_asset_label", "id",
					),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.0.key",
						"guardicore_label.test_asset_label", "key",
					),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.0.value",
						"guardicore_label.test_asset_label", "value",
					),
				),
			},
		},
	})
}

func testAccAssetResourceConfigWithLabelsAllFields(name, orchObjID string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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
	name := testAccRandomName("tf-acc-asset-mlbl")
	orchObjID := testAccRandomName("orch-mlbl")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccAssetPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
		Steps: []resource.TestStep{
			// Create with 3 labels — order must be preserved despite API reordering
			{
				Config: testAccAssetResourceConfigWithMultipleLabels(name, orchObjID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_asset.test", "id"),
					resource.TestCheckResourceAttr("guardicore_asset.test", "labels.#", "3"),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.0.id",
						"guardicore_label.label_a", "id",
					),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.1.id",
						"guardicore_label.label_b", "id",
					),
					resource.TestCheckResourceAttrPair(
						"guardicore_asset.test", "labels.2.id",
						"guardicore_label.label_c", "id",
					),
				),
			},
		},
	})
}

func testAccAssetResourceConfigWithMultipleLabels(name, orchObjID string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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
	assetName := testAccRandomName("tf-acc-asset-ws")
	orchObjID := testAccRandomName("orch-ws")
	worksiteName := testAccRandomName("tf-acc-ws")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
	assetName := testAccRandomName("tf-acc-asset-wsu")
	orchObjID := testAccRandomName("orch-wsu")
	worksiteName1 := testAccRandomName("tf-acc-ws1")
	worksiteName2 := testAccRandomName("tf-acc-ws2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_asset"),
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
				Config: testAccAssetResourceConfigWithWorksite2(assetName, orchObjID, worksiteName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_asset.test", "worksite_id", "guardicore_worksite.test2", "id"),
				),
			},
		},
	})
}

func TestAccAssetResource_invalidWorksiteRef(t *testing.T) {
	assetName := testAccRandomName("tf-acc-asset-iwsr")
	orchObjID := testAccRandomName("orch-iwsr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
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
				ExpectError: regexp.MustCompile(`(?i)does not exist|Invalid Worksite Reference`),
			},
		},
	})
}

func testAccAssetResourceConfigWithWorksite(name, orchObjID, worksiteName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
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

func testAccAssetResourceConfigWithWorksite2(name, orchObjID, worksiteName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test2" {
  name = %[3]q
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
`, name, orchObjID, worksiteName)
}

// Unit tests for modelLabelsToAPI

func TestModelLabelsToAPI_Nil(t *testing.T) {
	r := &AssetResource{}
	result := r.modelLabelsToAPI(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestModelLabelsToAPI_Empty(t *testing.T) {
	r := &AssetResource{}
	result := r.modelLabelsToAPI([]AssetLabelModel{})
	if result == nil {
		t.Fatal("expected non-nil for empty input")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

func TestModelLabelsToAPI_Populated(t *testing.T) {
	r := &AssetResource{}
	labels := []AssetLabelModel{
		{
			ID:    types.StringValue("id-1"),
			Key:   types.StringValue("env"),
			Value: types.StringValue("prod"),
		},
		{
			ID:    types.StringValue("id-2"),
			Key:   types.StringValue("role"),
			Value: types.StringValue("web"),
		},
	}
	result := r.modelLabelsToAPI(labels)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != "id-1" || result[0].Key != "env" || result[0].Value != "prod" {
		t.Errorf("unexpected first label: %+v", result[0])
	}
	if result[1].ID != "id-2" || result[1].Key != "role" || result[1].Value != "web" {
		t.Errorf("unexpected second label: %+v", result[1])
	}
}

// Unit tests for apiToModel label filtering

func TestApiToModel_ServerAssignedLabelsFiltered(t *testing.T) {
	// When API returns server-assigned labels but user didn't configure any,
	// labels should stay nil (server-assigned labels are not tracked).
	// API list endpoint typically returns labels with only the ID.
	// data.Name is set to simulate a normal Read (not import).
	r := &AssetResource{}
	data := &AssetResourceModel{
		Name: types.StringValue("test"),
	} // Labels is nil — user didn't configure labels
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "auto-1", Key: "", Value: ""},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Errorf("expected nil labels (user didn't configure), got %d labels", len(data.Labels))
	}
}

func TestApiToModel_ImportPopulatesLabels(t *testing.T) {
	// During import, data.Labels is nil and data.Name is null (ImportState only sets ID).
	// Labels from the API should be populated so they appear in state after import.
	r := &AssetResource{}
	data := &AssetResourceModel{} // Name is null (zero value) — simulates import
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "env", Value: "prod"},
			{ID: "lbl-2", Key: "role", Value: "web"},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels == nil {
		t.Fatal("expected labels to be populated during import, got nil")
	}
	if len(data.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected first label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
}

func TestApiToModel_NoLabelsNilModel(t *testing.T) {
	// When API returns no labels and user didn't configure any, labels stay nil.
	r := &AssetResource{}
	data := &AssetResourceModel{}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: nil,
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels != nil {
		t.Fatal("expected nil labels, got non-nil")
	}
}

func TestApiToModel_UserLabelsKept(t *testing.T) {
	// When user configured labels, only those labels should be kept from API response.
	// Matching is by ID. The API list endpoint returns labels with only the ID
	// (key and value are empty strings), so the user's planned key+value are preserved.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "", Value: ""},  // API returns only ID
			{ID: "auto-1", Key: "", Value: ""}, // server-assigned label
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (user-specified only), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[0].Key.ValueString() != "env" {
		t.Errorf("expected label key 'env' (preserved from plan), got %q", data.Labels[0].Key.ValueString())
	}
	if data.Labels[0].Value.ValueString() != "prod" {
		t.Errorf("expected label value 'prod' (preserved from plan), got %q", data.Labels[0].Value.ValueString())
	}
}

func TestApiToModel_MultipleUserLabels(t *testing.T) {
	// Multiple user labels are kept; server-assigned ones are filtered out.
	// Matching is by ID. API returns labels in different order than plan.
	// Result must preserve plan order.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-2"), Key: types.StringValue("role"), Value: types.StringValue("web")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-2", Key: "", Value: ""},  // API returns lbl-2 first
			{ID: "auto-1", Key: "", Value: ""}, // server-assigned
			{ID: "lbl-1", Key: "", Value: ""},  // API returns lbl-1 last
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(data.Labels))
	}
	// Verify plan order is preserved (lbl-1 first, lbl-2 second)
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected labels[0] ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
	if data.Labels[1].ID.ValueString() != "lbl-2" {
		t.Errorf("expected labels[1] ID 'lbl-2', got %q", data.Labels[1].ID.ValueString())
	}
}

func TestApiToModel_LabelOrderPreserved(t *testing.T) {
	// Verify that plan order is preserved when API returns 3 labels in a
	// completely different order, with a server-assigned label mixed in.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-2"), Key: types.StringValue("role"), Value: types.StringValue("web")},
			{ID: types.StringValue("lbl-3"), Key: types.StringValue("tier"), Value: types.StringValue("frontend")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-2", Key: "", Value: ""},
			{ID: "lbl-3", Key: "", Value: ""},
			{ID: "auto-1", Key: "", Value: ""},
			{ID: "lbl-1", Key: "", Value: ""},
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(data.Labels))
	}
	// Must match plan order: lbl-1, lbl-2, lbl-3
	expectedIDs := []string{"lbl-1", "lbl-2", "lbl-3"}
	for i, exp := range expectedIDs {
		if got := data.Labels[i].ID.ValueString(); got != exp {
			t.Errorf("labels[%d]: expected ID %q, got %q", i, exp, got)
		}
	}
	// Verify key+value preserved from plan
	if data.Labels[0].Key.ValueString() != "env" {
		t.Errorf("labels[0]: expected key 'env', got %q", data.Labels[0].Key.ValueString())
	}
	if data.Labels[2].Value.ValueString() != "frontend" {
		t.Errorf("labels[2]: expected value 'frontend', got %q", data.Labels[2].Value.ValueString())
	}
}

func TestApiToModel_UserLabelRemovedServerSide(t *testing.T) {
	// When a user-specified label is removed server-side, it disappears from state.
	// Matching is by ID.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
			{ID: types.StringValue("lbl-deleted"), Key: types.StringValue("old"), Value: types.StringValue("val")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: []client.AssetLabelRef{
			{ID: "lbl-1", Key: "", Value: ""},
			// lbl-deleted is no longer in API response
		},
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if len(data.Labels) != 1 {
		t.Fatalf("expected 1 label (remaining after server-side removal), got %d", len(data.Labels))
	}
	if data.Labels[0].ID.ValueString() != "lbl-1" {
		t.Errorf("expected label ID 'lbl-1', got %q", data.Labels[0].ID.ValueString())
	}
}

func TestApiToModel_UserLabelsEmptyAPIResponse(t *testing.T) {
	// When user configured labels but API returns none, set empty list.
	r := &AssetResource{}
	data := &AssetResourceModel{
		Labels: []AssetLabelModel{
			{ID: types.StringValue("lbl-1"), Key: types.StringValue("env"), Value: types.StringValue("prod")},
		},
	}
	asset := &client.Asset{
		ID:     "asset-1",
		Name:   "test",
		Status: "on",
		Labels: nil,
	}
	var diags diag.Diagnostics
	r.apiToModel(context.Background(), asset, data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected errors: %s", diags)
	}
	if data.Labels == nil {
		t.Fatal("expected non-nil labels (empty list), got nil")
	}
	if len(data.Labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(data.Labels))
	}
}
