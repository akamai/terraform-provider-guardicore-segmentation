package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccPolicyRuleTypedConfig(body string) string {
	return testAccProviderConfig() + "\nresource \"guardicore_policy_rule\" \"test\" {\n" + body + "\n}\n"
}

func testAccPolicyRuleAllowAnyAnyBody(comments string, ports string, protocols string) string {
	return fmt.Sprintf(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = %q

  ports        = %s
  ip_protocols = %s`, comments, ports, protocols)
}

func testAccPolicyRuleIndentedBody(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func TestAccPolicyRuleResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccPolicyRuleResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					testAccCheckPolicyRevisionPublished("guardicore_policy_rule.test"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_policy_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccPolicyRuleResource_multipleCreatesSingleApply(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigMultipleCreates(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.first", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.second", "id"),
					testAccCheckPolicyRevisionPublished("guardicore_policy_rule.first"),
					testAccCheckPolicyRevisionPublished("guardicore_policy_rule.second"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigMultipleCreates() string {
	return testAccProviderConfig() + `
resource "guardicore_policy_rule" "first" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Batch create first"

  ports        = [443]
  ip_protocols = ["TCP"]
}

resource "guardicore_policy_rule" "second" {
  action           = "ALERT"
  section_position = "ALERT"
  enabled          = true
  comments         = "Batch create second"

  ports        = [80]
  ip_protocols = ["TCP"]
}
`
}

func testAccPolicyRuleResourceConfig() string {
	labelKeySource := testAccRandomName("tf-acc-LabelKey")
	labelValueSource := testAccRandomName("tf-acc-LabelValue")
	labelKeyDest := testAccRandomName("tf-acc-LabelKey")
	labelValueDest := testAccRandomName("tf-acc-LabelValue")
	groupKeySource := testAccRandomName("tf-acc-GroupKey")
	groupValueSource := testAccRandomName("tf-acc-GroupValue")
	groupKeyDest := testAccRandomName("tf-acc-GroupKey")
	groupValueDest := testAccRandomName("tf-acc-GroupValue")

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "source" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "dest" {
  key   = %[3]q
  value = %[4]q
}

resource "guardicore_label_group" "source_group" {
  key   = %[5]q
  value = %[6]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.source.id]
      }
    ]
  }
}

resource "guardicore_label_group" "dest_group" {
  key   = %[7]q
  value = %[8]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.dest.id]
      }
    ]
  }
}

resource "guardicore_policy_rule" "test" {
	%s
}
`, labelKeySource, labelValueSource, labelKeyDest, labelValueDest, groupKeySource, groupValueSource, groupKeyDest, groupValueDest,
		testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("Test rule", "[443]", `["TCP"]`)))
}

