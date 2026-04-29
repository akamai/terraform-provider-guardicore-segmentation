
# LABEL policy group with include and exclude members.
resource "guardicore_policy_group" "production_servers" {
  name     = "Production Servers"
  type     = "LABEL"
  comments = "Production web and database assets excluding staging"

  members_json = jsonencode([
    [guardicore_label.environment_production.id, guardicore_label.application_web.id],
    [guardicore_label.environment_production.id, guardicore_label.application_database.id],
  ])

  exclude_members_json = jsonencode([
    [guardicore_label.environment_staging.id],
  ])
}

# FQDN policy group.
resource "guardicore_policy_group" "trusted_domains" {
  name     = "Trusted Domains"
  type     = "FQDN"
  comments = "Approved partner and corporate domains"

  members_json = jsonencode([
    "example.com",
    "*.corp.example.com",
    "api.partner.example",
  ])
}

# IP address policy group.
resource "guardicore_policy_group" "internal_networks" {
  name     = "Internal Networks"
  type     = "IP_ADDRESS"
  comments = "Internal corporate networks"

  members_json = jsonencode([
    { subnet = "10.0.0.0/8" },
    { subnet = "172.16.0.0/12" },
    { range = { start = "192.168.10.10", end = "192.168.10.50" } },
  ])
}
