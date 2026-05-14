package importer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func TestGenerateLabelsFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	labels := []client.Label{
		{
			ID:    "label-1",
			Key:   "Environment",
			Value: "Production",
		},
		{
			ID:    "label-2",
			Key:   "Application",
			Value: "Web Server",
			DynamicCriteria: []client.LabelCriteria{
				{Field: "scoping_details.worksite.id", Op: "", Argument: "worksite-id", Source: stringPtr("Worksite"), ReadOnly: boolPtr(true)},
				{Field: "name", Op: "CONTAINS", Argument: "web"},
			},
		},
	}

	idToName := map[string]string{
		"label-1": SanitizeName("Environment", "Production"),
		"label-2": SanitizeName("Application", "Web Server"),
	}
	named := DeduplicateNames(idToName)

	err := imp.generateLabelsFile(labels, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "labels.tf"))
	if err != nil {
		t.Fatalf("failed to read labels.tf: %v", err)
	}

	s := string(content)

	// Check resource blocks
	if !strings.Contains(s, `resource "guardicore_label" "environment_production"`) {
		t.Error("expected label resource block for environment_production")
	}
	if !strings.Contains(s, `resource "guardicore_label" "application_web_server"`) {
		t.Error("expected label resource block for application_web_server")
	}

	// Check import blocks
	if !strings.Contains(s, "to = guardicore_label.environment_production") {
		t.Error("expected import block for environment_production")
	}
	if !strings.Contains(s, `id = "label-1"`) {
		t.Error("expected import block with label-1 ID")
	}

	// Check criteria block
	if !strings.Contains(s, `field    = "name"`) {
		t.Error("expected criteria block with field name")
	}
	if !strings.Contains(s, `op       = "CONTAINS"`) {
		t.Error("expected criteria block with op CONTAINS")
	}
	if strings.Contains(s, `field    = "scoping_details.worksite.id"`) {
		t.Error("expected read-only Worksite-generated criteria to be skipped")
	}
	if strings.Contains(s, `field    = ""`) || strings.Contains(s, `op       = ""`) || strings.Contains(s, `argument = ""`) {
		t.Error("expected generated label criteria to never contain blank placeholder values")
	}
}

func TestGenerateLabelsFile_MultipleFlatCriteria(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	labels := []client.Label{
		{
			ID:    "label-1",
			Key:   "Role",
			Value: "App",
			DynamicCriteria: []client.LabelCriteria{
				{Field: "name", Op: "STARTSWITH", Argument: "app-"},
				{Field: "name", Op: "CONTAINS", Argument: "prod"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"label-1": SanitizeName("Role", "App")})

	err := imp.generateLabelsFile(labels, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "labels.tf"))
	if err != nil {
		t.Fatalf("failed to read labels.tf: %v", err)
	}

	s := string(content)
	if strings.Count(s, `field    = "name"`) != 2 {
		t.Fatalf("expected two flat criteria blocks, got:\n%s", s)
	}
	if strings.Contains(s, `compound_criteria = [`) {
		t.Fatalf("did not expect compound_criteria in flat-only case, got:\n%s", s)
	}
	if strings.Contains(s, `field    = ""`) || strings.Contains(s, `op       = ""`) || strings.Contains(s, `argument = ""`) {
		t.Fatalf("expected no blank placeholder criteria values, got:\n%s", s)
	}
}

func TestGenerateLabelsFile_CompoundCriteriaSingleItem(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	labels := []client.Label{
		{
			ID:    "label-1",
			Key:   "testDynamicLabelOrs",
			Value: "testDynamicLabelOrs",
			DynamicCriteria: []client.LabelCriteria{
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Argument: "Test", Op: "CONTAINS"}}},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Argument: "Test", Op: "STARTSWITH"}}},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"label-1": SanitizeName("testDynamicLabelOrs", "testDynamicLabelOrs")})

	err := imp.generateLabelsFile(labels, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "labels.tf"))
	if err != nil {
		t.Fatalf("failed to read labels.tf: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, `compound_criteria = [`) {
		t.Fatalf("expected compound_criteria to be rendered, got:\n%s", s)
	}
	if !strings.Contains(s, `field    = "container_command"`) || !strings.Contains(s, `field    = "container_labels"`) {
		t.Fatalf("expected compound criteria fields to be rendered, got:\n%s", s)
	}
	idxName := strings.Index(s, `field    = "name"`)
	idxCmd := strings.Index(s, `field    = "container_command"`)
	idxLabels := strings.Index(s, `field    = "container_labels"`)
	if idxName == -1 || idxCmd == -1 || idxLabels == -1 || idxName >= idxCmd || idxCmd >= idxLabels {
		t.Fatalf("expected criteria order name -> container_command -> container_labels, got:\n%s", s)
	}
	if strings.Contains(s, `field    = ""`) || strings.Contains(s, `op       = ""`) || strings.Contains(s, `argument = ""`) {
		t.Fatalf("expected no blank placeholder criteria values, got:\n%s", s)
	}
}

func TestGenerateLabelsFile_CompoundCriteriaMultiItem(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	labels := []client.Label{
		{
			ID:    "label-1",
			Key:   "testDynamicLabelOrsAnds",
			Value: "testDynamicLabelOrsAnds",
			DynamicCriteria: []client.LabelCriteria{
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{
					{Field: "image_name", Argument: "Test", Op: "STARTSWITH"},
					{Field: "container_command", Argument: "Test", Op: "STARTSWITH"},
					{Field: "container_labels", Argument: "Test", Op: "STARTSWITH"},
				}},
				{CompoundCriteria: []client.LabelCriteria{
					{Field: "image_name", Argument: "Test2", Op: "STARTSWITH"},
					{Field: "container_command", Argument: "Test2", Op: "STARTSWITH"},
					{Field: "container_labels", Argument: "Test2", Op: "STARTSWITH"},
				}},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"label-1": SanitizeName("testDynamicLabelOrsAnds", "testDynamicLabelOrsAnds")})

	err := imp.generateLabelsFile(labels, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "labels.tf"))
	if err != nil {
		t.Fatalf("failed to read labels.tf: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, `field    = "image_name"`) || !strings.Contains(s, `field    = "container_command"`) || !strings.Contains(s, `field    = "container_labels"`) {
		t.Fatalf("expected multi-item compound criteria fields to be rendered, got:\n%s", s)
	}
	idxName := strings.Index(s, `field    = "name"`)
	idxTestGroup := strings.Index(s, `argument = "Test"`)
	idxTest2Group := strings.Index(s, `argument = "Test2"`)
	if idxName == -1 || idxTestGroup == -1 || idxTest2Group == -1 || idxName >= idxTestGroup || idxTestGroup >= idxTest2Group {
		t.Fatalf("expected criteria order flat(Test) -> compound(Test) -> compound(Test2), got:\n%s", s)
	}
	if strings.Contains(s, `field    = ""`) || strings.Contains(s, `op       = ""`) || strings.Contains(s, `argument = ""`) {
		t.Fatalf("expected no blank placeholder criteria values, got:\n%s", s)
	}
}

func boolPtr(v bool) *bool { return &v }

func stringPtr(v string) *string { return &v }

func TestGenerateLabelGroupsFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	groups := []client.LabelGroup{
		{
			ID:       "group-1",
			Key:      "Role",
			Value:    "Web Servers",
			Comments: "All web servers",
			IncludeLabels: &client.OrLabelsRead{
				OrLabels: []client.AndLabelsRead{
					{
						AndLabels: []client.LabelInGroup{
							{ID: "label-1", Key: "Environment", Value: "Production"},
							{ID: "label-2", Key: "Application", Value: "Web"},
						},
					},
				},
			},
		},
	}

	idToName := map[string]string{"group-1": SanitizeName("Role", "Web Servers")}
	named := DeduplicateNames(idToName)

	// Build lookup with known labels to test reference generation
	lookup := &ResourceLookup{
		Labels: map[string]string{
			"label-1": "guardicore_label.environment_production.id",
			"label-2": "guardicore_label.application_web.id",
		},
		LabelGroups: map[string]string{},
		UserGroups:  map[string]string{},
		Assets:      map[string]string{},
	}

	err := imp.generateLabelGroupsFile(groups, named, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "label_groups.tf"))
	if err != nil {
		t.Fatalf("failed to read label_groups.tf: %v", err)
	}

	s := string(content)

	if !strings.Contains(s, `resource "guardicore_label_group" "role_web_servers"`) {
		t.Error("expected label group resource block")
	}
	if !strings.Contains(s, `comments = "All web servers"`) {
		t.Error("expected comments field")
	}
	if !strings.Contains(s, "to = guardicore_label_group.role_web_servers") {
		t.Error("expected import block")
	}

	// Verify typed include selector uses Terraform references instead of hardcoded IDs
	if !strings.Contains(s, "guardicore_label.environment_production.id") {
		t.Error("expected Terraform reference for label-1")
	}
	if !strings.Contains(s, "guardicore_label.application_web.id") {
		t.Error("expected Terraform reference for label-2")
	}
	if !strings.Contains(s, "include = {") || !strings.Contains(s, "or_groups = [") || !strings.Contains(s, "label_ids = [") {
		t.Error("expected typed include selector structure")
	}
	if strings.Contains(s, `"key"`) {
		t.Error("typed include selector should contain only IDs/references, not full label objects")
	}
	if strings.Contains(s, "include_labels = jsonencode(") || strings.Contains(s, "exclude_labels = jsonencode(") {
		t.Error("expected importer to avoid legacy include_labels/exclude_labels syntax")
	}
	if strings.Contains(s, `resource "centra_label_group"`) || strings.Contains(s, "to = centra_label_group.") {
		t.Error("expected importer to avoid legacy centra label group syntax")
	}
}

func TestGeneratePolicyRulesFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	rules := []map[string]interface{}{
		{
			"id":               "rule-1",
			"action":           "ALLOW",
			"enabled":          true,
			"comments":         "Allow web traffic",
			"section_position": "ALLOW",
			"source":           map[string]interface{}{"labels": map[string]interface{}{"or_labels": []interface{}{map[string]interface{}{"and_labels": []interface{}{"label-1"}}}}},
			"destination":      map[string]interface{}{"labels": map[string]interface{}{"or_labels": []interface{}{map[string]interface{}{"and_labels": []interface{}{"label-2"}}}}},
			"ports":            []interface{}{float64(443), float64(80)},
			"ip_protocols":     []interface{}{"TCP"},
			// Server-side fields that should be stripped
			"created_at": "2024-01-01",
			"state":      "PUBLISHED",
			"hit_count":  float64(100),
		},
	}

	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	err := imp.generatePolicyRulesFile(rules, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "policy_rules.tf"))
	if err != nil {
		t.Fatalf("failed to read policy_rules.tf: %v", err)
	}

	s := string(content)

	if !strings.Contains(s, `resource "guardicore_policy_rule" "allow_web_traffic"`) {
		t.Error("expected policy rule resource block")
	}
	if !strings.Contains(s, `action           = "ALLOW"`) {
		t.Error("expected typed action attribute")
	}
	if !strings.Contains(s, `ports = [`) {
		t.Error("expected typed ports attribute")
	}
	if !strings.Contains(s, "to = guardicore_policy_rule.allow_web_traffic") {
		t.Error("expected import block")
	}
	if !strings.Contains(s, `id = "rule-1"`) {
		t.Error("expected import block with rule-1 ID")
	}

	// Verify server-side fields are stripped
	if strings.Contains(s, "created_at") {
		t.Error("expected created_at to be stripped")
	}
	if strings.Contains(s, "hit_count") {
		t.Error("expected hit_count to be stripped")
	}
	if strings.Contains(s, `"state"`) {
		t.Error("expected state to be stripped")
	}
}

func TestBuildPolicyRuleBodyHCL_StripsReadOnlyAndEmptyTypedEndpoints(t *testing.T) {
	spec := map[string]interface{}{
		"action":          "ALLOW",
		"source":          map[string]interface{}{},
		"destination":     map[string]interface{}{},
		"recently_hit":    false,
		"creation_origin": "AUTO",
	}
	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	body := buildPolicyRuleBodyHCL(spec, lookup)

	if strings.Contains(body, "raw_spec_json") {
		t.Fatalf("expected no raw_spec_json, got:\n%s", body)
	}
	if strings.Contains(body, "recently_hit") {
		t.Fatalf("expected recently_hit to be stripped, got:\n%s", body)
	}
	if strings.Contains(body, "creation_origin") {
		t.Fatalf("expected creation_origin to be stripped, got:\n%s", body)
	}
	if strings.Contains(body, "source =") {
		t.Fatalf("expected empty source to be stripped, got:\n%s", body)
	}
	if strings.Contains(body, "destination =") {
		t.Fatalf("expected empty destination to be stripped, got:\n%s", body)
	}
}

func TestBuildPolicyRuleBodyHCL_ConvertsEndpointLabelObjectsToIDs(t *testing.T) {
	spec := map[string]interface{}{
		"source": map[string]interface{}{
			"labels": map[string]interface{}{
				"or_labels": []interface{}{
					map[string]interface{}{
						"and_labels": []interface{}{
							map[string]interface{}{
								"id":    "label-1",
								"key":   "os_name",
								"value": "Ubuntu 16.04 LTS",
							},
							map[string]interface{}{
								"id":    "label-2",
								"key":   "role",
								"value": "web",
							},
						},
					},
				},
			},
		},
	}
	lookup := &ResourceLookup{
		Labels:       map[string]string{"label-2": "guardicore_label.role_web.id"},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	body := buildPolicyRuleBodyHCL(spec, lookup)

	if !strings.Contains(body, `"label-1"`) {
		t.Fatalf("expected unresolved label object to become quoted label ID, got:\n%s", body)
	}
	if !strings.Contains(body, "guardicore_label.role_web.id") {
		t.Fatalf("expected imported label object to become Terraform reference, got:\n%s", body)
	}
	if strings.Contains(body, "os_name") || strings.Contains(body, "Ubuntu 16.04 LTS") || strings.Contains(body, `"web"`) {
		t.Fatalf("expected endpoint label objects to be collapsed to IDs, got:\n%s", body)
	}
}

func TestBuildPolicyRuleBodyHCL_PreservesEmptyICMPCodes(t *testing.T) {
	spec := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
		"icmp_matches": []interface{}{
			map[string]interface{}{
				"icmp_type":  float64(0),
				"version":    "4",
				"icmp_codes": []interface{}{},
			},
		},
	}

	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	body := buildPolicyRuleBodyHCL(spec, lookup)

	if !strings.Contains(body, `icmp_matches = [`) {
		t.Fatalf("expected icmp_matches to remain typed, got:\n%s", body)
	}
	if !strings.Contains(body, "icmp_codes") {
		t.Fatalf("expected empty icmp_codes to be preserved, got:\n%s", body)
	}
}

func TestBuildPolicyRuleBodyHCL_DefaultsMissingICMPCodes(t *testing.T) {
	spec := map[string]interface{}{
		"action":           "ALLOW",
		"section_position": "ALLOW",
		"enabled":          true,
		"icmp_matches": []interface{}{
			map[string]interface{}{
				"icmp_type": float64(8),
				"version":   "4",
			},
		},
	}

	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	body := buildPolicyRuleBodyHCL(spec, lookup)

	if !strings.Contains(body, `icmp_matches = [`) {
		t.Fatalf("expected icmp_matches to remain typed, got:\n%s", body)
	}
	if !strings.Contains(body, "icmp_codes") {
		t.Fatalf("expected missing icmp_codes to default to empty list, got:\n%s", body)
	}
}

func TestGenerateDnsSecurityFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	blocklists := []client.DnsBlocklist{
		{
			ID:      "dns-1",
			Name:    "Malware Domains",
			Type:    "CUSTOM_BLOCK",
			Domains: []string{"evil.com", "malware.org"},
			Enabled: true,
		},
		{
			ID:      "dns-2",
			Name:    "Gambling Category",
			Type:    "WEB_CATEGORY",
			Enabled: false,
		},
	}

	err := imp.generateDnsSecurityFile(blocklists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "dns_security.tf"))
	if err != nil {
		t.Fatalf("failed to read dns_security.tf: %v", err)
	}

	s := string(content)

	// Check resource blocks
	if !strings.Contains(s, `resource "guardicore_dns_security" "malware_domains"`) {
		t.Error("expected dns security resource block for malware_domains")
	}
	if strings.Contains(s, `resource "guardicore_dns_security" "gambling_category"`) {
		t.Error("expected no dns security resource block for system-managed web category")
	}
	if !strings.Contains(s, `data "guardicore_dns_security" "gambling_category"`) {
		t.Error("expected dns security data source block for gambling_category")
	}

	// Check import blocks
	if !strings.Contains(s, "to = guardicore_dns_security.malware_domains") {
		t.Error("expected import block for malware_domains")
	}
	if !strings.Contains(s, `id = "dns-1"`) {
		t.Error("expected import block with dns-1 ID")
	}
	if !strings.Contains(s, `id = "dns-2"`) {
		t.Error("expected dns-2 ID in dns_security output")
	}

	// Check domains
	if !strings.Contains(s, `"evil.com"`) {
		t.Error("expected domain evil.com in blocklist")
	}
	if !strings.Contains(s, `"malware.org"`) {
		t.Error("expected domain malware.org in blocklist")
	}

	// Check type
	if !strings.Contains(s, `type = "CUSTOM_BLOCK"`) {
		t.Error("expected type CUSTOM_BLOCK")
	}
	if strings.Contains(s, `type = "WEB_CATEGORY"`) {
		t.Error("expected no type field in system-managed data source block")
	}

	// Check name
	if !strings.Contains(s, `name = "Malware Domains"`) {
		t.Error("expected name Malware Domains")
	}
	if strings.Contains(s, `name = "Gambling Category"`) {
		t.Error("expected no name field in system-managed data source block")
	}
}

func TestGenerateWorksitesFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	worksites := []client.Worksite{
		{
			ID:      "worksite-1",
			Name:    "Headquarters",
			Comment: "Main office building",
		},
		{
			ID:   "worksite-2",
			Name: "Branch Office",
		},
		{
			ID:      "worksite-3",
			Name:    "Test",
			Comment: "rwar xomwa\n",
		},
	}

	idToName := map[string]string{
		"worksite-1": SanitizeName("", "Headquarters"),
		"worksite-2": SanitizeName("", "Branch Office"),
		"worksite-3": SanitizeName("", "Test"),
	}
	named := DeduplicateNames(idToName)

	err := imp.generateWorksitesFile(worksites, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "worksites.tf"))
	if err != nil {
		t.Fatalf("failed to read worksites.tf: %v", err)
	}

	s := string(content)

	// Check resource blocks
	if !strings.Contains(s, `resource "guardicore_worksite" "headquarters"`) {
		t.Error("expected worksite resource block for headquarters")
	}
	if !strings.Contains(s, `resource "guardicore_worksite" "branch_office"`) {
		t.Error("expected worksite resource block for branch_office")
	}

	// Check import blocks
	if !strings.Contains(s, "to = guardicore_worksite.headquarters") {
		t.Error("expected import block for headquarters")
	}
	if !strings.Contains(s, `id = "worksite-1"`) {
		t.Error("expected import block with worksite-1 ID")
	}
	if !strings.Contains(s, `id = "worksite-2"`) {
		t.Error("expected import block with worksite-2 ID")
	}

	// Check name and comment
	if !strings.Contains(s, `name = "Headquarters"`) {
		t.Error("expected name Headquarters")
	}
	if !strings.Contains(s, `comment = "Main office building"`) {
		t.Error("expected comment for headquarters")
	}

	// Branch office should not have a comment line
	if strings.Contains(s, `comment = ""`) {
		t.Error("branch office should not have an empty comment attribute")
	}
	if !strings.Contains(s, `comment = "rwar xomwa\n"`) {
		t.Error("expected multiline worksite comment to be escaped")
	}
	if strings.Contains(s, "comment = \"rwar xomwa\n\"") {
		t.Error("worksite comment contains a literal newline inside quoted HCL string")
	}
}

func TestGenerateIncidentsFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	incidents := []map[string]interface{}{
		{
			"id":       "incident-abc-123",
			"type":     "Network Scan",
			"severity": "HIGH",
			"time":     float64(1504688829035),
			"tags": map[string]interface{}{
				"data":  []interface{}{"Internal", "Critical"},
				"count": 2,
			},
			"affected_assets": map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"type": "IP", "display_name": "10.0.0.1", "value": "000000000000000000000000000000000000167772161"},
				},
				"count": 1,
			},
		},
	}

	err := imp.generateIncidentsFile(incidents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "incidents.tf"))
	if err != nil {
		t.Fatalf("failed to read incidents.tf: %v", err)
	}

	s := string(content)

	// Verify it's a commented-out resource block
	if !strings.Contains(s, "# resource \"guardicore_incident\"") {
		t.Error("expected commented-out guardicore_incident resource block")
	}
	if !strings.Contains(s, "NOTE: Incidents are immutable") {
		t.Error("expected immutability note")
	}
	// Verify type and severity
	if !strings.Contains(s, `type        = "Network Scan"`) {
		t.Error("expected incident type 'Network Scan'")
	}
	if !strings.Contains(s, `severity    = "HIGH"`) {
		t.Error("expected severity 'HIGH'")
	}
	// Verify tags
	if !strings.Contains(s, `"Internal"`) {
		t.Error("expected tag 'Internal'")
	}
	if !strings.Contains(s, `"Critical"`) {
		t.Error("expected tag 'Critical'")
	}
	// Verify affected_assets_json
	if !strings.Contains(s, "affected_assets_json") {
		t.Error("expected affected_assets_json field")
	}
	if !strings.Contains(s, "10.0.0.1") {
		t.Error("expected IP 10.0.0.1 in affected assets")
	}
	if !strings.Contains(s, "#   [") {
		t.Error("expected commented JSON array start in affected_assets_json")
	}
	if strings.Contains(s, "\n    {\n") {
		t.Error("found uncommented JSON object line in affected_assets_json")
	}
}

func TestGenerateUserGroupsFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	userGroups := []client.UserGroup{
		{
			ID:    "ug-1",
			Title: "Development Team",
			GroupsByDomainName: map[string]client.DomainGroupInfo{
				"corp.example.com": {
					Groups:          []client.DomainGroup{{ID: "group-a", Name: "Group A"}},
					OrchestrationID: "orch-uuid-1",
				},
			},
		},
	}

	idToNameUG := map[string]string{"ug-1": SanitizeName("", "Development Team")}
	namedUG := DeduplicateNames(idToNameUG)

	written, err := imp.generateUserGroupsFile(userGroups, namedUG, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 1 {
		t.Errorf("expected 1 user group written, got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "user_groups.tf"))
	if err != nil {
		t.Fatalf("failed to read user_groups.tf: %v", err)
	}

	s := string(content)

	if !strings.Contains(s, `resource "guardicore_user_group"`) {
		t.Error("expected guardicore_user_group resource block")
	}
	if !strings.Contains(s, `title = "Development Team"`) {
		t.Error("expected title 'Development Team'")
	}
	if !strings.Contains(s, `orchestration_id = "orch-uuid-1"`) {
		t.Error("expected orchestration_id extracted from groups_by_domain_name")
	}
	if !strings.Contains(s, `"group-a"`) {
		t.Error("expected group ID 'group-a' extracted from groups_by_domain_name")
	}
	if !strings.Contains(s, "import {") {
		t.Error("expected import block")
	}
	if !strings.Contains(s, `id = "ug-1"`) {
		t.Error("expected import id 'ug-1'")
	}
}

func TestGenerateUserGroupsFile_SkipsLocalGroups(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	userGroups := []client.UserGroup{
		{
			ID:    "local_administrators",
			Title: "Local Administrators",
			GroupsByDomainName: map[string]client.DomainGroupInfo{
				"Local": {
					Groups:          []client.DomainGroup{{ID: "local_administrators", Name: "Local Administrators"}},
					OrchestrationID: "local",
				},
			},
		},
		{
			ID:    "ug-real",
			Title: "AD Security Group",
			GroupsByDomainName: map[string]client.DomainGroupInfo{
				"corp.example.com": {
					Groups:          []client.DomainGroup{{ID: "ad-group-1", Name: "Security Team"}},
					OrchestrationID: "orch-uuid-real",
				},
			},
		},
	}

	idToName := map[string]string{
		"local_administrators": SanitizeName("", "Local Administrators"),
		"ug-real":              SanitizeName("", "AD Security Group"),
	}
	named := DeduplicateNames(idToName)

	systemManagedIDs := map[string]struct{}{
		"local_administrators": {},
	}
	written, err := imp.generateUserGroupsFile(userGroups, named, systemManagedIDs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 2 {
		t.Errorf("expected 2 user groups written (1 data source + 1 resource), got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "user_groups.tf"))
	if err != nil {
		t.Fatalf("failed to read user_groups.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `data "guardicore_user_group" "local_administrators"`) {
		t.Error("expected data source block for system-managed local user group")
	}
	if !strings.Contains(s, `title = "Local Administrators"`) {
		t.Error("expected title in data source block for system-managed user group")
	}
	if !strings.Contains(s, `resource "guardicore_user_group"`) {
		t.Error("expected resource block for non-local user group")
	}
	if !strings.Contains(s, `id = "ug-real"`) {
		t.Error("expected non-local user group to be included")
	}
}

func TestGenerateUserGroupsFile_KeepsMixedOrchestrations(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	userGroups := []client.UserGroup{
		{
			ID:    "ug-mixed",
			Title: "Mixed Group",
			GroupsByDomainName: map[string]client.DomainGroupInfo{
				"Local": {
					Groups:          []client.DomainGroup{{ID: "local_users", Name: "Local Users"}},
					OrchestrationID: "local",
				},
				"corp.example.com": {
					Groups:          []client.DomainGroup{{ID: "ad-group", Name: "AD Group"}},
					OrchestrationID: "orch-uuid-abc",
				},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"ug-mixed": SanitizeName("", "Mixed Group")})

	written, err := imp.generateUserGroupsFile(userGroups, named, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 1 {
		t.Errorf("expected 1 user group written, got %d", written)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "user_groups.tf"))
	s := string(content)

	if !strings.Contains(s, `resource "guardicore_user_group"`) {
		t.Error("expected resource block for mixed-orchestration user group")
	}
	if !strings.Contains(s, `id = "ug-mixed"`) {
		t.Error("expected mixed-orchestration user group to be included")
	}
}

func TestGenerateAssetsFile(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{
		OutputDir: tmpDir,
	}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "web-server-01",
			Status:             "on",
			OrchestrationObjID: "orch-123",
			Comments:           "Production web server",
			Nics: []client.AssetNIC{
				{
					IPAddresses: []string{"10.0.0.1", "10.0.0.2"},
					MacAddress:  "00:11:22:33:44:55",
				},
			},
		},
	}

	idToNameA := map[string]string{"asset-1": SanitizeName("", "web-server-01")}
	namedA := DeduplicateNames(idToNameA)
	lookup := &ResourceLookup{
		Labels:      map[string]string{},
		LabelGroups: map[string]string{},
		UserGroups:  map[string]string{},
		Assets:      map[string]string{},
	}

	_, err := imp.generateAssetsFile(context.Background(), assets, namedA, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}

	s := string(content)

	if !strings.Contains(s, `resource "guardicore_asset"`) {
		t.Error("expected guardicore_asset resource block")
	}
	if !strings.Contains(s, `name                 = "web-server-01"`) {
		t.Error("expected name 'web-server-01'")
	}
	if !strings.Contains(s, `orchestration_obj_id = "orch-123"`) {
		t.Error("expected orchestration_obj_id 'orch-123'")
	}
	if strings.Contains(s, orchObjIDPlaceholder) {
		t.Error("should not contain placeholder when OrchestrationObjID is provided")
	}
	if !strings.Contains(s, `comments = "Production web server"`) {
		t.Error("expected comments")
	}
	if !strings.Contains(s, `status = "on"`) {
		t.Error("expected status 'on'")
	}
	// Verify NIC block
	if !strings.Contains(s, `"10.0.0.1"`) {
		t.Error("expected IP 10.0.0.1")
	}
	if !strings.Contains(s, `"10.0.0.2"`) {
		t.Error("expected IP 10.0.0.2")
	}
	if !strings.Contains(s, `mac_address  = "00:11:22:33:44:55"`) {
		t.Error("expected MAC address")
	}
	// Verify import block
	if !strings.Contains(s, "import {") {
		t.Error("expected import block")
	}
	if !strings.Contains(s, `id = "asset-1"`) {
		t.Error("expected import id 'asset-1'")
	}
	if !strings.Contains(s, "to = guardicore_asset.") {
		t.Error("expected import to reference")
	}
}

func TestGenerateAssetsFile_OrchestrationDetailsExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:     "asset-uuid-123",
			Name:   "vcenter-vm",
			Status: "on",
			OrchestrationDetails: []client.OrchestrationDetail{
				{
					OrchestrationID:    "orch-id-1",
					OrchestrationName:  "VCENTER",
					OrchestrationObjID: "vm-53242",
					OrchestrationType:  "vSphere",
				},
			},
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "aa:bb:cc:dd:ee:ff"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-uuid-123": SanitizeName("", "vcenter-vm")})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	if _, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `orchestration_obj_id = "vm-53242"`) {
		t.Error("expected orchestration_obj_id extracted from orchestration_details")
	}
	if strings.Contains(s, "asset-uuid-123") && strings.Contains(s, "orchestration_obj_id = \"asset-uuid-123\"") {
		t.Error("orchestration_obj_id should NOT fall back to asset ID")
	}
	if strings.Contains(s, orchObjIDPlaceholder) {
		t.Error("should not contain placeholder when orchestration_details has a value")
	}
	if strings.Contains(s, "# TODO") {
		t.Error("should not contain TODO comment when value is available")
	}
}

func TestGenerateAssetsFile_EmptyOrchestrationObjID(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:     "asset-uuid-456",
			Name:   "bare-metal-server",
			Status: "on",
			// No OrchestrationDetails and no OrchestrationObjID — simulates asset without orchestration.
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.2"}, MacAddress: "11:22:33:44:55:66"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-uuid-456": SanitizeName("", "bare-metal-server")})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	if _, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if strings.Contains(s, `orchestration_obj_id = "asset-uuid-456"`) {
		t.Error("orchestration_obj_id should NOT fall back to asset ID")
	}
	if !strings.Contains(s, orchObjIDPlaceholder) {
		t.Errorf("expected placeholder %q in output", orchObjIDPlaceholder)
	}
	if !strings.Contains(s, "# TODO") {
		t.Error("expected TODO comment explaining the placeholder")
	}
}

func TestGenerateAssetsFile_CreateOnlyFields(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-co-1",
			Name:               "cloud-vm",
			Status:             "on",
			OrchestrationObjID: "orch-x",
			InstanceID:         "i-0abc123def456",
			HwUUID:             "hw-uuid-789",
			BiosUUID:           "bios-uuid-012",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.5"}, MacAddress: "aa:bb:cc:dd:ee:01"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-co-1": SanitizeName("", "cloud-vm")})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	if _, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `instance_id          = "i-0abc123def456"`) {
		t.Error("expected instance_id in output")
	}
	if !strings.Contains(s, `hw_uuid              = "hw-uuid-789"`) {
		t.Error("expected hw_uuid in output")
	}
}

func TestGenerateAssetsFile_CreateOnlyFieldsOmittedWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-co-2",
			Name:               "agent-only",
			Status:             "on",
			OrchestrationObjID: "orch-y",
			// InstanceID, HwUUID, BiosUUID all empty
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.6"}, MacAddress: "aa:bb:cc:dd:ee:02"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-co-2": SanitizeName("", "agent-only")})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	if _, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if strings.Contains(s, "instance_id") {
		t.Error("instance_id should be omitted when empty")
	}
	if strings.Contains(s, "hw_uuid") {
		t.Error("hw_uuid should be omitted when empty")
	}
	if strings.Contains(s, "bios_uuid") {
		t.Error("bios_uuid should not be emitted")
	}
}

func TestGenerateLabelGroupsFile_UnresolvedRef(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	groups := []client.LabelGroup{
		{
			ID:    "group-1",
			Key:   "Role",
			Value: "DB",
			IncludeLabels: &client.OrLabelsRead{
				OrLabels: []client.AndLabelsRead{
					{
						AndLabels: []client.LabelInGroup{
							{ID: "unknown-label-id", Key: "env", Value: "dev"},
						},
					},
				},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"group-1": SanitizeName("Role", "DB")})
	// Empty label lookup — all label refs will be unresolvable
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	err := imp.generateLabelGroupsFile(groups, named, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "label_groups.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	// Unresolvable ref should fall back to quoted literal with comment
	if !strings.Contains(s, `"unknown-label-id"`) {
		t.Error("expected fallback to quoted literal for unresolvable label ID")
	}
	if !strings.Contains(s, `, # reference not imported`) {
		t.Error("expected '# reference not imported' comment after trailing comma")
	}
	if strings.Contains(s, `" # reference not imported]`) {
		t.Error("inline comment must not appear before closing bracket — produces invalid HCL")
	}
}

func TestGeneratePolicyRulesFile_WithRefs(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	rules := []map[string]interface{}{
		{
			"id":      "rule-1",
			"action":  "ALLOW",
			"enabled": true,
			"source":  map[string]interface{}{"label_group_ids": []interface{}{"lg-1"}},
			"destination": map[string]interface{}{
				"user_group_ids": []interface{}{
					map[string]interface{}{"id": "ug-1", "name": "Dev Team"},
					map[string]interface{}{"id": "local_administrators", "name": "Local Administrators"},
				},
				"asset_ids": []interface{}{"asset-1"},
			},
		},
	}

	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{"lg-1": "guardicore_label_group.web_servers.id"},
		PolicyGroups: map[string]string{},
		UserGroups: map[string]string{
			"ug-1":                 "guardicore_user_group.dev_team.id",
			"local_administrators": "data.guardicore_user_group.local_administrators.id",
		},
		Assets: map[string]string{"asset-1": "guardicore_asset.web_01.id"},
	}

	err := imp.generatePolicyRulesFile(rules, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "policy_rules.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `label_group_ids = [`) || !strings.Contains(s, "guardicore_label_group.web_servers.id") {
		t.Error("expected typed label group reference in policy rule body")
	}
	if !strings.Contains(s, `user_group_ids = [`) || !strings.Contains(s, "guardicore_user_group.dev_team.id") {
		t.Error("expected typed user group reference in policy rule body")
	}
	if !strings.Contains(s, "data.guardicore_user_group.local_administrators.id") {
		t.Error("expected system-managed user group to use data source reference")
	}
	if strings.Contains(s, `name = "Dev Team"`) {
		t.Error("expected user_group_ids entries to render as IDs only, not objects")
	}
	if !strings.Contains(s, `asset_ids = [`) || !strings.Contains(s, "guardicore_asset.web_01.id") {
		t.Error("expected typed asset reference in policy rule body")
	}
}

func TestGenerateAssetsFile_WithLabelRefs(t *testing.T) {
	tmpDir := t.TempDir()

	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
			},
			Labels: []client.AssetLabelRef{
				{ID: "label-1", Key: "env", Value: "prod"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{
		Labels:       map[string]string{"label-1": "guardicore_label.env_prod.id"},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	nonAssignable := map[string]string{"label-read-only": "read-only"}
	_, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, nonAssignable, map[string]struct{}{"label-user": {}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "guardicore_label.env_prod.id") {
		t.Error("expected label id reference in asset labels block")
	}
	if !strings.Contains(s, "guardicore_label.env_prod.key") {
		t.Error("expected label key reference in asset labels block")
	}
	if !strings.Contains(s, "guardicore_label.env_prod.value") {
		t.Error("expected label value reference in asset labels block")
	}
	if !strings.Contains(s, "labels = [") {
		t.Error("expected labels attribute list in asset")
	}
}

func TestGenerateAssetsFile_SkipsReadOnlyLabels(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}
	readOnly := true

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
			},
			Labels: []client.AssetLabelRef{
				{ID: "label-user", Key: "Role", Value: "Server"},
				{ID: "label-read-only", Key: "os_type", Value: "Linux", ReadOnly: &readOnly},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{
		Labels:       map[string]string{"label-user": "guardicore_label.role_server.id"},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}
	nonAssignable := map[string]string{"label-read-only": "read-only"}
	explicitAssignable := map[string]struct{}{"label-user": {}}
	_, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, nonAssignable, explicitAssignable)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "guardicore_label.role_server.id") {
		t.Error("expected user-managed label reference in asset labels block")
	}
	if strings.Contains(s, "label-read-only") || strings.Contains(s, "os_type") {
		t.Fatalf("expected read-only labels to be skipped, got:\n%s", s)
	}
}

