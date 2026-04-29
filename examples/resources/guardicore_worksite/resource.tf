
resource "guardicore_worksite" "headquarters" {
  name    = "Headquarters"
  comment = "Primary corporate campus"
}

resource "guardicore_worksite" "disaster_recovery" {
  name    = "Disaster Recovery"
  comment = "Secondary site used for failover testing"
}
