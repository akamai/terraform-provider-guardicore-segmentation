package dnssecurity

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDnsSecurityDataSource_MetadataSchemaAndConfigure(t *testing.T) {
	ds := &DnsSecurityDataSource{}

	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_dns_security" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "name", "type", "domains", "enabled"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}

	t.Run("nil provider data", func(t *testing.T) {
		resp := &datasource.ConfigureResponse{}
		ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("unexpected provider data type", func(t *testing.T) {
		resp := &datasource.ConfigureResponse{}
		ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: "bad"}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if resp.Diagnostics.Errors()[0].Summary() != "Unexpected Configure Type" {
			t.Fatalf("unexpected summary: %s", resp.Diagnostics.Errors()[0].Summary())
		}
	})

	t.Run("sets client", func(t *testing.T) {
		c, err := client.NewClient(client.Config{BaseURL: "https://example.com", AccessToken: "token"})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		resp := &datasource.ConfigureResponse{}
		ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: &common.ProviderData{Client: c}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if ds.client != c {
			t.Fatal("expected client to be set from provider data")
		}
	})
}

func TestDnsSecurityDataSource_ReadModel(t *testing.T) {
	t.Run("invalid configuration", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{})
		if err == nil || err.Summary != "Invalid Configuration" {
			t.Fatalf("expected invalid configuration error, got %+v", err)
		}
	})

	t.Run("read by id not found", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{ID: types.StringValue("dns-1")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by id feature disabled", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{getErr: client.ErrDnsSecurityFeatureDisabled}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{ID: types.StringValue("dns-1")})
		if err == nil || err.Summary != "DNS Security Feature Disabled" {
			t.Fatalf("expected feature disabled error, got %+v", err)
		}
	})

	t.Run("read by id client error", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{getErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{ID: types.StringValue("dns-1")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by id success", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{getResult: &client.DnsBlocklist{
			ID:      "dns-1",
			Name:    "block-a",
			Type:    "CUSTOM_BLOCK",
			Domains: []string{"z.example.com", "a.example.com", "z.example.com"},
			Enabled: true,
		}}}

		got, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{ID: types.StringValue("dns-1")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if got.ID.ValueString() != "dns-1" || got.Name.ValueString() != "block-a" || got.Type.ValueString() != "CUSTOM_BLOCK" {
			t.Fatalf("unexpected mapped model: %+v", got)
		}
		if !got.Enabled.ValueBool() {
			t.Fatal("expected enabled=true")
		}

		var domains []string
		diags := got.Domains.ElementsAs(context.Background(), &domains, false)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if len(domains) != 2 || domains[0] != "a.example.com" || domains[1] != "z.example.com" {
			t.Fatalf("unexpected normalized domains: %#v", domains)
		}
	})

	t.Run("read by name no match", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{listResult: []client.DnsBlocklist{{Name: "other"}}}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{Name: types.StringValue("wanted")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by name feature disabled", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{listErr: client.ErrDnsSecurityFeatureDisabled}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{Name: types.StringValue("wanted")})
		if err == nil || err.Summary != "DNS Security Feature Disabled" {
			t.Fatalf("expected feature disabled error, got %+v", err)
		}
	})

	t.Run("read by name client error", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{listErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{Name: types.StringValue("wanted")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by name multiple matches", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{listResult: []client.DnsBlocklist{{Name: "dup"}, {Name: "dup"}}}}
		_, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{Name: types.StringValue("dup")})
		if err == nil || err.Summary != "Multiple Results" {
			t.Fatalf("expected multiple results error, got %+v", err)
		}
	})

	t.Run("read by name success", func(t *testing.T) {
		ds := &DnsSecurityDataSource{client: &fakeDnsSecurityDataSourceClient{listResult: []client.DnsBlocklist{
			{ID: "dns-2", Name: "wanted", Type: "CUSTOM_EXCLUSION", Domains: []string{"x.example.com"}, Enabled: false},
			{ID: "dns-3", Name: "other", Type: "CUSTOM_BLOCK", Domains: []string{"y.example.com"}, Enabled: true},
		}}}

		got, err := ds.readModel(context.Background(), DnsSecurityDataSourceModel{Name: types.StringValue("wanted")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if got.ID.ValueString() != "dns-2" || got.Name.ValueString() != "wanted" || got.Enabled.ValueBool() {
			t.Fatalf("unexpected mapped model: %+v", got)
		}
	})
}

type fakeDnsSecurityDataSourceClient struct {
	getResult  *client.DnsBlocklist
	getErr     error
	listResult []client.DnsBlocklist
	listErr    error
}

func (f *fakeDnsSecurityDataSourceClient) GetDnsBlocklist(context.Context, string) (*client.DnsBlocklist, error) {
	return f.getResult, f.getErr
}

func (f *fakeDnsSecurityDataSourceClient) ListDnsBlocklists(context.Context, string, string) ([]client.DnsBlocklist, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}
