
# Basic static labels used by other examples.
resource "guardicore_label" "environment_production" {
  key   = "Environment"
  value = "Production"
}

resource "guardicore_label" "environment_staging" {
  key   = "Environment"
  value = "Staging"
}

resource "guardicore_label" "application_web" {
  key   = "Application"
  value = "Web"
}

resource "guardicore_label" "application_app" {
  key   = "Application"
  value = "Application"
}

resource "guardicore_label" "application_database" {
  key   = "Application"
  value = "Database"
}

resource "guardicore_label" "compliance_pci" {
  key   = "Compliance"
  value = "PCI-DSS"
}

resource "guardicore_label" "location_us_east" {
  key   = "Location"
  value = "US-East"
}

# Dynamic labels demonstrate the nested criteria fields.
resource "guardicore_label" "linux_servers" {
  key   = "Operating System"
  value = "Linux"

  criteria = [
    {
      field    = "os_name"
      op       = "CONTAINS"
      argument = "Linux"
    },
    {
      field    = "os_name"
      op       = "CONTAINS"
      argument = "Ubuntu"
    },
  ]
}

resource "guardicore_label" "dmz_assets" {
  key   = "Network Zone"
  value = "DMZ"

  criteria = [
    {
      field    = "ip"
      op       = "STARTSWITH"
      argument = "10.1.0."
    },
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "dmz"
    },
  ]
}
