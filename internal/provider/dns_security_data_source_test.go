package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDnsSecurityDataSource_byID(t *testing.T) {
	name := testAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityDataSourceConfigByID(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_dns_security.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
				),
			},
		},
	})
}

func testAccDnsSecurityDataSourceConfigByID(name, listType string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test" {
  name    = %[1]q
  type    = %[2]q
  domains = ["test.example.com"]
}

data "guardicore_dns_security" "test" {
  id = guardicore_dns_security.test.id
}
`, name, listType)
}

func TestAccDnsSecurityDataSource_byName(t *testing.T) {
	name := testAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityDataSourceConfigByName(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_dns_security.test", "name", name),
					resource.TestCheckResourceAttr("data.guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
				),
			},
		},
	})
}

func testAccDnsSecurityDataSourceConfigByName(name, listType string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test" {
  name    = %[1]q
  type    = %[2]q
  domains = ["test.example.com"]
}

data "guardicore_dns_security" "test" {
  name = guardicore_dns_security.test.name
}
`, name, listType)
}
