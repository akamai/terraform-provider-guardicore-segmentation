# Terraform Provider for Akamai Guardicore Segmentation

A Terraform provider for managing resources in [Akamai Guardicore Segmentation](https://www.akamai.com/products/akamai-guardicore-segmentation), built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Features

- **Labels**: Create and manage labels for categorizing assets with optional dynamic criteria for automatic assignment
- **Label Groups**: Define logical groupings of labels using AND/OR logic for policy rules
- **Policy Rules**: Configure visibility and segmentation policy rules and worksite assignment
- **Policy Groups**: Create collections of labels, FQDNs, or IP addresses for use in policy rules (supports LABEL, FQDN, and IP_ADDRESS types)
- **DNS Security**: Manage custom DNS blocklists for blocking or excluding domains; system-managed blocklists are available via data source lookup
- **Incidents**: Create security incidents with full metadata (immutable — no update or delete via API; reads use `/api/v3.0/generic-incidents`)
- **Worksites**: Manage worksites for organizing agents, assets, and policy rules by physical or logical location
- **User Groups**: Manage user groups that associate Active Directory orchestration groups for visibility policies
- **Assets**: Manage network assets with NICs, assignable labels, orchestration metadata, and worksite assignments (DELETE deactivates rather than permanently removes)
- **Agent Aggregators**: Read-only data source for looking up agent aggregators by ID or hostname (system-managed infrastructure)
- **Asset Label Safety**: Validates asset label assignability; read-only labels and dynamic-criteria labels are rejected, while server-assigned labels are ignored unless explicitly configured
- **Bulk Operation Batching**: Provider-side batching coalesces concurrent operations for bulk-capable endpoints (labels, label groups, policy rules, user groups, assets, DNS security, incident create, and worksite delete)
- **Worksite Delete Safety**: Worksite deletes now fail when the API returns bulk-delete skips/failures in a `200` response (for example, when entities are still assigned), so `terraform destroy` surfaces a clear actionable error instead of silently succeeding
- **Cross-Reference Validation**: Validates that referenced label, label group, policy group, user group, asset, and worksite IDs exist in Akamai Guardicore Segmentation during `terraform plan` and `terraform apply`, with clear error messages
- **Lifecycle Protection**: Optional destroy-time warnings or errors when deleting labels, label groups, user groups, or policy groups that are referenced by other resources (`validate_references_on_destroy`, `strict_references_on_destroy`)

## Documentation

- **Full Documentation**: Available in the [`docs/`](docs/) directory and on the [Terraform Registry](https://registry.terraform.io/providers/akamai/guardicore/latest/docs)

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5 when using importer-generated `import` blocks
- [Go](https://golang.org/doc/install) >= 1.25.5
- GNU Make for repository `make` targets

## Building The Provider

1. Clone the repository.
1. Enter the repository directory.
1. Build the provider:

```shell
make build

# or build and install to $GOPATH/bin
make install

# direct Go alternative
go install -v ./...
```

Useful development targets:

```shell
make fmt
make lint
make test
make testacc
make generate
make build-importer
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the Provider

### Provider Configuration

```hcl
provider "guardicore" {
  base_url = "https://your-guardicore-instance.example.com"

  # Preferred for MFA-enabled accounts and automation
  access_token = var.guardicore_access_token

  # Alternatives:
  # refresh_token = var.guardicore_refresh_token
  # username      = var.guardicore_username
  # password      = var.guardicore_password

  # Optional: skip TLS verification for self-signed certificates
  insecure_skip_verify = false

  # Optional: HTTP request timeout in seconds (default: 30)
  # Increase for bulk operations or slow networks
  # request_timeout = 120

  # Optional: warn before destroying labels, label groups, user groups, or policy groups referenced by other resources
  validate_references_on_destroy = true

  # Optional: block (error) instead of warn when destroying referenced resources (implies validate_references_on_destroy)
  # strict_references_on_destroy = true
}
```

Authentication precedence is access token, refresh token, then username/password. Username/password authentication does not work for accounts that require MFA; use `access_token` or `refresh_token` for those accounts.

### Environment Variables

The provider can be configured using environment variables (used as fallbacks when the corresponding attribute is not set in the provider block):

| Environment Variable | Description |
|----------------------|-------------|
| `GUARDICORE_BASE_URL` | Akamai Guardicore Segmentation API URL |
| `GUARDICORE_USERNAME` | Username for authentication |
| `GUARDICORE_PASSWORD` | Password for authentication |
| `GUARDICORE_ACCESS_TOKEN` | Direct API access token |
| `GUARDICORE_REFRESH_TOKEN` | Refresh token for authentication |
| `GUARDICORE_INSECURE_SKIP_VERIFY` | Skip TLS certificate verification (`true`/`false`) |
| `GUARDICORE_REQUEST_TIMEOUT` | HTTP request timeout in seconds (default: `30`) |

### Example Usage

```hcl
# Create a label
resource "guardicore_label" "production" {
  key   = "Environment"
  value = "Production"
}

# Create a label with dynamic criteria
resource "guardicore_label" "web_servers" {
  key   = "Application"
  value = "Web Server"

  criteria = [
    {
      field    = "name"
      op       = "CONTAINS"
      argument = "web"
    }
  ]
}

# Create a label group
resource "guardicore_label_group" "prod_web" {
  key      = "Server Group"
  value    = "Production Web Servers"
  comments = "All production web servers"

  include = {
    or_groups = [
      {
        label_ids = [
          guardicore_label.production.id,
          guardicore_label.web_servers.id,
        ]
      }
    ]
  }
}
```

See the [`examples/`](examples/) directory for more examples.

## Importer Tool

The importer CLI tool snapshots all existing resources from an Akamai Guardicore Segmentation instance and generates Terraform configuration files with [import blocks](https://developer.hashicorp.com/terraform/language/import) (Terraform 1.5+).

### Build

```shell
make build-importer

# or build directly
# Linux/macOS:
go build -o ./bin/guardicore-importer ./cmd/importer/
# Windows:
go build -o .\bin\guardicore-importer.exe .\cmd\importer\
```

### Usage

```shell
# Linux/macOS:
./bin/guardicore-importer --base-url https://guardicore.example.com --access-token "your-token" --output-dir ./imported/

# Windows PowerShell:
.\bin\guardicore-importer.exe --base-url https://guardicore.example.com --access-token "your-token" --output-dir .\imported\
```

#### Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `--base-url` | `GUARDICORE_BASE_URL` | Akamai Guardicore Segmentation API base URL (required) |
| `--username` | `GUARDICORE_USERNAME` | Username for authentication |
| `--password` | `GUARDICORE_PASSWORD` | Password for authentication |
| `--access-token` | `GUARDICORE_ACCESS_TOKEN` | Direct API access token |
| `--refresh-token` | `GUARDICORE_REFRESH_TOKEN` | Refresh token for authentication |
| `--insecure` | | Skip TLS certificate verification |
| `--request-timeout` | `GUARDICORE_REQUEST_TIMEOUT` | HTTP request timeout in seconds (default: 120) |
| `--output-dir` | | Output directory (default: `.`) |

### Output

The tool generates files with **Terraform resource reference expressions** where possible (e.g., `guardicore_label.foo.id` instead of hardcoded UUIDs), enabling Terraform to automatically infer creation and deletion order:
- `labels.tf` — All labels with optional dynamic criteria
- `label_groups.tf` — All label groups with include/exclude labels (references `guardicore_label.*.id`)
- `policy_rules.tf` — All policy rules with typed Terraform attributes and raw JSON escape hatches when needed (references `guardicore_label.*.id`, `guardicore_label_group.*.id`, `guardicore_user_group.*.id`, `guardicore_asset.*.id`, `guardicore_worksite.*.id`; policy group IDs remain literals today)
- `dns_security.tf` — All DNS blocklists with domains and enabled state
- `incidents.tf` — All incidents as commented-out reference blocks generated from `/api/v3.0/generic-incidents` reads (import not supported due to create/read schema asymmetry)
- `worksites.tf` — All worksites with name and comment (requires worksites feature to be enabled)
- `user_groups.tf` — User groups with orchestration group associations; local user groups are skipped where they cannot be managed through orchestration
- `assets.tf` — All assets with NICs, assignable labels (references `guardicore_label.*.id`, `.key`, `.value`), worksite assignments (references `guardicore_worksite.*.id`), and orchestration metadata. Read-only labels and dynamic-criteria labels are skipped; labels with unresolved assignability are included with stderr warnings.
- `agent_aggregators.tf` — Agent aggregators generated as data source blocks (read-only, system-managed infrastructure)

`policy_groups.tf` is not generated yet. Policy group IDs that appear in imported policy rules remain as literal IDs with a `# reference not imported` comment.

IDs that cannot be resolved to an imported resource fall back to hardcoded string literals with a `# reference not imported` comment.

Each resource includes an `import` block. After generation:

```shell
cd ./imported/
terraform init
terraform plan  # Should show no changes if resources match
```

## Current Status & Known Limitations

### ✅ Production Ready
- **Labels**: Full CRUD operations with dynamic criteria
- **Label Groups**: Full CRUD operations with OR/AND logic
- **Policy Rules**: Full CRUD operations with typed Terraform attributes plus raw JSON escape hatches; create, update, and delete operations publish a policy revision automatically; supports worksite assignment
- **DNS Security**: Full CRUD operations with bulk endpoints; requires DNS Security to be enabled on the Akamai Guardicore Segmentation instance
- **Incidents**: Create-only operations (immutable after creation, no update/delete API)
- **Worksites**: Full CRUD operations (requires worksites feature to be enabled on Akamai Guardicore Segmentation instance)
- **User Groups**: Full CRUD operations with nested orchestration groups (acceptance coverage requires AD orchestration setup)
- **Assets**: Full CRUD operations with NICs, assignable labels, orchestration metadata, and worksite assignments (DELETE deactivates)
- **Batching Runtime**: Bulk-capable resources use provider-side request coalescing to send true batched API calls rather than one-item bulk requests
- **Policy Groups**: Full CRUD operations for label, FQDN, and IP address collections (requires policy groups feature enabled on Akamai Guardicore Segmentation instance)
- **Data Sources**: All data sources for labels, label groups, policy rules, policy groups, DNS blocklists, incidents, worksites, user groups, assets, and agent aggregators

### Known Limitations

#### API Constraints (Handled by Provider)

1. **Label criteria required on create** (empty list accepted)
2. **Label groups must contain at least one label**
3. **Policy rule list fields** may be returned in a different order than sent
4. **DNS blocklists of type CUSTOM_BLOCK require at least one domain** on create
5. **DNS blocklist type is immutable** after creation
6. **System-managed DNS blocklist types are not creatable** — `AKAMAI_INTELLIGENCE` and `WEB_CATEGORY` cannot be used when creating `guardicore_dns_security` resources; use the data source to reference them
7. **Incidents are immutable** — no update or delete endpoints; the provider removes from state only
8. **Incident create/read schema asymmetry** — reads come from `/api/v3.0/generic-incidents` and many create fields are not returned; import is not supported
9. **Worksites require feature flag** — the worksites feature must be enabled on the Akamai Guardicore Segmentation instance; the API returns a 400 error when disabled
10. **DNS Security requires feature flag** — the DNS Security feature must be enabled on the Akamai Guardicore Segmentation instance; the API returns a 403 `OperationFailed` with an `error_dump` indicating DNS Security is not enabled
11. **Worksite assignment** — assets and policy rules support `worksite_id` attribute for worksite assignment; policy rules use a separate bulk move endpoint
12. **Asset GET/DELETE single endpoints return 405** — provider uses list with `?id=` filter and bulk deactivate instead
13. **Asset deactivation** — deleting an asset sets its status to `"deleted"` rather than permanently removing it
14. **Asset create-only fields** — `orchestration_obj_id`, `instance_id`, `hw_uuid` are only set on create and not returned on read
15. **Policy groups require feature flag** — the policy groups feature must be enabled on the Akamai Guardicore Segmentation instance; the API returns 403 when disabled
16. **Policy group type is immutable** — changing `type` after creation requires resource replacement
17. **Policy group member limit** — maximum 100 members per group; the provider validates this at plan time

#### Provider Limitations

1. **Asset label assignability validation** — `guardicore_asset.labels` accepts only directly assignable labels. Labels marked `read_only = true` and labels with dynamic criteria are rejected during plan/apply.
2. **Asset server-assigned labels** — the platform may auto-assign labels (e.g., "Agent: Not Installed"); only user-specified labels are tracked in Terraform state. The API list endpoint returns labels with only the `id` field (key/value empty); each label requires all three fields (`id`, `key`, `value`) via Terraform references
3. **Agent aggregators are read-only** — exposed as a data source only because aggregators are system-managed infrastructure
4. **Importer does not export policy groups yet** — policy group IDs in imported policy rules remain literal IDs with a `# reference not imported` comment
5. Using this provider in tandem with external Orchestrations that also manage the same entities (for example labels and assets) can cause drift and unexpected plan/apply behavior due to competing updates.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `make build`. To build and install the provider binary to `$GOPATH/bin`, run `make install`.


To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```
