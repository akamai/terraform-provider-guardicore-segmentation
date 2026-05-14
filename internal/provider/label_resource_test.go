package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					resource.TestCheckResourceAttr("guardicore_label.test", "system_managed", "false"),
					resource.TestCheckResourceAttr("guardicore_label.test", "managed_by", "terraform"),
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
					testCheckLabelCriteriaContains("guardicore_label.test", "CONTAINS", "web"),
				),
			},
		},
	})
}

func TestAccLabelResource_updateKeyOnly(t *testing.T) {
	key1 := testAccRandomName("tf-acc-KeyOld")
	key2 := testAccRandomName("tf-acc-KeyNew")
	value := testAccRandomName("tf-acc-Value")
	var id1 string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfig(key1, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCaptureResourceID("guardicore_label.test", &id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key1),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
				),
			},
			{
				Config: testAccLabelResourceConfig(key2, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key2),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
				),
			},
		},
	})
}

func TestAccLabelResource_updateKeyAndValue(t *testing.T) {
	key1 := testAccRandomName("tf-acc-KeyOld")
	value1 := testAccRandomName("tf-acc-ValueOld")
	key2 := testAccRandomName("tf-acc-KeyNew")
	value2 := testAccRandomName("tf-acc-ValueNew")
	var id1 string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfig(key1, value1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCaptureResourceID("guardicore_label.test", &id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key1),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value1),
				),
			},
			{
				Config: testAccLabelResourceConfig(key2, value2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key2),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value2),
				),
			},
		},
	})
}

func TestAccLabelResource_updateToExistingKeyValueExpectError(t *testing.T) {
	keyA := testAccRandomName("tf-acc-KeyA")
	valueA := testAccRandomName("tf-acc-ValueA")
	keyB := testAccRandomName("tf-acc-KeyB")
	valueB := testAccRandomName("tf-acc-ValueB")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfigTwoLabels(keyA, valueA, keyB, valueB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.one", "key", keyA),
					resource.TestCheckResourceAttr("guardicore_label.one", "value", valueA),
					resource.TestCheckResourceAttr("guardicore_label.two", "key", keyB),
					resource.TestCheckResourceAttr("guardicore_label.two", "value", valueB),
				),
			},
			{
				Config:      testAccLabelResourceConfigTwoLabels(keyA, valueA, keyA, valueA),
				ExpectError: regexp.MustCompile("Label Already Exists"),
			},
		},
	})
}

