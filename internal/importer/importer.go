package importer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
)

// orchObjIDPlaceholder is used when orchestration_obj_id cannot be determined from the API response.
// The field is create-only and may not appear in read responses for assets without orchestration details.
const orchObjIDPlaceholder = "REPLACE_ME_orchestration_obj_id_not_returned_by_api"

// Importer fetches all resources from Akamai Guardicore Segmentation and generates .tf files.
type Importer struct {
	Client    *client.Client
	OutputDir string
}

// Result contains a summary of the import operation.
type Result struct {
	Labels           int
	LabelGroups      int
	PolicyRules      int
	DnsBlocklists    int
	Incidents        int
	Worksites        int
	UserGroups       int
	Assets           int
	AgentAggregators int
}

// ResourceLookup maps API IDs to Terraform resource reference expressions.
// Values are HCL expressions like "guardicore_label.env_prod.id" for managed
// resources, or "data.guardicore_label.env_prod.id" for system-managed data sources.
type ResourceLookup struct {
	Labels       map[string]string // API ID → "guardicore_label.<name>.id" or "data.guardicore_label.<name>.id"
	LabelGroups  map[string]string // API ID → "guardicore_label_group.<name>.id"
	PolicyGroups map[string]string // API ID → "guardicore_policy_group.<name>.id"
	UserGroups   map[string]string // API ID → "guardicore_user_group.<name>.id" or "data.guardicore_user_group.<name>.id"
	Assets       map[string]string // API ID → "guardicore_asset.<name>.id"
	Worksites    map[string]string // API ID → "guardicore_worksite.<name>.id"
}

type assetLabelAssignabilityStatus int

const (
	assetLabelAssignabilityUnknown assetLabelAssignabilityStatus = iota
	assetLabelAssignable
	assetLabelNonAssignable
)

type assetLabelAssignabilityResult struct {
	Status  assetLabelAssignabilityStatus
	Reason  string
	Warning string
}

func nonAssignableReasonForLabel(label client.Label) string {
	readOnly := label.ReadOnly != nil && *label.ReadOnly
	dynamic := len(label.DynamicCriteria) > 0

	switch {
	case readOnly && dynamic:
		return "read-only and dynamic"
	case readOnly:
		return "read-only"
	case dynamic:
		return "dynamic"
	default:
		return ""
	}
}

func buildNonAssignableAssetLabelReasons(labels []client.Label) map[string]string {
	reasons := make(map[string]string)
	for _, label := range labels {
		if reason := nonAssignableReasonForLabel(label); reason != "" {
			reasons[label.ID] = reason
		}
	}
	return reasons
}

func buildExplicitAssignableAssetLabelIDs(labels []client.Label) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, label := range labels {
		if label.ReadOnly != nil && !*label.ReadOnly && len(label.DynamicCriteria) == 0 {
			ids[label.ID] = struct{}{}
		}
	}
	return ids
}

func (imp *Importer) resolveAssetLabelAssignability(
	ctx context.Context,
	label client.AssetLabelRef,
	nonAssignableLabelReasons map[string]string,
	explicitAssignableLabelIDs map[string]struct{},
	cache map[string]assetLabelAssignabilityResult,
) assetLabelAssignabilityResult {
	labelID := label.ID
	if labelID == "" {
		return assetLabelAssignabilityResult{Status: assetLabelAssignabilityUnknown}
	}

	if result, ok := cache[labelID]; ok {
		return result
	}

	if reason, ok := nonAssignableLabelReasons[labelID]; ok {
		result := assetLabelAssignabilityResult{Status: assetLabelNonAssignable, Reason: reason}
		cache[labelID] = result
		return result
	}

	if _, ok := explicitAssignableLabelIDs[labelID]; ok {
		result := assetLabelAssignabilityResult{Status: assetLabelAssignable}
		cache[labelID] = result
		return result
	}

	if imp.Client == nil {
		result := assetLabelAssignabilityResult{
			Status:  assetLabelAssignabilityUnknown,
			Warning: "labels API client is not configured",
		}
		cache[labelID] = result
		return result
	}

	apiLabel, err := imp.Client.GetLabel(ctx, labelID)
	if err != nil {
		result := assetLabelAssignabilityResult{
			Status:  assetLabelAssignabilityUnknown,
			Warning: fmt.Sprintf("failed to read label %q from labels API: %s", labelID, err),
		}
		cache[labelID] = result
		return result
	}

	if apiLabel == nil {
		result := assetLabelAssignabilityResult{
			Status:  assetLabelAssignabilityUnknown,
			Warning: fmt.Sprintf("label %q was not found in labels API", labelID),
		}
		cache[labelID] = result
		return result
	}

	key := apiLabel.Key
	if key == "" {
		key = label.Key
	}
	value := apiLabel.Value
	if value == "" {
		value = label.Value
	}

	if key == "" || value == "" {
		result := assetLabelAssignabilityResult{
			Status:  assetLabelAssignabilityUnknown,
			Warning: fmt.Sprintf("label %q is missing key/value metadata for assignability checks", labelID),
		}
		cache[labelID] = result
		return result
	}

	labels, err := imp.Client.ListLabels(ctx, key, value)
	if err != nil {
		result := assetLabelAssignabilityResult{
			Status:  assetLabelAssignabilityUnknown,
			Warning: fmt.Sprintf("failed to list labels for %q/%q while verifying label %q: %s", key, value, labelID, err),
		}
		cache[labelID] = result
		return result
	}

	for _, candidate := range labels {
		if candidate.ID != labelID {
			continue
		}
		if reason := nonAssignableReasonForLabel(candidate); reason != "" {
			result := assetLabelAssignabilityResult{Status: assetLabelNonAssignable, Reason: reason}
			cache[labelID] = result
			return result
		}

		result := assetLabelAssignabilityResult{Status: assetLabelAssignable}
		cache[labelID] = result
		return result
	}

	result := assetLabelAssignabilityResult{
		Status:  assetLabelAssignabilityUnknown,
		Warning: fmt.Sprintf("label %q was not returned by labels API lookup for key=%q value=%q", labelID, key, value),
	}
	cache[labelID] = result
	return result
}

