package client

import (
	"encoding/json"
	"strings"
)

// LabelCreate is used for create/update requests (uses "criteria" field).
type LabelCreate struct {
	ID       string          `json:"id,omitempty"`
	Key      string          `json:"key"`
	Value    string          `json:"value"`
	Criteria []LabelCriteria `json:"criteria"`
}

// LabelUpdate is used for update requests.
type LabelUpdate struct {
	Key      string          `json:"key"`
	Value    string          `json:"value"`
	Criteria []LabelCriteria `json:"criteria,omitempty"`
}

// Label is used for reading labels from the API (uses "dynamic_criteria" field).
type Label struct {
	ID              string          `json:"id,omitempty"`
	Key             string          `json:"key"`
	Value           string          `json:"value"`
	Origin          *string         `json:"origin,omitempty"`
	ReadOnly        *bool           `json:"read_only,omitempty"`
	DynamicCriteria []LabelCriteria `json:"dynamic_criteria,omitempty"`
	StaticCriteria  []LabelCriteria `json:"static_criteria,omitempty"`
}

// LabelCriteria represents criteria for dynamic label assignment.
type LabelCriteria struct {
	ID               string          `json:"id,omitempty"`
	Field            string          `json:"field"`
	Op               string          `json:"op"`
	Argument         string          `json:"argument"`
	CompoundCriteria []LabelCriteria `json:"compound_criteria,omitempty"`
	Source           *string         `json:"source,omitempty"`
	ReadOnly         *bool           `json:"read_only,omitempty"`
}

// LabelDynamicCompoundCriterion is an inner member of a compound criterion.
// Inner members do not have IDs.
type LabelDynamicCompoundCriterion struct {
	Field    string `json:"field"`
	Op       string `json:"op"`
	Argument string `json:"argument"`
}

// LabelDynamicCriterionChange is a top-level dynamic criterion change item.
// A criterion is either flat (field/op/argument) or compound_criteria.
type LabelDynamicCriterionChange struct {
	ID               string                          `json:"id"`
	Source           string                          `json:"source"`
	Field            string                          `json:"field,omitempty"`
	Op               string                          `json:"op,omitempty"`
	Argument         string                          `json:"argument,omitempty"`
	CompoundCriteria []LabelDynamicCompoundCriterion `json:"compound_criteria,omitempty"`
}

// LabelDynamicCriteriaChangesRequest is the payload for applying add/modify/delete
// changes to an existing label's dynamic criteria.
type LabelDynamicCriteriaChangesRequest struct {
	Added    []LabelDynamicCriterionChange `json:"added"`
	Modified []LabelDynamicCriterionChange `json:"modified"`
	Deleted  []string                      `json:"deleted"`
}

// IsReadOnlyWorksiteGenerated reports whether this criterion is API-generated
// for Worksite scoping and therefore not user-manageable.
func (c LabelCriteria) IsReadOnlyWorksiteGenerated() bool {
	if c.ReadOnly != nil && *c.ReadOnly && c.Source != nil && strings.EqualFold(*c.Source, "Worksite") {
		return true
	}

	return strings.EqualFold(c.Field, "scoping_details.worksite.id")
}

// LabelBulkResponse represents bulk label create/update/delete operation results.
type LabelBulkResponse struct {
	Result    string   `json:"result"`
	Succeeded []string `json:"succeeded"`
	Failed    []string `json:"failed"`
	Missing   []string `json:"missing"`
}

// LabelBulkDeleteItem represents a single item in a bulk label delete request.
type LabelBulkDeleteItem struct {
	ID string `json:"id"`
}

// LabelGroupCreate is used for create/update requests (uses string IDs for labels).
type LabelGroupCreate struct {
	ID            string          `json:"id,omitempty"`
	Key           string          `json:"key"`
	Value         string          `json:"value"`
	Comments      string          `json:"comments"`
	IncludeLabels *OrLabelsCreate `json:"include_labels,omitempty"`
	ExcludeLabels *OrLabelsCreate `json:"exclude_labels,omitempty"`
}

// LabelGroup is used for reading label groups from the API (contains full label objects).
type LabelGroup struct {
	ID            string        `json:"id,omitempty"`
	Key           string        `json:"key"`
	Value         string        `json:"value"`
	Comments      string        `json:"comments,omitempty"`
	IncludeLabels *OrLabelsRead `json:"include_labels,omitempty"`
	ExcludeLabels *OrLabelsRead `json:"exclude_labels,omitempty"`
}