func TestAccLabelResource_importSystemManagedLabel(t *testing.T) {
	label := testAccReadOnlyLabel(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccLabelResourceConfig(label.Key, label.Value),
				ResourceName:      "guardicore_label.test",
				ImportState:       true,
				ImportStateId:     label.ID,
				ImportStateVerify: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "system_managed", "true"),
				),
			},
			{
				Config:      testAccLabelResourceConfig(label.Key, label.Value+"-updated"),
				ExpectError: regexp.MustCompile("(?i)System-Managed Resource|reserved for auto"),
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

func testAccLabelResourceConfigTwoLabels(key1, value1, key2, value2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "one" {
  key   = %[1]q
  value = %[2]q
}

resource "guardicore_label" "two" {
  key   = %[3]q
  value = %[4]q
}
`, key1, value1, key2, value2)
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
					testCheckLabelCriteriaContains("guardicore_label.test", "CONTAINS", "web"),
					testCheckLabelCriteriaContains("guardicore_label.test", "STARTSWITH", "prod"),
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
			// Plan stability check when criteria order changes in config.
			{
				Config:   testAccLabelResourceConfigMultipleCriteriaReordered(key, value),
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

func testAccLabelResourceConfigMultipleCriteriaReordered(key, value string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = "STARTSWITH"
      argument = "prod"
    },
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
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
					testCheckLabelCriteriaContains("guardicore_label.test", "STARTSWITH", "db-"),
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
					testCheckLabelCriteriaContains("guardicore_label.test", "CONTAINS", "__tf_clear__"),
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
					testCheckLabelCriteriaContains("guardicore_label.test", "CONTAINS", "dmz"),
				),
			},
			// Modify criteria field, op, and argument
			{
				Config: testAccLabelResourceConfigModifyCriteria2(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					testCheckLabelCriteriaContains("guardicore_label.test", "ENDSWITH", "-server"),
				),
			},
		},
	})
}

func TestAccLabelResource_dynamicCriteriaChangesEndpointLifecycle(t *testing.T) {
	key1 := testAccRandomName("tf-acc-DynKey")
	value1 := testAccRandomName("tf-acc-DynVal")
	key2 := testAccRandomName("tf-acc-DynKeyNew")
	value2 := testAccRandomName("tf-acc-DynValNew")
	var id1 string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			// Create a new label with dynamic criteria.
			{
				Config: testAccLabelResourceConfigDynamicSingle(key1, value1, "STARTSWITH", "or-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCaptureResourceID("guardicore_label.test", &id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "1"),
					testCheckLabelCriteriaContains("guardicore_label.test", "STARTSWITH", "or-1"),
				),
			},
			// Update with added OR dynamic criteria (added).
			{
				Config: testAccLabelResourceConfigDynamicTwoSingles(key1, value1, "STARTSWITH", "or-1", "ENDSWITH", "or-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					testCheckLabelCriteriaContains("guardicore_label.test", "STARTSWITH", "or-1"),
					testCheckLabelCriteriaContains("guardicore_label.test", "ENDSWITH", "or-2"),
				),
			},
			// Update by replacing one OR criterion with an AND (compound) criterion.
			{
				Config: testAccLabelResourceConfigDynamicSingleAndCompound(key1, value1, "STARTSWITH", "or-1", []string{"container_labels", "image_name"}, []string{"STARTSWITH", "CONTAINS"}, []string{"and-1", "and-2"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					testCheckLabelCriteriaContains("guardicore_label.test", "STARTSWITH", "or-1"),
					testCheckLabelCompoundCriteriaContains("container_labels", "STARTSWITH", "and-1"),
					testCheckLabelCompoundCriteriaContains("image_name", "CONTAINS", "and-2"),
				),
			},
			// Update existing AND (compound) criterion by adding an inner AND row.
			{
				Config: testAccLabelResourceConfigDynamicSingleAndCompound(key1, value1, "STARTSWITH", "or-1", []string{"container_labels", "image_name", "container_command"}, []string{"STARTSWITH", "CONTAINS", "ENDSWITH"}, []string{"and-1", "and-2", "and-3"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					testCheckLabelCompoundCriteriaContains("container_command", "ENDSWITH", "and-3"),
				),
			},
			// Update existing AND (compound) criterion by removing an inner AND row.
			{
				Config: testAccLabelResourceConfigDynamicSingleAndCompound(key1, value1, "STARTSWITH", "or-1", []string{"container_labels", "container_command"}, []string{"STARTSWITH", "ENDSWITH"}, []string{"and-1", "and-3"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					testCheckLabelCompoundCriteriaContains("container_labels", "STARTSWITH", "and-1"),
					testCheckLabelCompoundCriteriaContains("container_command", "ENDSWITH", "and-3"),
				),
			},
			// Update by removing all dynamic criteria (deleted).
			{
				Config: testAccLabelResourceConfig(key1, value1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "0"),
				),
			},
			// Combine key/value change and dynamic criteria changes in one update.
			{
				Config: testAccLabelResourceConfigDynamicSingleAndCompound(key2, value2, "CONTAINS", "or-final", []string{"container_labels", "image_name"}, []string{"CONTAINS", "STARTSWITH"}, []string{"and-final-1", "and-final-2"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckResourceIDEquals(&id1),
					resource.TestCheckResourceAttr("guardicore_label.test", "key", key2),
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value2),
					resource.TestCheckResourceAttr("guardicore_label.test", "criteria.#", "2"),
					testCheckLabelCriteriaContains("guardicore_label.test", "CONTAINS", "or-final"),
					testCheckLabelCompoundCriteriaContains("container_labels", "CONTAINS", "and-final-1"),
					testCheckLabelCompoundCriteriaContains("image_name", "STARTSWITH", "and-final-2"),
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

func testAccLabelResourceConfigDynamicSingle(key, value, op, argument string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = %[3]q
      argument = %[4]q
    }
  ]
}
`, key, value, op, argument)
}

func testAccLabelResourceConfigDynamicTwoSingles(key, value, op1, arg1, op2, arg2 string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = %[3]q
      argument = %[4]q
    },
    {
      field    = "name"
      op       = %[5]q
      argument = %[6]q
    }
  ]
}
`, key, value, op1, arg1, op2, arg2)
}

func testAccLabelResourceConfigDynamicSingleAndCompound(key, value, op, argument string, fields, ops, args []string) string {
	parts := make([]string, len(fields))
	for i := range fields {
		parts[i] = fmt.Sprintf(`      {
        field    = %q
        op       = %q
        argument = %q
      }`, fields[i], ops[i], args[i])
	}

	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_label" "test" {
  key   = %[1]q
  value = %[2]q

  criteria = [
    {
      field    = "name"
      op       = %[3]q
      argument = %[4]q
    },
    {
      compound_criteria = [
%[5]s
      ]
    }
  ]
}
`, key, value, op, argument, strings.Join(parts, ",\n"))
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

