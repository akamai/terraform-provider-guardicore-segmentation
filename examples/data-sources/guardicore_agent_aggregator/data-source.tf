# Look up an agent aggregator by hostname.
data "guardicore_agent_aggregator" "primary" {
  hostname = "gc-aggregator-172-235-229-101"
}

# Look up an agent aggregator by ID.
data "guardicore_agent_aggregator" "by_id" {
  id = "69fba8c91aaa98c136026092"
}

output "aggregator_details" {
  value = {
    id             = data.guardicore_agent_aggregator.primary.id
    hostname       = data.guardicore_agent_aggregator.primary.hostname
    ip_address     = data.guardicore_agent_aggregator.primary.ip_address
    display_status = data.guardicore_agent_aggregator.primary.display_status
    state          = data.guardicore_agent_aggregator.primary.state
    version        = data.guardicore_agent_aggregator.primary.version
  }
}
