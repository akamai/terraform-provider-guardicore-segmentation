package incident

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIncidentResource_Metadata(t *testing.T) {
	r := &IncidentResource{}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, resp)

	if resp.TypeName != "guardicore_incident" {
		t.Fatalf("unexpected type name: %s", resp.TypeName)
	}
}

func TestIncidentResource_Schema(t *testing.T) {
	r := &IncidentResource{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	if len(resp.Schema.Attributes) < 10 {
		t.Fatalf("expected many attributes, got %d", len(resp.Schema.Attributes))
	}
	if _, ok := resp.Schema.Attributes["id"]; !ok {
		t.Fatal("expected id attribute")
	}
	if _, ok := resp.Schema.Attributes["type"]; !ok {
		t.Fatal("expected type attribute")
	}
	if _, ok := resp.Schema.Attributes["severity"]; !ok {
		t.Fatal("expected severity attribute")
	}
	if _, ok := resp.Schema.Attributes["affected_assets_json"]; !ok {
		t.Fatal("expected affected_assets_json attribute")
	}
}

func TestIncidentResource_modelToAPI_RequiredOnly(t *testing.T) {
	r := &IncidentResource{}
	tags, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"tag-1"})
	if diags.HasError() {
		t.Fatalf("tags diag: %+v", diags)
	}

	model := &IncidentResourceModel{
		Type:               types.StringValue("CustomIncident"),
		Severity:           types.StringValue("LOW"),
		AffectedAssetsJSON: types.StringValue(`[{"type":"IP","value":"10.0.0.1"}]`),
		Time:               types.Int64Value(1),
		Tags:               tags,
		Description:        types.StringValue("d"),
		Summary:            types.StringValue("s"),
	}

	api, outDiags := r.modelToAPI(context.Background(), model)
	if outDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", outDiags)
	}
	if api.Type != "CustomIncident" || api.Severity != "LOW" {
		t.Fatalf("unexpected core fields: %+v", api)
	}
	if string(api.AffectedAssets) != `[{"type":"IP","value":"10.0.0.1"}]` {
		t.Fatalf("unexpected affected_assets: %s", string(api.AffectedAssets))
	}
	if len(api.Tags) != 1 || api.Tags[0] != "tag-1" {
		t.Fatalf("unexpected tags: %#v", api.Tags)
	}
	if api.Origin != nil || api.Mitigation != nil || api.AttachedFiles != nil {
		t.Fatalf("unexpected optional fields set: %+v", api)
	}
}

func TestIncidentResource_modelToAPI_WithOptionalFields(t *testing.T) {
	r := &IncidentResource{}
	tags, tDiags := types.ListValueFrom(context.Background(), types.StringType, []string{"t1", "t2"})
	if tDiags.HasError() {
		t.Fatalf("tags diag: %+v", tDiags)
	}
	files, fDiags := types.ListValueFrom(context.Background(), types.StringType, []string{"f1", "f2"})
	if fDiags.HasError() {
		t.Fatalf("files diag: %+v", fDiags)
	}

	model := &IncidentResourceModel{
		Type:                     types.StringValue("CustomIncident"),
		Severity:                 types.StringValue("HIGH"),
		AffectedAssetsJSON:       types.StringValue(`[]`),
		Time:                     types.Int64Value(2),
		Tags:                     tags,
		Description:              types.StringValue("desc"),
		Summary:                  types.StringValue("summary"),
		Origin:                   types.StringValue("Terraform"),
		Mitigation:               types.StringValue("Mitigate"),
		CefExtensionsJSON:        types.StringValue(`{"k":"v"}`),
		AttachedFiles:            files,
		MapDetailsJSON:           types.StringValue(`{"map":true}`),
		CustomDefinedObjectsJSON: types.StringValue(`[{"id":1}]`),
		PropertiesJSON:           types.StringValue(`[{"key":"k","value":"v"}]`),
	}

	api, outDiags := r.modelToAPI(context.Background(), model)
	if outDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", outDiags)
	}
	if api.Origin == nil || *api.Origin != "Terraform" {
		t.Fatalf("unexpected origin: %#v", api.Origin)
	}
	if api.Mitigation == nil || *api.Mitigation != "Mitigate" {
		t.Fatalf("unexpected mitigation: %#v", api.Mitigation)
	}
	if string(api.CefExtensions) != `{"k":"v"}` || string(api.MapDetails) != `{"map":true}` {
		t.Fatalf("unexpected json optional fields")
	}
	if !reflect.DeepEqual(api.AttachedFiles, []string{"f1", "f2"}) {
		t.Fatalf("attached files mismatch: %#v", api.AttachedFiles)
	}
	if string(api.CustomDefinedObjects) != `[{"id":1}]` || string(api.Properties) != `[{"key":"k","value":"v"}]` {
		t.Fatalf("unexpected json optional list fields")
	}
}

func TestAddIncidentClientError(t *testing.T) {
	var diags diag.Diagnostics
	addIncidentClientError(&diags, "create", errors.New("boom"))
	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if diags.Errors()[0].Summary() != "Client Error" {
		t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
	}
}

func TestIncidentResource_ModelHelpers(t *testing.T) {
	tags, d := types.ListValueFrom(context.Background(), types.StringType, []string{"t"})
	if d.HasError() {
		t.Fatalf("unexpected list diagnostics: %+v", d)
	}

	r := &IncidentResource{
		client:        &fakeIncidentResourceClient{},
		createBatcher: &fakeIncidentCreateBatcher{id: "i-1"},
	}

	model := IncidentResourceModel{
		Type:               types.StringValue("CustomIncident"),
		Severity:           types.StringValue("LOW"),
		AffectedAssetsJSON: types.StringValue(`[]`),
		Time:               types.Int64Value(1),
		Tags:               tags,
		Description:        types.StringValue("d"),
		Summary:            types.StringValue("s"),
	}

	created, err := r.createModel(context.Background(), model)
	if err != nil {
		t.Fatalf("createModel error: %v", err)
	}
	if created.ID.ValueString() != "i-1" {
		t.Fatalf("unexpected created model: %+v", created)
	}

	read := r.readModel(context.Background(), IncidentResourceModel{ID: types.StringValue("i-1")})
	if read.ID.ValueString() != "i-1" {
		t.Fatalf("unexpected read model: %+v", read)
	}
}

type fakeIncidentResourceClient struct {
	incident map[string]any
	err      error
}

func (f *fakeIncidentResourceClient) GetIncident(context.Context, string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.incident, nil
}

type fakeIncidentCreateBatcher struct {
	id  string
	err error
}

func (f *fakeIncidentCreateBatcher) Enqueue(context.Context, *client.IncidentCreate) (string, error) {
	return f.id, f.err
}
