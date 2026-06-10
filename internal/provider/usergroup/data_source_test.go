package usergroup

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUserGroupDataSource_MetadataAndSchema(t *testing.T) {
	ds := &UserGroupDataSource{}
	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_user_group" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	if len(schemaResp.Schema.Attributes) != 5 {
		t.Fatalf("expected 5 attributes, got %d", len(schemaResp.Schema.Attributes))
	}
	for _, key := range []string{"id", "title", "orchestrations_groups", "system_managed", "managed_by"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}
}

func TestFindExactUserGroupByTitle(t *testing.T) {
	groups := []client.UserGroup{
		{ID: "u-1", Title: "admins"},
		{ID: "u-2", Title: "dev"},
		{ID: "u-3", Title: "dev"},
	}

	t.Run("no match", func(t *testing.T) {
		got, count := findExactUserGroupByTitle(groups, "ops")
		if got != nil {
			t.Fatal("expected nil match")
		}
		if count != 0 {
			t.Fatalf("expected 0 matches, got %d", count)
		}
	})

	t.Run("single match", func(t *testing.T) {
		got, count := findExactUserGroupByTitle(groups, "admins")
		if got == nil || got.ID != "u-1" {
			t.Fatalf("unexpected match: %+v", got)
		}
		if count != 1 {
			t.Fatalf("expected 1 match, got %d", count)
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		got, count := findExactUserGroupByTitle(groups, "dev")
		if got == nil || got.ID != "u-2" {
			t.Fatalf("unexpected first match: %+v", got)
		}
		if count != 2 {
			t.Fatalf("expected 2 matches, got %d", count)
		}
	})
}

func TestAddUserGroupDataSourceClientError(t *testing.T) {
	var diags diag.Diagnostics
	addUserGroupDataSourceClientError(&diags, "list user groups", errors.New("bad"))
	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if diags.Errors()[0].Summary() != "Client Error" {
		t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
	}
}

func TestUserGroupDataSource_readModel(t *testing.T) {
	ds := &UserGroupDataSource{client: &fakeUserGroupDataSourceClient{}}

	_, err := ds.readModel(context.Background(), UserGroupDataSourceModel{}, &diag.Diagnostics{})
	if err == nil || err.Summary != "Invalid Configuration" {
		t.Fatalf("expected invalid configuration, got: %+v", err)
	}

	ds.client = &fakeUserGroupDataSourceClient{idResult: nil}
	_, err = ds.readModel(context.Background(), UserGroupDataSourceModel{ID: types.StringValue("u-1")}, &diag.Diagnostics{})
	if err == nil || err.Summary != "Not Found" {
		t.Fatalf("expected not found, got: %+v", err)
	}

	ds.client = &fakeUserGroupDataSourceClient{titleResults: []client.UserGroup{{ID: "u-1", Title: "dup"}, {ID: "u-2", Title: "dup"}}}
	_, err = ds.readModel(context.Background(), UserGroupDataSourceModel{Title: types.StringValue("dup")}, &diag.Diagnostics{})
	if err == nil || err.Summary != "Multiple Results" {
		t.Fatalf("expected multiple results, got: %+v", err)
	}

	ds.client = &fakeUserGroupDataSourceClient{idResult: &client.UserGroup{ID: "u-1", Title: "admins"}}
	got, err := ds.readModel(context.Background(), UserGroupDataSourceModel{ID: types.StringValue("u-1")}, &diag.Diagnostics{})
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if got.ID.ValueString() != "u-1" || got.Title.ValueString() != "admins" {
		t.Fatalf("unexpected model: %+v", got)
	}
}

type fakeUserGroupDataSourceClient struct {
	idResult     *client.UserGroup
	idErr        error
	titleResults []client.UserGroup
	titleErr     error
}

func (f *fakeUserGroupDataSourceClient) GetUserGroup(context.Context, string) (*client.UserGroup, error) {
	return f.idResult, f.idErr
}

func (f *fakeUserGroupDataSourceClient) ListUserGroups(context.Context, string) ([]client.UserGroup, error) {
	return f.titleResults, f.titleErr
}
