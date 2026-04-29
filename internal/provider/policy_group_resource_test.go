package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyGroupResource_labelType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-label")
	labelKey1 := testAccRandomName("tf-acc-label")
	labelValue1 := testAccRandomName("tf-acc-value")
	labelKey2 := testAccRandomName("tf-acc-label")
	labelValue2 := testAccRandomName("tf-acc-value")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPolicyGroupResourceConfigLabelType(name, labelKey1, labelValue1, labelKey2, labelValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "LABEL"),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "comments", "Test policy group for labels"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "members_json"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "guardicore_policy_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccPolicyGroupResourceConfigLabelTypeUpdated(name, labelKey1, labelValue1, labelKey2, labelValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "comments", "Updated comment"),
				),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigLabelType(name, labelKey1, labelValue1, labelKey2, labelValue2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "label1" {
  key   = %[2]q
  value = %[3]q
}

resource "guardicore_label" "label2" {
  key   = %[4]q
  value = %[5]q
}

resource "guardicore_policy_group" "test" {
  name     = %[1]q
  type     = "LABEL"
  comments = "Test policy group for labels"

  members_json = jsonencode([
    [guardicore_label.label1.id],
    [guardicore_label.label2.id]
  ])
}
`, name, labelKey1, labelValue1, labelKey2, labelValue2)
}

func testAccPolicyGroupResourceConfigLabelTypeUpdated(name, labelKey1, labelValue1, labelKey2, labelValue2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "label1" {
  key   = %[2]q
  value = %[3]q
}

resource "guardicore_label" "label2" {
  key   = %[4]q
  value = %[5]q
}

resource "guardicore_policy_group" "test" {
  name     = %[1]q
  type     = "LABEL"
  comments = "Updated comment"

  members_json = jsonencode([
    [guardicore_label.label1.id, guardicore_label.label2.id]
  ])
}
`, name, labelKey1, labelValue1, labelKey2, labelValue2)
}

func TestAccPolicyGroupResource_labelTypeWithExclude(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-exclude")
	labelKey1 := testAccRandomName("tf-acc-label")
	labelValue1 := testAccRandomName("tf-acc-value")
	labelKey2 := testAccRandomName("tf-acc-label")
	labelValue2 := testAccRandomName("tf-acc-value")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupResourceConfigLabelTypeWithExclude(name, labelKey1, labelValue1, labelKey2, labelValue2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "LABEL"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "members_json"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "exclude_members_json"),
				),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigLabelTypeWithExclude(name, labelKey1, labelValue1, labelKey2, labelValue2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "include" {
  key   = %[2]q
  value = %[3]q
}

resource "guardicore_label" "exclude" {
  key   = %[4]q
  value = %[5]q
}

resource "guardicore_policy_group" "test" {
  name     = %[1]q
  type     = "LABEL"

  members_json = jsonencode([
    [guardicore_label.include.id]
  ])

  exclude_members_json = jsonencode([
    [guardicore_label.exclude.id]
  ])
}
`, name, labelKey1, labelValue1, labelKey2, labelValue2)
}

func TestAccPolicyGroupResource_fqdnType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-fqdn")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupResourceConfigFQDNType(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "FQDN"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "members_json"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "guardicore_policy_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPolicyGroupResourceConfigFQDNType(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "FQDN"

  members_json = jsonencode([
    "example.com",
    "*.test.com",
    "api.example.org"
  ])
}
`, name)
}

func TestAccPolicyGroupResource_ipAddressType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-ip")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupResourceConfigIPAddressType(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "name", name),
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "IP_ADDRESS"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "members_json"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "guardicore_policy_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPolicyGroupResourceConfigIPAddressType(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "IP_ADDRESS"

  members_json = jsonencode([
    { subnet = "10.0.0.0/8" },
    { subnet = "172.16.0.0/12" },
    { range = { start = "192.168.1.1", end = "192.168.1.254" } }
  ])
}
`, name)
}

func TestAccPolicyGroupResource_typeImmutability(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-immutable")
	labelKey := testAccRandomName("tf-acc-label")
	labelValue := testAccRandomName("tf-acc-value")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupResourceConfigLabelType(name, labelKey, labelValue, labelKey, labelValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "LABEL"),
				),
			},
			// Changing type should require replacement
			{
				Config: testAccPolicyGroupResourceConfigFQDNType(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_policy_group.test", "type", "FQDN"),
				),
			},
		},
	})
}

func TestAccPolicyGroupResource_disappears(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-disappears")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_policy_group"),
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyGroupResourceConfigFQDNType(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_policy_group.test", "id"),
				),
			},
		},
	})
}

func TestAccPolicyGroupResource_invalidType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-invalid")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigInvalidType(name),
				ExpectError: regexp.MustCompile(`Attribute type value must be one of`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigInvalidType(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "INVALID_TYPE"

  members_json = jsonencode(["test"])
}
`, name)
}

func TestAccPolicyGroupResource_emptyMembers(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-empty")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigEmptyMembers(name),
				ExpectError: regexp.MustCompile(`Empty|must contain at least one`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigEmptyMembers(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "FQDN"

  members_json = jsonencode([])
}
`, name)
}

func TestAccPolicyGroupResource_excludeMembersOnNonLabelType(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-exclude-invalid")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigExcludeOnFQDN(name),
				ExpectError: regexp.MustCompile(`exclude_members_json can only be used with LABEL type`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigExcludeOnFQDN(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "FQDN"

  members_json = jsonencode(["example.com"])

  exclude_members_json = jsonencode(["test.com"])
}
`, name)
}

func TestAccPolicyGroupResource_invalidLabelReference(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-invalid-ref")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigInvalidLabelRef(name),
				ExpectError: regexp.MustCompile(`does not exist in Akamai Guardicore Segmentation|Invalid Label Reference`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigInvalidLabelRef(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "LABEL"

  members_json = jsonencode([
    ["00000000-0000-0000-0000-000000000000"]
  ])
}
`, name)
}

func TestAccPolicyGroupResource_nameValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigInvalidName(),
				ExpectError: regexp.MustCompile(`string length must be between 1 and 100`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigInvalidName() string {
	// Create a name longer than 100 characters
	longName := "a"
	for i := 0; i < 101; i++ {
		longName += "a"
	}

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name = %[1]q
  type = "FQDN"

  members_json = jsonencode(["example.com"])
}
`, longName)
}

func TestAccPolicyGroupResource_commentsValidation(t *testing.T) {
	name := testAccRandomName("tf-acc-pg-comments")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPolicyGroupPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPolicyGroupResourceConfigInvalidComments(name),
				ExpectError: regexp.MustCompile(`string length must be at most 200`),
			},
		},
	})
}

func testAccPolicyGroupResourceConfigInvalidComments(name string) string {
	// Create a comment longer than 200 characters
	longComment := ""
	for i := 0; i < 201; i++ {
		longComment += "a"
	}

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_policy_group" "test" {
  name     = %[1]q
  type     = "FQDN"
  comments = %[2]q

  members_json = jsonencode(["example.com"])
}
`, name, longComment)
}
