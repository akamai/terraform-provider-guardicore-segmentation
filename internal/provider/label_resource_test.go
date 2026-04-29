package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLabelResource_basic(t *testing.T) {
	key := testAccRandomName("tf-acc-Env")
	value1 := testAccRandomName("tf-acc-Prod")
	value2 := testAccRandomName("tf-acc-Stage")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing.
			{
				Config: testAccLabelResourceConfig(key, value1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label.test", "id"),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value1),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "guardicore_label.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing.
			{
				Config: testAccLabelResourceConfig(key, value2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label.test", "id"),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value2),
				),
			},
			// Delete testing automatically occurs in TestCase.
		},
	})
}

func TestAccLabelResource_withCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-App")
	value := testAccRandomName("tf-acc-Web")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfigWithCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.op", "CONTAINS"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "web"),
				),
			},
		},
	})
}

func testAccLabelResourceConfig(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q
}
`, key, value)
}

func testAccLabelResourceConfigWithCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
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
`, key, value)
}

func TestAccLabelResource_multipleCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-Env")
	value := testAccRandomName("tf-acc-Prod")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			// Create with multiple criteria
			{
				Config: testAccLabelResourceConfigMultipleCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.op", "CONTAINS"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "web"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.1.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.1.op", "STARTSWITH"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.1.argument", "prod"),
				),
			},
			// Import
			{
				ResourceName:      "guardicore_label.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Plan stability check
			{
				Config:   testAccLabelResourceConfigMultipleCriteria(key, value),
				PlanOnly: true,
			},
		},
	})
}

func testAccLabelResourceConfigMultipleCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
    },
    {
      field    = "name"
      op       = "STARTSWITH"
      argument = "prod"
    }
  ]
}
`, key, value)
}

func TestAccLabelResource_updateAddCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-App")
	value := testAccRandomName("tf-acc-DB")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			// Create without criteria
			{
				Config: testAccLabelResourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "0"),
				),
			},
			// Update to add criteria
			{
				Config: testAccLabelResourceConfigUpdateAddCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.op", "STARTSWITH"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "db-"),
				),
			},
		},
	})
}

func testAccLabelResourceConfigUpdateAddCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "STARTSWITH"
      argument = "db-"
    }
  ]
}
`, key, value)
}

func testAccLabelResourceConfigUpdateClearCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "__tf_clear__"
    }
  ]
}
`, key, value)
}

func TestAccLabelResource_updateRemoveCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-Service")
	value := testAccRandomName("tf-acc-API")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			// Create with criteria
			{
				Config: testAccLabelResourceConfigUpdateAddCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
				),
			},
			// Update to remove all criteria
			{
				Config: testAccLabelResourceConfigUpdateClearCriteria(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "__tf_clear__"),
				),
			},
		},
	})
}

func TestAccLabelResource_updateModifyCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-Zone")
	value := testAccRandomName("tf-acc-DMZ")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			// Create with initial criteria
			{
				Config: testAccLabelResourceConfigModifyCriteria1(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.op", "CONTAINS"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "dmz"),
				),
			},
			// Modify criteria field, op, and argument
			{
				Config: testAccLabelResourceConfigModifyCriteria2(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.field", "name"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.op", "ENDSWITH"),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.0.argument", "-server"),
				),
			},
		},
	})
}

func testAccLabelResourceConfigModifyCriteria1(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "dmz"
    }
  ]
}
`, key, value)
}

func testAccLabelResourceConfigModifyCriteria2(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "ENDSWITH"
      argument = "-server"
    }
  ]
}
`, key, value)
}

func TestAccLabelResource_disappears(t *testing.T) {
	key := testAccRandomName("tf-acc-Test")
	value := testAccRandomName("tf-acc-Disappear")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_label.test", "id"),
				),
			},
			// Delete out-of-band
			{
				Config: testAccLabelResourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccDeleteLabelOutOfBand("guardicore_label.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccLabelResource_invalidCriteriaOp(t *testing.T) {
	key := testAccRandomName("tf-acc-Invalid")
	value := testAccRandomName("tf-acc-Op")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelResourceConfigInvalidOp(key, value),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func testAccLabelResourceConfigInvalidOp(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "INVALID_OP"
      argument = "test"
    }
  ]
}
`, key, value)
}

func TestAccLabelResource_emptyCriteria(t *testing.T) {
	key := testAccRandomName("tf-acc-Empty")
	value := testAccRandomName("tf-acc-Criteria")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelResourceConfigEmptyCriteria(key, value),
				ExpectError: regexp.MustCompile(`.*Attribute criteria list must contain at least 1 elements.*`),
			},
		},
	})
}

func testAccLabelResourceConfigEmptyCriteria(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key      = %[1]q
  value    = %[2]q
  criteria = []
}
`, key, value)
}

func TestAccLabelResource_emptyKeyValue(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLabelResourceConfigEmptyKey(),
				ExpectError: regexp.MustCompile(".*"),
			},
			{
				Config:      testAccLabelResourceConfigEmptyValue(),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func testAccLabelResourceConfigEmptyKey() string {
	return testAccProviderConfig() + `
resource "guardicore_label" "test" {
  key   = ""
  value = "SomeValue"
}
`
}

func testAccLabelResourceConfigEmptyValue() string {
	return testAccProviderConfig() + `
resource "guardicore_label" "test" {
  key   = "SomeKey"
  value = ""
}
`
}
