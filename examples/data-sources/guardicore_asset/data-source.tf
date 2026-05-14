
# Look up an asset by ID.
data "guardicore_asset" "by_id" {
  id = "00000000-0000-0000-0000-000000000050"
}

# Look up an asset by name.
data "guardicore_asset" "database_primary" {
  name = "db-primary-01"
}

output "asset_identity" {
  value = {
    id                   = data.guardicore_asset.database_primary.id
    name                 = data.guardicore_asset.database_primary.name
    orchestration_obj_id = data.guardicore_asset.database_primary.orchestration_obj_id
    status               = data.guardicore_asset.database_primary.status
    worksite_id          = data.guardicore_asset.database_primary.worksite_id
    instance_id          = data.guardicore_asset.database_primary.instance_id
    hw_uuid              = data.guardicore_asset.database_primary.hw_uuid
    bios_uuid            = data.guardicore_asset.database_primary.bios_uuid
  }
}

output "asset_inventory_details" {
  value = {
    nics                   = data.guardicore_asset.database_primary.nics
    labels                 = data.guardicore_asset.database_primary.labels
    comments               = data.guardicore_asset.database_primary.comments
    orchestration_metadata = data.guardicore_asset.database_primary.orchestration_metadata
    first_seen             = data.guardicore_asset.database_primary.first_seen
    last_seen              = data.guardicore_asset.database_primary.last_seen
  }
}
