package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccAgentAggregatorPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
}

func testAccDiscoverAgentAggregator(t *testing.T) *client.AgentAggregator {
	t.Helper()

	apiClient, err := client.NewClient(client.Config{
		BaseURL:            testConfig.BaseURL,
		Username:           testConfig.Username,
		Password:           testConfig.Password,
		AccessToken:        testConfig.AccessToken,
		RefreshToken:       testConfig.RefreshToken,
		InsecureSkipVerify: testConfig.InsecureSkipVerify,
	})
	if err != nil {
		t.Skipf("Skipping agent aggregator test: unable to create API client: %s", err)
	}

	aggregators, err := apiClient.ListAgentAggregators(context.Background(), "")
	if err != nil {
		t.Skipf("Skipping agent aggregator test: unable to list agent aggregators: %s", err)
	}

	if len(aggregators) == 0 {
		t.Skip("Skipping agent aggregator test: no agent aggregators found in the environment")
	}

	return &aggregators[0]
}

func TestAccAgentAggregatorDataSource_byHostname(t *testing.T) {
	testAccAgentAggregatorPreCheck(t)
	aggregator := testAccDiscoverAgentAggregator(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentAggregatorDataSourceConfigByHostname(aggregator.Hostname),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "id"),
					resource.TestCheckResourceAttr("data.guardicore_agent_aggregator.test", "hostname", aggregator.Hostname),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "ip_address"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "display_status"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "state"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "aggregator_type"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "version"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "component_id"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "agent_id"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "first_seen"),
				),
			},
		},
	})
}

func testAccAgentAggregatorDataSourceConfigByHostname(hostname string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
data "guardicore_agent_aggregator" "test" {
  hostname = %[1]q
}
`, hostname)
}

func TestAccAgentAggregatorDataSource_byID(t *testing.T) {
	testAccAgentAggregatorPreCheck(t)
	aggregator := testAccDiscoverAgentAggregator(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentAggregatorDataSourceConfigByID(aggregator.ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.guardicore_agent_aggregator.test", "id", aggregator.ID),
					resource.TestCheckResourceAttr("data.guardicore_agent_aggregator.test", "hostname", aggregator.Hostname),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "ip_address"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "display_status"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "state"),
					resource.TestCheckResourceAttrSet("data.guardicore_agent_aggregator.test", "aggregator_type"),
				),
			},
		},
	})
}

func testAccAgentAggregatorDataSourceConfigByID(id string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
data "guardicore_agent_aggregator" "test" {
  id = %[1]q
}
`, id)
}
