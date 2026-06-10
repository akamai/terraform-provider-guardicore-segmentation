package worksite

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorksiteDataSource_MetadataAndSchema(t *testing.T) {
	ds := &WorksiteDataSource{}
	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_worksite" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	if len(schemaResp.Schema.Attributes) != 5 {
		t.Fatalf("expected 5 attributes, got %d", len(schemaResp.Schema.Attributes))
	}
	for _, key := range []string{"id", "name", "comment", "system_managed", "managed_by"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}
}

func TestFindExactWorksiteByName(t *testing.T) {
	worksites := []client.Worksite{
		{ID: "w-1", Name: "Default"},
		{ID: "w-2", Name: "HQ"},
		{ID: "w-3", Name: "HQ"},
	}

	t.Run("no match", func(t *testing.T) {
		got, count := findExactWorksiteByName(worksites, "Missing")
		if got != nil {
			t.Fatal("expected nil match")
		}
		if count != 0 {
			t.Fatalf("expected count 0, got %d", count)
		}
	})

	t.Run("single match", func(t *testing.T) {
		got, count := findExactWorksiteByName(worksites, "Default")
		if got == nil || got.ID != "w-1" {
			t.Fatalf("unexpected match: %+v", got)
		}
		if count != 1 {
			t.Fatalf("expected count 1, got %d", count)
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		got, count := findExactWorksiteByName(worksites, "HQ")
		if got == nil || got.ID != "w-2" {
			t.Fatalf("unexpected first match: %+v", got)
		}
		if count != 2 {
			t.Fatalf("expected count 2, got %d", count)
		}
	})
}

func TestAddWorksiteDataSourceClientError(t *testing.T) {
	t.Run("feature disabled", func(t *testing.T) {
		var diags diag.Diagnostics
		addWorksiteDataSourceClientError(&diags, "list worksites", client.ErrWorksitesFeatureDisabled)
		if !diags.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if diags.Errors()[0].Summary() != "Worksites Feature Disabled" {
			t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
		}
	})

	t.Run("generic client error", func(t *testing.T) {
		var diags diag.Diagnostics
		addWorksiteDataSourceClientError(&diags, "read worksite by ID", errors.New("bad"))
		if !diags.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if diags.Errors()[0].Summary() != "Client Error" {
			t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
		}
	})
}

func TestWorksiteDataSource_readModelErrors(t *testing.T) {
	ds := &WorksiteDataSource{client: &fakeWorksiteDataSourceClient{}}

	_, err := ds.readModel(context.Background(), WorksiteDataSourceModel{})
	if err == nil || err.Summary != "Invalid Configuration" {
		t.Fatalf("expected invalid configuration error, got: %+v", err)
	}

	ds.client = &fakeWorksiteDataSourceClient{idResult: nil}
	_, err = ds.readModel(context.Background(), WorksiteDataSourceModel{ID: types.StringValue("w-1")})
	if err == nil || err.Summary != "Not Found" {
		t.Fatalf("expected not found by id, got: %+v", err)
	}

	ds.client = &fakeWorksiteDataSourceClient{nameResults: []client.Worksite{{ID: "w-1", Name: "dup"}, {ID: "w-2", Name: "dup"}}}
	_, err = ds.readModel(context.Background(), WorksiteDataSourceModel{Name: types.StringValue("dup")})
	if err == nil || err.Summary != "Multiple Results" {
		t.Fatalf("expected multiple results, got: %+v", err)
	}

	ds.client = &fakeWorksiteDataSourceClient{idErr: client.ErrWorksitesFeatureDisabled}
	_, err = ds.readModel(context.Background(), WorksiteDataSourceModel{ID: types.StringValue("w-1")})
	if err == nil || err.Summary != "Worksites Feature Disabled" {
		t.Fatalf("expected feature disabled error, got: %+v", err)
	}
}

func TestWorksiteDataSource_readModelSuccess(t *testing.T) {
	ds := &WorksiteDataSource{client: &fakeWorksiteDataSourceClient{idResult: &client.Worksite{ID: "w-1", Name: "Default", Comment: "c"}}}
	got, err := ds.readModel(context.Background(), WorksiteDataSourceModel{ID: types.StringValue("w-1")})
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if got.ManagedBy.ValueString() != "system" || !got.SystemManaged.ValueBool() {
		t.Fatalf("expected system managed mapping, got: managed_by=%s system=%v", got.ManagedBy.ValueString(), got.SystemManaged.ValueBool())
	}
}

type fakeWorksiteDataSourceClient struct {
	idResult    *client.Worksite
	idErr       error
	nameResults []client.Worksite
	nameErr     error
}

func (f *fakeWorksiteDataSourceClient) GetWorksite(context.Context, string) (*client.Worksite, error) {
	return f.idResult, f.idErr
}

func (f *fakeWorksiteDataSourceClient) ListWorksites(context.Context, string) ([]client.Worksite, error) {
	return f.nameResults, f.nameErr
}