// OrLabelsCreate represents OR logic for label matching in create/update requests.
type OrLabelsCreate struct {
	OrLabels []AndLabelsCreate `json:"or_labels"`
}

// OrLabelsRead represents OR logic for label matching in read responses.
type OrLabelsRead struct {
	OrLabels []AndLabelsRead `json:"or_labels"`
}

// AndLabelsCreate represents AND logic for label matching in create/update requests.
type AndLabelsCreate struct {
	AndLabels []string `json:"and_labels"`
}

// AndLabelsRead represents AND logic for label matching in read responses.
type AndLabelsRead struct {
	AndLabels []LabelInGroup `json:"and_labels"`
}

// LabelInGroup represents a label within a group response.
type LabelInGroup struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	Name       string `json:"name,omitempty"`
	ColorIndex int    `json:"color_index,omitempty"`
}

// PolicyRule represents an Akamai Guardicore Segmentation policy rule (stored as raw JSON due to complexity).
type PolicyRule struct {
	ID   string                 `json:"id,omitempty"`
	Spec map[string]interface{} `json:"-"`
}

// AuthRequest represents authentication request body.
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents authentication response.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// CreateResponse represents a generic create response with ID.
type CreateResponse struct {
	ID string `json:"id"`
}

// LabelGetResponse wraps the label GET response (single item returns wrapped).
type LabelGetResponse struct {
	Objects []Label `json:"objects"`
}

// LabelGroupGetResponse wraps the label group GET response (single item returns wrapped).
type LabelGroupGetResponse struct {
	Objects []LabelGroup `json:"objects"`
}

// PolicyRuleGetResponse wraps the policy rule GET response (single item returns wrapped).
type PolicyRuleGetResponse struct {
	Objects []map[string]interface{} `json:"objects"`
}

// PolicyRulesBulkCreateResponse represents the response from bulk policy rule create.
type PolicyRulesBulkCreateResponse struct {
	NumberOfFailed    int      `json:"number_of_failed"`
	NumberOfSucceeded int      `json:"number_of_succeeded"`
	Result            string   `json:"result"`
	Succeeded         []string `json:"succeeded"`
	TotalNumber       int      `json:"total_number"`
}

// PolicyRuleBulkUpdateItem represents a single item in a bulk policy rule update request.
type PolicyRuleBulkUpdateItem struct {
	ID   string         `json:"id"`
	Rule map[string]any `json:"rule"`
}

// PolicyRuleBulkDeleteItem represents a single item in a bulk policy rule delete request.
type PolicyRuleBulkDeleteItem struct {
	ID string `json:"id"`
}

// PolicyRevisionRequest represents a policy revision publish request.
type PolicyRevisionRequest struct {
	Comments                string   `json:"comments"`
	RulesetName             *string  `json:"ruleset_name,omitempty"`
	Rulesets                []string `json:"rulesets,omitempty"`
	ResetHitCount           bool     `json:"reset_hit_count,omitempty"`
	ResetHitCountForRuleset bool     `json:"reset_hit_count_for_ruleset,omitempty"`
	Origin                  *string  `json:"origin,omitempty"`
}

// UserGroupRevisionRequest represents a user group revision publish request.
type UserGroupRevisionRequest struct {
	Comments string `json:"comments"`
}

// ListLabelsResponse represents the response from listing labels.
type ListLabelsResponse struct {
	Objects    []Label `json:"objects"`
	TotalCount int     `json:"total_count"`
}

// ListLabelGroupsResponse represents the response from listing label groups.
type ListLabelGroupsResponse struct {
	Objects    []LabelGroup `json:"objects"`
	TotalCount int          `json:"total_count"`
}

// ListPolicyRulesResponse represents the response from listing policy rules.
type ListPolicyRulesResponse struct {
	Objects    []map[string]interface{} `json:"objects"`
	TotalCount int                      `json:"total_count"`
}

// DnsBlocklistCreate is used for create requests.
type DnsBlocklistCreate struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Domains []string `json:"domains,omitempty"`
}

