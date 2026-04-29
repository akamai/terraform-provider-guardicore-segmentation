package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyRuleDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyRuleDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_policy_rule.test", "id"),
					resource.TestCheckResourceAttrSet("data.guardicore_policy_rule.test", "spec_json"),
					// Use semantic JSON comparison helper
					testAccCheckJSONAttr("data.guardicore_policy_rule.test", "spec_json", `{
						"action": "ALLOW",
						"section_position": "ALLOW",
						"enabled": true,
						"comments": "Data source test rule",
						"source": {},
						"destination": {},
						"ports": [443, 8443],
						"ip_protocols": ["TCP"]
					}`),
				),
			},
		},
	})
}

func testAccPolicyRuleDataSourceConfig() string {
	return testAccProviderConfig() + `
resource "guardicore_policy_rule" "test" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Data source test rule"

	  ports        = [443, 8443]
	  ip_protocols = ["TCP"]
}

data "guardicore_policy_rule" "test" {
  id = guardicore_policy_rule.test.id
}
`
}