func TestAccPolicyRuleResource_blockRule(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigBlock(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigBlock() string {
	return testAccPolicyRuleTypedConfig(`  action           = "BLOCK"
  section_position = "BLOCK"
  enabled          = true
  comments         = "Block all SSH from external"

  source = {
    subnets = ["0.0.0.0/0"]
  }

  ports        = [22]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_alertRule(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigAlert(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigAlert() string {
	labelKeySource := testAccRandomName("tf-acc-LabelKey")
	labelValueSource := testAccRandomName("tf-acc-LabelValue")
	labelKeyDest := testAccRandomName("tf-acc-LabelKey")
	labelValueDest := testAccRandomName("tf-acc-LabelValue")

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "source" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "dest" {
  key   = %[3]q
  value = %[4]q
}

resource "guardicore_policy_rule" "test" {
	%s
}
`, labelKeySource, labelValueSource, labelKeyDest, labelValueDest,
		testAccPolicyRuleIndentedBody(`action           = "ALERT"
section_position = "ALERT"
enabled          = true
comments         = "Alert on external to DMZ"

ports        = [80, 443]
ip_protocols = ["TCP"]`))
}

func TestAccPolicyRuleResource_update(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	labelKeyUpdated := testAccRandomName("tf-acc-LabelKey")
	labelValueUpdated := testAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			// Initial creation
			{
				Config: testAccPolicyRuleResourceConfigUpdate1(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					testAccCheckPolicyRuleRemoteComments("guardicore_policy_rule.test", "Initial rule"),
				),
			},
			// Update comments and ports
			{
				Config: testAccPolicyRuleResourceConfigUpdate2(labelKeyUpdated, labelValueUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					testAccCheckPolicyRuleRemoteComments("guardicore_policy_rule.test", "Updated rule with HTTPS"),
				),
			},
			// Plan stability check
			{
				Config:   testAccPolicyRuleResourceConfigUpdate2(labelKeyUpdated, labelValueUpdated),
				PlanOnly: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigUpdate1(labelKeyApp, labelValueApp string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "app" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  %s
}
`, labelKeyApp, labelValueApp, testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("Initial rule", "[80]", `["TCP"]`)))
}

func testAccPolicyRuleResourceConfigUpdate2(labelKeyApp, labelValueApp string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "app" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  %s
}
`, labelKeyApp, labelValueApp, testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("Updated rule with HTTPS", "[80, 443]", `["TCP"]`)))
}

func TestAccPolicyRuleResource_withSubnets(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigSubnets(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigSubnets() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Allow from specific subnets"

  source = {
    subnets = ["10.0.0.0/24", "10.1.0.0/24"]
  }

  destination = {
    subnets = ["192.168.0.0/16"]
  }

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_withAnySource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigAnySource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigAnySource() string {
	labelKeyDb := testAccRandomName("tf-acc-LabelKey")
	labelValueDb := testAccRandomName("tf-acc-LabelValue")

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "db" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  %s
}
`, labelKeyDb, labelValueDb, testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("Any source to database", "[3306]", `["TCP"]`)))
}

func TestAccPolicyRuleResource_withAnyExternal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigAnyExternal(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "source.address_classification", "Internet"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigAnyExternal() string {
	labelKeyWeb := testAccRandomName("tf-acc-LabelKey")
	labelValueWeb := testAccRandomName("tf-acc-LabelValue")

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "web" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Allow external to web frontend"

	  source = {
	    address_classification = "Internet"
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`, labelKeyWeb, labelValueWeb)
}

func TestAccPolicyRuleResource_withAnyPort(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigAnyPort(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigAnyPort() string {
	labelKeyTrusted := testAccRandomName("tf-acc-LabelKey")
	labelValueTrusted := testAccRandomName("tf-acc-LabelValue")

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "trusted" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  %s
}
`, labelKeyTrusted, labelValueTrusted, testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("Allow all ports for trusted systems", "[1024, 1025]", `["TCP", "UDP"]`)))
}

func TestAccPolicyRuleResource_withPriority(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyRuleResourceConfigPriority(),
				ExpectError: regexp.MustCompile(`(?i)priority|unknown field`),
			},
			{
				Config: testAccPolicyRuleResourceConfigPriorityFallback(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigPriority() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "High priority rule"
  priority         = 10

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func testAccPolicyRuleResourceConfigPriorityFallback() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Priority fallback rule"

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_disabled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigDisabled(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
			// Import
			{
				ResourceName:      "guardicore_policy_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPolicyRuleResource_icmpEmptyCodesImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigICMPEmptyCodes(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "icmp_matches.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "icmp_matches.0.icmp_codes.#", "0"),
				),
			},
			{
				ResourceName:      "guardicore_policy_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigICMPEmptyCodes() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALERT"
  section_position = "ALERT"
  enabled          = true
  comments         = "ICMP with empty codes"

  ip_protocols = ["ICMP"]

  icmp_matches = [
    {
      icmp_type  = 8
      icmp_codes = []
      version    = "4"
    },
  ]`)
}

func testAccPolicyRuleResourceConfigDisabled() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = false
  comments         = "Disabled rule"

  ports        = [80]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_multipleProtocols(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigMultiProto(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
			// Plan stability check for list ordering
			{
				Config:   testAccPolicyRuleResourceConfigMultiProto(labelKey, labelValue),
				PlanOnly: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigMultiProto(labelKeyDns, labelValueDns string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "dns" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_policy_rule" "test" {
	  %s
}
`, labelKeyDns, labelValueDns, testAccPolicyRuleIndentedBody(testAccPolicyRuleAllowAnyAnyBody("DNS over TCP and UDP", "[53]", `["TCP", "UDP"]`)))
}

func TestAccPolicyRuleResource_withLabelGroups(t *testing.T) {
	key := testAccRandomName("tf-acc-LG")
	value := testAccRandomName("tf-acc-Web")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigLabelGroups(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigLabelGroups(key, value string) string {
	labelKeyApp := testAccRandomName("tf-acc-LabelKey")
	labelKeyEnv := testAccRandomName("tf-acc-LabelKey")
	labelValueApp := testAccRandomName("tf-acc-LabelValue")
	labelValueEnv := testAccRandomName("tf-acc-LabelValue")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "web" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "prod" {
  key   = %[3]q
  value = %[4]q
}

resource "guardicore_label_group" "prod_web" {
  key   = %[5]q
  value = %[6]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.web.id, guardicore_label.prod.id]
      }
    ]
  }
}

resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Allow to prod web servers"

	  destination = {
	    label_group_ids = [guardicore_label_group.prod_web.id]
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`, labelKeyApp, labelValueApp, labelKeyEnv, labelValueEnv, key, value)
}

func TestAccPolicyRuleResource_complexRule(t *testing.T) {
	key := testAccRandomName("tf-acc-Complex")
	value := testAccRandomName("tf-acc-Rule")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigComplex(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigComplex(key, value string) string {
	labelKeyBackend := testAccRandomName("tf-acc-LabelKey")
	labelValueBackend := testAccRandomName("tf-acc-LabelValue")
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "backend" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "backends" {
  key   = %[3]q
  value = %[4]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.backend.id]
      }
    ]
  }
}

resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Complex rule with multiple attributes"

	  source = {
	    subnets = ["10.0.0.0/8"]
	  }

	  destination = {
	    subnets = ["192.168.1.0/24"]
	  }

	  ports        = [8080, 8443, 9000]
	  ip_protocols = ["TCP"]
}
`, labelKeyBackend, labelValueBackend, key, value)
}

func TestAccPolicyRuleResource_disappears(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigSimpleWithLabelGroup(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
			// Delete out-of-band
			{
				Config: testAccPolicyRuleResourceConfigSimpleWithLabelGroup(labelKey, labelValue, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeletePolicyRuleOutOfBand("guardicore_policy_rule.test"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigSimpleWithLabelGroup(labelKey, labelValue, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "simple" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label_group" "simple" {
  key   = %[3]q
  value = %[4]q
  include = {
    or_groups = [{
      label_ids = [guardicore_label.simple.id]
    }]
  }
}

resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true

	  source = {
	    label_group_ids = [guardicore_label_group.simple.id]
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`, labelKey, labelValue, groupKey, groupValue)
}

