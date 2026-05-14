
resource "guardicore_worksite" "headquarters" {
  name    = "Headquarters"
  comment = "Primary corporate campus"
}

resource "guardicore_worksite" "disaster_recovery" {
  name    = "Disaster Recovery"
  comment = "Secondary site used for failover testing"
}

# The "Default" worksite is system-managed and cannot be modified by Terraform.
# Use the data source to reference it:
#
#   data "guardicore_worksite" "default" {
#     name = "Default"
#   }
#
# Then reference it as: data.guardicore_worksite.default.id
