
# Look up a worksite by ID.
data "guardicore_worksite" "by_id" {
  id = "00000000-0000-0000-0000-000000000030"
}

# Look up a worksite by name.
data "guardicore_worksite" "headquarters" {
  name = "Headquarters"
}

output "worksite_details" {
  value = {
    id      = data.guardicore_worksite.headquarters.id
    name    = data.guardicore_worksite.headquarters.name
    comment = data.guardicore_worksite.headquarters.comment
  }
}

# Look up the system-managed "Default" worksite.
# The Default worksite is created by the platform and cannot be modified
# by Terraform.
data "guardicore_worksite" "default" {
  name = "Default"
}

output "worksite_system_managed" {
  value = data.guardicore_worksite.default.system_managed
}