func TestGenerateAssetsFile_SkipsDynamicLabels(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
			},
			Labels: []client.AssetLabelRef{
				{ID: "label-user", Key: "Role", Value: "Server"},
				{ID: "label-dynamic", Key: "app", Value: "database"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{
		Labels:       map[string]string{"label-user": "guardicore_label.role_server.id", "label-dynamic": "guardicore_label.app_database.id"},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	nonAssignable := map[string]string{"label-dynamic": "dynamic"}
	_, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, nonAssignable, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "guardicore_label.role_server.id") {
		t.Error("expected user-managed label reference in asset labels block")
	}
	if strings.Contains(s, "guardicore_label.app_database.id") || strings.Contains(s, "app") {
		t.Fatalf("expected dynamic labels to be skipped, got:\n%s", s)
	}
}

func TestBuildNonAssignableAssetLabelReasons(t *testing.T) {
	readOnly := true
	labels := []client.Label{
		{ID: "label-read-only", ReadOnly: &readOnly},
		{ID: "label-dynamic", DynamicCriteria: []client.LabelCriteria{{Field: "name", Op: "CONTAINS", Argument: "db"}}},
		{ID: "label-both", ReadOnly: &readOnly, DynamicCriteria: []client.LabelCriteria{{Field: "name", Op: "CONTAINS", Argument: "db"}}},
		{ID: "label-user"},
	}

	reasons := buildNonAssignableAssetLabelReasons(labels)

	if reasons["label-read-only"] != "read-only" {
		t.Fatalf("expected read-only reason, got %q", reasons["label-read-only"])
	}
	if reasons["label-dynamic"] != "dynamic" {
		t.Fatalf("expected dynamic reason, got %q", reasons["label-dynamic"])
	}
	if reasons["label-both"] != "read-only and dynamic" {
		t.Fatalf("expected combined reason, got %q", reasons["label-both"])
	}
	if _, ok := reasons["label-user"]; ok {
		t.Fatal("expected user label to be absent from non-assignable map")
	}
}

func TestBuildExplicitAssignableAssetLabelIDs(t *testing.T) {
	readOnlyTrue := true
	readOnlyFalse := false

	labels := []client.Label{
		{ID: "label-assignable", ReadOnly: &readOnlyFalse},
		{ID: "label-readonly", ReadOnly: &readOnlyTrue},
		{ID: "label-dynamic", ReadOnly: &readOnlyFalse, DynamicCriteria: []client.LabelCriteria{{Field: "name", Op: "CONTAINS", Argument: "db"}}},
		{ID: "label-unknown"},
	}

	assignable := buildExplicitAssignableAssetLabelIDs(labels)

	if _, ok := assignable["label-assignable"]; !ok {
		t.Fatal("expected label-assignable to be marked explicitly assignable")
	}
	if _, ok := assignable["label-readonly"]; ok {
		t.Fatal("did not expect read-only label to be marked explicitly assignable")
	}
	if _, ok := assignable["label-dynamic"]; ok {
		t.Fatal("did not expect dynamic label to be marked explicitly assignable")
	}
	if _, ok := assignable["label-unknown"]; ok {
		t.Fatal("did not expect label with unknown read_only to be marked explicitly assignable")
	}
}

func TestGenerateAssetsFile_SkipsIncompleteUnresolvedLabels(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
			},
			Labels: []client.AssetLabelRef{
				{ID: "label-resolved", Key: "Role", Value: "Server"},
				{ID: "label-id-only", Key: "", Value: ""},
				{ID: "label-missing-value", Key: "DynamicLabelKey", Value: ""},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{
		Labels:       map[string]string{"label-resolved": "guardicore_label.role_server.id"},
		LabelGroups:  map[string]string{},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{},
		Assets:       map[string]string{},
	}

	_, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "guardicore_label.role_server.id") {
		t.Error("expected resolved user-managed label reference in asset labels block")
	}
	if strings.Contains(s, `id    = "label-id-only"`) {
		t.Fatalf("expected unresolved id-only label to be skipped, got:\n%s", s)
	}
	if strings.Contains(s, `id    = "label-missing-value"`) {
		t.Fatalf("expected unresolved label with missing value to be skipped, got:\n%s", s)
	}
	if strings.Contains(s, `key   = ""`) || strings.Contains(s, `value = ""`) {
		t.Fatalf("expected no empty-string key/value labels, got:\n%s", s)
	}
}

func TestGenerateAssetsFile_UnknownAssignabilityIncludedWithWarning(t *testing.T) {
	labelsListCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels/"):
			// GetLabel by ID: return no object so assignability stays unknown.
			_ = json.NewEncoder(w).Encode(client.LabelGetResponse{Objects: []client.Label{}})
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels"):
			labelsListCalls++
			_ = json.NewEncoder(w).Encode(client.ListLabelsResponse{Objects: []client.Label{}, TotalCount: 0})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir, Client: apiClient}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics:               []client.AssetNIC{{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"}},
			Labels:             []client.AssetLabelRef{{ID: "label-unknown", Key: "Role", Value: "Server"}},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{Labels: map[string]string{}, LabelGroups: map[string]string{}, PolicyGroups: map[string]string{}, UserGroups: map[string]string{}, Assets: map[string]string{}}

	origStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create stderr pipe: %v", pipeErr)
	}
	os.Stderr = w

	_, err = imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	_ = w.Close()
	os.Stderr = origStderr
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderrBytes, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("failed to read stderr output: %v", readErr)
	}
	_ = r.Close()

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, `id    = "label-unknown"`) {
		t.Fatalf("expected unresolved label to be included, got:\n%s", s)
	}

	stderrOut := string(stderrBytes)
	if !strings.Contains(stderrOut, "unable to verify assignability") {
		t.Fatalf("expected unknown assignability warning, got: %s", stderrOut)
	}
	if labelsListCalls != 0 {
		t.Fatalf("expected no key/value list lookups when label is missing from GetLabel, got %d", labelsListCalls)
	}
}