func TestAccLabelResource_driftOnRemoteValueChange(t *testing.T) {
	key := testAccRandomName("tf-acc-Drift")
	value := testAccRandomName("tf-acc-Original")
	remoteValue := testAccRandomName("tf-acc-Remote")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_label"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("guardicore_label.test", "value", value),
				),
			},
			{
				Config: testAccLabelResourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccUpdateLabelValueOutOfBand("guardicore_label.test", remoteValue),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				Config:             testAccLabelResourceConfig(key, value),
				PlanOnly:           true,
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
				ExpectError: regexp.MustCompile(`.*Attribute criteria .* must contain at least 1 elements.*`),
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

func TestAccLabelResource_worksiteGeneratedLabelSuppressesReadOnlyCriteria(t *testing.T) {
	worksiteName := testAccRandomName("tf-acc-worksite-label")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorksitePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckResourceDestroyed("guardicore_worksite"),
		Steps: []resource.TestStep{
			{
				Config: testAccLabelResourceConfigWithWorksiteGeneratedLabel(worksiteName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("guardicore_worksite.test", "id"),
					resource.TestCheckResourceAttrSet("guardicore_label.worksite", "id"),
					resource.TestCheckResourceAttr("guardicore_label.worksite", "key", "Worksite"),
					resource.TestCheckResourceAttr("guardicore_label.worksite", "value", worksiteName),
					resource.TestCheckResourceAttr("guardicore_label.worksite", "criteria.#", "0"),
				),
			},
			{
				Config:   testAccLabelResourceConfigWithWorksiteGeneratedLabel(worksiteName),
				PlanOnly: true,
			},
		},
	})
}

