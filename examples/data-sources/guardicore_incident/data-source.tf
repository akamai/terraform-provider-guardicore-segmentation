
# Look up an incident by ID.
data "guardicore_incident" "example" {
  id = "4506a1ba-15d1-4d10-8299-5c10f34975cb"
}

output "incident_typed_fields" {
  value = {
    incident_type = data.guardicore_incident.example.incident_type
    severity      = data.guardicore_incident.example.severity
    start_time    = data.guardicore_incident.example.start_time
    end_time      = data.guardicore_incident.example.end_time
    ended         = data.guardicore_incident.example.ended
    source_ip     = data.guardicore_incident.example.source_ip
  }
}

output "incident_raw_json" {
  value = data.guardicore_incident.example.raw_json
}
