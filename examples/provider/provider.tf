
# Example 1: Username and password authentication
provider "guardicore" {
  base_url = "https://guardicore.example.com"
  username = var.guardicore_username
  password = var.guardicore_password
}

# Example 2: Access token authentication for automation
# provider "guardicore" {
#   base_url     = "https://guardicore.example.com"
#   access_token = var.guardicore_access_token
# }

# Example 3: Refresh token authentication
# provider "guardicore" {
#   base_url      = "https://guardicore.example.com"
#   refresh_token = var.guardicore_refresh_token
# }

# Example 4: Advanced provider configuration
# provider "guardicore" {
#   base_url                       = "https://guardicore-dev.example.com"
#   access_token                   = var.guardicore_access_token
#   insecure_skip_verify           = true
#   request_timeout                = 120
#   validate_references_on_destroy = true
#   strict_references_on_destroy   = true
# }

# Example 5: Custom publish comment
#
# Creating, updating or deleting policy rules or user groups automatically
# publishes a revision. By default the comment reads "Published via Terraform".
# Set publish_comment to tie each publication to a change request, deployment
# reference or pipeline run, so the change is traceable in the Guardicore
# publication history.
#
# provider "guardicore" {
#   base_url        = "https://guardicore.example.com"
#   access_token    = var.guardicore_access_token
#   publish_comment = "CHG0045231 - add application bubble RTEST4"
# }
#
# In CI, supply it from the environment so the pipeline injects its own run
# identifier without editing configuration:
#
#   export GUARDICORE_PUBLISH_COMMENT="terraform apply - pipeline #${CI_RUN_ID}"
#
# The comment applies to every revision published during the run. It is not a
# per-resource setting, because one revision can cover many resources.

# Example 6: Use environment variables instead of explicit configuration
# provider "guardicore" {}
#
# Supported environment variables:
# - GUARDICORE_BASE_URL
# - GUARDICORE_USERNAME
# - GUARDICORE_PASSWORD
# - GUARDICORE_ACCESS_TOKEN
# - GUARDICORE_REFRESH_TOKEN
# - GUARDICORE_INSECURE_SKIP_VERIFY
# - GUARDICORE_REQUEST_TIMEOUT
# - GUARDICORE_PUBLISH_COMMENT

variable "guardicore_username" {
  description = "Username for Akamai Guardicore Segmentation authentication"
  type        = string
  sensitive   = true
  default     = null
}

variable "guardicore_password" {
  description = "Password for Akamai Guardicore Segmentation authentication"
  type        = string
  sensitive   = true
  default     = null
}

variable "guardicore_access_token" {
  description = "Access token for Akamai Guardicore Segmentation authentication"
  type        = string
  sensitive   = true
  default     = null
}

variable "guardicore_refresh_token" {
  description = "Refresh token for Akamai Guardicore Segmentation authentication"
  type        = string
  sensitive   = true
  default     = null
}
