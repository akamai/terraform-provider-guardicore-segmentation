package provider

import (
	"context"
	"strings"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGetStringValue(t *testing.T) {
	t.Setenv("TEST_STRING_ENV", "from-env")

	if got := getStringValue(types.StringValue("from-config"), "TEST_STRING_ENV"); got != "from-config" {
		t.Fatalf("expected config value, got %q", got)
	}

	if got := getStringValue(types.StringNull(), "TEST_STRING_ENV"); got != "from-env" {
		t.Fatalf("expected env fallback, got %q", got)
	}

	if got := getStringValue(types.StringUnknown(), "TEST_STRING_ENV"); got != "from-env" {
		t.Fatalf("expected env fallback for unknown value, got %q", got)
	}
}

func TestGetBoolValue(t *testing.T) {
	if got := getBoolValue(types.BoolValue(true), "TEST_BOOL_ENV"); !got {
		t.Fatalf("expected config bool to win")
	}

	t.Setenv("TEST_BOOL_ENV", "true")
	if got := getBoolValue(types.BoolNull(), "TEST_BOOL_ENV"); !got {
		t.Fatalf("expected true from env")
	}

	t.Setenv("TEST_BOOL_ENV", "not-a-bool")
	if got := getBoolValue(types.BoolUnknown(), "TEST_BOOL_ENV"); got {
		t.Fatalf("expected false when env bool parse fails")
	}
}

func TestGetInt64Value(t *testing.T) {
	if got := getInt64Value(types.Int64Value(42), "TEST_I64_ENV"); got != 42 {
		t.Fatalf("expected config int64, got %d", got)
	}

	t.Setenv("TEST_I64_ENV", "99")
	if got := getInt64Value(types.Int64Null(), "TEST_I64_ENV"); got != 99 {
		t.Fatalf("expected env int64, got %d", got)
	}

	t.Setenv("TEST_I64_ENV", "bad")
	if got := getInt64Value(types.Int64Unknown(), "TEST_I64_ENV"); got != 0 {
		t.Fatalf("expected zero when env int64 parse fails, got %d", got)
	}
}

func TestProviderMetadata(t *testing.T) {
	p := &GuardicoreProvider{version: "v-test"}
	resp := &frameworkprovider.MetadataResponse{}
	p.Metadata(context.Background(), frameworkprovider.MetadataRequest{}, resp)

	if resp.TypeName != "guardicore" {
		t.Fatalf("expected typename guardicore, got %q", resp.TypeName)
	}
	if resp.Version != "v-test" {
		t.Fatalf("expected version v-test, got %q", resp.Version)
	}
}

func TestProviderSchemaResourcesAndDataSources(t *testing.T) {
	p := &GuardicoreProvider{version: "v-test"}

	schemaResp := &frameworkprovider.SchemaResponse{}
	p.Schema(context.Background(), frameworkprovider.SchemaRequest{}, schemaResp)

	if len(schemaResp.Schema.Attributes) == 0 {
		t.Fatal("expected provider schema attributes")
	}
	if _, ok := schemaResp.Schema.Attributes["base_url"]; !ok {
		t.Fatal("expected base_url attribute in schema")
	}

	if got := len(p.Resources(context.Background())); got != 9 {
		t.Fatalf("expected 9 resources, got %d", got)
	}
	if got := len(p.DataSources(context.Background())); got != 10 {
		t.Fatalf("expected 10 data sources, got %d", got)
	}
}

func TestNewReturnsProviderWithVersion(t *testing.T) {
	factory := New("1.2.3")
	p := factory()

	gcp, ok := p.(*GuardicoreProvider)
	if !ok {
		t.Fatalf("expected *GuardicoreProvider, got %T", p)
	}
	if gcp.version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", gcp.version)
	}
}

// TestInsecureSkipVerifyWarning covers F-02: the warning text must name the
// targeted base_url so an operator scanning logs across providers can identify
// which tenant has TLS verification disabled.
func TestInsecureSkipVerifyWarning(t *testing.T) {
	baseURL := "https://guardicore.example.com"
	summary, detail := insecureSkipVerifyWarning(baseURL)

	if summary != "TLS Certificate Verification Disabled" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !strings.Contains(detail, baseURL) {
		t.Fatalf("expected detail to contain base_url %q, got: %q", baseURL, detail)
	}
	if !strings.Contains(detail, "man-in-the-middle") {
		t.Fatalf("expected detail to retain MITM warning, got: %q", detail)
	}
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "https ok", input: "https://guardicore.example.com", want: "https"},
		{name: "http allowed", input: "http://guardicore.example.com", want: "http"},
		{name: "missing scheme", input: "guardicore.example.com", wantErr: true},
		{name: "unsupported scheme", input: "ftp://guardicore.example.com", wantErr: true},
		{name: "missing host", input: "https:///", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateBaseURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected scheme %q, got %q", tc.want, got)
			}
		})
	}
}
