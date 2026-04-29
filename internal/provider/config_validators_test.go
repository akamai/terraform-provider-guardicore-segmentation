package provider

import (
	"context"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestLabelGroupRefValidator_NilClient(t *testing.T) {
	v := &labelGroupRefValidator{client: nil}

	resp := &resource.ValidateConfigResponse{
		Diagnostics: diag.Diagnostics{},
	}
	v.ValidateResource(context.Background(), resource.ValidateConfigRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatal("expected no errors when client is nil")
	}
}

func TestLabelGroupRefValidator_Description(t *testing.T) {
	v := &labelGroupRefValidator{}

	desc := v.Description(context.Background())
	if desc == "" {
		t.Fatal("expected non-empty description")
	}

	md := v.MarkdownDescription(context.Background())
	if md == "" {
		t.Fatal("expected non-empty markdown description")
	}
}

func TestPolicyRuleRefValidator_NilClient(t *testing.T) {
	v := &policyRuleRefValidator{client: nil}

	resp := &resource.ValidateConfigResponse{
		Diagnostics: diag.Diagnostics{},
	}
	v.ValidateResource(context.Background(), resource.ValidateConfigRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatal("expected no errors when client is nil")
	}
}

func TestPolicyRuleRefValidator_Description(t *testing.T) {
	v := &policyRuleRefValidator{}

	desc := v.Description(context.Background())
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
}

func TestAssetRefValidator_NilClient(t *testing.T) {
	v := &assetRefValidator{client: nil}

	resp := &resource.ValidateConfigResponse{
		Diagnostics: diag.Diagnostics{},
	}
	v.ValidateResource(context.Background(), resource.ValidateConfigRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatal("expected no errors when client is nil")
	}
}

func TestAssetRefValidator_Description(t *testing.T) {
	v := &assetRefValidator{}

	desc := v.Description(context.Background())
	if desc == "" {
		t.Fatal("expected non-empty description")
	}
}

// Verify that *client.Client satisfies the ReferenceChecker interface.
// This is a compile-time check.
var _ ReferenceChecker = (*client.Client)(nil)
