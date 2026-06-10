package policyrule_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccPolicyRuleTypedConfig(body string) string {
	return acctest.TestAccProviderConfig() + "\nresource \"guardicore_policy_rule\" \"test\" {\n" + body + "\n}\n"
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccPolicyRuleResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					acctest.TestAccCheckPolicyRevisionPublished("guardicore_policy_rule.test"),
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigMultipleCreates(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.first", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.second", "id"),
					acctest.TestAccCheckPolicyRevisionPublished("guardicore_policy_rule.first"),
					acctest.TestAccCheckPolicyRevisionPublished("guardicore_policy_rule.second"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigMultipleCreates() string {
	return acctest.TestAccProviderConfig() + `
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
	labelKeySource := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueSource := acctest.TestAccRandomName("tf-acc-LabelValue")
	labelKeyDest := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueDest := acctest.TestAccRandomName("tf-acc-LabelValue")
	groupKeySource := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValueSource := acctest.TestAccRandomName("tf-acc-GroupValue")
	groupKeyDest := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValueDest := acctest.TestAccRandomName("tf-acc-GroupValue")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	labelKeySource := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueSource := acctest.TestAccRandomName("tf-acc-LabelValue")
	labelKeyDest := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueDest := acctest.TestAccRandomName("tf-acc-LabelValue")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	labelKeyUpdated := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueUpdated := acctest.TestAccRandomName("tf-acc-LabelValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			// Initial creation
			{
				Config: testAccPolicyRuleResourceConfigUpdate1(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					acctest.TestAccCheckPolicyRuleRemoteComments("guardicore_policy_rule.test", "Initial rule"),
				),
			},
			// Update comments and ports
			{
				Config: testAccPolicyRuleResourceConfigUpdate2(labelKeyUpdated, labelValueUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					acctest.TestAccCheckPolicyRuleRemoteComments("guardicore_policy_rule.test", "Updated rule with HTTPS"),
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	labelKeyDb := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueDb := acctest.TestAccRandomName("tf-acc-LabelValue")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	labelKeyWeb := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueWeb := acctest.TestAccRandomName("tf-acc-LabelValue")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	labelKeyTrusted := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueTrusted := acctest.TestAccRandomName("tf-acc-LabelValue")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	key := acctest.TestAccRandomName("tf-acc-LG")
	value := acctest.TestAccRandomName("tf-acc-Web")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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

// TestAccPolicyRuleResource_withLabelGroupsIdempotent guards the fix for the
// API mirroring label_group_ids into the policy_groups response field: after
// apply, a second plan against the same config must report no changes (no
// phantom policy_group_ids removal).
func TestAccPolicyRuleResource_withLabelGroupsIdempotent(t *testing.T) {
	key := acctest.TestAccRandomName("tf-acc-LG")
	value := acctest.TestAccRandomName("tf-acc-Web")
	// The config helper generates random label names internally, so it must be
	// rendered once and reused across both steps to keep names stable.
	config := testAccPolicyRuleResourceConfigLabelGroups(key, value)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckNoResourceAttr("guardicore_policy_rule.test", "destination.policy_group_ids"),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func testAccPolicyRuleResourceConfigLabelGroups(key, value string) string {
	labelKeyApp := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelKeyEnv := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueApp := acctest.TestAccRandomName("tf-acc-LabelValue")
	labelValueEnv := acctest.TestAccRandomName("tf-acc-LabelValue")
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	key := acctest.TestAccRandomName("tf-acc-Complex")
	value := acctest.TestAccRandomName("tf-acc-Rule")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	labelKeyBackend := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValueBackend := acctest.TestAccRandomName("tf-acc-LabelValue")
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
					acctest.TestAccDeletePolicyRuleOutOfBand("guardicore_policy_rule.test"),
				),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigSimpleWithLabelGroup(labelKey, labelValue, groupKey, groupValue string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	labelKey := acctest.TestAccRandomName("tf-acc-LabelKey")
	labelValue := acctest.TestAccRandomName("tf-acc-LabelValue")
	groupKey := acctest.TestAccRandomName("tf-acc-GroupKey")
	groupValue := acctest.TestAccRandomName("tf-acc-GroupValue")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrder(labelKey, labelValue, groupKey, groupValue, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelExpectFailure("guardicore_label.base"),
				),
			},
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrder(labelKey, labelValue, groupKey, groupValue, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelOutOfBand("guardicore_label.base"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccPolicyRuleResourceConfigDeleteOrderLabelOnly(labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteLabelOutOfBand("guardicore_label.base"),
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

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}

%s
`, labelKey, labelValue, ruleBlock)
}

func testAccPolicyRuleResourceConfigDeleteOrderLabelOnly(labelKey, labelValue string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "base" {
  key   = %[1]q
  value = %[2]q
}
`, labelKey, labelValue)
}

func TestAccPolicyRuleResource_invalidJSON(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
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
	labelKey1 := acctest.TestAccRandomName("tf-acc-E2E-Label1")
	labelValue1 := acctest.TestAccRandomName("tf-acc-Web")
	labelKey2 := acctest.TestAccRandomName("tf-acc-E2E-Label2")
	labelValue2 := acctest.TestAccRandomName("tf-acc-DB")
	groupKey := acctest.TestAccRandomName("tf-acc-E2E-Group")
	groupValue := acctest.TestAccRandomName("tf-acc-WebToDB")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			acctest.TestAccCheckResourceDestroyed("guardicore_label"),
			acctest.TestAccCheckResourceDestroyed("guardicore_label_group"),
			acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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

// Worksite assignment acceptance tests

func TestAccPolicyRuleResource_withWorksite(t *testing.T) {
	worksiteName := acctest.TestAccRandomName("tf-acc-ws-pr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
	worksiteName1 := acctest.TestAccRandomName("tf-acc-ws-pru1")
	worksiteName2 := acctest.TestAccRandomName("tf-acc-ws-pru2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
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
				Config: testAccPolicyRuleResourceConfigWithWorksite2(worksiteName1, worksiteName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("guardicore_policy_rule.test", "worksite_id", "guardicore_worksite.test2", "id"),
				),
			},
		},
	})
}

func TestAccPolicyRuleResource_invalidWorksiteRef(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.TestAccProviderConfig() + `
resource "guardicore_policy_rule" "test" {
  worksite_id = "00000000-0000-0000-0000-000000000000"

	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
`,
				ExpectError: regexp.MustCompile(`(?is)(does not exist|invalid worksite reference|unable to assign policy rule to worksite|failed to move policy rules to worksite|status\s*\d+)`),
			},
		},
	})
}

func testAccPolicyRuleResourceConfigWithWorksite(worksiteName string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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

func testAccPolicyRuleResourceConfigWithWorksite2(worksiteName1, worksiteName2 string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

resource "guardicore_worksite" "test2" {
  name = %[2]q
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
`, worksiteName1, worksiteName2)
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
	labelKey := acctest.TestAccRandomName("tf-acc-scope-key")
	labelValue := acctest.TestAccRandomName("tf-acc-scope-value")
	sourceLabelKeyA := acctest.TestAccRandomName("tf-acc-src-label-key")
	sourceLabelValueA := acctest.TestAccRandomName("tf-acc-src-label-value")
	sourceLabelKeyB := acctest.TestAccRandomName("tf-acc-src-label-key")
	sourceLabelValueB := acctest.TestAccRandomName("tf-acc-src-label-value")
	destLabelKeyA := acctest.TestAccRandomName("tf-acc-dst-label-key")
	destLabelValueA := acctest.TestAccRandomName("tf-acc-dst-label-value")
	destLabelKeyB := acctest.TestAccRandomName("tf-acc-dst-label-key")
	destLabelValueB := acctest.TestAccRandomName("tf-acc-dst-label-value")

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
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_policy_rule"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_rule.selector", "id"),
					acctest.TestAccCheckPolicyRevisionPublished("guardicore_policy_rule.test"),
					acctest.TestAccCheckPolicyRevisionPublished("guardicore_policy_rule.selector"),
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
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
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

// Unit tests for updatePolicyRuleModelFromAPI are in resource_unit_test.go
// (package policyrule) because they exercise unexported functions.
