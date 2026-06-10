package label

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLabelDataSource_MetadataSchemaAndConfigure(t *testing.T) {
	ds := &LabelDataSource{}

	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_label" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "key", "value", "criteria", "system_managed", "managed_by"} {
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

func TestMapLabelToDataSourceModel(t *testing.T) {
	readOnly := true
	origin := "system"
	label := &client.Label{
		ID:       "l-1",
		Key:      "env",
		Value:    "prod",
		ReadOnly: &readOnly,
		Origin:   &origin,
		DynamicCriteria: []client.LabelCriteria{
			{Field: "name", Op: "CONTAINS", Argument: "web"},
			{CompoundCriteria: []client.LabelCriteria{{Field: "container_name", Op: "EQUALS", Argument: "nginx"}}},
		},
	}

	got := mapLabelToDataSourceModel(label)
	if got.ID.ValueString() != "l-1" || got.Key.ValueString() != "env" || got.Value.ValueString() != "prod" {
		t.Fatalf("unexpected identity mapping: %+v", got)
	}
	if !got.SystemManaged.ValueBool() || got.ManagedBy.ValueString() != "system" {
		t.Fatalf("unexpected management mapping: system_managed=%v managed_by=%s", got.SystemManaged.ValueBool(), got.ManagedBy.ValueString())
	}
	if len(got.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(got.Criteria))
	}
	if got.Criteria[0].Field.ValueString() != "name" || got.Criteria[0].Op.ValueString() != "CONTAINS" {
		t.Fatalf("unexpected flat criterion mapping: %+v", got.Criteria[0])
	}
	if len(got.Criteria[1].CompoundCriteria) != 1 || got.Criteria[1].CompoundCriteria[0].Field.ValueString() != "container_name" {
		t.Fatalf("unexpected compound criterion mapping: %+v", got.Criteria[1])
	}
}

func TestLabelDataSource_ReadModel(t *testing.T) {
	t.Run("invalid configuration", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{})
		if err == nil || err.Summary != "Invalid Configuration" {
			t.Fatalf("expected invalid configuration error, got %+v", err)
		}
	})

	t.Run("read by id not found", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{ID: types.StringValue("l-1")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by id client error", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{getErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{ID: types.StringValue("l-1")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by id supplements readonly from list", func(t *testing.T) {
		readOnly := true
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{
			getResult: &client.Label{ID: "l-1", Key: "env", Value: "prod"},
			listResult: []client.Label{
				{ID: "l-1", Key: "env", Value: "prod", ReadOnly: &readOnly},
			},
		}}

		got, err := ds.readModel(context.Background(), LabelDataSourceModel{ID: types.StringValue("l-1")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if !got.SystemManaged.ValueBool() {
			t.Fatal("expected system_managed=true after readonly supplementation")
		}
	})

	t.Run("read by key and value not found", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Not Found" {
			t.Fatalf("expected not found error, got %+v", err)
		}
	})

	t.Run("read by key and value client error", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{listErr: errors.New("boom")}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Client Error" {
			t.Fatalf("expected client error, got %+v", err)
		}
	})

	t.Run("read by key and value multiple results", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{listResult: []client.Label{{ID: "1"}, {ID: "2"}}}}
		_, err := ds.readModel(context.Background(), LabelDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err == nil || err.Summary != "Multiple Results" {
			t.Fatalf("expected multiple results error, got %+v", err)
		}
	})

	t.Run("read by key and value success", func(t *testing.T) {
		ds := &LabelDataSource{client: &fakeLabelDataSourceClient{listResult: []client.Label{{ID: "l-2", Key: "env", Value: "prod"}}}}
		got, err := ds.readModel(context.Background(), LabelDataSourceModel{Key: types.StringValue("env"), Value: types.StringValue("prod")})
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if got.ID.ValueString() != "l-2" || got.Key.ValueString() != "env" || got.Value.ValueString() != "prod" {
			t.Fatalf("unexpected mapped model: %+v", got)
		}
	})
}

type fakeLabelDataSourceClient struct {
	getResult  *client.Label
	getErr     error
	listResult []client.Label
	listErr    error
}

func (f *fakeLabelDataSourceClient) GetLabel(context.Context, string) (*client.Label, error) {
	return f.getResult, f.getErr
}

func (f *fakeLabelDataSourceClient) ListLabels(context.Context, string, string) ([]client.Label, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}