// buildLookupMap converts a list of named resources into ID-to-reference map.
func buildLookupMap(named []NamedResource, resourceType string) map[string]string {
	m := make(map[string]string, len(named))
	for _, nr := range named {
		m[nr.ID] = fmt.Sprintf("%s.%s.id", resourceType, nr.Name)
	}
	return m
}

// buildLookupMapSplit builds a lookup map where system-managed resources use
// data source references and managed resources use resource references.
func buildLookupMapSplit(named []NamedResource, resourceType string, systemManagedIDs map[string]struct{}) map[string]string {
	m := make(map[string]string, len(named))
	for _, nr := range named {
		if _, isSM := systemManagedIDs[nr.ID]; isSM {
			m[nr.ID] = fmt.Sprintf("data.%s.%s.id", resourceType, nr.Name)
		} else {
			m[nr.ID] = fmt.Sprintf("%s.%s.id", resourceType, nr.Name)
		}
	}
	return m
}

func filterImportableAssets(assets []client.Asset) ([]client.Asset, int) {
	filtered := make([]client.Asset, 0, len(assets))
	skipped := 0
	for _, asset := range assets {
		if strings.EqualFold(asset.Status, "deleted") {
			skipped++
			continue
		}
		filtered = append(filtered, asset)
	}
	return filtered, skipped
}

func partitionLabels(labels []client.Label) (manageable, systemManaged []client.Label) {
	for _, label := range labels {
		if label.ReadOnly != nil && *label.ReadOnly {
			systemManaged = append(systemManaged, label)
		} else {
			manageable = append(manageable, label)
		}
	}
	return
}

// Run fetches all resources and writes .tf files to the output directory.
func (imp *Importer) Run(ctx context.Context) (*Result, error) {
	// Phase 1: Fetch all resources
	labels, err := imp.Client.ListLabels(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}
	nonAssignableAssetLabelReasons := buildNonAssignableAssetLabelReasons(labels)
	explicitAssignableAssetLabelIDs := buildExplicitAssignableAssetLabelIDs(labels)
	manageableLabels, systemManagedLabels := partitionLabels(labels)
	if len(systemManagedLabels) > 0 {
		fmt.Fprintf(os.Stderr, "Note: %d read-only labels will be generated as data sources\n", len(systemManagedLabels))
	}

	labelGroups, err := imp.Client.ListLabelGroups(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list label groups: %w", err)
	}

	policyRules, err := imp.Client.ListPolicyRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list policy rules: %w", err)
	}

	dnsBlocklists, err := imp.Client.ListDnsBlocklists(ctx, "", "")
	if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
		fmt.Fprintln(os.Stderr, "Note: DNS Security feature is disabled, skipping DNS blocklists import")
		dnsBlocklists = nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to list DNS blocklists: %w", err)
	}

	incidents, err := imp.Client.ListIncidents(ctx, 946684800000, time.Now().UnixMilli()) // from 2000-01-01
	if err != nil {
		return nil, fmt.Errorf("failed to list incidents: %w", err)
	}

	worksites, err := imp.Client.ListWorksites(ctx, "")
	if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
		fmt.Fprintln(os.Stderr, "Note: worksites feature is disabled, skipping worksites import")
		worksites = nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to list worksites: %w", err)
	}

	userGroups, err := imp.Client.ListUserGroups(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list user groups: %w", err)
	}

	assets, err := imp.Client.ListAssets(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}
	assets, skippedDeletedAssets := filterImportableAssets(assets)
	if skippedDeletedAssets > 0 {
		fmt.Fprintf(os.Stderr, "Note: skipping %d deleted assets; Terraform treats deactivated assets as removed\n", skippedDeletedAssets)
	}

	agentAggregators, err := imp.Client.ListAgentAggregators(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list agent aggregators: %w", err)
	}

	// Phase 2: Build name maps and deduplicate
	allLabels := append(manageableLabels, systemManagedLabels...)
	labelIDToName := make(map[string]string, len(allLabels))
	for _, l := range allLabels {
		labelIDToName[l.ID] = SanitizeName(l.Key, l.Value)
	}
	namedLabels := DeduplicateNames(labelIDToName)

	systemManagedLabelIDs := make(map[string]struct{}, len(systemManagedLabels))
	for _, l := range systemManagedLabels {
		systemManagedLabelIDs[l.ID] = struct{}{}
	}

	labelGroupIDToName := make(map[string]string, len(labelGroups))
	for _, g := range labelGroups {
		labelGroupIDToName[g.ID] = SanitizeName(g.Key, g.Value)
	}
	namedLabelGroups := DeduplicateNames(labelGroupIDToName)

	userGroupIDToName := make(map[string]string, len(userGroups))
	systemManagedUGIDs := make(map[string]struct{})
	for _, ug := range userGroups {
		userGroupIDToName[ug.ID] = SanitizeName("", ug.Title)
		if shouldSkipUserGroup(ug) {
			systemManagedUGIDs[ug.ID] = struct{}{}
		}
	}
	namedUserGroups := DeduplicateNames(userGroupIDToName)

	assetIDToName := make(map[string]string, len(assets))
	for _, a := range assets {
		assetIDToName[a.ID] = SanitizeName("", a.Name)
	}
	namedAssets := DeduplicateNames(assetIDToName)

	worksiteIDToName := make(map[string]string, len(worksites))
	systemManagedWorksiteIDs := make(map[string]struct{})
	for _, w := range worksites {
		worksiteIDToName[w.ID] = SanitizeName("", w.Name)
		if w.Name == "Default" {
			systemManagedWorksiteIDs[w.ID] = struct{}{}
		}
	}
	namedWorksites := DeduplicateNames(worksiteIDToName)

	// Build global resource lookup for cross-references
	lookup := &ResourceLookup{
		Labels:       buildLookupMapSplit(namedLabels, "guardicore_label", systemManagedLabelIDs),
		LabelGroups:  buildLookupMap(namedLabelGroups, "guardicore_label_group"),
		PolicyGroups: map[string]string{},
		UserGroups:   buildLookupMapSplit(namedUserGroups, "guardicore_user_group", systemManagedUGIDs),
		Assets:       buildLookupMap(namedAssets, "guardicore_asset"),
		Worksites:    buildLookupMapSplit(namedWorksites, "guardicore_worksite", systemManagedWorksiteIDs),
	}

	// Phase 3: Generate files
	if err := os.MkdirAll(imp.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := imp.generateLabelsFile(allLabels, namedLabels, systemManagedLabelIDs); err != nil {
		return nil, fmt.Errorf("failed to generate labels.tf: %w", err)
	}

	if err := imp.generateLabelGroupsFile(labelGroups, namedLabelGroups, lookup); err != nil {
		return nil, fmt.Errorf("failed to generate label_groups.tf: %w", err)
	}

	if err := imp.generatePolicyRulesFile(policyRules, lookup); err != nil {
		return nil, fmt.Errorf("failed to generate policy_rules.tf: %w", err)
	}

	if err := imp.generateDnsSecurityFile(dnsBlocklists); err != nil {
		return nil, fmt.Errorf("failed to generate dns_security.tf: %w", err)
	}

	if err := imp.generateIncidentsFile(incidents); err != nil {
		return nil, fmt.Errorf("failed to generate incidents.tf: %w", err)
	}

	if err := imp.generateWorksitesFile(worksites, namedWorksites, systemManagedWorksiteIDs); err != nil {
		return nil, fmt.Errorf("failed to generate worksites.tf: %w", err)
	}

	userGroupsWritten, err := imp.generateUserGroupsFile(userGroups, namedUserGroups, systemManagedUGIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate user_groups.tf: %w", err)
	}

	assetsWritten, err := imp.generateAssetsFile(ctx, assets, namedAssets, lookup, nonAssignableAssetLabelReasons, explicitAssignableAssetLabelIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate assets.tf: %w", err)
	}

	if err := imp.generateAgentAggregatorsFile(agentAggregators); err != nil {
		return nil, fmt.Errorf("failed to generate agent_aggregators.tf: %w", err)
	}

	return &Result{
		Labels:           len(allLabels),
		LabelGroups:      len(labelGroups),
		PolicyRules:      len(policyRules),
		DnsBlocklists:    len(dnsBlocklists),
		Incidents:        len(incidents),
		Worksites:        len(worksites),
		UserGroups:       userGroupsWritten,
		Assets:           assetsWritten,
		AgentAggregators: len(agentAggregators),
	}, nil
}