func TestGenerateAssetsFile_SkipsReadOnlyViaLabelAPIWhenAssetLabelMetadataMissing(t *testing.T) {
	tmpDir := t.TempDir()
	readOnly := true

	labelsList := client.ListLabelsResponse{
		Objects:    []client.Label{{ID: "label-user", Key: "Role", Value: "Server", ReadOnly: boolPtr(false)}},
		TotalCount: 1,
	}
	labelGetReadOnly := client.LabelGetResponse{
		Objects: []client.Label{{ID: "label-read-only", Key: "os_type", Value: "Linux"}},
	}
	labelListReadOnly := client.ListLabelsResponse{
		Objects:    []client.Label{{ID: "label-read-only", Key: "os_type", Value: "Linux", ReadOnly: &readOnly}},
		TotalCount: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4.0/labels":
			key := r.URL.Query().Get("key")
			value := r.URL.Query().Get("value")
			if key == "" && value == "" {
				_ = json.NewEncoder(w).Encode(labelsList)
				return
			}
			if key == "os_type" && value == "Linux" {
				_ = json.NewEncoder(w).Encode(labelListReadOnly)
				return
			}
			_ = json.NewEncoder(w).Encode(client.ListLabelsResponse{Objects: []client.Label{}, TotalCount: 0})
		case "/api/v4.0/labels/label-read-only":
			_ = json.NewEncoder(w).Encode(labelGetReadOnly)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{BaseURL: server.URL, AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	imp := &Importer{OutputDir: tmpDir, Client: apiClient}

	assets := []client.Asset{
		{
			ID:                 "asset-1",
			Name:               "db-server",
			OrchestrationObjID: "orch-1",
			Nics:               []client.AssetNIC{{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"}},
			Labels: []client.AssetLabelRef{
				{ID: "label-user", Key: "Role", Value: "Server"},
				{ID: "label-read-only", Key: "os_type", Value: "Linux"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{"asset-1": SanitizeName("", "db-server")})
	lookup := &ResourceLookup{Labels: map[string]string{"label-user": "guardicore_label.role_server.id"}, LabelGroups: map[string]string{}, PolicyGroups: map[string]string{}, UserGroups: map[string]string{}, Assets: map[string]string{}}

	nonAssignable := buildNonAssignableAssetLabelReasons(labelsList.Objects)
	explicitAssignable := buildExplicitAssignableAssetLabelIDs(labelsList.Objects)

	_, err = imp.generateAssetsFile(context.Background(), assets, named, lookup, nonAssignable, explicitAssignable)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "guardicore_label.role_server.id") {
		t.Error("expected user-managed label reference in asset labels block")
	}
	if strings.Contains(s, "label-read-only") || strings.Contains(s, "os_type") {
		t.Fatalf("expected read-only labels to be skipped after label API resolution, got:\n%s", s)
	}
}

func TestShouldSkipAsset(t *testing.T) {
	tests := []struct {
		name     string
		asset    client.Asset
		expected bool
	}{
		{
			name:     "empty nics",
			asset:    client.Asset{ID: "a1", Name: "no-nics"},
			expected: true,
		},
		{
			name:     "nil nics",
			asset:    client.Asset{ID: "a2", Name: "nil-nics", Nics: nil},
			expected: true,
		},
		{
			name: "has nics",
			asset: client.Asset{
				ID:   "a3",
				Name: "with-nics",
				Nics: []client.AssetNIC{{IPAddresses: []string{"10.0.0.1"}, MacAddress: "aa:bb:cc:dd:ee:ff"}},
			},
			expected: false,
		},
		{
			name: "all nics have empty ip_addresses",
			asset: client.Asset{
				ID:   "a4",
				Name: "empty-ip-nics",
				Nics: []client.AssetNIC{
					{IPAddresses: []string{}, MacAddress: "aa:bb:cc:dd:ee:01"},
					{IPAddresses: nil, MacAddress: "aa:bb:cc:dd:ee:02"},
				},
			},
			expected: true,
		},
		{
			name: "mixed nics - some valid some empty",
			asset: client.Asset{
				ID:   "a5",
				Name: "mixed-nics",
				Nics: []client.AssetNIC{
					{IPAddresses: []string{}, MacAddress: "aa:bb:cc:dd:ee:01"},
					{IPAddresses: []string{"10.0.0.1"}, MacAddress: "aa:bb:cc:dd:ee:02"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipAsset(tt.asset)
			if got != tt.expected {
				t.Errorf("shouldSkipAsset() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPartitionLabels(t *testing.T) {
	readOnly := true
	labels := []client.Label{
		{ID: "l1", Key: "Env", Value: "Prod"},
		{ID: "l2", Key: "OS", Value: "Linux", ReadOnly: &readOnly},
		{ID: "l3", Key: "Role", Value: "Web"},
	}

	manageable, systemManaged := partitionLabels(labels)
	if len(systemManaged) != 1 {
		t.Fatalf("expected 1 system-managed label, got %d", len(systemManaged))
	}
	if systemManaged[0].ID != "l2" {
		t.Fatalf("expected system-managed label l2, got %s", systemManaged[0].ID)
	}
	if len(manageable) != 2 {
		t.Fatalf("expected 2 manageable labels, got %d", len(manageable))
	}
	if manageable[0].ID != "l1" || manageable[1].ID != "l3" {
		t.Fatalf("unexpected manageable labels: %+v", manageable)
	}
}

func TestGenerateAssetsFile_SkipsEmptyNics(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-no-nics",
			Name:               "stale-vm",
			Status:             "on",
			OrchestrationObjID: "orch-stale",
		},
		{
			ID:                 "asset-ok",
			Name:               "healthy-vm",
			Status:             "on",
			OrchestrationObjID: "orch-healthy",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "aa:bb:cc:dd:ee:ff"},
			},
		},
	}

	idToName := map[string]string{
		"asset-no-nics": SanitizeName("", "stale-vm"),
		"asset-ok":      SanitizeName("", "healthy-vm"),
	}
	named := DeduplicateNames(idToName)
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	written, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 1 {
		t.Errorf("expected 1 asset written, got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `# NOTE: Asset "stale-vm" has no valid NICs`) {
		t.Error("expected commented-out block for skipped asset with empty NICs")
	}
	if !strings.Contains(s, `# resource "guardicore_asset"`) {
		t.Error("expected commented-out resource block for skipped asset")
	}
	if !strings.Contains(s, `resource "guardicore_asset" "healthy_vm"`) {
		t.Error("expected resource block for asset with NICs")
	}
	if strings.Contains(s, `id = "asset-no-nics"`) {
		t.Error("skipped asset should not have an import block")
	}
}

func TestGenerateAssetsFile_AllEmptyNics(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-empty-1",
			Name:               "ghost-vm-1",
			OrchestrationObjID: "orch-1",
		},
		{
			ID:                 "asset-empty-2",
			Name:               "ghost-vm-2",
			OrchestrationObjID: "orch-2",
		},
	}

	named := DeduplicateNames(map[string]string{
		"asset-empty-1": SanitizeName("", "ghost-vm-1"),
		"asset-empty-2": SanitizeName("", "ghost-vm-2"),
	})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	written, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("expected 0 assets written, got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "# NOTE:") {
		t.Error("expected commented-out note blocks")
	}
	if strings.Contains(s, "import {") {
		t.Error("no import blocks should exist when all assets are skipped")
	}
}

func TestRun_Integration(t *testing.T) {
	labelsResp := client.ListLabelsResponse{
		Objects: []client.Label{
			{ID: "label-1", Key: "Environment", Value: "Production"},
		},
		TotalCount: 1,
	}

	labelGroupsResp := client.ListLabelGroupsResponse{
		Objects: []client.LabelGroup{
			{
				ID:       "group-1",
				Key:      "Role",
				Value:    "Web",
				Comments: "Web servers",
				IncludeLabels: &client.OrLabelsRead{
					OrLabels: []client.AndLabelsRead{
						{AndLabels: []client.LabelInGroup{{ID: "label-1"}}},
					},
				},
			},
		},
		TotalCount: 1,
	}

	policyRulesResp := client.ListPolicyRulesResponse{
		Objects: []map[string]interface{}{
			{
				"id":      "rule-1",
				"action":  "ALLOW",
				"enabled": true,
			},
		},
		TotalCount: 1,
	}

	dnsBlocklistsResp := client.ListDnsBlocklistsResponse{
		Objects: []client.DnsBlocklist{
			{
				ID:      "dns-1",
				Name:    "Test Blocklist",
				Type:    "custom",
				Domains: []string{"bad.com"},
				Enabled: true,
			},
		},
		TotalCount: 1,
	}

	incidentsResp := client.ListIncidentsResponse{
		Objects: []map[string]interface{}{
			{
				"id":       "incident-1",
				"type":     "Network Scan",
				"severity": "MEDIUM",
				"time":     float64(1504688829035),
				"tags": map[string]interface{}{
					"data":  []interface{}{"Internal"},
					"count": 1,
				},
				"affected_assets": map[string]interface{}{
					"data": []interface{}{
						map[string]interface{}{"type": "IP", "display_name": "10.0.0.1", "value": "000000000000000000000000000000000000167772161"},
					},
					"count": 1,
				},
			},
		},
		TotalCount: 1,
	}

	worksitesResp := client.ListWorksitesResponse{
		Objects: []client.Worksite{
			{
				ID:      "worksite-1",
				Name:    "Headquarters",
				Comment: "Main office",
			},
		},
		TotalCount: 1,
	}

	userGroupsResp := client.ListUserGroupsResponse{
		Objects: []client.UserGroup{
			{
				ID:    "ug-1",
				Title: "Development Team",
				GroupsByDomainName: map[string]client.DomainGroupInfo{
					"corp.example.com": {
						Groups: []client.DomainGroup{
							{ID: "group-1", Name: "Group 1"},
							{ID: "group-2", Name: "Group 2"},
						},
						OrchestrationID: "orch-1",
					},
				},
			},
		},
		TotalCount: 1,
	}

	assetsResp := client.ListAssetsResponse{
		Objects: []client.Asset{
			{
				ID:                 "asset-1",
				Name:               "web-server-01",
				Status:             "on",
				OrchestrationObjID: "orch-123",
				Nics: []client.AssetNIC{
					{IPAddresses: []string{"10.0.0.1"}, MacAddress: "00:11:22:33:44:55"},
				},
			},
			{
				ID:                 "asset-deleted",
				Name:               "deleted-server",
				Status:             "deleted",
				OrchestrationObjID: "orch-deleted",
				Nics: []client.AssetNIC{
					{IPAddresses: []string{"10.0.0.2"}, MacAddress: "00:11:22:33:44:66"},
				},
			},
			{
				ID:                 "asset-no-nics",
				Name:               "empty-nic-server",
				Status:             "on",
				OrchestrationObjID: "orch-no-nics",
			},
		},
		TotalCount: 3,
	}

	agentAggregatorsResp := client.ListAgentAggregatorsResponse{
		Objects: []client.AgentAggregator{
			{
				ID:       "agg-1",
				Hostname: "gc-aggregator-172-235-229-101",
			},
		},
		TotalCount: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/label-groups"):
			_ = json.NewEncoder(w).Encode(labelGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels"):
			_ = json.NewEncoder(w).Encode(labelsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/user-groups"):
			_ = json.NewEncoder(w).Encode(userGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/policy/rules"):
			_ = json.NewEncoder(w).Encode(policyRulesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/dns_security"):
			_ = json.NewEncoder(w).Encode(dnsBlocklistsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/worksites"):
			_ = json.NewEncoder(w).Encode(worksitesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/assets"):
			_ = json.NewEncoder(w).Encode(assetsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/generic-incidents"):
			_ = json.NewEncoder(w).Encode(incidentsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/agent_aggregators"):
			_ = json.NewEncoder(w).Encode(agentAggregatorsResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir := t.TempDir()
	imp := &Importer{
		Client:    apiClient,
		OutputDir: tmpDir,
	}

	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Labels != 1 {
		t.Errorf("expected 1 label, got %d", result.Labels)
	}
	if result.LabelGroups != 1 {
		t.Errorf("expected 1 label group, got %d", result.LabelGroups)
	}
	if result.PolicyRules != 1 {
		t.Errorf("expected 1 policy rule, got %d", result.PolicyRules)
	}
	if result.DnsBlocklists != 1 {
		t.Errorf("expected 1 DNS blocklist, got %d", result.DnsBlocklists)
	}
	if result.Incidents != 1 {
		t.Errorf("expected 1 incident, got %d", result.Incidents)
	}
	if result.Worksites != 1 {
		t.Errorf("expected 1 worksite, got %d", result.Worksites)
	}
	if result.UserGroups != 1 {
		t.Errorf("expected 1 user group, got %d", result.UserGroups)
	}
	if result.Assets != 1 {
		t.Errorf("expected 1 asset, got %d", result.Assets)
	}
	if result.AgentAggregators != 1 {
		t.Errorf("expected 1 agent aggregator, got %d", result.AgentAggregators)
	}

	// Verify all files exist
	for _, file := range []string{"labels.tf", "label_groups.tf", "policy_rules.tf", "dns_security.tf", "incidents.tf", "worksites.tf", "user_groups.tf", "assets.tf", "agent_aggregators.tf"} {
		content, err := os.ReadFile(filepath.Join(tmpDir, file))
		if err != nil {
			t.Errorf("failed to read %s: %v", file, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("%s is empty", file)
		}
	}

	assetsContent, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	if strings.Contains(string(assetsContent), "deleted-server") {
		t.Error("expected deleted asset to be skipped")
	}
	if !strings.Contains(string(assetsContent), `# NOTE: Asset "empty-nic-server" has no valid NICs`) {
		t.Error("expected commented-out block for asset with empty NICs")
	}
	if strings.Contains(string(assetsContent), `id = "asset-no-nics"`) {
		t.Error("expected empty-NIC asset to not have an import block")
	}
}

func TestRun_Integration_LocalUserGroupRefsRemainLiteral(t *testing.T) {
	labelsResp := client.ListLabelsResponse{TotalCount: 0}
	labelGroupsResp := client.ListLabelGroupsResponse{TotalCount: 0}
	policyRulesResp := client.ListPolicyRulesResponse{
		Objects: []map[string]interface{}{
			{
				"id":               "rule-1",
				"action":           "ALLOW",
				"enabled":          true,
				"section_position": "ALLOW",
				"source": map[string]interface{}{
					"user_group_ids": []interface{}{
						map[string]interface{}{"id": "ug-1", "name": "Development Team"},
						map[string]interface{}{"id": "local_administrators", "name": "Local Administrators"},
					},
				},
				"destination": map[string]interface{}{"any": true},
			},
		},
		TotalCount: 1,
	}
	dnsBlocklistsResp := client.ListDnsBlocklistsResponse{TotalCount: 0}
	incidentsResp := client.ListIncidentsResponse{TotalCount: 0}
	worksitesResp := client.ListWorksitesResponse{TotalCount: 0}
	userGroupsResp := client.ListUserGroupsResponse{
		Objects: []client.UserGroup{
			{
				ID:    "ug-1",
				Title: "Development Team",
				GroupsByDomainName: map[string]client.DomainGroupInfo{
					"corp.example.com": {
						Groups:          []client.DomainGroup{{ID: "dev-team", Name: "Development Team"}},
						OrchestrationID: "orch-1",
					},
				},
			},
			{
				ID:    "local_administrators",
				Title: "Local Administrators",
				GroupsByDomainName: map[string]client.DomainGroupInfo{
					"Local": {
						Groups:          []client.DomainGroup{{ID: "local_administrators", Name: "Local Administrators"}},
						OrchestrationID: "local",
					},
				},
			},
		},
		TotalCount: 2,
	}
	assetsResp := client.ListAssetsResponse{TotalCount: 0}
	agentAggregatorsResp := client.ListAgentAggregatorsResponse{TotalCount: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/label-groups"):
			_ = json.NewEncoder(w).Encode(labelGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels"):
			_ = json.NewEncoder(w).Encode(labelsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/user-groups"):
			_ = json.NewEncoder(w).Encode(userGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/policy/rules"):
			_ = json.NewEncoder(w).Encode(policyRulesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/dns_security"):
			_ = json.NewEncoder(w).Encode(dnsBlocklistsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/worksites"):
			_ = json.NewEncoder(w).Encode(worksitesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/assets"):
			_ = json.NewEncoder(w).Encode(assetsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/generic-incidents"):
			_ = json.NewEncoder(w).Encode(incidentsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/agent_aggregators"):
			_ = json.NewEncoder(w).Encode(agentAggregatorsResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir := t.TempDir()
	imp := &Importer{Client: apiClient, OutputDir: tmpDir}

	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserGroups != 2 {
		t.Fatalf("expected 2 user groups (1 managed + 1 system-managed data source), got %d", result.UserGroups)
	}

	policyRulesContent, err := os.ReadFile(filepath.Join(tmpDir, "policy_rules.tf"))
	if err != nil {
		t.Fatalf("failed to read policy_rules.tf: %v", err)
	}
	policyRules := string(policyRulesContent)

	if !strings.Contains(policyRules, "guardicore_user_group.development_team.id") {
		t.Fatal("expected imported user group reference for non-local user group")
	}
	if !strings.Contains(policyRules, "data.guardicore_user_group.local_administrators.id") {
		t.Fatal("expected data source reference for system-managed user group")
	}

	userGroupsContent, err := os.ReadFile(filepath.Join(tmpDir, "user_groups.tf"))
	if err != nil {
		t.Fatalf("failed to read user_groups.tf: %v", err)
	}
	userGroups := string(userGroupsContent)

	if !strings.Contains(userGroups, `data "guardicore_user_group" "local_administrators"`) {
		t.Fatal("expected system-managed user group to be generated as data source block")
	}
}

func TestRun_WorksitesFeatureDisabled(t *testing.T) {
	labelsResp := client.ListLabelsResponse{
		Objects:    []client.Label{{ID: "label-1", Key: "Env", Value: "Prod"}},
		TotalCount: 1,
	}
	labelGroupsResp := client.ListLabelGroupsResponse{TotalCount: 0}
	policyRulesResp := client.ListPolicyRulesResponse{TotalCount: 0}
	dnsBlocklistsResp := client.ListDnsBlocklistsResponse{TotalCount: 0}
	incidentsResp := client.ListIncidentsResponse{
		Objects: []map[string]interface{}{
			{
				"id":       "incident-1",
				"type":     "Network Scan",
				"severity": "HIGH",
				"time":     float64(1504688829035),
				"tags": map[string]interface{}{
					"data":  []interface{}{"test-tag"},
					"count": 1,
				},
				"affected_assets": map[string]interface{}{
					"data": []interface{}{
						map[string]interface{}{"type": "IP", "display_name": "10.0.0.1", "value": "167772161"},
					},
					"count": 1,
				},
			},
		},
		TotalCount: 1,
	}
	userGroupsResp := client.ListUserGroupsResponse{TotalCount: 0}
	assetsResp := client.ListAssetsResponse{TotalCount: 0}
	agentAggregatorsResp := client.ListAgentAggregatorsResponse{TotalCount: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/label-groups"):
			_ = json.NewEncoder(w).Encode(labelGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels"):
			_ = json.NewEncoder(w).Encode(labelsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/user-groups"):
			_ = json.NewEncoder(w).Encode(userGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/policy/rules"):
			_ = json.NewEncoder(w).Encode(policyRulesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/dns_security"):
			_ = json.NewEncoder(w).Encode(dnsBlocklistsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/worksites"):
			// Simulate worksites feature disabled
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "worksites feature is disabled"})
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/assets"):
			_ = json.NewEncoder(w).Encode(assetsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/generic-incidents"):
			_ = json.NewEncoder(w).Encode(incidentsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/agent_aggregators"):
			_ = json.NewEncoder(w).Encode(agentAggregatorsResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir := t.TempDir()
	imp := &Importer{
		Client:    apiClient,
		OutputDir: tmpDir,
	}

	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("expected no error when worksites disabled, got: %v", err)
	}

	if result.Worksites != 0 {
		t.Errorf("expected 0 worksites, got %d", result.Worksites)
	}
	if result.Incidents != 1 {
		t.Errorf("expected 1 incident, got %d", result.Incidents)
	}
	if result.Labels != 1 {
		t.Errorf("expected 1 label, got %d", result.Labels)
	}

	// Verify incidents.tf exists and has content
	content, err := os.ReadFile(filepath.Join(tmpDir, "incidents.tf"))
	if err != nil {
		t.Fatalf("failed to read incidents.tf: %v", err)
	}
	if len(content) == 0 {
		t.Error("incidents.tf is empty")
	}
	if !strings.Contains(string(content), "Network Scan") {
		t.Error("expected incident type 'Network Scan' in incidents.tf")
	}
}

func TestRun_DnsSecurityFeatureDisabled(t *testing.T) {
	labelsResp := client.ListLabelsResponse{
		Objects:    []client.Label{{ID: "label-1", Key: "Env", Value: "Prod"}},
		TotalCount: 1,
	}
	labelGroupsResp := client.ListLabelGroupsResponse{TotalCount: 0}
	policyRulesResp := client.ListPolicyRulesResponse{TotalCount: 0}
	incidentsResp := client.ListIncidentsResponse{TotalCount: 0}
	worksitesResp := client.ListWorksitesResponse{TotalCount: 0}
	userGroupsResp := client.ListUserGroupsResponse{TotalCount: 0}
	assetsResp := client.ListAssetsResponse{TotalCount: 0}
	agentAggregatorsResp := client.ListAgentAggregatorsResponse{TotalCount: 0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/label-groups"):
			_ = json.NewEncoder(w).Encode(labelGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/labels"):
			_ = json.NewEncoder(w).Encode(labelsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/user-groups"):
			_ = json.NewEncoder(w).Encode(userGroupsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/visibility/policy/rules"):
			_ = json.NewEncoder(w).Encode(policyRulesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/dns_security"):
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"description": "Could not complete the operation due to a server error. See error for more details",
				"error_code":  "OperationFailed",
				"error_dump":  "('%s is not enabled', 'DNS Security')",
			})
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/worksites"):
			_ = json.NewEncoder(w).Encode(worksitesResp)
		case strings.HasPrefix(r.URL.Path, "/api/v4.0/assets"):
			_ = json.NewEncoder(w).Encode(assetsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/generic-incidents"):
			_ = json.NewEncoder(w).Encode(incidentsResp)
		case strings.HasPrefix(r.URL.Path, "/api/v3.0/agent_aggregators"):
			_ = json.NewEncoder(w).Encode(agentAggregatorsResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	apiClient, err := client.NewClient(client.Config{
		BaseURL:     server.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tmpDir := t.TempDir()
	imp := &Importer{
		Client:    apiClient,
		OutputDir: tmpDir,
	}

	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("expected no error when DNS Security disabled, got: %v", err)
	}

	if result.DnsBlocklists != 0 {
		t.Errorf("expected 0 DNS blocklists, got %d", result.DnsBlocklists)
	}
	if result.Labels != 1 {
		t.Errorf("expected 1 label, got %d", result.Labels)
	}
}

func TestGenerateAgentAggregatorsFile(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	aggregators := []client.AgentAggregator{
		{
			ID:       "agg-1",
			Hostname: "gc-aggregator-172-235-229-101",
		},
		{
			ID:       "agg-2",
			Hostname: "gc-aggregator-10-0-0-5",
		},
	}

	err := imp.generateAgentAggregatorsFile(aggregators)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "agent_aggregators.tf"))
	if err != nil {
		t.Fatalf("failed to read agent_aggregators.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `data "guardicore_agent_aggregator"`) {
		t.Error("expected data source block for agent aggregator")
	}
	if !strings.Contains(s, `hostname = "gc-aggregator-172-235-229-101"`) {
		t.Error("expected hostname for first aggregator")
	}
	if !strings.Contains(s, `hostname = "gc-aggregator-10-0-0-5"`) {
		t.Error("expected hostname for second aggregator")
	}
	if strings.Contains(s, "import {") {
		t.Error("agent aggregator data sources should not have import blocks")
	}
	if strings.Contains(s, `resource "guardicore_agent_aggregator"`) {
		t.Error("agent aggregators should not generate resource blocks")
	}
}

func TestGenerateAgentAggregatorsFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	err := imp.generateAgentAggregatorsFile(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = os.ReadFile(filepath.Join(tmpDir, "agent_aggregators.tf"))
	if err == nil {
		t.Error("expected no file for empty aggregators list")
	}
}

func TestGenerateAssetsFile_MixedNicsFilteredByIPAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-mixed",
			Name:               "mixed-nic-vm",
			Status:             "on",
			OrchestrationObjID: "orch-mixed",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{"10.0.0.1"}, MacAddress: "aa:bb:cc:dd:ee:01"},
				{IPAddresses: []string{}, MacAddress: "aa:bb:cc:dd:ee:02"},
				{IPAddresses: []string{"10.0.0.3"}, MacAddress: "aa:bb:cc:dd:ee:03"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{
		"asset-mixed": SanitizeName("", "mixed-nic-vm"),
	})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	written, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 1 {
		t.Errorf("expected 1 asset written, got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `"10.0.0.1"`) {
		t.Error("expected valid NIC with IP 10.0.0.1 in output")
	}
	if !strings.Contains(s, `"10.0.0.3"`) {
		t.Error("expected valid NIC with IP 10.0.0.3 in output")
	}
	if strings.Contains(s, "aa:bb:cc:dd:ee:02") {
		t.Error("expected NIC with empty ip_addresses to be filtered out")
	}
	if strings.Contains(s, "# NOTE:") {
		t.Error("asset with at least one valid NIC should not be skipped")
	}
}

func TestGenerateAssetsFile_AllNicsEmptyIPAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	imp := &Importer{OutputDir: tmpDir}

	assets := []client.Asset{
		{
			ID:                 "asset-all-empty-ip",
			Name:               "all-empty-ip-vm",
			Status:             "on",
			OrchestrationObjID: "orch-empty-ip",
			Nics: []client.AssetNIC{
				{IPAddresses: []string{}, MacAddress: "aa:bb:cc:dd:ee:01"},
				{IPAddresses: nil, MacAddress: "aa:bb:cc:dd:ee:02"},
			},
		},
	}

	named := DeduplicateNames(map[string]string{
		"asset-all-empty-ip": SanitizeName("", "all-empty-ip-vm"),
	})
	lookup := &ResourceLookup{
		Labels: map[string]string{}, LabelGroups: map[string]string{},
		UserGroups: map[string]string{}, Assets: map[string]string{},
	}

	written, err := imp.generateAssetsFile(context.Background(), assets, named, lookup, map[string]string{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("expected 0 assets written (all NICs have empty IPs), got %d", written)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "assets.tf"))
	if err != nil {
		t.Fatalf("failed to read assets.tf: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `# NOTE: Asset "all-empty-ip-vm" has no valid NICs`) {
		t.Error("expected commented-out block for asset with all-empty-IP NICs")
	}
	if strings.Contains(s, "import {") {
		t.Error("no import blocks should exist for skipped asset")
	}
}
