package worksite

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorksiteResource_Metadata(t *testing.T) {
	r := &WorksiteResource{}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, resp)

	if resp.TypeName != "guardicore_worksite" {
		t.Fatalf("unexpected type name: %s", resp.TypeName)
	}
}

func TestWorksiteResource_Schema(t *testing.T) {
	r := &WorksiteResource{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	if len(resp.Schema.Attributes) != 5 {
		t.Fatalf("expected 5 attributes, got %d", len(resp.Schema.Attributes))
	}
	if _, ok := resp.Schema.Attributes["id"]; !ok {
		t.Fatal("expected id attribute")
	}
	if _, ok := resp.Schema.Attributes["name"]; !ok {
		t.Fatal("expected name attribute")
	}
	if _, ok := resp.Schema.Attributes["comment"]; !ok {
		t.Fatal("expected comment attribute")
	}
	if _, ok := resp.Schema.Attributes["system_managed"]; !ok {
		t.Fatal("expected system_managed attribute")
	}
	if _, ok := resp.Schema.Attributes["managed_by"]; !ok {
		t.Fatal("expected managed_by attribute")
	}
}

func TestWorksiteResource_modelToAPI(t *testing.T) {
	r := &WorksiteResource{}
	model := &WorksiteResourceModel{
		Name:    types.StringValue("HQ"),
		Comment: types.StringValue("primary"),
	}

	got := r.modelToAPI(model)
	if got.Name != "HQ" || got.Comment != "primary" {
		t.Fatalf("unexpected modelToAPI result: %+v", got)
	}
}

func TestWorksiteResource_apiToModel(t *testing.T) {
	r := &WorksiteResource{}
	model := &WorksiteResourceModel{}

	r.apiToModel(&client.Worksite{ID: "w-1", Name: "HQ", Comment: "note"}, model)

	if model.ID.ValueString() != "w-1" {
		t.Fatalf("id mismatch: %s", model.ID.ValueString())
	}
	if model.Name.ValueString() != "HQ" {
		t.Fatalf("name mismatch: %s", model.Name.ValueString())
	}
	if model.Comment.ValueString() != "note" {
		t.Fatalf("comment mismatch: %s", model.Comment.ValueString())
	}
}

func TestAddWorksiteClientError(t *testing.T) {
	t.Run("feature disabled", func(t *testing.T) {
		var diags diag.Diagnostics
		addWorksiteClientError(&diags, "read", client.ErrWorksitesFeatureDisabled)
		if !diags.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if diags.Errors()[0].Summary() != "Worksites Feature Disabled" {
			t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
		}
	})

	t.Run("generic client error", func(t *testing.T) {
		var diags diag.Diagnostics
		addWorksiteClientError(&diags, "update", errors.New("boom"))
		if !diags.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if diags.Errors()[0].Summary() != "Client Error" {
			t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
		}
	})
}

func TestWorksiteResource_createReadUpdateDeleteModelHelpers(t *testing.T) {
	r := &WorksiteResource{client: &fakeWorksiteResourceClient{createID: "w-1", getResult: &client.Worksite{ID: "w-1", Name: "HQ", Comment: "x"}}, deleteBatcher: &fakeWorksiteDeleteBatcher{}}

	created, err := r.createModel(context.Background(), WorksiteResourceModel{Name: types.StringValue("HQ"), Comment: types.StringValue("x")})
	if err != nil {
		t.Fatalf("createModel error: %v", err)
	}
	if created.ID.ValueString() != "w-1" || created.ManagedBy.ValueString() != "terraform" {
		t.Fatalf("unexpected created model: %+v", created)
	}

	read, remove, err := r.readModel(context.Background(), WorksiteResourceModel{ID: types.StringValue("w-1")})
	if err != nil || remove {
		t.Fatalf("readModel unexpected result err=%v remove=%v", err, remove)
	}
	if read.Name.ValueString() != "HQ" {
		t.Fatalf("unexpected read model: %+v", read)
	}

	updated, gone, err := r.updateModel(context.Background(), WorksiteResourceModel{ID: types.StringValue("w-1"), Name: types.StringValue("HQ2"), Comment: types.StringValue("y")}, WorksiteResourceModel{SystemManaged: types.BoolValue(false), ManagedBy: types.StringValue("terraform")})
	if err != nil {
		t.Fatalf("updateModel error: %v", err)
	}
	if gone {
		t.Fatalf("unexpected gone=true from updateModel: %+v", updated)
	}
	if updated.ID.ValueString() != "w-1" {
		t.Fatalf("unexpected updated model: %+v", updated)
	}

	if err := r.deleteModel(context.Background(), WorksiteResourceModel{ID: types.StringValue("w-1")}); err != nil {
		t.Fatalf("deleteModel error: %v", err)
	}
}

func TestWorksiteResource_readModelNotFound(t *testing.T) {
	r := &WorksiteResource{client: &fakeWorksiteResourceClient{getResult: nil}}
	_, remove, err := r.readModel(context.Background(), WorksiteResourceModel{ID: types.StringValue("w-1")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !remove {
		t.Fatal("expected remove=true")
	}
}

// TestWorksiteResource_updateModelGoneAfterWrite covers F-01: if the worksite
// is deleted server-side between UpdateWorksite and the follow-up GetWorksite,
// updateModel must report gone=true so the caller can RemoveResource instead
// of writing stale state.
func TestWorksiteResource_updateModelGoneAfterWrite(t *testing.T) {
	r := &WorksiteResource{client: &fakeWorksiteResourceClient{getResult: nil}}
	plan := WorksiteResourceModel{ID: types.StringValue("w-1"), Name: types.StringValue("HQ2"), Comment: types.StringValue("y")}
	state := WorksiteResourceModel{SystemManaged: types.BoolValue(false), ManagedBy: types.StringValue("terraform")}

	_, gone, err := r.updateModel(context.Background(), plan, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gone {
		t.Fatal("expected gone=true when GetWorksite returns nil")
	}
}

type fakeWorksiteResourceClient struct {
	createID   string
	createErr  error
	getResult  *client.Worksite
	getErr     error
	updateErr  error
	lastUpdate *client.WorksiteUpdate
}

func (f *fakeWorksiteResourceClient) CreateWorksite(context.Context, *client.WorksiteCreate) (string, error) {
	return f.createID, f.createErr
}

func (f *fakeWorksiteResourceClient) GetWorksite(context.Context, string) (*client.Worksite, error) {
	return f.getResult, f.getErr
}

func (f *fakeWorksiteResourceClient) UpdateWorksite(_ context.Context, w *client.WorksiteUpdate) error {
	f.lastUpdate = w
	return f.updateErr
}

type fakeWorksiteDeleteBatcher struct {
	err    error
	inputs []string
}

func (f *fakeWorksiteDeleteBatcher) Enqueue(_ context.Context, in string) (struct{}, error) {
	f.inputs = append(f.inputs, in)
	return struct{}{}, f.err
}
