
# Look up a user group by ID.
data "guardicore_user_group" "by_id" {
  id = "00000000-0000-0000-0000-000000000040"
}

# Look up a user group by title.
data "guardicore_user_group" "platform_engineers" {
  title = "Platform Engineers"
}

output "user_group_details" {
  value = {
    id                    = data.guardicore_user_group.platform_engineers.id
    title                 = data.guardicore_user_group.platform_engineers.title
    orchestrations_groups = data.guardicore_user_group.platform_engineers.orchestrations_groups
  }
}
