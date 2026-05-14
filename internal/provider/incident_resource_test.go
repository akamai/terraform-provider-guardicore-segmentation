package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIncidentResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_incident"),
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccIncidentResourceConfigBasic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_incident.test", "id"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "type", "CustomIncident"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "severity", "LOW"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "description", "Terraform acceptance test incident"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "tags.0", "tf-acc-test"),
				),
			},
		},
	})
}

func TestAccIncidentResource_full(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_incident"),
		Steps: []resource.TestStep{
			{
				Config: testAccIncidentResourceConfigFull(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_incident.test", "id"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "type", "CustomIncident"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "severity", "HIGH"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "description", "Full acceptance test incident"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "origin", "Terraform Test"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "mitigation", "### Mitigation\n\nBlock the IP."),
					resource.TestCheckResourceAttr("guardicore_incident.test", "tags.#", "2"),
				),
			},
		},
	})
}

func TestAccIncidentResource_requiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_incident"),
		Steps: []resource.TestStep{
			// Create initial incident.
			{
				Config: testAccIncidentResourceConfigWithSeverity("LOW"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_incident.test", "id"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "severity", "LOW"),
				),
			},
			// Update severity — should force replacement (destroy + create).
			{
				Config: testAccIncidentResourceConfigWithSeverity("HIGH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_incident.test", "id"),
					resource.TestCheckResourceAttr("guardicore_incident.test", "severity", "HIGH"),
				),
			},
		},
	})
}

func TestAccIncidentResource_invalidSeverity(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccIncidentResourceConfigWithSeverity("CRITICAL"),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func testAccIncidentResourceConfigBasic() string {
	return testAccProviderConfig() + `
resource "guardicore_incident" "test" {
  type        = "CustomIncident"
  severity    = "LOW"
  time        = 1621957270000
  description = "Terraform acceptance test incident"
  summary     = "### Test Incident\n\nThis is a test incident created by Terraform acceptance tests."
  tags        = ["tf-acc-test"]

  affected_assets_json = jsonencode([
    {
      type  = "IP"
      value = "10.0.0.1"
    }
  ])
}
`
}

func testAccIncidentResourceConfigFull() string {
	return testAccProviderConfig() + `
resource "guardicore_incident" "test" {
  type        = "CustomIncident"
  severity    = "HIGH"
  time        = 1621957270000
  description = "Full acceptance test incident"
  summary     = "### Full Test Incident\n\nThis tests all optional fields."
  tags        = ["tf-acc-test", "full-test"]
  origin      = "Terraform Test"
  mitigation  = "### Mitigation\n\nBlock the IP."

  affected_assets_json = jsonencode([
    {
      type  = "IP"
      value = "10.0.0.1"
    },
    {
      type  = "IP"
      value = "10.0.0.2"
    }
  ])

  cef_extensions_json = jsonencode({
    sproc = "powershell"
  })

  properties_json = jsonencode([
    {
      key   = "test_property"
      type  = "text"
      value = "test_value"
    }
  ])
}
`
}

func testAccIncidentResourceConfigWithSeverity(severity string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_incident" "test" {
  type        = "CustomIncident"
  severity    = %[1]q
  time        = 1621957270000
  description = "Terraform severity test incident"
  summary     = "### Test\n\nSeverity test."
  tags        = ["tf-acc-test"]

  affected_assets_json = jsonencode([
    {
      type  = "IP"
      value = "10.0.0.1"
    }
  ])
}
`, severity)
}

func TestAccIncidentResource_multipleResources(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_incident"),
		Steps: []resource.TestStep{{
			Config: testAccIncidentResourceConfigMultiple(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("guardicore_incident.test1", "id"),
				resource.TestCheckResourceAttrSet("guardicore_incident.test2", "id"),
			),
		}},
	})
}

func testAccIncidentResourceConfigMultiple() string {
	return testAccProviderConfig() + `
resource "guardicore_incident" "test1" {
  type        = "CustomIncident"
  severity    = "LOW"
  time        = 1621957270000
  description = "Terraform acceptance test incident 1"
  summary     = "### Test Incident 1"
  tags        = ["tf-acc-test-1"]

  affected_assets_json = jsonencode([{ type = "IP", value = "10.0.0.1" }])
}

resource "guardicore_incident" "test2" {
  type        = "CustomIncident"
  severity    = "LOW"
  time        = 1621957271000
  description = "Terraform acceptance test incident 2"
  summary     = "### Test Incident 2"
  tags        = ["tf-acc-test-2"]

  affected_assets_json = jsonencode([{ type = "IP", value = "10.0.0.2" }])
}
`
}
