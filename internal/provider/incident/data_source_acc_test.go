package incident_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIncidentDataSource_byID(t *testing.T) {
	acctest.TestAccPreCheck(t)

	// First, find an existing incident via the API to use for the data source test.
	// Incidents created via v4.0 may not be immediately queryable via v3.0 generic-incidents,
	// so we use a pre-existing incident from the environment.
	tc := acctest.TestConfigInstance
	if tc == nil {
		t.Fatal("test config not loaded")
		return
	}

	apiClient, err := client.NewClient(client.Config{
		BaseURL:            tc.BaseURL,
		Username:           tc.Username,
		Password:           tc.Password,
		AccessToken:        tc.AccessToken,
		RefreshToken:       tc.RefreshToken,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// List incidents to find one we can query
	incidents, err := apiClient.ListIncidents(context.Background(), 946684800000, 4102444800000)
	if err != nil {
		t.Skipf("failed to list incidents: %v", err)
	}
	if len(incidents) == 0 {
		t.Skip("no incidents found in environment, skipping data source test")
	}

	incidentID, _ := incidents[0]["id"].(string)
	if incidentID == "" {
		t.Skip("first incident has no id field, skipping")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIncidentDataSourceConfigByID(incidentID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.guardicore_incident.test", "id", incidentID),
					resource.TestCheckResourceAttrSet("data.guardicore_incident.test", "incident_type"),
					resource.TestCheckResourceAttrSet("data.guardicore_incident.test", "severity"),
					resource.TestMatchResourceAttr("data.guardicore_incident.test", "severity", regexp.MustCompile("^(LOW|MEDIUM|HIGH)$")),
					resource.TestCheckResourceAttrSet("data.guardicore_incident.test", "raw_json"),
				),
			},
		},
	})
}

func testAccIncidentDataSourceConfigByID(id string) string {
	return acctest.TestAccProviderConfig() + `
data "guardicore_incident" "test" {
  id = "` + id + `"
}
`
}