func testAccLabelResourceConfigWithWorksiteGeneratedLabel(worksiteName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "guardicore_worksite" "test" {
  name = %[1]q
}

resource "guardicore_label" "worksite" {
  key   = "Worksite"
  value = guardicore_worksite.test.name
}
`, worksiteName)
}

func TestLabelResourceAPIToModel_SkipsReadOnlyWorksiteCriteria(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{}

	label := &client.Label{
		ID:    "label-1",
		Key:   "Worksite",
		Value: "Default",
		DynamicCriteria: []client.LabelCriteria{
			{Field: "z", Op: "EQUALS", Argument: "z"},
			{Field: "scoping_details.worksite.id", Op: "", Argument: "ws-1", Source: stringPtr("Worksite"), ReadOnly: boolPtr(true)},
			{Field: "a", Op: "CONTAINS", Argument: "a"},
		},
	}

	r.apiToModel(label, model)

	if len(model.Criteria) != 2 {
		t.Fatalf("expected 2 criteria after filtering, got %d", len(model.Criteria))
	}

	if model.Criteria[0].Field.ValueString() != "a" || model.Criteria[0].Op.ValueString() != "CONTAINS" {
		t.Fatalf("expected first criterion to be canonicalized to a/CONTAINS, got %s/%s", model.Criteria[0].Field.ValueString(), model.Criteria[0].Op.ValueString())
	}

	if model.Criteria[1].Field.ValueString() != "z" || model.Criteria[1].Op.ValueString() != "EQUALS" {
		t.Fatalf("expected second criterion to be canonicalized to z/EQUALS, got %s/%s", model.Criteria[1].Field.ValueString(), model.Criteria[1].Op.ValueString())
	}
}

func TestLabelResourceAPIToModel_CanonicalizesCriteriaOrderPermutations(t *testing.T) {
	tests := []struct {
		name  string
		input []client.LabelCriteria
		want  []string
	}{
		{
			name: "flat-compound-compound",
			input: []client.LabelCriteria{
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
		{
			name: "compound-flat-compound",
			input: []client.LabelCriteria{
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
		{
			name: "compound-compound-flat",
			input: []client.LabelCriteria{
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LabelResource{}
			model := &LabelResourceModel{}
			label := &client.Label{
				ID:              "label-1",
				Key:             "k",
				Value:           "v",
				DynamicCriteria: tt.input,
			}

			r.apiToModel(label, model)
			if len(model.Criteria) != len(tt.want) {
				t.Fatalf("expected %d criteria, got %d", len(tt.want), len(model.Criteria))
			}

			for i, want := range tt.want {
				got := criteriaFingerprint(model.Criteria[i])
				if got != want {
					t.Fatalf("criteria[%d] mismatch: got %q want %q", i, got, want)
				}

				if model.Criteria[i].Field.ValueString() == "" && model.Criteria[i].Op.ValueString() == "" && model.Criteria[i].Argument.ValueString() == "" && len(model.Criteria[i].CompoundCriteria) == 0 {
					t.Fatalf("criteria[%d] is an empty placeholder", i)
				}
			}
		})
	}
}

func TestLabelResourceAPIToModel_CanonicalizesNestedCompoundCriteriaOrder(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{}

	label := &client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{
				CompoundCriteria: []client.LabelCriteria{
					{Field: "z_field", Op: "ENDSWITH", Argument: "z"},
					{Field: "a_field", Op: "CONTAINS", Argument: "a"},
					{Field: "m_field", Op: "EQUALS", Argument: "m"},
				},
			},
		},
	}

	r.apiToModel(label, model)

	if len(model.Criteria) != 1 {
		t.Fatalf("expected 1 criteria, got %d", len(model.Criteria))
	}

	compound := model.Criteria[0].CompoundCriteria
	if len(compound) != 3 {
		t.Fatalf("expected 3 compound criteria, got %d", len(compound))
	}

	got := []string{
		compound[0].Field.ValueString(),
		compound[1].Field.ValueString(),
		compound[2].Field.ValueString(),
	}
	want := []string{"a_field", "m_field", "z_field"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("compound canonical order mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestLabelResourceAPIToModel_CanonicalizationProducesNoDiffAcrossAPIOrderings(t *testing.T) {
	r := &LabelResource{}

	first := &LabelResourceModel{}
	second := &LabelResourceModel{}

	r.apiToModel(&client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{Field: "name", Op: "CONTAINS", Argument: "web"},
			{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}, {Field: "container_command", Op: "CONTAINS", Argument: "run"}}},
		},
	}, first)

	r.apiToModel(&client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "run"}, {Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}}},
			{Field: "name", Op: "CONTAINS", Argument: "web"},
		},
	}, second)

	if !labelCriteriaEqual(first.Criteria, second.Criteria) {
		t.Fatalf("expected canonicalized criteria to be equal across API order permutations; first=%v second=%v", first.Criteria, second.Criteria)
	}
}

func criteriaFingerprint(c LabelCriteriaModel) string {
	if len(c.CompoundCriteria) > 0 {
		first := c.CompoundCriteria[0]
		return "compound:" + first.Field.ValueString() + "|" + first.Op.ValueString() + "|" + first.Argument.ValueString()
	}

	return "flat:" + c.Field.ValueString() + "|" + c.Op.ValueString() + "|" + c.Argument.ValueString()
}

func TestWorksiteLabelEditableChange(t *testing.T) {
	base := LabelResourceModel{
		Key:   types.StringValue("Worksite"),
		Value: types.StringValue("ws-a"),
		Criteria: []LabelCriteriaModel{
			{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("a")},
		},
	}

	t.Run("no changes", func(t *testing.T) {
		if worksiteLabelEditableChange(base, base) {
			t.Fatal("expected no editable change")
		}
	})

	t.Run("value changed", func(t *testing.T) {
		plan := base
		plan.Value = types.StringValue("ws-b")
		if !worksiteLabelEditableChange(base, plan) {
			t.Fatal("expected editable change when value changes")
		}
	})

	t.Run("criteria changed", func(t *testing.T) {
		plan := base
		plan.Criteria = []LabelCriteriaModel{
			{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("b")},
		}
		if !worksiteLabelEditableChange(base, plan) {
			t.Fatal("expected editable change when criteria changes")
		}
	})
}

func testCheckLabelCriteriaContains(resourceName, op, argument string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		for key, value := range rs.Primary.Attributes {
			if !strings.HasPrefix(key, "criteria.") || !strings.HasSuffix(key, ".field") {
				continue
			}
			if value != "name" {
				continue
			}

			prefix := strings.TrimSuffix(key, ".field")
			if rs.Primary.Attributes[prefix+".op"] == op && rs.Primary.Attributes[prefix+".argument"] == argument {
				return nil
			}
		}

		return fmt.Errorf("criterion not found on %s: field=%q op=%q argument=%q", resourceName, "name", op, argument)
	}
}

func testCheckLabelCompoundCriteriaContains(field, op, argument string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["guardicore_label.test"]
		if !ok {
			return fmt.Errorf("resource not found: guardicore_label.test")
		}

		for key, value := range rs.Primary.Attributes {
			if !strings.HasPrefix(key, "criteria.") || !strings.Contains(key, ".compound_criteria.") || !strings.HasSuffix(key, ".field") {
				continue
			}
			if value != field {
				continue
			}

			prefix := strings.TrimSuffix(key, ".field")
			if rs.Primary.Attributes[prefix+".op"] == op && rs.Primary.Attributes[prefix+".argument"] == argument {
				return nil
			}
		}

		return fmt.Errorf("compound criterion not found on guardicore_label.test: field=%q op=%q argument=%q", field, op, argument)
	}
}
