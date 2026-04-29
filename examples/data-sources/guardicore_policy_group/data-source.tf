
# Look up a policy group by ID.
data "guardicore_policy_group" "by_id" {
  id = "550e8400-e29b-41d4-a716-446655440000"
}

# Look up a policy group by name and type.
data "guardicore_policy_group" "production_servers" {
  name = "Production Servers"
  type = "LABEL"
}

output "policy_group_details" {
  value = {
    id                   = data.guardicore_policy_group.production_servers.id
    name                 = data.guardicore_policy_group.production_servers.name
    type                 = data.guardicore_policy_group.production_servers.type
    comments             = data.guardicore_policy_group.production_servers.comments
    members_json         = data.guardicore_policy_group.production_servers.members_json
    exclude_members_json = data.guardicore_policy_group.production_servers.exclude_members_json
  }
}
