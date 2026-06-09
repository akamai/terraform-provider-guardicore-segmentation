package dnssecurity_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDnsSecurityResource_basic(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-dns")
	name2 := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccDnsSecurityResourceConfig(name1, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "name", name1),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "enabled"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_dns_security.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing.
			{
				Config: testAccDnsSecurityResourceConfig(name2, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "name", name2),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
				),
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccDnsSecurityResource_withDomains(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityResourceConfigWithDomains(name, "CUSTOM_BLOCK", []string{"malware.example.com", "phishing.evil.org"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "domains.#", "2"),
				),
			},
		},
	})
}

func TestAccDnsSecurityResource_updateDomains(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			// Create with 2 domains.
			{
				Config: testAccDnsSecurityResourceConfigWithDomains(name, "CUSTOM_BLOCK", []string{"malware.example.com", "phishing.evil.org"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "domains.#", "2"),
				),
			},
			// Update to 3 domains.
			{
				Config: testAccDnsSecurityResourceConfigWithDomains(name, "CUSTOM_BLOCK", []string{"malware.example.com", "phishing.evil.org", "spam.bad.net"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "domains.#", "3"),
				),
			},
		},
	})
}

func TestAccDnsSecurityResource_enabled(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			// Create with enabled=true.
			{
				Config: testAccDnsSecurityResourceConfigFull(name, "CUSTOM_BLOCK", []string{"malware.example.com"}, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "enabled", "true"),
				),
			},
			// Update to enabled=false.
			{
				Config: testAccDnsSecurityResourceConfigFull(name, "CUSTOM_BLOCK", []string{"malware.example.com"}, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "enabled", "false"),
				),
			},
		},
	})
}

func TestAccDnsSecurityResource_typeImmutability(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns-immutable")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityResourceConfig(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
				),
			},
			// Changing type should require replacement.
			{
				Config: testAccDnsSecurityResourceConfig(name, "CUSTOM_EXCLUSION"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "type", "CUSTOM_EXCLUSION"),
				),
			},
		},
	})
}

func TestAccDnsSecurityResource_disappears(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityResourceConfig(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "id"),
				),
			},
			// Delete out-of-band.
			{
				Config: testAccDnsSecurityResourceConfig(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					acctest.TestAccDeleteDnsSecurityOutOfBand("guardicore_dns_security.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccDnsSecurityResource_invalidType(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDnsSecurityResourceConfig(name, "INVALID"),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func TestAccDnsSecurityResource_createAkamaiIntelligenceBlocked(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDnsSecurityResourceConfig(name, "AKAMAI_INTELLIGENCE"),
				ExpectError: regexp.MustCompile(`(?is)system-managed.*cannot be used\s+when\s+creating`),
			},
		},
	})
}

func TestAccDnsSecurityResource_createWebCategoryBlocked(t *testing.T) {
	name := acctest.TestAccRandomName("tf-acc-dns")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDnsSecurityResourceConfig(name, "WEB_CATEGORY"),
				ExpectError: regexp.MustCompile(`(?is)system-managed.*cannot be used\s+when\s+creating`),
			},
		},
	})
}

func TestAccDnsSecurityResource_eventualConsistencyReadAfterCreate(t *testing.T) {
	restoreHook := common.SetReadAfterCreateVisibilityHookForTest(func(resourceName string, attempt int) bool {
		return resourceName == "dns blocklist" && attempt <= 2
	})
	t.Cleanup(restoreHook)

	name := acctest.TestAccRandomName("tf-acc-dns-ec")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{
			{
				Config: testAccDnsSecurityResourceConfig(name, "CUSTOM_BLOCK"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_dns_security.test", "id"),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_dns_security.test", "type", "CUSTOM_BLOCK"),
				),
			},
		},
	})
}

func testAccDnsSecurityResourceConfig(name, listType string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test" {
  name    = %[1]q
  type    = %[2]q
  domains = ["test.example.com"]
}
`, name, listType)
}

func testAccDnsSecurityResourceConfigWithDomains(name, listType string, domains []string) string {
	quotedDomains := make([]string, len(domains))
	for i, d := range domains {
		quotedDomains[i] = fmt.Sprintf("%q", d)
	}
	domainsList := strings.Join(quotedDomains, ", ")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test" {
  name    = %[1]q
  type    = %[2]q
  domains = [%[3]s]
}
`, name, listType, domainsList)
}

func testAccDnsSecurityResourceConfigFull(name, listType string, domains []string, enabled bool) string {
	quotedDomains := make([]string, len(domains))
	for i, d := range domains {
		quotedDomains[i] = fmt.Sprintf("%q", d)
	}
	domainsList := strings.Join(quotedDomains, ", ")

	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test" {
  name    = %[1]q
  type    = %[2]q
  domains = [%[3]s]
  enabled = %[4]t
}
`, name, listType, domainsList, enabled)
}

func TestAccDnsSecurityResource_multipleResources(t *testing.T) {
	name1 := acctest.TestAccRandomName("tf-acc-dns-m1")
	name2 := acctest.TestAccRandomName("tf-acc-dns-m2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccDnsSecurityPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		CheckDestroy:             acctest.TestAccCheckResourceDestroyed("guardicore_dns_security"),
		Steps: []resource.TestStep{{
			Config: testAccDnsSecurityResourceConfigMultiple(name1, name2),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("guardicore_dns_security.test1", "id"),
				resource.TestCheckResourceAttrSet("guardicore_dns_security.test2", "id"),
			),
		}},
	})
}

func testAccDnsSecurityResourceConfigMultiple(name1, name2 string) string {
	return acctest.TestAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_dns_security" "test1" {
  name    = %[1]q
  type    = "CUSTOM_BLOCK"
  domains = ["multi-1.example.com"]
}

resource "guardicore_dns_security" "test2" {
  name    = %[2]q
  type    = "CUSTOM_BLOCK"
  domains = ["multi-2.example.com"]
}
`, name1, name2)
}
