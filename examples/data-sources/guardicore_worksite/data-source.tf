
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
