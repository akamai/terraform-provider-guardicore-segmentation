package dnssecurity

import (
	"context"
	"reflect"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDnsSecurityResource_MetadataSchemaAndConfigure(t *testing.T) {
	r := &DnsSecurityResource{}

	metaResp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_dns_security" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "name", "type", "domains", "enabled"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}

	t.Run("nil provider data", func(t *testing.T) {
		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("unexpected provider data type", func(t *testing.T) {
		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "bad"}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if resp.Diagnostics.Errors()[0].Summary() != "Unexpected Configure Type" {
			t.Fatalf("unexpected summary: %s", resp.Diagnostics.Errors()[0].Summary())
		}
	})

	t.Run("sets dependencies", func(t *testing.T) {
		c, err := client.NewClient(client.Config{BaseURL: "https://example.com", AccessToken: "token"})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		createBatcher := &common.Batcher[*client.DnsBlocklistCreate, string]{}
		updateBatcher := &common.Batcher[common.DnsSecurityUpdateReq, struct{}]{}
		deleteBatcher := &common.Batcher[string, struct{}]{}

		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &common.ProviderData{
			Client:                   c,
			DnsSecurityCreateBatcher: createBatcher,
			DnsSecurityUpdateBatcher: updateBatcher,
			DnsSecurityDeleteBatcher: deleteBatcher,
		}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if r.client != c || r.createBatcher != createBatcher || r.updateBatcher != updateBatcher || r.deleteBatcher != deleteBatcher {
			t.Fatal("expected configure to set client and batchers")
		}
	})
}

func TestDnsSecurityResourceModelToAPI(t *testing.T) {
	r := &DnsSecurityResource{}
	ctx := context.Background()

	t.Run("null domains become empty slice", func(t *testing.T) {
		model := &DnsSecurityResourceModel{
			Name:    types.StringValue("list-a"),
			Type:    types.StringValue("CUSTOM_BLOCK"),
			Domains: types.SetNull(types.StringType),
		}

		api := r.modelToAPI(ctx, model)
		if api.Name != "list-a" || api.Type != "CUSTOM_BLOCK" {
			t.Fatalf("unexpected name/type mapping: %#v", api)
		}
		if len(api.Domains) != 0 {
			t.Fatalf("expected empty domains for null set, got %#v", api.Domains)
		}
	})

	t.Run("domains set maps to slice", func(t *testing.T) {
		domainsSet, diags := types.SetValue(types.StringType, []attr.Value{
			types.StringValue("b.example.com"),
			types.StringValue("a.example.com"),
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating domains set: %v", diags)
		}

		model := &DnsSecurityResourceModel{
			Name:    types.StringValue("list-b"),
			Type:    types.StringValue("CUSTOM_EXCLUSION"),
			Domains: domainsSet,
		}

		api := r.modelToAPI(ctx, model)
		if len(api.Domains) != 2 {
			t.Fatalf("expected 2 domains, got %#v", api.Domains)
		}
		seen := map[string]bool{}
		for _, d := range api.Domains {
			seen[d] = true
		}
		if !seen["a.example.com"] || !seen["b.example.com"] {
			t.Fatalf("mapped domains missing expected values: %#v", api.Domains)
		}
	})
}

func TestDnsSecurityResourceAPIToModel(t *testing.T) {
	r := &DnsSecurityResource{}
	ctx := context.Background()

	model := &DnsSecurityResourceModel{}
	diags := r.apiToModel(&client.DnsBlocklist{
		ID:      "dns-1",
		Name:    "blocklist-1",
		Type:    "CUSTOM_BLOCK",
		Domains: []string{"z.example.com", "a.example.com", "z.example.com"},
		Enabled: true,
	}, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics mapping API model: %v", diags)
	}

	if model.ID.ValueString() != "dns-1" || model.Name.ValueString() != "blocklist-1" || model.Type.ValueString() != "CUSTOM_BLOCK" {
		t.Fatalf("unexpected identity mapping: %#v", model)
	}
	if !model.Enabled.ValueBool() {
		t.Fatal("expected enabled=true mapping")
	}

	var gotDomains []string
	elementDiags := model.Domains.ElementsAs(ctx, &gotDomains, false)
	if elementDiags.HasError() {
		t.Fatalf("unexpected diagnostics decoding domains: %v", elementDiags)
	}
	wantDomains := []string{"a.example.com", "z.example.com"}
	if !reflect.DeepEqual(gotDomains, wantDomains) {
		t.Fatalf("domains mismatch: got=%v want=%v", gotDomains, wantDomains)
	}
}
