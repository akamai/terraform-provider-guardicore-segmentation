
# Look up a policy rule by ID.
data "guardicore_policy_rule" "existing_rule" {
  id = "00000000-0000-0000-0000-000000000100"
}

output "policy_rule_core_fields" {
  value = {
    id               = data.guardicore_policy_rule.existing_rule.id
    action           = data.guardicore_policy_rule.existing_rule.action
    section_position = data.guardicore_policy_rule.existing_rule.section_position
    enabled          = data.guardicore_policy_rule.existing_rule.enabled
    comments         = data.guardicore_policy_rule.existing_rule.comments
    ruleset_name     = data.guardicore_policy_rule.existing_rule.ruleset_name
    priority         = data.guardicore_policy_rule.existing_rule.priority
    network_profile  = data.guardicore_policy_rule.existing_rule.network_profile
    scope            = data.guardicore_policy_rule.existing_rule.scope
    worksite_id      = data.guardicore_policy_rule.existing_rule.worksite_id
  }
}

output "policy_rule_match_fields" {
  value = {
    ip_protocols        = data.guardicore_policy_rule.existing_rule.ip_protocols
    ports               = data.guardicore_policy_rule.existing_rule.ports
    port_ranges         = data.guardicore_policy_rule.existing_rule.port_ranges
    exclude_ports       = data.guardicore_policy_rule.existing_rule.exclude_ports
    exclude_port_ranges = data.guardicore_policy_rule.existing_rule.exclude_port_ranges
    icmp_matches        = data.guardicore_policy_rule.existing_rule.icmp_matches
    schedule            = data.guardicore_policy_rule.existing_rule.schedule
  }
}

output "policy_rule_endpoints_and_json" {
  value = {
    source      = data.guardicore_policy_rule.existing_rule.source
    destination = data.guardicore_policy_rule.existing_rule.destination
    spec_json   = data.guardicore_policy_rule.existing_rule.spec_json
    raw_json    = data.guardicore_policy_rule.existing_rule.raw_json
  }
}