// DnsBlocklistEdit is used for PATCH update requests (partial update).
type DnsBlocklistEdit struct {
	Name    *string  `json:"name,omitempty"`
	Domains []string `json:"domains,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// DnsBlocklist is used for reading DNS blocklists from the API.
type DnsBlocklist struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Domains []string `json:"domains,omitempty"`
	Enabled bool     `json:"enabled"`
}

// ListDnsBlocklistsResponse represents the response from listing DNS blocklists.
type ListDnsBlocklistsResponse struct {
	Objects    []DnsBlocklist `json:"objects"`
	TotalCount int            `json:"total"`
}

// BulkCreateDnsBlocklistRequest is used for bulk create requests.
type BulkCreateDnsBlocklistRequest struct {
	Items []DnsBlocklistCreate `json:"items"`
}

// BulkCreateDnsBlocklistResponse represents the bulk create response.
type BulkCreateDnsBlocklistResponse struct {
	IDs []string `json:"ids"`
}

// BulkEditDnsBlocklistItem represents a single item in a bulk edit request.
type BulkEditDnsBlocklistItem struct {
	ID      string   `json:"id"`
	Name    *string  `json:"name,omitempty"`
	Domains []string `json:"domains,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// BulkEditDnsBlocklistRequest is used for bulk edit requests.
type BulkEditDnsBlocklistRequest struct {
	Items []BulkEditDnsBlocklistItem `json:"items"`
}

// BulkEditDnsBlocklistResponse represents the bulk edit response.
type BulkEditDnsBlocklistResponse struct {
	IDs []string `json:"ids"`
}

// IncidentCreate is used for create requests to POST /api/v4.0/incidents.
type IncidentCreate struct {
	Type                 string          `json:"type"`
	Severity             string          `json:"severity"`
	AffectedAssets       json.RawMessage `json:"affected_assets"`
	Time                 int64           `json:"time"`
	Tags                 []string        `json:"tags"`
	Description          string          `json:"description"`
	Summary              string          `json:"summary"`
	Origin               *string         `json:"origin,omitempty"`
	Mitigation           *string         `json:"mitigation,omitempty"`
	CefExtensions        json.RawMessage `json:"cef_extensions,omitempty"`
	AttachedFiles        []string        `json:"attached_files,omitempty"`
	MapDetails           json.RawMessage `json:"map_details,omitempty"`
	CustomDefinedObjects json.RawMessage `json:"custom_defined_objects,omitempty"`
	Properties           json.RawMessage `json:"properties,omitempty"`
}

// CreateIncidentResponse represents the response from creating an incident.
// NOTE: Uses incident_id, not id (unlike other resources).
type CreateIncidentResponse struct {
	IncidentID string `json:"incident_id"`
}

// BulkCreateIncidentRequest is used for bulk create requests.
// NOTE: Uses "incidents" key, not "items" (unlike DNS blocklists).
type BulkCreateIncidentRequest struct {
	Incidents []IncidentCreate `json:"incidents"`
}

// BulkCreateIncidentResponse represents the bulk create response.
// NOTE: Uses incident_ids, not ids (unlike other resources).
type BulkCreateIncidentResponse struct {
	IncidentIDs []string `json:"incident_ids"`
}

// ListIncidentsResponse represents the response from listing incidents.
// Uses map[string]interface{} because the read schema differs significantly from create.
type ListIncidentsResponse struct {
	Objects    []map[string]interface{} `json:"objects"`
	TotalCount int                      `json:"total_count"`
}

// WorksiteCreate is used for POST create requests.
type WorksiteCreate struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// WorksiteUpdate is used for PUT update requests (id in body, not URL).
type WorksiteUpdate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// Worksite is used for reading worksites from the API.
type Worksite struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
}

// ListWorksitesResponse represents the response from listing worksites.
type ListWorksitesResponse struct {
	Objects    []Worksite `json:"objects"`
	TotalCount int        `json:"total_count"`
}

// DeleteWorksitesRequest is used for bulk delete via POST /worksites/delete_worksites.
type DeleteWorksitesRequest struct {
	ComponentIDs []string               `json:"component_ids"`
	NegateArgs   *DeleteWorksitesNegate `json:"negate_args"`
}

// DeleteWorksitesNegate is the negate_args object required by the bulk delete endpoint.
type DeleteWorksitesNegate struct {
	Unselected []any          `json:"unselected"`
	Filters    map[string]any `json:"filters"`
	Quantity   int            `json:"quantity"`
}

// DeleteWorksitesResponse represents the response from POST /worksites/delete_worksites.
// The API can return HTTP 200 with per-item skip/failure counts when deletions are blocked.
type DeleteWorksitesResponse struct {
	AssignedDetails   string `json:"assigned_details"`
	AssignedWorksites int    `json:"assigned_worksites"`
	Details           string `json:"details"`
	Failures          int    `json:"failures"`
	Skips             int    `json:"skips"`
	Successes         int    `json:"successes"`
}

