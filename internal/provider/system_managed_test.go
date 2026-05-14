package provider

import (
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLabelIsSystemManaged(t *testing.T) {
	origin := "worksite"
	tests := []struct {
		name          string
		label         *client.Label
		wantManaged   bool
		wantManagedBy string
	}{
		{
			"nil ReadOnly",
			&client.Label{},
			false, "terraform",
		},
		{
			"ReadOnly false",
			&client.Label{ReadOnly: boolPtr(false)},
			false, "terraform",
		},
		{
			"ReadOnly true no origin",
			&client.Label{ReadOnly: boolPtr(true)},
			true, "system",
		},
		{
			"ReadOnly true with origin",
			&client.Label{ReadOnly: boolPtr(true), Origin: &origin},
			true, "worksite",
		},
		{
			"ReadOnly true with empty origin",
			&client.Label{ReadOnly: boolPtr(true), Origin: stringPtr("")},
			true, "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			managed, managedBy := LabelIsSystemManaged(tt.label)
			if managed != tt.wantManaged {
				t.Errorf("systemManaged = %v, want %v", managed, tt.wantManaged)
			}
			if managedBy != tt.wantManagedBy {
				t.Errorf("managedBy = %q, want %q", managedBy, tt.wantManagedBy)
			}
		})
	}
}

func TestUserGroupIsSystemManaged(t *testing.T) {
	tests := []struct {
		name          string
		ug            *client.UserGroup
		wantManaged   bool
		wantManagedBy string
	}{
		{
			"empty groups",
			&client.UserGroup{},
			true, "system",
		},
		{
			"all local orchestrations_groups",
			&client.UserGroup{
				OrchestrationsGroups: []client.OrchestrationGroup{
					{OrchestrationID: "local", Groups: []string{"g1"}},
				},
			},
			true, "system",
		},
		{
			"non-local orchestrations_groups",
			&client.UserGroup{
				OrchestrationsGroups: []client.OrchestrationGroup{
					{OrchestrationID: "ad-primary", Groups: []string{"g1"}},
				},
			},
			false, "terraform",
		},
		{
			"mixed orchestrations_groups",
			&client.UserGroup{
				OrchestrationsGroups: []client.OrchestrationGroup{
					{OrchestrationID: "local", Groups: []string{"g1"}},
					{OrchestrationID: "ad-primary", Groups: []string{"g2"}},
				},
			},
			false, "terraform",
		},
		{
			"all local groups_by_domain_name",
			&client.UserGroup{
				GroupsByDomainName: map[string]client.DomainGroupInfo{
					"local": {OrchestrationID: "local", Groups: []client.DomainGroup{{ID: "g1"}}},
				},
			},
			true, "system",
		},
		{
			"non-local groups_by_domain_name",
			&client.UserGroup{
				GroupsByDomainName: map[string]client.DomainGroupInfo{
					"domain.com": {OrchestrationID: "ad-1", Groups: []client.DomainGroup{{ID: "g1"}}},
				},
			},
			false, "terraform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			managed, managedBy := UserGroupIsSystemManaged(tt.ug)
			if managed != tt.wantManaged {
				t.Errorf("systemManaged = %v, want %v", managed, tt.wantManaged)
			}
			if managedBy != tt.wantManagedBy {
				t.Errorf("managedBy = %q, want %q", managedBy, tt.wantManagedBy)
			}
		})
	}
}

func TestWorksiteIsSystemManaged(t *testing.T) {
	tests := []struct {
		name          string
		ws            *client.Worksite
		wantManaged   bool
		wantManagedBy string
	}{
		{
			"Default worksite",
			&client.Worksite{Name: "Default"},
			true, "system",
		},
		{
			"custom worksite",
			&client.Worksite{Name: "Headquarters"},
			false, "terraform",
		},
		{
			"empty name",
			&client.Worksite{},
			false, "terraform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			managed, managedBy := WorksiteIsSystemManaged(tt.ws)
			if managed != tt.wantManaged {
				t.Errorf("systemManaged = %v, want %v", managed, tt.wantManaged)
			}
			if managedBy != tt.wantManagedBy {
				t.Errorf("managedBy = %q, want %q", managedBy, tt.wantManagedBy)
			}
		})
	}
}

func TestDiagnoseSystemManagedMutation(t *testing.T) {
	t.Run("not system managed", func(t *testing.T) {
		var diags diag.Diagnostics
		DiagnoseSystemManagedMutation(types.BoolValue(false), "label", "test-id", "updated", &diags)
		if diags.HasError() {
			t.Error("expected no error for non-system-managed resource")
		}
	})

	t.Run("system managed blocks mutation", func(t *testing.T) {
		var diags diag.Diagnostics
		DiagnoseSystemManagedMutation(types.BoolValue(true), "label", "test-id", "update", &diags)
		if !diags.HasError() {
			t.Error("expected error for system-managed resource")
		}
		if got := diags.Errors()[0].Summary(); got != "System-Managed Resource Cannot Be updated" {
			t.Errorf("unexpected summary: %s", got)
		}
	})
}

func TestDnsBlocklistTypeIsSystemManaged(t *testing.T) {
	tests := []struct {
		name     string
		listType string
		want     bool
	}{
		{name: "Akamai intelligence", listType: "AKAMAI_INTELLIGENCE", want: true},
		{name: "Web category", listType: "WEB_CATEGORY", want: true},
		{name: "Case insensitive", listType: "web_category", want: true},
		{name: "Custom block", listType: "CUSTOM_BLOCK", want: false},
		{name: "Custom exclusion", listType: "CUSTOM_EXCLUSION", want: false},
		{name: "Custom blocklist", listType: "CUSTOM_BLOCKLIST", want: false},
		{name: "Exclusion list", listType: "EXCLUSION_LIST", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DnsBlocklistTypeIsSystemManaged(tt.listType)
			if got != tt.want {
				t.Errorf("DnsBlocklistTypeIsSystemManaged(%q) = %v, want %v", tt.listType, got, tt.want)
			}
		})
	}
}