//nolint:unused // test helper kept for future use
func testAccPolicyRuleResourceConfigSimple() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_deleteOrder(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-LabelKey")
	labelValue := testAccRandomName("tf-acc-LabelValue")
	groupKey := testAccRandomName("tf-acc-GroupKey")
	groupValue := testAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrder(labelKey, labelValue, groupKey, groupValue, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelExpectFailure("guardicore_label.base"),
				),
			},
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrder(labelKey, labelValue, groupKey, groupValue, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelOutOfBand("guardicore_label.base"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrderLabelOnly(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelOutOfBand("guardicore_label.base"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigDeleteOrder(labelKey, labelValue, groupKey, groupValue string, includeRule bool) string {
	ruleBlock := ""
	if includeRule {
		ruleBlock = fmt.Sprintf(`
resource "guardicore_label_group" "test" {
  key   = %q
  value = %q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.base.id]
      }
    ]
  }
}

resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Delete order test"

	  source = {
	    label_group_ids = [guardicore_label_group.test.id]
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]

}

`, groupKey, groupValue)
	}

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

%s
`, labelKey, labelValue, ruleBlock)
}

func testAccPolicyRuleResourceConfigDeleteOrderLabelOnly(labelKey, labelValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}
`, labelKey, labelValue)
}

func TestAccPolicyRuleResource_invalidJSON(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyRuleResourceConfigInvalidJSON(),
				ExpectError: regexp.MustCompile(".*invalid.*"),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigInvalidJSON() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  raw_spec_json    = "{invalid json"

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_emptySpec(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyRuleResourceConfigEmpty(),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigEmpty() string {
	return testAccPolicyRuleTypedConfig(`  raw_spec_json = jsonencode({
    attributes = {
      enforcement_mode = "strict"
    }
  })`)
}

// End-to-End smoke test: label → label_group → policy_rule chain.
func TestAccEndToEnd_labelToGroupToRule(t *testing.T) {
	labelKey1 := testAccRandomName("tf-acc-E2E-Label1")
	labelValue1 := testAccRandomName("tf-acc-Web")
	labelKey2 := testAccRandomName("tf-acc-E2E-Label2")
	labelValue2 := testAccRandomName("tf-acc-DB")
	groupKey := testAccRandomName("tf-acc-E2E-Group")
	groupValue := testAccRandomName("tf-acc-WebToDB")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckResourceDestroyed("guardicore_label"),
			testAccCheckResourceDestroyed("guardicore_label_group"),
			testAccCheckResourceDestroyed("guardicore_policy_rule"),
		),
		Steps: []resource.TestStep{
			// Create full chain
			{
				Config: testAccEndToEndConfig1(labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Check labels
					resource.TestCheckResourceAttrSet("guardicore_label.web", "id"),
					resource.TestCheckResourceAttr("guardicore_label.web", "key", labelKey1),
					resource.TestCheckResourceAttrSet("guardicore_label.db", "id"),
					resource.TestCheckResourceAttr("guardicore_label.db", "key", labelKey2),
					// Check label group
					resource.TestCheckResourceAttrSet("guardicore_label_group.web_to_db", "id"),
					resource.TestCheckResourceAttr("guardicore_label_group.web_to_db", "key", groupKey),
					resource.TestCheckResourceAttrSet("guardicore_label_group.web_to_db", "include_json"),
					// Check policy rule
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.allow_web_to_db", "id"),
				),
			},
			// Update the policy rule
			{
				Config: testAccEndToEndConfig2(labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.allow_web_to_db", "id"),
				),
			},
			// Plan stability check
			{
				Config:   testAccEndToEndConfig2(labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue),
				PlanOnly: true,
			},
		},
	})
}

func testAccEndToEndConfig1(labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
# Create two labels
resource "guardicore_label" "web" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
    }
  ]
}