// OrchestrationGroup represents a single orchestration with its AD groups.
type OrchestrationGroup struct {
	OrchestrationID string   `json:"orchestration_id"`
	Groups          []string `json:"groups"`
}

// UserGroupCreate is used for POST create and PUT update requests.
type UserGroupCreate struct {
	Title                string               `json:"title"`
	OrchestrationsGroups []OrchestrationGroup `json:"orchestrations_groups"`
}

// DomainGroup represents a single group within a domain in the list response.
type DomainGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DomainGroupInfo represents orchestration information for a domain in the list response.
// The list API returns groups_by_domain_name with this structure instead of orchestrations_groups.
type DomainGroupInfo struct {
	Groups          []DomainGroup `json:"groups"`
	OrchestrationID string        `json:"orchestration_id"`
}

// UserGroup is used for reading user groups from the API.
type UserGroup struct {
	ID                   string                     `json:"id"`
	Title                string                     `json:"title"`
	OrchestrationsGroups []OrchestrationGroup       `json:"orchestrations_groups"`
	GroupsByDomainName   map[string]DomainGroupInfo `json:"groups_by_domain_name,omitempty"`
}

// ListUserGroupsResponse represents the response from listing user groups.
type ListUserGroupsResponse struct {
	Objects    []UserGroup `json:"objects"`
	TotalCount int         `json:"total_count"`
}

// AssetNIC represents a network interface on an asset.
type AssetNIC struct {
	VifID                string   `json:"vif_id,omitempty"`
	MacAddress           string   `json:"mac_address,omitempty"`
	NetworkID            string   `json:"network_id,omitempty"`
	NetworkName          string   `json:"network_name,omitempty"`
	IsCloudPublic        *bool    `json:"is_cloud_public,omitempty"`
	IsCorporateInterface *bool    `json:"is_corporate_interface,omitempty"`
	SwitchID             string   `json:"switch_id,omitempty"`
	IPAddresses          []string `json:"ip_addresses"`
}

// AssetLabelRef represents a label reference on an asset.
type AssetLabelRef struct {
	ID       string  `json:"id,omitempty"`
	Key      string  `json:"key,omitempty"`
	Value    string  `json:"value,omitempty"`
	Origin   *string `json:"origin,omitempty"`
	ReadOnly *bool   `json:"read_only,omitempty"`
}

// AssetCreate is used for POST /api/v4.0/assets/bulk create requests.
type AssetCreate struct {
	Name                  string           `json:"name"`
	Nics                  []AssetNIC       `json:"nics"`
	OrchestrationObjID    string           `json:"orchestration_obj_id"`
	Status                string           `json:"status,omitempty"`
	Labels                *[]AssetLabelRef `json:"labels,omitempty"`
	Comments              string           `json:"comments,omitempty"`
	OrchestrationMetadata json.RawMessage  `json:"orchestration_metadata,omitempty"`
	Worksite              *string          `json:"worksite,omitempty"`
	InstanceID            string           `json:"instance_id,omitempty"`
	HwUUID                string           `json:"hw_uuid,omitempty"`
	BiosUUID              string           `json:"bios_uuid,omitempty"`
}

// OrchestrationDetail represents an entry in the orchestration_details array returned by the API.
type OrchestrationDetail struct {
	OrchestrationID    string `json:"orchestration_id"`
	OrchestrationName  string `json:"orchestration_name"`
	OrchestrationObjID string `json:"orchestration_obj_id"`
	OrchestrationType  string `json:"orchestration_type"`
	RevisionID         any    `json:"revision_id,omitempty"`
}

// AssetScopingDetails represents the scoping_details object in asset read responses.
type AssetScopingDetails struct {
	Worksite *AssetWorksiteInfo `json:"worksite,omitempty"`
}

// AssetWorksiteInfo represents the worksite object within scoping_details.
type AssetWorksiteInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Modified int64  `json:"modified,omitempty"`
}

