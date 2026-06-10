package labelgroup

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLabelGroupDataSource_MetadataSchemaAndConfigure(t *testing.T) {
	ds := &LabelGroupDataSource{}

	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_label_group" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "key", "value", "comments", "include", "exclude", "include_json", "exclude_json"} {
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

func TestMapLabelGroupToDataSourceModel(t *testing.T) {
	selector := &client.OrLabelsRead{OrLabels: []client.AndLabelsRead{{AndLabels: []client.LabelInGroup{{ID: "l-1"}, {ID: "l-2"}}}}}
	group := &client.LabelGroup{
		ID:            "lg-1",
		Key:           "env",
		Value:         "prod",
		Comments:      "note",
		IncludeLabels: selector,
		ExcludeLabels: selector,
	}

	got, err := mapLabelGroupToDataSourceModel(context.Background(), group)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if got.ID.ValueString() != "lg-1" || got.Key.ValueString() != "env" || got.Value.ValueString() != "prod" {
		t.Fatalf("unexpected identity mapping: %+v", got)
	}
	if got.Comments.ValueString() != "note" {
		t.Fatalf("unexpected comments: %s", got.Comments.ValueString())
	}
	if got.Include.IsNull() || got.Exclude.IsNull() {
		t.Fatal("expected non-null include/exclude objects")
	}
	if got.IncludeJSON.IsNull() || got.ExcludeJSON.IsNull() {
		t.Fatal("expected non-null include_json/exclude_json")
	}
}

func TestLabelGroupDataSource_ReadModel(t *testing.T) {
	t.Run("invalid configuration", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{})
		if err == nil || err.Summary != "Invalid Configuration" {
			t.Fatalf("expected invalid configuration error, got %+v", err)
		}
	})

	t.Run("read by id not found", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{ID: types.StringValue("lg-1")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by id client error", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{getErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{ID: types.StringValue("lg-1")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by id success", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{getResult: &client.LabelGroup{ID: "lg-1", Key: "env", Value: "prod"}}}
		got, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{ID: types.StringValue("lg-1")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if got.ID.ValueString() != "lg-1" || got.Key.ValueString() != "env" || got.Value.ValueString() != "prod" {
			t.Fatalf("unexpected mapped model: %+v", got)
		}
		if !got.Comments.IsNull() {
			t.Fatal("expected null comments for empty input comments")
		}
	})

	t.Run("read by key and value not found", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by key and value client error", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{listErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by key and value multiple results", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{listResult: []client.LabelGroup{{ID: "1"}, {ID: "2"}}}}
		_, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Multiple Results" {
			t.Fatalf("expected multiple results error, got %+v", err)
		}
	})

	t.Run("read by key and value success", func(t *testing.T) {
		ds := &LabelGroupDataSource{client: &fakeLabelGroupDataSourceClient{listResult: []client.LabelGroup{{ID: "lg-2", Key: "env", Value: "prod"}}}}
		got, err := ds.readModel(context.Background(), LabelGroupDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if got.ID.ValueString() != "lg-2" || got.Key.ValueString() != "env" || got.Value.ValueString() != "prod" {
			t.Fatalf("unexpected mapped model: %+v", got)
		}
	})
}

type fakeLabelGroupDataSourceClient struct {
	getResult  *client.LabelGroup
	getErr     error
	listResult []client.LabelGroup
	listErr    error
}

func (f *fakeLabelGroupDataSourceClient) GetLabelGroup(context.Context, string) (*client.LabelGroup, error) {
	return f.getResult, f.getErr
}

func (f *fakeLabelGroupDataSourceClient) ListLabelGroups(context.Context, string, string) ([]client.LabelGroup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}
