
# Look up a DNS security blocklist by ID.
data "guardicore_dns_security" "by_id" {
  id = "00000000-0000-0000-0000-000000000020"
}

# Look up a DNS security blocklist by name.
data "guardicore_dns_security" "malware_list" {
  name = "Known Malware Domains"
}

# System-managed DNS blocklist types (for example, AKAMAI_INTELLIGENCE and
# WEB_CATEGORY) should be referenced via data sources.
data "guardicore_dns_security" "akamai_intelligence" {
  name = "Akamai Intelligence Feed"
}

output "dns_security_details" {
  value = {
    id      = data.guardicore_dns_security.malware_list.id
    name    = data.guardicore_dns_security.malware_list.name
    type    = data.guardicore_dns_security.malware_list.type
    domains = data.guardicore_dns_security.malware_list.domains
    enabled = data.guardicore_dns_security.malware_list.enabled
  }
}