// Asset is used for reading assets from the API.
type Asset struct {
	ID                    string                `json:"id"`
	Name                  string                `json:"name"`
	Status                string                `json:"status,omitempty"`
	FirstSeen             any                   `json:"first_seen,omitempty"`
	LastSeen              any                   `json:"last_seen,omitempty"`
	Nics                  []AssetNIC            `json:"nics,omitempty"`
	BiosUUID              string                `json:"bios_uuid,omitempty"`
	Labels                []AssetLabelRef       `json:"labels,omitempty"`
	Comments              string                `json:"comments,omitempty"`
	OrchestrationMetadata json.RawMessage       `json:"orchestration_metadata,omitempty"`
	OrchestrationObjID    string                `json:"orchestration_obj_id,omitempty"`
	OrchestrationDetails  []OrchestrationDetail `json:"orchestration_details,omitempty"`
	InstanceID            string                `json:"instance_id,omitempty"`
	HwUUID                string                `json:"hw_uuid,omitempty"`
	ScopingDetails        *AssetScopingDetails  `json:"scoping_details,omitempty"`
}

// ListAssetsResponse represents the response from listing assets.
// The assets API returns "total" (not "total_count" like labels/label groups).
type ListAssetsResponse struct {
	Objects    []Asset `json:"objects"`
	TotalCount int     `json:"total"`
}

// AssetBulkUpdateItem represents a single item in a bulk update request.
type AssetBulkUpdateItem struct {
	AssetID               string           `json:"asset_id"`
	Name                  string           `json:"name,omitempty"`
	Nics                  []AssetNIC       `json:"nics,omitempty"`
	Status                string           `json:"status,omitempty"`
	Labels                *[]AssetLabelRef `json:"labels,omitempty"`
	Comments              string           `json:"comments,omitempty"`
	OrchestrationMetadata json.RawMessage  `json:"orchestration_metadata,omitempty"`
	Worksite              *string          `json:"worksite,omitempty"`
}

// BulkDeactivateAssetItem represents a single item in a bulk deactivate request.
type BulkDeactivateAssetItem struct {
	AssetID string `json:"asset_id"`
}

// BulkAssetError represents an individual error entry in a bulk asset response.
type BulkAssetError struct {
	Error              string `json:"error"`
	OrchestrationObjID string `json:"orchestration_obj_id"`
	AssetID            string `json:"asset_id"`
}

// BulkCreateAssetsResponse represents the response from POST /api/v4.0/assets/bulk.
type BulkCreateAssetsResponse struct {
	NumberOfSucceeded int               `json:"number_of_succeeded"`
	NumberOfFailed    int               `json:"number_of_failed"`
	TotalNumber       int               `json:"total_number"`
	Errors            []BulkAssetError  `json:"errors"`
	CreatedAssetIDs   map[string]string `json:"created_asset_ids"`
}

// BulkUpdateAssetsResponse represents the response from PUT /api/v4.0/assets/bulk.
type BulkUpdateAssetsResponse struct {
	NumberOfSucceeded int              `json:"number_of_succeeded"`
	NumberOfFailed    int              `json:"number_of_failed"`
	TotalNumber       int              `json:"total_number"`
	Errors            []BulkAssetError `json:"errors"`
}

// WorksiteAssignRequest is used for POST /api/v4.0/worksites/assign to assign entities to a worksite.
type WorksiteAssignRequest struct {
	ID         string   `json:"id"`
	EntityType string   `json:"entity_type"`
	EntityIDs  []string `json:"entity_ids"`
}

// PolicyRuleBulkWorksiteMoveRequest is used for POST /api/v3.0/visibility/policy/rules-bulk/worksite/move/{worksite_id}.
type PolicyRuleBulkWorksiteMoveRequest struct {
	IDs        []string                          `json:"ids"`
	NegateArgs *PolicyRuleBulkWorksiteMoveNegate `json:"negate_args"`
}

// PolicyRuleBulkWorksiteMoveNegate represents the negate_args for the bulk worksite move endpoint.
type PolicyRuleBulkWorksiteMoveNegate struct {
	Filters map[string]any `json:"filters"`
}

// PolicyRuleBulkWorksiteMoveResponse represents the response from the bulk worksite move endpoint.
type PolicyRuleBulkWorksiteMoveResponse struct {
	InsertedCount int `json:"inserted_count"`
	ModifiedCount int `json:"modified_count"`
	Worksite      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"worksite"`
}

// PolicyGroupCreate is used for POST create requests to /api/v4.0/policy-groups.
type PolicyGroupCreate struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"` // LABEL, FQDN, IP_ADDRESS
	Comments       string          `json:"comments,omitempty"`
	IncludeMembers json.RawMessage `json:"include_members"`
	ExcludeMembers json.RawMessage `json:"exclude_members,omitempty"` // LABEL only
}