resource "guardicore_label" "db" {
  key   = %[3]q
  value = %[4]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "db"
    }
  ]
}

# Create a label group referencing both labels
resource "guardicore_label_group" "web_to_db" {
  key   = %[5]q
  value = %[6]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.web.id]
      },
      {
        label_ids = [guardicore_label.db.id]
      }
    ]
  }
}

# Create a policy rule using the label group
resource "guardicore_policy_rule" "allow_web_to_db" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "E2E test: Allow web to database"

	  destination = {
	    label_group_ids = [guardicore_label_group.web_to_db.id]
	  }

	  ports        = [3306]
	  ip_protocols = ["TCP"]
}
`, labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue)
}

func testAccEndToEndConfig2(labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
# Create two labels
resource "guardicore_label" "web" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
    }
  ]
}

resource "guardicore_label" "db" {
  key   = %[3]q
  value = %[4]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "db"
    }
  ]
}

# Create a label group referencing both labels
resource "guardicore_label_group" "web_to_db" {
  key   = %[5]q
  value = %[6]q

  include = {
    or_groups = [
      {
        label_ids = [guardicore_label.web.id]
      },
      {
        label_ids = [guardicore_label.db.id]
      }
    ]
  }
}

# Update the policy rule - add HTTPS port
resource "guardicore_policy_rule" "allow_web_to_db" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "E2E test: Updated - Allow web to database on multiple ports"

	  destination = {
	    label_group_ids = [guardicore_label_group.web_to_db.id]
	  }

	  ports        = [3306, 5432]
	  ip_protocols = ["TCP"]
}
`, labelKey1, labelValue1, labelKey2, labelValue2, groupKey, groupValue)
}

// Unit tests for extractWorksiteIDFromRule

