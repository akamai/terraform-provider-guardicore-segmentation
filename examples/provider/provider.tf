
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

# Example 5: Use environment variables instead of explicit configuration
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