// PolicyGroup is used for reading policy groups from the API.
type PolicyGroup struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Type                string          `json:"type"`
	Comments            string          `json:"comments,omitempty"`
	IncludeMembers      json.RawMessage `json:"include_members"`
	ExcludeMembers      json.RawMessage `json:"exclude_members,omitempty"`
	State               string          `json:"state,omitempty"`                 // Not stored in TF state
	AuthorID            string          `json:"author_id,omitempty"`             // Not stored in TF state
	CreationTime        int64           `json:"creation_time,omitempty"`         // Not stored in TF state
	LastChangeTime      int64           `json:"last_change_time,omitempty"`      // Not stored in TF state
	MatchingRulesCount  *int            `json:"matching_rules_count,omitempty"`  // Not stored in TF state
	MatchingAssetsCount *int            `json:"matching_assets_count,omitempty"` // Not stored in TF state
}

// ListPolicyGroupsResponse represents the response from listing policy groups.
type ListPolicyGroupsResponse struct {
	Objects    []PolicyGroup `json:"objects"`
	TotalCount int           `json:"total_count"`
}

// Agent Aggregator models (read-only)

type AgentAggregatorFullVersion struct {
	Major string `json:"major"`
	Minor string `json:"minor"`
	Tag   string `json:"tag"`
}

type AgentAggregatorInterface struct {
	Interface string `json:"interface"`
	IPAddress string `json:"ip_address"`
	Netmask   string `json:"netmask"`
}

type AgentAggregator struct {
	ID                          string                      `json:"id"`
	InternalID                  string                      `json:"_id"`
	Cls                         string                      `json:"_cls"`
	ComponentID                 string                      `json:"component_id"`
	AgentID                     string                      `json:"agent_id"`
	Version                     string                      `json:"version"`
	FullVersion                 *AgentAggregatorFullVersion `json:"full_version,omitempty"`
	BuildCommit                 string                      `json:"build_commit"`
	BuildDate                   any                         `json:"build_date,omitempty"`
	InstallDate                 any                         `json:"install_date,omitempty"`
	IPAddress                   string                      `json:"ip_address"`
	Hostname                    string                      `json:"hostname"`
	Interfaces                  []AgentAggregatorInterface  `json:"interfaces,omitempty"`
	FirstSeen                   any                         `json:"first_seen,omitempty"`
	AssociatedMgmtConfiguration json.RawMessage             `json:"associated_mgmt_configuration,omitempty"`
	DocVersion                  int                         `json:"doc_version"`
	LastSeen                    any                         `json:"last_seen,omitempty"`
	DisplayStatus               string                      `json:"display_status"`
	IsMissing                   bool                        `json:"is_missing"`
	State                       string                      `json:"state"`
	AggregatorType              string                      `json:"aggregator_type"`
	AggregatorFeatures          []string                    `json:"aggregator_features,omitempty"`
	ClusterID                   string                      `json:"cluster_id"`
	ZookeeperID                 int                         `json:"zookeeper_id"`
	SubComponents               []string                    `json:"sub_components,omitempty"`
	NetworkDevices              json.RawMessage             `json:"network_devices,omitempty"`
	IntegrationSDKCapabilities  []string                    `json:"integration_sdk_capabilities,omitempty"`
	ManagementHosts             []string                    `json:"management_hosts,omitempty"`
	ExternalFQDNAddresses       []string                    `json:"external_fqdn_addresses,omitempty"`
	AggrCertSerialNumber        string                      `json:"aggr_cert_serial_number"`
	CollectorType               *string                     `json:"collector_type"`
	LegacyComponentID           string                      `json:"component-id"`
	EnforcementID               string                      `json:"enforcement-id"`
	EventletVersion             string                      `json:"eventlet-version"`
	ExternalAddress             string                      `json:"external-address"`
	HostIPs                     []string                    `json:"host_ips,omitempty"`
	InternalAddress             string                      `json:"internal-address"`
	ManagementHost              string                      `json:"management_host"`
	SystemUptime                string                      `json:"system-uptime"`
	GuestInstallationDetails    json.RawMessage             `json:"guest_installation_details,omitempty"`
	MitigationID                string                      `json:"mitigation-id"`
	TenantName                  *string                     `json:"tenant_name"`
	IsConfigurationDirty        bool                        `json:"is_configuration_dirty"`
}

type ListAgentAggregatorsResponse struct {
	Objects    []AgentAggregator `json:"objects"`
	TotalCount int               `json:"total_count"`
}
