package importer

import (
	"context"
	"encoding/json"
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
				{Field: "name", Op: "CONTAINS", Argument: "web"},
			},
		},
	}

	idToName := map[string]string{
		"label-1": SanitizeName("Environment", "Production"),
		"label-2": SanitizeName("Application", "Web Server"),
	}
	named := DeduplicateNames(idToName)

	err := imp.generateLabelsFile(labels, named)
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
}

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
		"action":       "ALLOW",
		"source":       map[string]interface{}{},
		"destination":  map[string]interface{}{},
		"recently_hit": false,
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
	if strings.Contains(body, "source =") {
		t.Fatalf("expected empty source to be stripped, got:\n%s", body)
	}
	if strings.Contains(body, "destination =") {
		t.Fatalf("expected empty destination to be stripped, got:\n%s", body)
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
			Type:    "custom",
			Domains: []string{"evil.com", "malware.org"},
			Enabled: true,
		},
		{
			ID:      "dns-2",
			Name:    "Ad Servers",
			Type:    "predefined",
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
	if !strings.Contains(s, `resource "guardicore_dns_security" "ad_servers"`) {
		t.Error("expected dns security resource block for ad_servers")
	}

	// Check import blocks
	if !strings.Contains(s, "to = guardicore_dns_security.malware_domains") {
		t.Error("expected import block for malware_domains")
	}
	if !strings.Contains(s, `id = "dns-1"`) {
		t.Error("expected import block with dns-1 ID")
	}
	if !strings.Contains(s, `id = "dns-2"`) {
		t.Error("expected import block with dns-2 ID")
	}

	// Check domains
	if !strings.Contains(s, `"evil.com"`) {
		t.Error("expected domain evil.com in blocklist")
	}
	if !strings.Contains(s, `"malware.org"`) {
		t.Error("expected domain malware.org in blocklist")
	}

	// Check type
	if !strings.Contains(s, `type = "custom"`) {
		t.Error("expected type custom")
	}
	if !strings.Contains(s, `type = "predefined"`) {
		t.Error("expected type predefined")
	}

	// Check name
	if !strings.Contains(s, `name = "Malware Domains"`) {
		t.Error("expected name Malware Domains")
	}
	if !strings.Contains(s, `name = "Ad Servers"`) {
		t.Error("expected name Ad Servers")
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
	}

	err := imp.generateWorksitesFile(worksites)
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
			OrchestrationsGroups: []client.OrchestrationGroup{
				{
					OrchestrationID: "orch-1",
					Groups:          []string{"group-a", "group-b"},
				},
			},
		},
	}

	idToNameUG := map[string]string{"ug-1": SanitizeName("", "Development Team")}
	namedUG := DeduplicateNames(idToNameUG)

	written, err := imp.generateUserGroupsFile(userGroups, namedUG)
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
	if !strings.Contains(s, `orchestration_id = "orch-1"`) {
		t.Error("expected orchestration_id 'orch-1'")
	}
	if !strings.Contains(s, `"group-a"`) {
		t.Error("expected group 'group-a'")
	}
	if !strings.Contains(s, `"group-b"`) {
		t.Error("expected group 'group-b'")
	}
	// Verify import block
	if !strings.Contains(s, "import {") {
		t.Error("expected import block")
	}
	if !strings.Contains(s, `id = "ug-1"`) {
		t.Error("expected import id 'ug-1'")
	}
	if !strings.Contains(s, "to = guardicore_user_group.") {
		t.Error("expected import to reference")
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

	err := imp.generateAssetsFile(assets, namedA, lookup)
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

	if err := imp.generateAssetsFile(assets, named, lookup); err != nil {
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

	if err := imp.generateAssetsFile(assets, named, lookup); err != nil {
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

	if err := imp.generateAssetsFile(assets, named, lookup); err != nil {
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
	if !strings.Contains(s, `bios_uuid            = "bios-uuid-012"`) {
		t.Error("expected bios_uuid in output")
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

	if err := imp.generateAssetsFile(assets, named, lookup); err != nil {
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
		t.Error("bios_uuid should be omitted when empty")
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
	if !strings.Contains(s, "# reference not imported") {
		t.Error("expected '# reference not imported' comment for unresolvable ref")
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
				"user_group_ids": []interface{}{"ug-1"},
				"asset_ids":      []interface{}{"asset-1"},
			},
		},
	}

	lookup := &ResourceLookup{
		Labels:       map[string]string{},
		LabelGroups:  map[string]string{"lg-1": "guardicore_label_group.web_servers.id"},
		PolicyGroups: map[string]string{},
		UserGroups:   map[string]string{"ug-1": "guardicore_user_group.dev_team.id"},
		Assets:       map[string]string{"asset-1": "guardicore_asset.web_01.id"},
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

	err := imp.generateAssetsFile(assets, named, lookup)
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
				OrchestrationsGroups: []client.OrchestrationGroup{
					{
						OrchestrationID: "orch-1",
						Groups:          []string{"group-1", "group-2"},
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

	// Verify all files exist
	for _, file := range []string{"labels.tf", "label_groups.tf", "policy_rules.tf", "dns_security.tf", "incidents.tf", "worksites.tf", "user_groups.tf", "assets.tf"} {
		content, err := os.ReadFile(filepath.Join(tmpDir, file))
		if err != nil {
			t.Errorf("failed to read %s: %v", file, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("%s is empty", file)
		}
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
