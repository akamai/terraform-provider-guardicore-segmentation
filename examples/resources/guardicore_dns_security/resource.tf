
resource "guardicore_dns_security" "akamai_intelligence" {
  name    = "Akamai Intelligence Feed"
  type    = "AKAMAI_INTELLIGENCE"
  enabled = true
}

resource "guardicore_dns_security" "custom_block" {
  name    = "Known Malware Domains"
  type    = "CUSTOM_BLOCK"
  enabled = true

  domains = [
    "malware.example.com",
    "c2.bad-domain.example",
  ]
}

resource "guardicore_dns_security" "custom_exclusion" {
  name    = "Approved Domains"
  type    = "CUSTOM_EXCLUSION"
  enabled = true

  domains = [
    "internal.example.com",
    "trusted.partner.example",
  ]
}

resource "guardicore_dns_security" "web_category" {
  name    = "Gambling Category"
  type    = "WEB_CATEGORY"
  enabled = true
}

resource "guardicore_dns_security" "custom_blocklist" {
  name    = "Staging Blocklist"
  type    = "CUSTOM_BLOCKLIST"
  enabled = false

  domains = [
    "suspicious-staging.example",
  ]
}

resource "guardicore_dns_security" "exclusion_list" {
  name    = "Shared Exclusion List"
  type    = "EXCLUSION_LIST"
  enabled = true

  domains = [
    "allow.example.net",
  ]
}