func (imp *Importer) generateLabelsFile(labels []client.Label, named []NamedResource, systemManagedIDs map[string]struct{}) error {
	labelByID := make(map[string]client.Label, len(labels))
	for _, l := range labels {
		labelByID[l.ID] = l
	}

	var buf bytes.Buffer
	for i, nr := range named {
		l := labelByID[nr.ID]

		if _, isSM := systemManagedIDs[nr.ID]; isSM {
			data := LabelTemplateData{
				Name:  nr.Name,
				Key:   l.Key,
				Value: l.Value,
			}
			if err := labelDataTemplate.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to render data label %s: %w", l.ID, err)
			}
		} else {
			var criteria []CriteriaData
			for _, c := range l.DynamicCriteria {
				if c.IsReadOnlyWorksiteGenerated() {
					continue
				}

				if len(c.CompoundCriteria) > 0 {
					compound := make([]CompoundCriteriaData, 0, len(c.CompoundCriteria))
					for _, cc := range c.CompoundCriteria {
						if cc.Field == "" || cc.Op == "" || cc.Argument == "" {
							return fmt.Errorf("label %q/%q (%s) has invalid compound dynamic criterion with empty required fields", l.Key, l.Value, l.ID)
						}
						compound = append(compound, CompoundCriteriaData{
							Field:    cc.Field,
							Op:       cc.Op,
							Argument: cc.Argument,
						})
					}
					if len(compound) == 0 {
						return fmt.Errorf("label %q/%q (%s) has empty compound dynamic criterion", l.Key, l.Value, l.ID)
					}
					criteria = append(criteria, CriteriaData{
						IsCompound:       true,
						CompoundCriteria: compound,
					})
					continue
				}

				if c.Field == "" || c.Op == "" || c.Argument == "" {
					return fmt.Errorf("label %q/%q (%s) has unsupported or incomplete dynamic criterion: expected either flat field/op/argument or compound_criteria", l.Key, l.Value, l.ID)
				}
				criteria = append(criteria, CriteriaData{
					Field:      c.Field,
					Op:         c.Op,
					Argument:   c.Argument,
					IsCompound: false,
				})
			}

			data := LabelTemplateData{
				Name:     nr.Name,
				ID:       l.ID,
				Key:      l.Key,
				Value:    l.Value,
				Criteria: criteria,
			}

			if err := labelTemplate.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to render label %s: %w", l.ID, err)
			}
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "labels.tf"), buf.Bytes(), 0644)
}