func TestExtractWorksiteIDFromRule_WithWorksite(t *testing.T) {
	rule := map[string]any{
		"action": "ALLOW",
		"worksite": map[string]any{
			"id":   "ws-123",
			"name": "Headquarters",
		},
	}

	result := extractWorksiteIDFromRule(rule)
	if result.IsNull() {
		t.Fatal("expected non-null worksite ID")
	}
	if result.ValueString() != "ws-123" {
		t.Errorf("expected 'ws-123', got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_NoWorksite(t *testing.T) {
	rule := map[string]any{
		"action": "ALLOW",
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null, got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_EmptyWorksiteID(t *testing.T) {
	rule := map[string]any{
		"worksite": map[string]any{
			"id":   "",
			"name": "",
		},
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null for empty worksite ID, got '%s'", result.ValueString())
	}
}

func TestExtractWorksiteIDFromRule_WorksiteNotMap(t *testing.T) {
	rule := map[string]any{
		"worksite": "not-a-map",
	}

	result := extractWorksiteIDFromRule(rule)
	if !result.IsNull() {
		t.Errorf("expected null for non-map worksite, got '%s'", result.ValueString())
	}
}

func TestPolicyRuleCommentFromAPI(t *testing.T) {
	tests := []struct {
		name string
		rule map[string]interface{}
		want string
		ok   bool
	}{
		{
			name: "top-level comments",
			rule: map[string]interface{}{"comments": "top-level"},
			want: "top-level",
			ok:   true,
		},
		{
			name: "attributes comments",
			rule: map[string]interface{}{"attributes": map[string]interface{}{"comments": "nested"}},
			want: "nested",
			ok:   true,
		},
		{
			name: "missing comments",
			rule: map[string]interface{}{},
			want: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := policyRuleCommentFromAPI(tt.rule)
			if ok != tt.ok {
				t.Fatalf("expected ok %v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected comments %q, got %q", tt.want, got)
			}
		})
	}
}

// Worksite assignment acceptance tests

func TestAccPolicyRuleResource_withWorksite(t *testing.T) {
	worksiteName := testAccRandomName("tf-acc-ws-pr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			// Create policy rule with worksite
			{
				Config: testAccPolicyRuleResourceConfigWithWorksite(worksiteName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "worksite_id"),
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.test", "worksite_id", "guardicore_worksite.test", "id"),
				),
			},
		},
	})
}

func TestAccPolicyRuleResource_worksiteUpdate(t *testing.T) {
	worksiteName1 := testAccRandomName("tf-acc-ws-pru1")
	worksiteName2 := testAccRandomName("tf-acc-ws-pru2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			// Create policy rule without worksite
			{
				Config: testAccPolicyRuleResourceConfigSimpleNoWorksite(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
				),
			},
			// Add worksite
			{
				Config: testAccPolicyRuleResourceConfigWithWorksite(worksiteName1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.test", "worksite_id", "guardicore_worksite.test", "id"),
				),
			},
			// Change worksite
			{
				Config: testAccPolicyRuleResourceConfigWithWorksite2(worksiteName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.test", "worksite_id", "guardicore_worksite.test2", "id"),
				),
			},
		},
	})
}

func TestAccPolicyRuleResource_invalidWorksiteRef(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "guardicore_policy_rule" "test" {
  worksite_id = "00000000-0000-0000-0000-000000000000"

	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`,
				ExpectError: regexp.MustCompile(`(?i)does not exist|Invalid Worksite Reference`),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigWithWorksite(worksiteName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

resource "guardicore_policy_rule" "test" {
  worksite_id = guardicore_worksite.test.id

	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Policy rule with worksite"

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`, worksiteName)
}

func testAccPolicyRuleResourceConfigWithWorksite2(worksiteName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test2" {
  name = %[1]q
}

