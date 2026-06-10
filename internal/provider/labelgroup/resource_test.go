package labelgroup

import (
	"context"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestLabelGroupResource_MetadataSchemaAndConfigure(t *testing.T) {
	r := &LabelGroupResource{}

	metaResp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_label_group" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "key", "value", "comments", "include", "exclude", "raw_include_json", "raw_exclude_json", "include_json", "exclude_json"} {
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
		createBatcher := &common.Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate]{}
		updateBatcher := &common.Batcher[common.LabelGroupUpdateReq, *client.LabelGroupCreate]{}
		deleteBatcher := &common.Batcher[string, struct{}]{}

		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &common.ProviderData{
			Client:                  c,
			LabelGroupCreateBatcher: createBatcher,
			LabelGroupUpdateBatcher: updateBatcher,
			LabelGroupDeleteBatcher: deleteBatcher,
			ValidateRefsOnDestroy:   true,
			StrictRefsOnDestroy:     true,
		}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if r.client != c || r.createBatcher != createBatcher || r.updateBatcher != updateBatcher || r.deleteBatcher != deleteBatcher {
			t.Fatal("expected configure to set client and batchers")
		}
		if !r.validateRefsOnDestroy || !r.strictRefsOnDestroy {
			t.Fatal("expected destroy reference flags to be set")
		}
	})
}

func TestPolicyRuleReferencesLabelGroup(t *testing.T) {
	t.Run("source contains reference", func(t *testing.T) {
		rule := map[string]any{
			"source": map[string]any{
				"label_group_ids": []any{"lg-1", "lg-2"},
			},
		}
		if !policyRuleReferencesLabelGroup(rule, "lg-2") {
			t.Fatal("expected source reference to be detected")
		}
	})

	t.Run("destination contains reference", func(t *testing.T) {
		rule := map[string]any{
			"destination": map[string]any{
				"label_group_ids": []any{"lg-9"},
			},
		}
		if !policyRuleReferencesLabelGroup(rule, "lg-9") {
			t.Fatal("expected destination reference to be detected")
		}
	})

	t.Run("missing reference returns false", func(t *testing.T) {
		rule := map[string]any{
			"source": map[string]any{
				"label_group_ids": []any{"lg-1"},
			},
		}
		if policyRuleReferencesLabelGroup(rule, "lg-2") {
			t.Fatal("expected missing reference to return false")
		}
	})

	t.Run("non-map and non-slice shapes are ignored", func(t *testing.T) {
		rule := map[string]any{
			"source":      "unexpected",
			"destination": map[string]any{"label_group_ids": "not-a-list"},
		}
		if policyRuleReferencesLabelGroup(rule, "lg-1") {
			t.Fatal("expected malformed structures to return false")
		}
	})
}
