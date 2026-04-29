
# Example 1: Standard allow rule using typed source and destination selectors.
resource "guardicore_policy_rule" "web_to_app_https" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Allow HTTPS from the web tier to the application tier"
  ruleset_name     = "default"
  priority         = 100

  source = {
    label_group_ids = [guardicore_label_group.production_web.id]
  }

  destination = {
    label_group_ids = [guardicore_label_group.application_tier.id]
  }

  ip_protocols = ["TCP"]
  ports        = [443, 8443]
}

# Example 2: Rule using subnets, domains, port ranges, and exclusion ranges.
resource "guardicore_policy_rule" "operations_window" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Allow operations access during a maintenance window"
  ruleset_name     = "operations"
  priority         = 110
  network_profile  = "CORPORATE"

  source = {
    subnets = ["10.20.0.0/24"]
    domains = ["jumpbox.example.com"]
  }

  destination = {
    labels = {
      or_labels = [
        {
          and_labels = [guardicore_label.environment_production.id, guardicore_label.application_database.id]
        }
      ]
    }
  }

  ip_protocols = ["TCP"]

  port_ranges = [
    {
      start = 32000
      end   = 32100
    },
  ]

  exclude_ports = [32005]

  exclude_port_ranges = [
    {
      start = 32090
      end   = 32099
    },
  ]

  schedule = {
    recurrence = "DTSTART:20250701T080000Z\nRRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;UNTIL=20250731T080000Z"
    end_time   = 1751295600000
  }
}

# Example 3: ICMP rule using asset, user group, and policy group references.
resource "guardicore_policy_rule" "infrastructure_ping" {
  action           = "ALERT"
  section_position = "ALERT"
  enabled          = true
  comments         = "Alert when managed users ping production assets"
  priority         = 120

  source = {
    user_group_ids   = [guardicore_user_group.platform_engineers.id]
    policy_group_ids = [guardicore_policy_group.internal_networks.id]
  }

  destination = {
    asset_ids = [guardicore_asset.database_primary.id]
  }

  ip_protocols = ["ICMP"]

  icmp_matches = [
    {
      icmp_type  = 8
      icmp_codes = [0]
      version    = "IPV4"
    },
  ]
}

# Example 4: Omit source to target any source and use typed scope/worksite fields.
resource "guardicore_policy_rule" "scoped_block" {
  action           = "BLOCK"
  section_position = "BLOCK"
  enabled          = true
  comments         = "Block scoped access to sensitive assets"
  priority         = 200
  worksite_id      = guardicore_worksite.headquarters.id
  scope            = [guardicore_label.environment_production.id]

  destination = {
    policy_group_ids = [guardicore_policy_group.production_servers.id]
    domains          = ["finance.example.internal"]
  }

  ip_protocols = ["TCP", "UDP"]

  ports = [443]
}

# Example 5: Typed endpoint fields for processes and Windows services.
resource "guardicore_policy_rule" "service_aware_dns" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Allow DNS traffic from Windows DNS clients"

  source = {
    address_classification = "Private"
    processes              = ["dns.exe"]
    windows_services = [
      {
        names = ["Dnscache"]
      },
    ]
  }

  destination = {
    processes = ["named"]
    domains   = ["dns.example.internal"]
  }

  ports        = [53]
  ip_protocols = ["UDP"]
}

# Example 6: raw_spec_json only for unsupported top-level extras.
resource "guardicore_policy_rule" "unsupported_extras" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Use raw_spec_json only for unsupported extras"

  destination = {
    subnets = ["10.99.0.0/24"]
  }

  ip_protocols = ["UDP"]
  ports        = [53]

  raw_spec_json = jsonencode({
    # Keep raw_spec_json only for unsupported top-level extras.
    attributes = {
      enforcement_mode = "strict"
    }
  })
}
