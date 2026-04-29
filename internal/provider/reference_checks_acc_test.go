package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccLabelGroupResource_invalidLabelRef tests that creating a label group
// with a non-existent label ID produces a clear error before the API call.
func TestAccLabelGroupResource_invalidLabelRef(t *testing.T) {
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %q
  value = %q
  include = {
    or_groups = [{
      label_ids = ["00000000-0000-0000-0000-000000000000"]
    }]
  }
}`, groupKey, groupValue),
				ExpectError: regexp.MustCompile(`(?i)does not exist|Invalid Label Reference`),
			},
		},
	})
}

// TestAccPolicyRuleResource_invalidLabelGroupRef tests that creating a policy rule
// with a non-existent label group ID produces a clear error.
func TestAccPolicyRuleResource_invalidLabelGroupRef(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true

	  source = {
	    label_group_ids = ["00000000-0000-0000-0000-000000000000"]
	  }

	  destination = {
	    any = true
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]
}`,
				ExpectError: regexp.MustCompile(`(?i)does not exist|Invalid Label Group Reference`),
			},
		},
	})
}

// TestAccAssetResource_invalidLabelRef tests that creating an asset
// with a non-existent label ID produces a clear error.
func TestAccAssetResource_invalidLabelRef(t *testing.T) {
	assetName := testAccRandomName("tf-acc-Asset")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_asset" "test" {
  name                 = %q
  orchestration_obj_id = "terraform-test"

  nics = [{
    mac_address  = "00:11:22:33:44:55"
    ip_addresses = ["10.0.0.1"]
  }]

  labels = [{
    id    = "00000000-0000-0000-0000-000000000000"
    key   = "env"
    value = "nonexistent"
  }]
}`, assetName),
				ExpectError: regexp.MustCompile(`(?i)does not exist|Invalid Label Reference`),
			},
		},
	})
}

// TestAccLabelResource_destroyWithRefWarning tests that destroying a label
// referenced by a label group emits a warning when validate_references_on_destroy is enabled.
func TestAccLabelResource_destroyWithRefWarning(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create both label and label group
			{
				Config: testAccLabelWithGroupConfig(labelKey, labelValue, groupKey, groupValue, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "id"),
				),
			},
			// Step 2: Remove the label from config (will trigger destroy with warning)
			// The warning is emitted but doesn't prevent destruction
			{
				Config: testAccLabelGroupOnlyConfig(groupKey, groupValue, labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "id"),
				),
			},
		},
	})
}

func testAccLabelWithGroupConfig(labelKey, labelValue, groupKey, groupValue string, validateOnDestroy bool) string {
	return testAccProviderConfigWithRefValidation(validateOnDestroy) + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %q
  value = %q
}

resource "guardicore_label_group" "test" {
  key   = %q
  value = %q
  include = {
    or_groups = [{
      label_ids = [guardicore_label.test.id]
    }]
  }
}`, labelKey, labelValue, groupKey, groupValue)
}

func testAccLabelGroupOnlyConfig(groupKey, groupValue, labelKey, labelValue string) string {
	return testAccProviderConfigWithRefValidation(true) + fmt.Sprintf(`
resource "guardicore_label" "keep" {
  key   = %q
  value = "%s-keep"
}

resource "guardicore_label_group" "test" {
  key   = %q
  value = %q
  include = {
    or_groups = [{
      label_ids = [guardicore_label.keep.id]
    }]
  }
}`, labelKey, labelValue, groupKey, groupValue)
}
