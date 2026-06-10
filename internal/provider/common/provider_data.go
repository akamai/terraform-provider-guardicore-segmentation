package common

import (
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// LabelUpdateReq is the request type for batched label updates.
type LabelUpdateReq struct {
	ID    string
	Label *client.LabelUpdate
}

// PolicyRuleUpdateReq is the request type for batched policy rule updates.
type PolicyRuleUpdateReq struct {
	ID   string
	Spec map[string]any
}

// LabelGroupUpdateReq is the request type for batched label group updates.
type LabelGroupUpdateReq struct {
	ID         string
	LabelGroup *client.LabelGroupCreate
}

// UserGroupUpdateReq is the request type for batched user group updates.
type UserGroupUpdateReq struct {
	ID        string
	UserGroup *client.UserGroupCreate
}

// DnsSecurityUpdateReq is the request type for batched DNS security updates.
type DnsSecurityUpdateReq struct {
	ID   string
	Edit *client.DnsBlocklistEdit
}

// ProviderData wraps the API client and provider-level configuration
// that resources and data sources receive via Configure.
type ProviderData struct {
	Client          *client.Client
	RuntimeSettings client.RuntimeSettings

	AssetLabelIgnoreCache *AssetLabelIgnoreCache

	LabelCreateBatcher *Batcher[*client.LabelCreate, string]
	LabelUpdateBatcher *Batcher[LabelUpdateReq, struct{}]
	LabelDeleteBatcher *Batcher[string, struct{}]

	PolicyRuleCreateBatcher *Batcher[map[string]any, string]
	PolicyRuleUpdateBatcher *Batcher[PolicyRuleUpdateReq, struct{}]
	PolicyRuleDeleteBatcher *Batcher[string, struct{}]

	LabelGroupCreateBatcher *Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate]
	LabelGroupUpdateBatcher *Batcher[LabelGroupUpdateReq, *client.LabelGroupCreate]
	LabelGroupDeleteBatcher *Batcher[string, struct{}]

	UserGroupCreateBatcher *Batcher[*client.UserGroupCreate, string]
	UserGroupUpdateBatcher *Batcher[UserGroupUpdateReq, struct{}]
	UserGroupDeleteBatcher *Batcher[string, struct{}]

	AssetCreateBatcher *Batcher[*client.AssetCreate, string]
	AssetUpdateBatcher *Batcher[*client.AssetBulkUpdateItem, struct{}]
	AssetDeleteBatcher *Batcher[string, struct{}]

	DnsSecurityCreateBatcher *Batcher[*client.DnsBlocklistCreate, string]
	DnsSecurityUpdateBatcher *Batcher[DnsSecurityUpdateReq, struct{}]
	DnsSecurityDeleteBatcher *Batcher[string, struct{}]

	IncidentCreateBatcher *Batcher[*client.IncidentCreate, string]

	WorksiteDeleteBatcher *Batcher[string, struct{}]

	ValidateRefsOnDestroy bool
	StrictRefsOnDestroy   bool
}

// ProviderDataFromRequest extracts *ProviderData from a resource/data source
// Configure request payload and appends a diagnostic on type mismatch.
func ProviderDataFromRequest(req any, diags *diag.Diagnostics) *ProviderData {
	if req == nil {
		return nil
	}

	providerData, ok := req.(*ProviderData)
	if !ok {
		diags.AddError(
			"Unexpected Configure Type",
			fmt.Sprintf("Expected *common.ProviderData, got: %T. Please report this issue to the provider developers.", req),
		)
		return nil
	}

	return providerData
}
