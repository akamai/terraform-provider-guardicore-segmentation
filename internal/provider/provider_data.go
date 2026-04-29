package provider

import "github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"

// ProviderData wraps the API client and provider-level configuration
// that resources and data sources receive via Configure.
type ProviderData struct {
	Client                *client.Client
	PolicyRuleBatcher     *PolicyRuleCreateBatcher
	ValidateRefsOnDestroy bool
	StrictRefsOnDestroy   bool
}
