package provider

import (
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

func TestLabelIsReadOnly(t *testing.T) {
	tests := []struct {
		name     string
		label    *client.Label
		expected bool
	}{
		{"nil label", nil, false},
		{"nil ReadOnly", &client.Label{}, false},
		{"ReadOnly false", &client.Label{ReadOnly: boolPtr(false)}, false},
		{"ReadOnly true", &client.Label{ReadOnly: boolPtr(true)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := labelIsReadOnly(tt.label); got != tt.expected {
				t.Errorf("labelIsReadOnly() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func TestLabelCriteriaIsReadOnlyWorksiteGenerated(t *testing.T) {
	tests := []struct {
		name     string
		criteria client.LabelCriteria
		expected bool
	}{
		{
			name:     "empty criterion",
			criteria: client.LabelCriteria{},
			expected: false,
		},
		{
			name: "read-only worksite criterion",
			criteria: client.LabelCriteria{
				ReadOnly: boolPtr(true),
				Source:   stringPtr("Worksite"),
			},
			expected: true,
		},
		{
			name: "read-only worksite criterion case-insensitive source",
			criteria: client.LabelCriteria{
				ReadOnly: boolPtr(true),
				Source:   stringPtr("worksite"),
			},
			expected: true,
		},
		{
			name: "read-only non-worksite criterion",
			criteria: client.LabelCriteria{
				ReadOnly: boolPtr(true),
				Source:   stringPtr("CMDB"),
			},
			expected: false,
		},
		{
			name: "worksite criterion not read-only",
			criteria: client.LabelCriteria{
				ReadOnly: boolPtr(false),
				Source:   stringPtr("Worksite"),
			},
			expected: false,
		},
		{
			name: "worksite criterion by field fallback",
			criteria: client.LabelCriteria{
				Field: "scoping_details.worksite.id",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.criteria.IsReadOnlyWorksiteGenerated(); got != tt.expected {
				t.Errorf("IsReadOnlyWorksiteGenerated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func stringPtr(s string) *string { return &s }