resource "guardicore_policy_rule" "test" {
  worksite_id = guardicore_worksite.test2.id

	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Policy rule with worksite"

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`, worksiteName)
}

func testAccPolicyRuleResourceConfigSimpleNoWorksite() string {
	return testAccPolicyRuleTypedConfig(`  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Policy rule without worksite"

  ports        = [443]
  ip_protocols = ["TCP"]`)
}

func TestAccPolicyRuleResource_typedEndpointFieldsAndScope(t *testing.T) {
	labelKey := testAccRandomName("tf-acc-scope-key")
	labelValue := testAccRandomName("tf-acc-scope-value")
	sourceLabelKeyA := testAccRandomName("tf-acc-src-label-key")
	sourceLabelValueA := testAccRandomName("tf-acc-src-label-value")
	sourceLabelKeyB := testAccRandomName("tf-acc-src-label-key")
	sourceLabelValueB := testAccRandomName("tf-acc-src-label-value")
	destLabelKeyA := testAccRandomName("tf-acc-dst-label-key")
	destLabelValueA := testAccRandomName("tf-acc-dst-label-value")
	destLabelKeyB := testAccRandomName("tf-acc-dst-label-key")
	destLabelValueB := testAccRandomName("tf-acc-dst-label-value")

	config := testAccPolicyRuleResourceConfigTypedEndpointFieldsAndScope(
		labelKey,
		labelValue,
		sourceLabelKeyA,
		sourceLabelValueA,
		sourceLabelKeyB,
		sourceLabelValueB,
		destLabelKeyA,
		destLabelValueA,
		destLabelKeyB,
		destLabelValueB,
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.selector", "id"),
					testAccCheckPolicyRevisionPublished("guardicore_policy_rule.test"),
					testAccCheckPolicyRevisionPublished("guardicore_policy_rule.selector"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "source.processes.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "source.windows_services.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "source.windows_services.0.service_name", "Dnscache"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.test", "scope.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.selector", "source.labels.or_labels.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.selector", "source.labels.or_labels.0.and_labels.#", "2"),
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.selector", "source.labels.or_labels.0.and_labels.0", "guardicore_label.source_selector_a", "id"),
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.selector", "source.labels.or_labels.0.and_labels.1", "guardicore_label.source_selector_b", "id"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.selector", "destination.labels.or_labels.#", "1"),
					resource.TestCheckResourceAttr("guardicore_policy_rule.selector", "destination.labels.or_labels.0.and_labels.#", "2"),
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.selector", "destination.labels.or_labels.0.and_labels.0", "guardicore_label.destination_selector_a", "id"),
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.selector", "destination.labels.or_labels.0.and_labels.1", "guardicore_label.destination_selector_b", "id"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigTypedEndpointFieldsAndScope(
	labelKey,
	labelValue,
	sourceLabelKeyA,
	sourceLabelValueA,
	sourceLabelKeyB,
	sourceLabelValueB,
	destLabelKeyA,
	destLabelValueA,
	destLabelKeyB,
	destLabelValueB string,
) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "scope" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "source_selector_a" {
  key   = %[3]q
  value = %[4]q
}

resource "guardicore_label" "source_selector_b" {
  key   = %[5]q
  value = %[6]q
}

resource "guardicore_label" "destination_selector_a" {
  key   = %[7]q
  value = %[8]q
}

resource "guardicore_label" "destination_selector_b" {
  key   = %[9]q
  value = %[10]q
}

resource "guardicore_policy_rule" "test" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Typed endpoint fields and scope"
  scope            = [guardicore_label.scope.id]

	  source = {
	    processes              = ["dns.exe"]
	    windows_services = [
	      {
	        service_name = "Dnscache"
	        display_name = "DNS Client"
	        allowed_image_names = ["svchost.exe"]
	      },
	    ]
	  }

  destination = {
    domains = ["dns.example.internal"]
  }

  ports        = [53]
  ip_protocols = ["UDP"]
}

resource "guardicore_policy_rule" "selector" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Typed source/destination labels selectors"

  source = {
    labels = {
      or_labels = [
        {
          and_labels = [
            guardicore_label.source_selector_a.id,
            guardicore_label.source_selector_b.id,
          ]
        },
      ]
    }
  }

  destination = {
    labels = {
      or_labels = [
        {
          and_labels = [
            guardicore_label.destination_selector_a.id,
            guardicore_label.destination_selector_b.id,
          ]
        },
      ]
    }
  }

  ports        = [443]
  ip_protocols = ["TCP"]
}
`, labelKey, labelValue, sourceLabelKeyA, sourceLabelValueA, sourceLabelKeyB, sourceLabelValueB, destLabelKeyA, destLabelValueA, destLabelKeyB, destLabelValueB)
}

