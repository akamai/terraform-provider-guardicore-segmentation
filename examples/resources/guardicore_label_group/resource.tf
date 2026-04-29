
# Typed include selector: Production web servers.
resource "guardicore_label_group" "production_web" {
  key      = "Server Group"
  value    = "Production Web"
  comments = "Assets labeled as both production and web"

  include = {
    or_groups = [
      {
        label_ids = [
          guardicore_label.environment_production.id,
          guardicore_label.application_web.id,
        ]
      }
    ]
  }
}

# Typed include with multiple OR groups and an exclusion selector.
resource "guardicore_label_group" "application_tier" {
  key      = "Server Group"
  value    = "Application Tier"
  comments = "Application servers across non-DMZ environments"

  include = {
    or_groups = [
      {
        label_ids = [
          guardicore_label.environment_production.id,
          guardicore_label.application_app.id,
        ]
      },
      {
        label_ids = [
          guardicore_label.environment_staging.id,
          guardicore_label.application_app.id,
        ]
      },
    ]
  }

  exclude = {
    or_groups = [
      {
        label_ids = [guardicore_label.dmz_assets.id]
      }
    ]
  }
}

# Typed selector combined with raw JSON overlays.
resource "guardicore_label_group" "pci_scope" {
  key      = "Server Group"
  value    = "PCI Scope"
  comments = "Production PCI systems in US-East, excluding staging"

  include = {
    or_groups = [
      {
        label_ids = [
          guardicore_label.environment_production.id,
          guardicore_label.compliance_pci.id,
        ]
      }
    ]
  }

  raw_include_json = jsonencode({
    or_labels = [
      {
        and_labels = [guardicore_label.location_us_east.id]
      }
    ]
  })

  raw_exclude_json = jsonencode({
    or_labels = [
      {
        and_labels = [guardicore_label.environment_staging.id]
      }
    ]
  })
}
