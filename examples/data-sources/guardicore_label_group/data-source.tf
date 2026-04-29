
# Look up a label group by ID.
data "guardicore_label_group" "by_id" {
  id = "00000000-0000-0000-0000-000000000010"
}

# Look up a label group by key and value.
data "guardicore_label_group" "production_web" {
  key   = "Server Group"
  value = "Production Web"
}

output "label_group_typed_selectors" {
  value = {
    include = data.guardicore_label_group.production_web.include
    exclude = data.guardicore_label_group.production_web.exclude
  }
}

output "label_group_json_selectors" {
  value = {
    include_json = data.guardicore_label_group.production_web.include_json
    exclude_json = data.guardicore_label_group.production_web.exclude_json
  }
}
