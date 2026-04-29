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
	Labels        int
	LabelGroups   int
	PolicyRules   int
	DnsBlocklists int
	Incidents     int
	Worksites     int
	UserGroups    int
	Assets        int
}

// ResourceLookup maps API IDs to Terraform resource reference expressions.
// Values are HCL expressions like "guardicore_label.env_prod.id".
type ResourceLookup struct {
	Labels       map[string]string // API ID → "guardicore_label.<name>.id"
	LabelGroups  map[string]string // API ID → "guardicore_label_group.<name>.id"
	PolicyGroups map[string]string // API ID → "guardicore_policy_group.<name>.id"
	UserGroups   map[string]string // API ID → "guardicore_user_group.<name>.id"
	Assets       map[string]string // API ID → "guardicore_asset.<name>.id"
	Worksites    map[string]string // API ID → "guardicore_worksite.<name>.id"
}

// buildLookupMap converts a list of named resources into ID-to-reference map.
func buildLookupMap(named []NamedResource, resourceType string) map[string]string {
	m := make(map[string]string, len(named))
	for _, nr := range named {
		m[nr.ID] = fmt.Sprintf("%s.%s.id", resourceType, nr.Name)
	}
	return m
}

// Run fetches all resources and writes .tf files to the output directory.
func (imp *Importer) Run(ctx context.Context) (*Result, error) {
	// Phase 1: Fetch all resources
	labels, err := imp.Client.ListLabels(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
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
	if err != nil {
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

	// Phase 2: Build name maps and deduplicate
	labelIDToName := make(map[string]string, len(labels))
	for _, l := range labels {
		labelIDToName[l.ID] = SanitizeName(l.Key, l.Value)
	}
	namedLabels := DeduplicateNames(labelIDToName)

	labelGroupIDToName := make(map[string]string, len(labelGroups))
	for _, g := range labelGroups {
		labelGroupIDToName[g.ID] = SanitizeName(g.Key, g.Value)
	}
	namedLabelGroups := DeduplicateNames(labelGroupIDToName)

	userGroupIDToName := make(map[string]string, len(userGroups))
	for _, ug := range userGroups {
		userGroupIDToName[ug.ID] = SanitizeName("", ug.Title)
	}
	namedUserGroups := DeduplicateNames(userGroupIDToName)

	assetIDToName := make(map[string]string, len(assets))
	for _, a := range assets {
		assetIDToName[a.ID] = SanitizeName("", a.Name)
	}
	namedAssets := DeduplicateNames(assetIDToName)

	worksiteIDToName := make(map[string]string, len(worksites))
	for _, w := range worksites {
		worksiteIDToName[w.ID] = SanitizeName("", w.Name)
	}
	namedWorksites := DeduplicateNames(worksiteIDToName)

	// Build global resource lookup for cross-references
	lookup := &ResourceLookup{
		Labels:       buildLookupMap(namedLabels, "guardicore_label"),
		LabelGroups:  buildLookupMap(namedLabelGroups, "guardicore_label_group"),
		PolicyGroups: map[string]string{},
		UserGroups:   buildLookupMap(namedUserGroups, "guardicore_user_group"),
		Assets:       buildLookupMap(namedAssets, "guardicore_asset"),
		Worksites:    buildLookupMap(namedWorksites, "guardicore_worksite"),
	}

	// Phase 3: Generate files
	if err := os.MkdirAll(imp.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := imp.generateLabelsFile(labels, namedLabels); err != nil {
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

	if err := imp.generateWorksitesFile(worksites); err != nil {
		return nil, fmt.Errorf("failed to generate worksites.tf: %w", err)
	}

	userGroupsWritten, err := imp.generateUserGroupsFile(userGroups, namedUserGroups)
	if err != nil {
		return nil, fmt.Errorf("failed to generate user_groups.tf: %w", err)
	}

	if err := imp.generateAssetsFile(assets, namedAssets, lookup); err != nil {
		return nil, fmt.Errorf("failed to generate assets.tf: %w", err)
	}

	return &Result{
		Labels:        len(labels),
		LabelGroups:   len(labelGroups),
		PolicyRules:   len(policyRules),
		DnsBlocklists: len(dnsBlocklists),
		Incidents:     len(incidents),
		Worksites:     len(worksites),
		UserGroups:    userGroupsWritten,
		Assets:        len(assets),
	}, nil
}

func (imp *Importer) generateLabelsFile(labels []client.Label, named []NamedResource) error {
	labelByID := make(map[string]client.Label, len(labels))
	for _, l := range labels {
		labelByID[l.ID] = l
	}

	var buf bytes.Buffer
	for i, nr := range named {
		l := labelByID[nr.ID]

		var criteria []CriteriaData
		for _, c := range l.DynamicCriteria {
			criteria = append(criteria, CriteriaData{
				Field:    c.Field,
				Op:       c.Op,
				Argument: c.Argument,
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
		buf.WriteString("        label_ids = [")
		for j, label := range or.AndLabels {
			if j > 0 {
				buf.WriteString(", ")
			}
			if ref, ok := lookup.Labels[label.ID]; ok {
				buf.WriteString(ref)
			} else {
				fmt.Fprintf(&buf, "%q", label.ID)
				buf.WriteString(" # reference not imported")
			}
		}
		buf.WriteString("]\n")
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

func (imp *Importer) generateWorksitesFile(worksites []client.Worksite) error {
	idToName := make(map[string]string, len(worksites))
	for _, w := range worksites {
		idToName[w.ID] = SanitizeName("", w.Name)
	}
	named := DeduplicateNames(idToName)

	worksiteByID := make(map[string]client.Worksite, len(worksites))
	for _, w := range worksites {
		worksiteByID[w.ID] = w
	}

	var buf bytes.Buffer
	for i, nr := range named {
		w := worksiteByID[nr.ID]

		data := WorksiteTemplateData{
			Name:         nr.Name,
			ID:           w.ID,
			ResourceName: w.Name,
			Comment:      w.Comment,
		}

		if err := worksiteTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render worksite %s: %w", w.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "worksites.tf"), buf.Bytes(), 0644)
}

func (imp *Importer) generateUserGroupsFile(userGroups []client.UserGroup, named []NamedResource) (int, error) {
	userGroupByID := make(map[string]client.UserGroup, len(userGroups))
	for _, ug := range userGroups {
		userGroupByID[ug.ID] = ug
	}

	var buf bytes.Buffer
	written := 0
	for _, nr := range named {
		ug := userGroupByID[nr.ID]

		var orchGroups []OrchestrationGroupData
		for _, og := range ug.OrchestrationsGroups {
			orchGroups = append(orchGroups, OrchestrationGroupData{
				OrchestrationID: og.OrchestrationID,
				Groups:          og.Groups,
			})
		}

		// Skip user groups with no orchestration groups — schema requires at least one.
		if len(orchGroups) == 0 {
			fmt.Fprintf(os.Stderr, "Note: skipping user group %q (%s): no orchestration groups\n", ug.Title, ug.ID)
			continue
		}

		data := UserGroupTemplateData{
			Name:                 nr.Name,
			ID:                   ug.ID,
			Title:                ug.Title,
			OrchestrationsGroups: orchGroups,
		}

		if err := userGroupTemplate.Execute(&buf, data); err != nil {
			return 0, fmt.Errorf("failed to render user group %s: %w", ug.ID, err)
		}
		if written > 0 {
			buf.WriteString("\n")
		}
		written++
	}

	return written, os.WriteFile(filepath.Join(imp.OutputDir, "user_groups.tf"), buf.Bytes(), 0644)
}

func (imp *Importer) generateAssetsFile(assets []client.Asset, named []NamedResource, lookup *ResourceLookup) error {
	assetByID := make(map[string]client.Asset, len(assets))
	for _, a := range assets {
		assetByID[a.ID] = a
	}

	var buf bytes.Buffer
	for i, nr := range named {
		a := assetByID[nr.ID]

		var nics []AssetNICData
		for _, nic := range a.Nics {
			nics = append(nics, AssetNICData{
				IPAddresses: nic.IPAddresses,
				MacAddress:  nic.MacAddress,
				VifID:       nic.VifID,
				NetworkName: nic.NetworkName,
			})
		}

		var labelRefs []AssetLabelRefData
		for _, label := range a.Labels {
			refData := AssetLabelRefData{}
			if ref, ok := lookup.Labels[label.ID]; ok {
				// Derive .id, .key, and .value references from the lookup
				base := strings.TrimSuffix(ref, ".id")
				refData.IDExpression = ref
				refData.KeyExpression = base + ".key"
				refData.ValueExpression = base + ".value"
			} else {
				// Fallback to quoted literals
				if label.ID != "" {
					refData.IDExpression = fmt.Sprintf("%q", label.ID)
				}
				refData.KeyExpression = fmt.Sprintf("%q", label.Key)
				refData.ValueExpression = fmt.Sprintf("%q", label.Value)
			}
			labelRefs = append(labelRefs, refData)
		}

		// Resolve worksite reference
		var worksiteExpr string
		if a.ScopingDetails != nil && a.ScopingDetails.Worksite != nil && a.ScopingDetails.Worksite.ID != "" {
			wsID := a.ScopingDetails.Worksite.ID
			if ref, found := lookup.Worksites[wsID]; found {
				worksiteExpr = ref
			} else {
				worksiteExpr = fmt.Sprintf("%q", wsID)
			}
		}

		// orchestration_obj_id is nested inside orchestration_details[] in the API response.
		// Fall back to the top-level field (may exist in some API versions), then to a placeholder.
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
			BiosUUID:           a.BiosUUID,
			Comments:           a.Comments,
			Status:             a.Status,
			Nics:               nics,
			Labels:             labelRefs,
			WorksiteExpression: worksiteExpr,
		}

		if err := assetTemplate.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render asset %s: %w", a.ID, err)
		}
		if i < len(named)-1 {
			buf.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(imp.OutputDir, "assets.tf"), buf.Bytes(), 0644)
}

// buildPolicyRuleBodyHCL generates typed HCL attributes for a policy rule,
// using raw_spec_json only for fields without a typed representation.
func buildPolicyRuleBodyHCL(spec map[string]interface{}, lookup *ResourceLookup) string {
	if spec == nil {
		return ""
	}

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
			labelID, ok := rawLabelID.(string)
			if !ok {
				continue
			}
			if tfRef, found := refMap[labelID]; found {
				andLabels[i] = tfRef
			}
		}
	}
}

func formatHCLObject(value map[string]interface{}, indent int) string {
	return formatHCLValue(value, indent)
}

func formatHCLValue(value interface{}, indent int) string {
	padding := strings.Repeat("  ", indent)
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "guardicore_") {
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
