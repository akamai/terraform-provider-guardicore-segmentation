
# Look up a label by ID.
data "guardicore_label" "by_id" {
  id = "00000000-0000-0000-0000-000000000001"
}

# Look up a label by key and value.
data "guardicore_label" "environment_production" {
  key   = "Environment"
  value = "Production"
}

output "label_lookup_by_key_value" {
  value = {
    id    = data.guardicore_label.environment_production.id
    key   = data.guardicore_label.environment_production.key
    value = data.guardicore_label.environment_production.value
  }
}

output "label_criteria" {
  value = data.guardicore_label.by_id.criteria
}
