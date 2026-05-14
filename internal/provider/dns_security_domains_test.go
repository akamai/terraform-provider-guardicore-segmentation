package provider

import (
	"context"
	"reflect"
	"testing"
)

func TestNormalizeDNSDomains(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "deduplicates and sorts",
			input:  []string{"b.example.com", "a.example.com", "b.example.com"},
			expect: []string{"a.example.com", "b.example.com"},
		},
		{
			name:   "empty input",
			input:  []string{},
			expect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := normalizeDNSDomains(tt.input)
			if !reflect.DeepEqual(actual, tt.expect) {
				t.Fatalf("normalizeDNSDomains() mismatch: got=%v want=%v", actual, tt.expect)
			}
		})
	}
}

func TestDNSDomainsSetValue(t *testing.T) {
	ctx := context.Background()

	t.Run("returns known null set for empty domains", func(t *testing.T) {
		set, diags := dnsDomainsSetValue(nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !set.IsNull() {
			t.Fatalf("expected null set, got %#v", set)
		}
		if set.IsUnknown() {
			t.Fatalf("expected known null set, got unknown")
		}
	})

	t.Run("returns known set with stable normalized values", func(t *testing.T) {
		set, diags := dnsDomainsSetValue([]string{"z.example.com", "a.example.com", "z.example.com"})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if set.IsNull() || set.IsUnknown() {
			t.Fatalf("expected known non-null set, got %#v", set)
		}

		var values []string
		elementDiags := set.ElementsAs(ctx, &values, false)
		if elementDiags.HasError() {
			t.Fatalf("unexpected element diagnostics: %v", elementDiags)
		}

		expected := []string{"a.example.com", "z.example.com"}
		if !reflect.DeepEqual(values, expected) {
			t.Fatalf("set values mismatch: got=%v want=%v", values, expected)
		}
	})

}
