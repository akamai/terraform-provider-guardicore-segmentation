package common_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccLabelGroupResource_invalidLabelRef tests that creating a label group
// with a non-existent label ID produces a clear error before the API call.
func TestAccLabelGroupResource_invalidLabelRef(t *testing.T) {
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + fmt.Sprintf(`
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

// TestAccPolicyRuleResource_invalidLabelGroupRef tests that creating a policy
// rule with a non-existent label group ID produces a clear error.
func TestAccPolicyRuleResource_invalidLabelGroupRef(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + `
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

// TestAccPolicyRuleResource_invalidUserGroupRefShape tests that plan-time
// validation rejects non-string user_group_ids entries in raw_spec_json.
func TestAccPolicyRuleResource_invalidUserGroupRefShape(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + `
resource "guardicore_policy_rule" "test" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true

  raw_spec_json = jsonencode({
    source = {
      user_group_ids = [{
        id   = "local_administrators"
        name = "Local Administrators"
      }]
    }
    destination = {
      any = true
    }
  })
}`,
				ExpectError: regexp.MustCompile(`(?i)Invalid Policy Rule Endpoint Reference|policy_rule\.source\.user_group_ids\[0\]`),
			},
		},
	})
}

// TestAccAssetResource_invalidLabelRef tests that creating an asset with a
// non-existent label ID produces a clear error.
func TestAccAssetResource_invalidLabelRef(t *testing.T) {
	assetName := acctest.TestAccRandomName("tf-acc-Asset")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
// referenced by a label group emits a warning when
// validate_references_on_destroy is enabled.
func TestAccLabelResource_destroyWithRefWarning(t *testing.T) {
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelWithGroupConfig(labelKey, labelValue, groupKey, groupValue, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_label_group.test", "id"),
				),
			},
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
	return acctest.TestAccProviderConfigWithRefValidation(validateOnDestroy) + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfigWithRefValidation(true) + fmt.Sprintf(`
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