func (imp *Importer) generateLabelGroupsFile(groups []client.LabelGroup, named []NamedResource, lookup *ResourceLookup) error {
	groupByID := make(map[string]client.LabelGroup, len(groups))
	for _, g := range groups {
		groupByID[g.ID] = g
	}

	var buf bytes.Buffer
	for i, nr := range named {
		g := groupByID[nr.ID]

		data := LabelGroupTemplateData{
			Name:     nr.Name,
			ID:       g.ID,
			Key:      g.Key,
			Value:    g.Value,
			Comments: g.Comments,
		}

		if g.IncludeLabels != nil && len(g.IncludeLabels.OrLabels) > 0 {
			data.Include = buildLabelGroupSelectorHCL(g.IncludeLabels, lookup)
		}

		if g.ExcludeLabels != nil && len(g.ExcludeLabels.OrLabels) > 0 {
			data.Exclude = buildLabelGroupSelectorHCL(g.ExcludeLabels, lookup)
		}

		if err := labelGroupTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render label group %s: %w", g.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "label_groups.tf"), buf.Bytes(), 0644)
}

// buildLabelGroupSelectorHCL generates an HCL expression string for the typed
// include/exclude selector shape with Terraform resource references where possible.
func buildLabelGroupSelectorHCL(read *client.OrLabelsRead, lookup *ResourceLookup) string {
	var buf strings.Builder
	buf.WriteString("{\n")
	buf.WriteString("    or_groups = [\n")
	for i, or := range read.OrLabels {
		buf.WriteString("      {\n")

		hasUnresolved := false
		for _, label := range or.AndLabels {
			if _, ok := lookup.Labels[label.ID]; !ok {
				hasUnresolved = true
				break
			}
		}

		if hasUnresolved {
			buf.WriteString("        label_ids = [\n")
			for _, label := range or.AndLabels {
				if ref, ok := lookup.Labels[label.ID]; ok {
					fmt.Fprintf(&buf, "          %s,\n", ref)
				} else {
					fmt.Fprintf(&buf, "          %q, # reference not imported\n", label.ID)
				}
			}
			buf.WriteString("        ]\n")
		} else {
			buf.WriteString("        label_ids = [")
			for j, label := range or.AndLabels {
				if j > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(lookup.Labels[label.ID])
			}
			buf.WriteString("]\n")
		}
		buf.WriteString("      }")
		if i < len(read.OrLabels)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("    ]\n")
	buf.WriteString("  }")
	return buf.String()
}

func (imp *Importer) generatePolicyRulesFile(rules []map[string]interface{}, lookup *ResourceLookup) error {
	idToName := make(map[string]string, len(rules))
	for _, r := range rules {
		id, _ := r["id"].(string)
		if id == "" {
			continue
		}
		name := policyRuleName(r)
		idToName[id] = name
	}
	named := DeduplicateNames(idToName)

	ruleByID := make(map[string]map[string]interface{}, len(rules))
	for _, r := range rules {
		id, _ := r["id"].(string)
		if id != "" {
			ruleByID[id] = r
		}
	}

	var buf bytes.Buffer
	for i, nr := range named {
		r := ruleByID[nr.ID]

		// Extract worksite before normalization strips it
		var worksiteExpr string
		if ws, ok := r["worksite"]; ok {
			if wsMap, ok := ws.(map[string]any); ok {
				if wsID, ok := wsMap["id"].(string); ok && wsID != "" {
					if ref, found := lookup.Worksites[wsID]; found {
						worksiteExpr = ref
					} else {
						worksiteExpr = fmt.Sprintf("%q", wsID)
					}
				}
			}
		}

		normalized := normalize.NormalizePolicyRuleSpec(r)
		bodyHCL := buildPolicyRuleBodyHCL(normalized, lookup)

		data := PolicyRuleTemplateData{
			Name:               nr.Name,
			ID:                 nr.ID,
			BodyHCL:            bodyHCL,
			WorksiteExpression: worksiteExpr,
		}

		if err := policyRuleTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render policy rule %s: %w", nr.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "policy_rules.tf"), buf.Bytes(), 0644)
}

func (imp *Importer) generateDnsSecurityFile(blocklists []client.DnsBlocklist) error {
	idToName := make(map[string]string, len(blocklists))
	for _, b := range blocklists {
		idToName[b.ID] = SanitizeName("", b.Name)
	}
	named := DeduplicateNames(idToName)

	blocklistByID := make(map[string]client.DnsBlocklist, len(blocklists))
	for _, b := range blocklists {
		blocklistByID[b.ID] = b
	}

	var buf bytes.Buffer
	for i, nr := range named {
		b := blocklistByID[nr.ID]

		if isSystemManagedDnsBlocklistType(b.Type) {
			fmt.Fprintf(os.Stderr, "Note: system-managed DNS blocklist %q (%s, type=%s) will be generated as data source\n", b.Name, b.ID, b.Type)
			if err := dnsSecurityDataTemplate.Execute(&buf, DnsSecurityTemplateData{Name: nr.Name, ID: b.ID}); err != nil {
				return fmt.Errorf("failed to render DNS blocklist data source %s: %w", b.ID, err)
			}
			if i < len(named)-1 {
				buf.WriteString("\n")
			}
			continue
		}

		data := DnsSecurityTemplateData{
			Name:         nr.Name,
			ID:           b.ID,
			ResourceName: b.Name,
			Type:         b.Type,
			Domains:      b.Domains,
			Enabled:      b.Enabled,
		}

		if err := dnsSecurityTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render DNS blocklist %s: %w", b.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "dns_security.tf"), buf.Bytes(), 0644)
}

func isSystemManagedDnsBlocklistType(listType string) bool {
	return strings.EqualFold(listType, "AKAMAI_INTELLIGENCE") || strings.EqualFold(listType, "WEB_CATEGORY")
}

func (imp *Importer) generateIncidentsFile(incidents []map[string]interface{}) error {
	idToName := make(map[string]string, len(incidents))
	for _, inc := range incidents {
		id, _ := inc["id"].(string)
		if id == "" {
			continue
		}
		name := incidentName(inc)
		idToName[id] = name
	}
	named := DeduplicateNames(idToName)

	incidentByID := make(map[string]map[string]interface{}, len(incidents))
	for _, inc := range incidents {
		id, _ := inc["id"].(string)
		if id != "" {
			incidentByID[id] = inc
		}
	}

	var buf bytes.Buffer
	for i, nr := range named {
		inc := incidentByID[nr.ID]

		incType, _ := inc["type"].(string)
		severity, _ := inc["severity"].(string)
		startTime, _ := inc["time"].(float64)

		// Extract tags from tags.data
		var tags []string
		if tagWrapper, ok := inc["tags"].(map[string]interface{}); ok {
			if tagList, ok := tagWrapper["data"].([]interface{}); ok {
				for _, t := range tagList {
					if name, ok := t.(string); ok {
						tags = append(tags, name)
					}
				}
			}
		}

		// Serialize affected_assets.data as JSON.
		affectedAssetsJSON := "[]"
		if aaWrapper, ok := inc["affected_assets"].(map[string]interface{}); ok {
			if aa, ok := aaWrapper["data"]; ok {
				if jsonBytes, err := json.MarshalIndent(aa, "  ", "  "); err == nil {
					affectedAssetsJSON = string(jsonBytes)
				}
			}
		}

		data := IncidentTemplateData{
			Name:                        nr.Name,
			ID:                          nr.ID,
			IncidentType:                incType,
			Severity:                    severity,
			StartTime:                   int64(startTime),
			Tags:                        tags,
			CommentedAffectedAssetsJSON: commentAllLines(affectedAssetsJSON, "#   "),
		}

		if err := incidentTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render incident %s: %w", nr.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "incidents.tf"), buf.Bytes(), 0644)
}

func commentAllLines(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = "#"
			continue
		}
		lines[i] = prefix + line
	}

	return strings.Join(lines, "\n")
}

func formatOptionalHCLString(value string) string {
	if value == "" {
		return ""
	}
	return formatHCLScalar(value)
}

func (imp *Importer) generateWorksitesFile(worksites []client.Worksite, named []NamedResource, systemManagedIDs map[string]struct{}) error {
	worksiteByID := make(map[string]client.Worksite, len(worksites))
	for _, w := range worksites {
		worksiteByID[w.ID] = w
	}

	var buf bytes.Buffer
	for i, nr := range named {
		w := worksiteByID[nr.ID]

		data := WorksiteTemplateData{
			Name:            nr.Name,
			ID:              w.ID,
			ResourceNameHCL: formatHCLScalar(w.Name),
			CommentHCL:      formatOptionalHCLString(w.Comment),
		}

		if _, isSM := systemManagedIDs[nr.ID]; isSM {
			fmt.Fprintf(os.Stderr, "Note: system-managed worksite %q (%s) will be generated as data source\n", w.Name, nr.Name)
			if err := worksiteDataTemplate.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to render worksite data source %s: %w", w.ID, err)
			}
		} else {
			if err := worksiteTemplate.Execute(&buf, data); err != nil {
				return fmt.Errorf("failed to render worksite %s: %w", w.ID, err)
			}
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "worksites.tf"), buf.Bytes(), 0644)
}

func (imp *Importer) generateUserGroupsFile(userGroups []client.UserGroup, named []NamedResource, systemManagedIDs map[string]struct{}) (int, error) {
	userGroupByID := make(map[string]client.UserGroup, len(userGroups))
	for _, ug := range userGroups {
		userGroupByID[ug.ID] = ug
	}

	var buf bytes.Buffer
	written := 0
	for _, nr := range named {
		ug := userGroupByID[nr.ID]

		if _, isSM := systemManagedIDs[nr.ID]; isSM {
			fmt.Fprintf(os.Stderr, "Note: system-managed user group %q (%s) will be generated as data source\n", ug.Title, ug.ID)
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			data := UserGroupTemplateData{
				Name:  nr.Name,
				Title: ug.Title,
			}
			if err := userGroupDataTemplate.Execute(&buf, data); err != nil {
				return 0, fmt.Errorf("failed to render data user group %s: %w", ug.ID, err)
			}
			written++
			continue
		}

		orchGroups := orchGroupsFromUserGroup(ug)

		data := UserGroupTemplateData{
			Name:                 nr.Name,
			ID:                   ug.ID,
			Title:                ug.Title,
			OrchestrationsGroups: orchGroups,
		}

		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		if err := userGroupTemplate.Execute(&buf, data); err != nil {
			return 0, fmt.Errorf("failed to render user group %s: %w", ug.ID, err)
		}
		written++
	}

	return written, os.WriteFile(filepath.Join(imp.OutputDir, "user_groups.tf"), buf.Bytes(), 0644)
}

// shouldSkipUserGroup returns true if a user group should be skipped during import.
// System user groups (e.g., "Local Administrators") have all orchestration IDs set to "local".
func shouldSkipUserGroup(ug client.UserGroup) bool {
	if len(ug.GroupsByDomainName) > 0 {
		for _, domainInfo := range ug.GroupsByDomainName {
			if domainInfo.OrchestrationID != "local" {
				return false
			}
		}
		return true
	}

	if len(ug.OrchestrationsGroups) > 0 {
		for _, og := range ug.OrchestrationsGroups {
			if og.OrchestrationID != "local" {
				return false
			}
		}
		return true
	}

	return true
}

// orchGroupsFromUserGroup extracts OrchestrationGroupData from a UserGroup,
// preferring GroupsByDomainName (list API format) over OrchestrationsGroups (create API format).
func orchGroupsFromUserGroup(ug client.UserGroup) []OrchestrationGroupData {
	if len(ug.GroupsByDomainName) > 0 {
		var result []OrchestrationGroupData
		for _, domainInfo := range ug.GroupsByDomainName {
			var groupIDs []string
			for _, g := range domainInfo.Groups {
				groupIDs = append(groupIDs, g.ID)
			}
			result = append(result, OrchestrationGroupData{
				OrchestrationID: domainInfo.OrchestrationID,
				Groups:          groupIDs,
			})
		}
		return result
	}

	var result []OrchestrationGroupData
	for _, og := range ug.OrchestrationsGroups {
		result = append(result, OrchestrationGroupData{
			OrchestrationID: og.OrchestrationID,
			Groups:          og.Groups,
		})
	}
	return result
}

func shouldSkipAsset(a client.Asset) bool {
	if len(a.Nics) == 0 {
		return true
	}
	for _, nic := range a.Nics {
		if len(nic.IPAddresses) > 0 {
			return false
		}
	}
	return true
}

func (imp *Importer) generateAssetsFile(
	ctx context.Context,
	assets []client.Asset,
	named []NamedResource,
	lookup *ResourceLookup,
	nonAssignableLabelReasons map[string]string,
	explicitAssignableLabelIDs map[string]struct{},
) (int, error) {
	assetByID := make(map[string]client.Asset, len(assets))
	for _, a := range assets {
		assetByID[a.ID] = a
	}

	assignabilityCache := make(map[string]assetLabelAssignabilityResult)

	var buf bytes.Buffer
	written := 0
	for _, nr := range named {
		a := assetByID[nr.ID]

		if shouldSkipAsset(a) {
			fmt.Fprintf(os.Stderr, "Note: skipping asset %q (%s): no valid NICs (all NICs have empty ip_addresses or none exist)\n", a.Name, a.ID)
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			orchObjID := ""
			if len(a.OrchestrationDetails) > 0 && a.OrchestrationDetails[0].OrchestrationObjID != "" {
				orchObjID = a.OrchestrationDetails[0].OrchestrationObjID
			}
			if orchObjID == "" {
				orchObjID = a.OrchestrationObjID
			}
			if orchObjID == "" {
				orchObjID = orchObjIDPlaceholder
			}
			data := AssetTemplateData{
				Name:               nr.Name,
				ID:                 a.ID,
				ResourceName:       a.Name,
				OrchestrationObjID: orchObjID,
			}
			if err := assetSkippedTemplate.Execute(&buf, data); err != nil {
				return 0, fmt.Errorf("failed to render skipped asset %s: %w", a.ID, err)
			}
			continue
		}

		var nics []AssetNICData
		for _, nic := range a.Nics {
			if len(nic.IPAddresses) == 0 {
				fmt.Fprintf(os.Stderr, "Note: skipping NIC (MAC: %s) on asset %q (%s): empty ip_addresses\n",
					nic.MacAddress, a.Name, a.ID)
				continue
			}
			nics = append(nics, AssetNICData{
				IPAddresses: nic.IPAddresses,
				MacAddress:  nic.MacAddress,
				VifID:       nic.VifID,
				NetworkName: nic.NetworkName,
			})
		}

		var labelRefs []AssetLabelRefData
		for _, label := range a.Labels {
			assignability := imp.resolveAssetLabelAssignability(
				ctx,
				label,
				nonAssignableLabelReasons,
				explicitAssignableLabelIDs,
				assignabilityCache,
			)

			if assignability.Status == assetLabelNonAssignable {
				fmt.Fprintf(
					os.Stderr,
					"Note: skipping non-assignable asset label %q on asset %q (%s): %s label\n",
					label.ID,
					a.Name,
					a.ID,
					assignability.Reason,
				)
				continue
			}

			refData := AssetLabelRefData{}
			if ref, ok := lookup.Labels[label.ID]; ok {
				base := strings.TrimSuffix(ref, ".id")
				refData.IDExpression = ref
				refData.KeyExpression = base + ".key"
				refData.ValueExpression = base + ".value"
			} else {
				if label.ID == "" || label.Key == "" || label.Value == "" {
					if assignability.Status == assetLabelAssignabilityUnknown {
						fmt.Fprintf(
							os.Stderr,
							"Note: skipping unresolved asset label %q on asset %q (%s): missing key/value and no managed label reference exists\n",
							label.ID,
							a.Name,
							a.ID,
						)
					}
					fmt.Fprintf(
						os.Stderr,
						"Note: skipping asset label %q on asset %q (%s): label details are incomplete and no managed label reference exists\n",
						label.ID,
						a.Name,
						a.ID,
					)
					continue
				}

				refData.IDExpression = fmt.Sprintf("%q", label.ID)
				refData.KeyExpression = fmt.Sprintf("%q", label.Key)
				refData.ValueExpression = fmt.Sprintf("%q", label.Value)
			}

			if assignability.Status == assetLabelAssignabilityUnknown {
				warning := assignability.Warning
				if warning == "" {
					warning = "label assignability could not be verified"
				}
				fmt.Fprintf(
					os.Stderr,
					"Note: unable to verify assignability for asset label %q on asset %q (%s): %s; including label as-is. If this label is read-only or dynamic, apply may fail.\n",
					label.ID,
					a.Name,
					a.ID,
					warning,
				)
			}
			labelRefs = append(labelRefs, refData)
		}

		var worksiteExpr string
		if a.ScopingDetails != nil && a.ScopingDetails.Worksite != nil && a.ScopingDetails.Worksite.ID != "" {
			wsID := a.ScopingDetails.Worksite.ID
			if ref, found := lookup.Worksites[wsID]; found {
				worksiteExpr = ref
			} else {
				worksiteExpr = fmt.Sprintf("%q", wsID)
			}
		}

		orchObjID := ""
		if len(a.OrchestrationDetails) > 0 && a.OrchestrationDetails[0].OrchestrationObjID != "" {
			orchObjID = a.OrchestrationDetails[0].OrchestrationObjID
		}
		if orchObjID == "" {
			orchObjID = a.OrchestrationObjID
		}
		var orchObjIDComment string
		if orchObjID == "" {
			orchObjID = orchObjIDPlaceholder
			orchObjIDComment = "TODO: orchestration_obj_id is create-only and not returned by the API.\n  # Replace this placeholder with the actual value from your orchestration system."
		}

		data := AssetTemplateData{
			Name:               nr.Name,
			ID:                 a.ID,
			ResourceName:       a.Name,
			OrchestrationObjID: orchObjID,
			OrchObjIDComment:   orchObjIDComment,
			InstanceID:         a.InstanceID,
			HwUUID:             a.HwUUID,
			Comments:           a.Comments,
			Status:             a.Status,
			Nics:               nics,
			Labels:             labelRefs,
			WorksiteExpression: worksiteExpr,
		}

		if err := assetTemplate.Execute(&buf, data); err != nil {
			return 0, fmt.Errorf("failed to render asset %s: %w", a.ID, err)
		}
		if written > 0 {
			buf.WriteString("\n")
		}
		written++
	}

	return written, os.WriteFile(filepath.Join(imp.OutputDir, "assets.tf"), buf.Bytes(), 0644)
}

// buildPolicyRuleBodyHCL generates typed HCL attributes for a policy rule,
// using raw_spec_json only for fields without a typed representation.
func buildPolicyRuleBodyHCL(spec map[string]interface{}, lookup *ResourceLookup) string {
	if spec == nil {
		return ""
	}

	normalizePolicyRuleICMPMatches(spec)

	// Replace IDs with references in source/destination endpoints
	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := spec[endpointKey]
		if !ok {
			continue
		}
		endpointMap, ok := endpoint.(map[string]interface{})
		if !ok {
			continue
		}
		replaceEndpointRefs(endpointMap, "label_group_ids", lookup.LabelGroups)
		replaceEndpointRefs(endpointMap, "policy_groups", lookup.PolicyGroups)
		replaceEndpointRefs(endpointMap, "user_group_ids", lookup.UserGroups)
		replaceEndpointRefs(endpointMap, "asset_ids", lookup.Assets)
		replaceNestedLabelRefs(endpointMap, lookup.Labels)
	}

	var lines []string
	if action, ok := spec["action"]; ok {
		lines = append(lines, fmt.Sprintf("  action           = %q", action))
		delete(spec, "action")
	}
	if section, ok := spec["section_position"]; ok {
		lines = append(lines, fmt.Sprintf("  section_position = %q", section))
		delete(spec, "section_position")
	}
	if enabled, ok := spec["enabled"]; ok {
		lines = append(lines, fmt.Sprintf("  enabled          = %v", enabled))
		delete(spec, "enabled")
	}
	if comments, ok := spec["comments"].(string); ok && comments != "" {
		lines = append(lines, fmt.Sprintf("  comments         = %q", comments))
		delete(spec, "comments")
	}
	if rulesetName, ok := spec["ruleset_name"].(string); ok && rulesetName != "" {
		lines = append(lines, fmt.Sprintf("  ruleset_name     = %q", rulesetName))
		delete(spec, "ruleset_name")
	}
	if priority, ok := spec["priority"]; ok {
		lines = append(lines, fmt.Sprintf("  priority         = %v", formatHCLScalar(priority)))
		delete(spec, "priority")
	}
	if networkProfile, ok := spec["network_profile"].(string); ok && networkProfile != "" {
		lines = append(lines, fmt.Sprintf("  network_profile  = %q", networkProfile))
		delete(spec, "network_profile")
	}
	if source, ok := spec["source"].(map[string]interface{}); ok && len(source) > 0 {
		lines = append(lines, "", "  source = "+formatHCLObject(source, 2))
	}
	if destination, ok := spec["destination"].(map[string]interface{}); ok && len(destination) > 0 {
		lines = append(lines, "", "  destination = "+formatHCLObject(destination, 2))
	}
	delete(spec, "source")
	delete(spec, "destination")
	for _, key := range []string{"ports", "ip_protocols", "exclude_ports", "port_ranges", "exclude_port_ranges", "icmp_matches", "schedule", "scope"} {
		if value, ok := spec[key]; ok {
			lines = append(lines, "", fmt.Sprintf("  %s = %s", key, formatHCLValue(value, 2)))
			delete(spec, key)
		}
	}
	delete(spec, "recently_hit")
	delete(spec, "creation_origin")

	if len(spec) > 0 {
		rawJSON, err := json.MarshalIndent(spec, "  ", "  ")
		if err == nil {
			result := string(rawJSON)
			for _, refMap := range []map[string]string{lookup.Labels, lookup.LabelGroups, lookup.PolicyGroups, lookup.UserGroups, lookup.Assets} {
				for _, ref := range refMap {
					quoted := fmt.Sprintf("%q", ref)
					result = strings.ReplaceAll(result, quoted, ref)
				}
			}
			lines = append(lines, "", "  raw_spec_json = jsonencode("+result+")")
		}
	}

	return strings.Join(lines, "\n")
}

func normalizePolicyRuleICMPMatches(spec map[string]interface{}) {
	rawMatches, ok := spec["icmp_matches"]
	if !ok {
		return
	}

	matches, ok := rawMatches.([]interface{})
	if !ok {
		return
	}

	for _, rawMatch := range matches {
		match, ok := rawMatch.(map[string]interface{})
		if !ok {
			continue
		}

		rawCodes, hasCodes := match["icmp_codes"]
		if !hasCodes {
			match["icmp_codes"] = []interface{}{}
			continue
		}

		if _, ok := rawCodes.([]interface{}); !ok {
			match["icmp_codes"] = []interface{}{}
		}
	}
}

// replaceEndpointRefs replaces string IDs in an endpoint's reference array
// with Terraform resource reference expressions from the lookup.
func replaceEndpointRefs(endpointMap map[string]interface{}, key string, refMap map[string]string) {
	refs, ok := endpointMap[key]
	if !ok {
		return
	}
	refSlice, ok := refs.([]interface{})
	if !ok {
		return
	}
	for i, ref := range refSlice {
		if refID, ok := ref.(string); ok {
			if tfRef, found := refMap[refID]; found {
				refSlice[i] = tfRef
			}
		}
	}
}

func replaceNestedLabelRefs(endpointMap map[string]interface{}, refMap map[string]string) {
	labels, ok := endpointMap["labels"].(map[string]interface{})
	if !ok {
		return
	}
	orLabels, ok := labels["or_labels"].([]interface{})
	if !ok {
		return
	}
	for _, rawOrLabel := range orLabels {
		orLabel, ok := rawOrLabel.(map[string]interface{})
		if !ok {
			continue
		}
		andLabels, ok := orLabel["and_labels"].([]interface{})
		if !ok {
			continue
		}
		for i, rawLabelID := range andLabels {
			labelID := labelIDFromPolicyRuleEndpoint(rawLabelID)
			if labelID == "" {
				continue
			}
			if tfRef, found := refMap[labelID]; found {
				andLabels[i] = tfRef
			} else {
				andLabels[i] = labelID
			}
		}
	}
}

func labelIDFromPolicyRuleEndpoint(value interface{}) string {
	if labelID, ok := value.(string); ok {
		return labelID
	}
	label, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	labelID, _ := label["id"].(string)
	return labelID
}

func formatHCLObject(value map[string]interface{}, indent int) string {
	return formatHCLValue(value, indent)
}

func formatHCLValue(value interface{}, indent int) string {
	padding := strings.Repeat("  ", indent)
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "guardicore_") || strings.HasPrefix(v, "data.guardicore_") {
			return v
		}
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64, int, int64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, padding+"  "+formatHCLValue(item, indent+1)+",")
		}
		return "[\n" + strings.Join(parts, "\n") + "\n" + padding + "]"
	case []string:
		items := make([]interface{}, len(v))
		for i, item := range v {
			items[i] = item
		}
		return formatHCLValue(items, indent)
	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s  %s = %s", padding, key, formatHCLValue(v[key], indent+1)))
		}
		return "{\n" + strings.Join(parts, "\n") + "\n" + padding + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatHCLScalar(value interface{}) string {
	return formatHCLValue(value, 0)
}

