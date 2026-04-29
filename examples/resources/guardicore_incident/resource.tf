
# Minimal incident with required fields.
resource "guardicore_incident" "malware_detected" {
  type        = "CustomIncident"
  severity    = "HIGH"
  time        = 1711612800000
  description = "Malware detected on a production server"
  summary     = "### Malware Detection\n\nEndpoint security detected malicious activity on a production asset."
  tags        = ["malware", "production"]

  affected_assets_json = jsonencode([
    {
      type  = "IP"
      value = "10.0.1.50"
    },
  ])
}

# Incident demonstrating every optional argument.
resource "guardicore_incident" "suspicious_activity" {
  type        = "CustomIncident"
  severity    = "MEDIUM"
  time        = 1711616400000
  description = "Suspicious lateral movement detected"
  summary     = "### Suspicious Activity\n\nMultiple east-west connections were detected between sensitive assets."
  tags        = ["lateral-movement", "investigation"]
  origin      = "Terraform Example"
  mitigation  = "### Recommended Actions\n\n1. Isolate the source host.\n2. Review recent credentials."

  affected_assets_json = jsonencode([
    {
      type  = "IP"
      value = "10.0.2.10"
    },
    {
      type  = "IP"
      value = "10.0.2.11"
    },
  ])

  cef_extensions_json = jsonencode({
    sproc = "powershell.exe"
    dproc = "sqlservr.exe"
  })

  attached_files = [
    "file-id-1",
    "file-id-2",
  ]

  map_details_json = jsonencode({
    type = "conversation"
    time_filter = {
      from = 1711612800000
      to   = 1711616400000
    }
  })

  custom_defined_objects_json = jsonencode([
    {
      key   = "campaign"
      type  = "text"
      value = "spring-incident-review"
    },
  ])

  properties_json = jsonencode([
    {
      key   = "scanner_type"
      type  = "text"
      value = "lateral-movement"
    },
  ])
}
