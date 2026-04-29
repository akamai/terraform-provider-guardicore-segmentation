# Akamai Guardicore Segmentation Terraform Importer

The importer CLI tool exports all existing resources from an Akamai Guardicore Segmentation instance and generates Terraform configuration files with [import blocks](https://developer.hashicorp.com/terraform/language/import) (requires Terraform 1.5+).

## Overview

When adopting Terraform for an existing Akamai Guardicore Segmentation deployment, you need to bring existing resources under Terraform management. The importer automates this by:

1. Fetching all labels, label groups, policy rules, DNS blocklists, incidents, worksites, user groups, and assets from the Akamai Guardicore Segmentation API
2. Generating `.tf` files with `resource` blocks matching each resource's current state
3. Including `import` blocks so Terraform can associate the configuration with existing resources

## Prerequisites

- Network access to your Akamai Guardicore Segmentation instance
- Valid API credentials (username/password, access token, or refresh token)
- Go 1.24+ (to build from source)

## Building

```shell
make build-importer

# or build directly
# Linux/macOS:
go build -o ./bin/guardicore-importer ./cmd/importer/
# Windows:
go build -o .\bin\guardicore-importer.exe .\cmd\importer\
```

## Authentication

The importer supports the same authentication methods as the Terraform provider:

| Method | Flags | Env Vars |
|--------|-------|----------|
| Username/Password | `--username`, `--password` | `GUARDICORE_USERNAME`, `GUARDICORE_PASSWORD` |
| Access Token | `--access-token` | `GUARDICORE_ACCESS_TOKEN` |
| Refresh Token | `--refresh-token` | `GUARDICORE_REFRESH_TOKEN` |

## CLI Flags Reference

| Flag | Env Var                    | Required | Default | Description |
|------|----------------------------|----------|---------|-------------|
| `--base-url` | `GUARDICORE_BASE_URL`      | Yes | | Akamai Guardicore Segmentation API base URL |
| `--username` | `GUARDICORE_USERNAME`      | No | | Username for authentication |
| `--password` | `GUARDICORE_PASSWORD`      | No | | Password for authentication |
| `--access-token` | `GUARDICORE_ACCESS_TOKEN`  | No | | Direct API access token |
| `--refresh-token` | `GUARDICORE_REFRESH_TOKEN` | No | | Refresh token |
| `--insecure` |                            | No | `false` | Skip TLS certificate verification |
| `--request-timeout` | `GUARDICORE_REQUEST_TIMEOUT`      | No | `30` | HTTP request timeout in seconds |
| `--output-dir` |                            | No | `.` | Directory for generated .tf files |

## Usage

```shell
# Using access token
./bin/guardicore-importer --base-url https://guardicore.example.com --access-token "your-token" --output-dir ./imported/

# Using username/password
./bin/guardicore-importer --base-url https://guardicore.example.com --username admin --password secret --output-dir ./imported/

# Using environment variables
# Linux/macOS:
export GUARDICORE_BASE_URL=https://guardicore.example.com
export GUARDICORE_ACCESS_TOKEN=your-token
./bin/guardicore-importer --output-dir ./imported/

# Windows PowerShell:
$env:GUARDICORE_BASE_URL="https://guardicore.example.com"
$env:GUARDICORE_ACCESS_TOKEN="your-token"
.\bin\guardicore-importer.exe --output-dir .\imported\
```

## Output Format

The importer generates the following files:

### labels.tf

```hcl
resource "guardicore_label" "environment_production" {
  key   = "Environment"
  value = "Production"
}

import {
  to = guardicore_label.environment_production
  id = "abc-123"
}

resource "guardicore_label" "application_web_server" {
  key   = "Application"
  value = "Web Server"

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
    },
  ]
}

import {
  to = guardicore_label.application_web_server
  id = "def-456"
}
```

### label_groups.tf

```hcl
resource "guardicore_label_group" "role_web_servers" {
  key      = "Role"
  value    = "Web Servers"
  comments = "All web servers"
  include = {
    or_groups = [
      {
        label_ids = [
          guardicore_label.environment_production.id,
          guardicore_label.application_web_server.id,
        ]
      }
    ]
  }
}

import {
  to = guardicore_label_group.role_web_servers
  id = "ghi-789"
}
```

### dns_security.tf

```hcl
resource "guardicore_dns_security" "malware_domains" {
  name    = "Known Malware Domains"
  type    = "CUSTOM_BLOCK"
  enabled = true
  domains = [
    "malware.example.com",
    "c2.bad-domain.net",
  ]
}

import {
  to = guardicore_dns_security.malware_domains
  id = "mno-345"
}
```

### incidents.tf

Incidents are exported as **commented-out reference blocks** because the `guardicore_incident` resource is immutable and cannot be imported. Incident reads come from `/api/v3.0/generic-incidents`, while creates use `/api/v4.0/incidents`, so required create fields like `description` and `summary` are hardcoded with placeholder values. The generated file serves as documentation of existing incidents.

```hcl
# NOTE: Incidents are immutable and cannot be imported into Terraform.
# This block documents incident abc-12345-def-67890 for reference only.
# resource "guardicore_incident" "network_scan_a1b2c3d4" {
#   type        = "Network Scan"
#   severity    = "HIGH"
#   time        = 1504688829035
#   description = "Imported incident"
#   summary     = "Imported incident abc-12345-def-67890"
#   tags        = ["Internal", "Critical"]
#
#   affected_assets_json = jsonencode([
#     {
#       "ip": "10.0.0.1"
#     }
#   ])
# }
```

### policy_rules.tf

```hcl
resource "guardicore_policy_rule" "allow_web_traffic" {
  action           = "ALLOW"
  section_position = "ALLOW"
  enabled          = true
  comments         = "Allow web traffic"
  priority         = 100

  source = {
    label_group_ids = [guardicore_label_group.role_web_servers.id]
  }

  destination = {
    label_group_ids = [guardicore_label_group.role_web_servers.id]
  }

	ports        = [80, 443]
	ip_protocols = ["TCP"]
	worksite_id = guardicore_worksite.headquarters.id
}

import {
  to = guardicore_policy_rule.allow_web_traffic
  id = "jkl-012"
}
```

Unsupported top-level policy rule extras are emitted through `raw_spec_json` when the importer cannot map them to typed Terraform attributes. Catch-all endpoints are represented by omitting `source` or `destination`, which the provider translates to empty endpoint objects for the API.

### worksites.tf

```hcl
resource "guardicore_worksite" "headquarters" {
  name    = "Headquarters"
  comment = "Main office building"
}

import {
  to = guardicore_worksite.headquarters
  id = "pqr-678"
}
```

### user_groups.tf

```hcl
resource "guardicore_user_group" "development_team" {
  title = "Development Team"

  orchestrations_groups {
    orchestration_id = "orch-id-1"
    groups           = ["group-id-1", "group-id-2"]
  }
}

import {
  to = guardicore_user_group.development_team
  id = "stu-901"
}
```

### assets.tf

```hcl
resource "guardicore_asset" "web_server_01" {
  name                 = "web-server-01"
  orchestration_obj_id = "orch-12345"
  comments = "Production web server"
  status = "on"
  worksite_id = guardicore_worksite.headquarters.id

  labels {
    id    = guardicore_label.environment_production.id
    key   = "Environment"
    value = "Production"
  }

  labels {
    id    = guardicore_label.application_web_server.id
    key   = "Application"
    value = "Web Server"
  }

  nics = [
    {
      ip_addresses = ["10.0.0.1"]
      mac_address  = "00:11:22:33:44:55"
    },
  ]
}

import {
  to = guardicore_asset.web_server_01
  id = "vwx-234"
}
```

## Post-Import Workflow

1. **Review** the generated `.tf` files for correctness
2. **Run** `terraform init` to initialize the Terraform working directory
3. **Run** `terraform plan` to verify the imported state matches (should show no changes)
4. **Run** `terraform apply` to complete the import
5. **Optionally** remove the `import` blocks after successful import (they are only needed once)

## Troubleshooting

### TLS certificate errors

If your Akamai Guardicore Segmentation instance uses a self-signed certificate:

```shell
# Linux/macOS:
./bin/guardicore-importer --base-url https://guardicore.example.com --insecure ...

# Windows PowerShell:
.\bin\guardicore-importer.exe --base-url https://guardicore.example.com --insecure ...
```

### Authentication failures

- Verify your credentials work by testing with `curl` against the Akamai Guardicore Segmentation API
- Access tokens may expire; try using username/password or refresh token instead
- Ensure the API user has read access to labels, label groups, policy rules, DNS blocklists, incidents, worksites, user groups, and assets

### Large environments

The importer paginates API requests (100 items per page) and handles environments of any size. For very large deployments, the import may take several minutes.

### Resource name collisions

When multiple resources would generate the same Terraform name (e.g., two labels with the same key/value), the importer automatically appends `_2`, `_3`, etc. suffixes. Resource names are sorted by ID for deterministic output.
