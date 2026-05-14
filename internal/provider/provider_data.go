package provider

import "github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"

// ProviderData wraps the API client and provider-level configuration
// that resources and data sources receive via Configure.
type ProviderData struct {
	Client          *client.Client
	RuntimeSettings client.RuntimeSettings

	AssetLabelIgnoreCache *assetLabelIgnoreCache

	LabelCreateBatcher *Batcher[*client.LabelCreate, string]
	LabelUpdateBatcher *Batcher[labelUpdateReq, struct{}]
	LabelDeleteBatcher *Batcher[string, struct{}]

	PolicyRuleCreateBatcher *Batcher[map[string]any, string]
	PolicyRuleUpdateBatcher *Batcher[policyRuleUpdateReq, struct{}]
	PolicyRuleDeleteBatcher *Batcher[string, struct{}]

	LabelGroupCreateBatcher *Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate]
	LabelGroupUpdateBatcher *Batcher[labelGroupUpdateReq, *client.LabelGroupCreate]
	LabelGroupDeleteBatcher *Batcher[string, struct{}]

	UserGroupCreateBatcher *Batcher[*client.UserGroupCreate, string]
	UserGroupUpdateBatcher *Batcher[userGroupUpdateReq, struct{}]
	UserGroupDeleteBatcher *Batcher[string, struct{}]

	AssetCreateBatcher *Batcher[*client.AssetCreate, string]
	AssetUpdateBatcher *Batcher[*client.AssetBulkUpdateItem, struct{}]
	AssetDeleteBatcher *Batcher[string, struct{}]

	DnsSecurityCreateBatcher *Batcher[*client.DnsBlocklistCreate, string]
	DnsSecurityUpdateBatcher *Batcher[dnsSecurityUpdateReq, struct{}]
	DnsSecurityDeleteBatcher *Batcher[string, struct{}]

	IncidentCreateBatcher *Batcher[*client.IncidentCreate, string]

	WorksiteDeleteBatcher *Batcher[string, struct{}]

	ValidateRefsOnDestroy bool
	StrictRefsOnDestroy   bool
}