// incidentName generates a Terraform resource name for an incident.
// Uses the incident type + short hash of ID.
func incidentName(incident map[string]interface{}) string {
	incType, _ := incident["type"].(string)
	id, _ := incident["id"].(string)
	if incType != "" && id != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
		return SanitizeName("", strings.ToLower(strings.ReplaceAll(incType, " ", "_"))+"_"+hash[:8])
	}
	return "incident"
}

// policyRuleName generates a Terraform resource name for a policy rule.
// Uses the comments field if available, otherwise generates from action + short hash.
func policyRuleName(rule map[string]interface{}) string {
	if comments, ok := rule["comments"].(string); ok && comments != "" {
		sanitized := SanitizeName("", comments)
		if sanitized != "resource" {
			return sanitized
		}
	}

	action, _ := rule["action"].(string)
	id, _ := rule["id"].(string)
	if action != "" && id != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
		return SanitizeName("", strings.ToLower(action)+"_"+hash[:8])
	}

	return "rule"
}

func (imp *Importer) generateAgentAggregatorsFile(aggregators []client.AgentAggregator) error {
	if len(aggregators) == 0 {
		return nil
	}

	idToName := make(map[string]string, len(aggregators))
	for _, a := range aggregators {
		idToName[a.ID] = SanitizeName("", a.Hostname)
	}
	named := DeduplicateNames(idToName)

	aggregatorByID := make(map[string]client.AgentAggregator, len(aggregators))
	for _, a := range aggregators {
		aggregatorByID[a.ID] = a
	}

	var buf bytes.Buffer
	for i, nr := range named {
		a := aggregatorByID[nr.ID]

		data := AgentAggregatorTemplateData{
			Name:     nr.Name,
			Hostname: a.Hostname,
		}

		if err := agentAggregatorDataTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render agent aggregator data source %s: %w", a.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "agent_aggregators.tf"), buf.Bytes(), 0644)
}