func TestUpdatePolicyRuleModelFromAPI_preservesEmptyLists(t *testing.T) {
	ctx := context.Background()
	emptyIntList, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{})
	emptyStringList, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	emptyRangeList, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}, []attr.Value{})

	tests := []struct {
		name  string
		field string
		model func() *PolicyRuleResourceModel
	}{
		{
			name:  "ports empty list preserved",
			field: "Ports",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{Ports: emptyIntList}
			},
		},
		{
			name:  "exclude_ports empty list preserved",
			field: "ExcludePorts",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{ExcludePorts: emptyIntList}
			},
		},
		{
			name:  "ip_protocols empty list preserved",
			field: "IPProtocols",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{IPProtocols: emptyStringList}
			},
		},
		{
			name:  "scope empty list preserved",
			field: "Scope",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{Scope: emptyStringList}
			},
		},
		{
			name:  "port_ranges empty list preserved",
			field: "PortRanges",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{PortRanges: emptyRangeList}
			},
		},
		{
			name:  "exclude_port_ranges empty list preserved",
			field: "ExcludePortRanges",
			model: func() *PolicyRuleResourceModel {
				return &PolicyRuleResourceModel{ExcludePortRanges: emptyRangeList}
			},
		},
	}

	apiRule := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.model()
			diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, apiRule)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			var got types.List
			switch tt.field {
			case "Ports":
				got = data.Ports
			case "ExcludePorts":
				got = data.ExcludePorts
			case "IPProtocols":
				got = data.IPProtocols
			case "Scope":
				got = data.Scope
			case "PortRanges":
				got = data.PortRanges
			case "ExcludePortRanges":
				got = data.ExcludePortRanges
			}

			if got.IsNull() {
				t.Fatalf("expected empty list, got null")
			}
			if len(got.Elements()) != 0 {
				t.Fatalf("expected 0 elements, got %d", len(got.Elements()))
			}
		})
	}
}

func TestUpdatePolicyRuleModelFromAPI_preservesEmptyPortsWhenEffectiveSpecOmitsPorts(t *testing.T) {
	ctx := context.Background()
	emptyPorts, _ := types.ListValueFrom(ctx, types.Int64Type, []int64{})

	data := &PolicyRuleResourceModel{Ports: emptyPorts}
	apiRule := map[string]interface{}{
		"action":           "BLOCK",
		"section_position": "BLOCK",
		"enabled":          true,
	}
	effectiveSpec := map[string]interface{}{
		"action":           "BLOCK",
		"section_position": "BLOCK",
		"enabled":          true,
		"source":           map[string]interface{}{"processes": []interface{}{"/usr/bin/curl"}},
		"destination":      map[string]interface{}{"address_classification": "Internet"},
	}

	diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, effectiveSpec)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Ports.IsNull() {
		t.Fatal("expected empty ports list, got null")
	}
	if len(data.Ports.Elements()) != 0 {
		t.Fatalf("expected 0 ports, got %d", len(data.Ports.Elements()))
	}
}

func TestUpdatePolicyRuleModelFromAPI_nullListsStayNull(t *testing.T) {
	ctx := context.Background()

	data := &PolicyRuleResourceModel{
		Ports:             types.ListNull(types.Int64Type),
		ExcludePorts:      types.ListNull(types.Int64Type),
		IPProtocols:       types.ListNull(types.StringType),
		Scope:             types.ListNull(types.StringType),
		PortRanges:        types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}),
		ExcludePortRanges: types.ListNull(types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()}),
	}

	apiRule := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
	}

	diags := updatePolicyRuleModelFromAPI(ctx, data, apiRule, apiRule)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	for _, tc := range []struct {
		name string
		got  types.List
	}{
		{"Ports", data.Ports},
		{"ExcludePorts", data.ExcludePorts},
		{"IPProtocols", data.IPProtocols},
		{"Scope", data.Scope},
		{"PortRanges", data.PortRanges},
		{"ExcludePortRanges", data.ExcludePortRanges},
	} {
		if !tc.got.IsNull() {
			t.Errorf("%s: expected null, got non-null list with %d elements", tc.name, len(tc.got.Elements()))
		}
	}
}
