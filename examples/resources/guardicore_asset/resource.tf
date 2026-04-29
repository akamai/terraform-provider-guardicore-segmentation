
# Asset example covering all configurable attributes.
resource "guardicore_asset" "database_primary" {
  name                 = "db-primary-01"
  orchestration_obj_id = "cmdb-db-primary-01"
  status               = "on"
  comments             = "Primary production database server"
  worksite_id          = guardicore_worksite.headquarters.id
  instance_id          = "i-0123456789abcdef0"
  hw_uuid              = "550e8400-e29b-41d4-a716-446655440000"
  bios_uuid            = "550e8400-e29b-41d4-a716-446655440001"

  nics = [
    {
      vif_id                 = "vif-001"
      mac_address            = "00:1A:2B:3C:4D:5E"
      network_id             = "net-prod-app"
      network_name           = "production-app"
      is_cloud_public        = false
      is_corporate_interface = true
      switch_id              = "switch-a1"
      ip_addresses           = ["10.0.2.10", "10.0.2.11"]
    },
    {
      vif_id                 = "vif-002"
      mac_address            = "00:1A:2B:3C:4D:5F"
      network_id             = "net-mgmt"
      network_name           = "management"
      is_cloud_public        = false
      is_corporate_interface = false
      switch_id              = "switch-m1"
      ip_addresses           = ["192.168.10.20"]
    },
  ]

  labels = [
    {
      id    = guardicore_label.environment_production.id
      key   = guardicore_label.environment_production.key
      value = guardicore_label.environment_production.value
    },
    {
      id    = guardicore_label.application_database.id
      key   = guardicore_label.application_database.key
      value = guardicore_label.application_database.value
    },
  ]

  orchestration_metadata_json = jsonencode({
    asset_type         = "Database"
    f5_device_hostname = "db-primary-f5.example.com"
    partition          = "Common"
    vs_name            = "db-primary-vs"
  })
}
