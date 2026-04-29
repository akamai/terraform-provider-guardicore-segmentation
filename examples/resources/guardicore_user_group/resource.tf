
resource "guardicore_user_group" "platform_engineers" {
  title = "Platform Engineers"

  orchestrations_groups = [
    {
      orchestration_id = "ad-orchestration-primary"
      groups = [
        "CN=Platform-Engineers,OU=Groups,DC=example,DC=com",
        "CN=Ops-OnCall,OU=Groups,DC=example,DC=com",
      ]
    },
    {
      orchestration_id = "ad-orchestration-secondary"
      groups = [
        "CN=Cloud-Operations,OU=Groups,DC=corp,DC=example",
      ]
    },
  ]
}
