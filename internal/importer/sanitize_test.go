package importer

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "basic",
			key:      "Environment",
			value:    "Production",
			expected: "environment_production",
		},
		{
			name:     "special characters",
			key:      "App/Server",
			value:    "Web+DB",
			expected: "app_server_web_db",
		},
		{
			name:     "leading digit",
			key:      "123",
			value:    "Test",
			expected: "_123_test",
		},
		{
			name:     "empty after sanitize",
			key:      "",
			value:    "",
			expected: "resource",
		},
		{
			name:     "unicode accented chars",
			key:      "Región",
			value:    "España",
			expected: "region_espana",
		},
		{
			name:     "multiple special chars",
			key:      "my--key",
			value:    "my..value",
			expected: "my_key_my_value",
		},
		{
			name:     "already clean",
			key:      "role",
			value:    "web",
			expected: "role_web",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeName(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("SanitizeName(%q, %q) = %q, want %q", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestTransliterate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "accented a variants",
			input:    "áàâäãå",
			expected: "aaaaaa",
		},
		{
			name:     "accented e variants",
			input:    "éèêë",
			expected: "eeee",
		},
		{
			name:     "accented i variants",
			input:    "íìîï",
			expected: "iiii",
		},
		{
			name:     "accented o variants",
			input:    "óòôöõ",
			expected: "ooooo",
		},
		{
			name:     "accented u variants",
			input:    "úùûü",
			expected: "uuuu",
		},
		{
			name:     "ñ and ç",
			input:    "ñç",
			expected: "nc",
		},
		{
			name:     "non-ascii fallback to underscore",
			input:    "日本語",
			expected: "___",
		},
		{
			name:     "mixed ascii and non-ascii",
			input:    "café résumé",
			expected: "cafe resume",
		},
		{
			name:     "plain ascii unchanged",
			input:    "hello world 123",
			expected: "hello world 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transliterate(tt.input)
			if result != tt.expected {
				t.Errorf("transliterate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDeduplicateNames(t *testing.T) {
	idToName := map[string]string{
		"aaa-111": "environment_production",
		"bbb-222": "environment_production",
		"ccc-333": "environment_production",
		"ddd-444": "role_web",
	}

	result := DeduplicateNames(idToName)

	if len(result) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result))
	}

	// Should be sorted by ID
	if result[0].ID != "aaa-111" || result[1].ID != "bbb-222" || result[2].ID != "ccc-333" || result[3].ID != "ddd-444" {
		t.Errorf("results not sorted by ID: %+v", result)
	}

	// First occurrence keeps original name
	if result[0].Name != "environment_production" {
		t.Errorf("expected first to be 'environment_production', got %q", result[0].Name)
	}

	// Second gets _2
	if result[1].Name != "environment_production_2" {
		t.Errorf("expected second to be 'environment_production_2', got %q", result[1].Name)
	}

	// Third gets _3
	if result[2].Name != "environment_production_3" {
		t.Errorf("expected third to be 'environment_production_3', got %q", result[2].Name)
	}

	// Unique name stays
	if result[3].Name != "role_web" {
		t.Errorf("expected fourth to be 'role_web', got %q", result[3].Name)
	}
}
