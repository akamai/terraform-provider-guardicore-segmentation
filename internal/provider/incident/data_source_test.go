package incident

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIncidentDataSource_MetadataAndSchema(t *testing.T) {
	ds := &IncidentDataSource{}
	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_incident" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	if len(schemaResp.Schema.Attributes) != 8 {
		t.Fatalf("expected 8 attributes, got %d", len(schemaResp.Schema.Attributes))
	}
	for _, key := range []string{"id", "incident_type", "severity", "start_time", "end_time", "ended", "source_ip", "raw_json"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}
}

func TestMapIncidentToDataSourceModel(t *testing.T) {
	incident := map[string]any{
		"type":      "Incident",
		"severity":  "LOW",
		"time":      float64(123),
		"end_time":  float64(456),
		"ended":     true,
		"source_ip": "10.0.0.1",
	}

	got, err := mapIncidentToDataSourceModel("i-1", incident)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID.ValueString() != "i-1" || got.IncidentType.ValueString() != "Incident" {
		t.Fatalf("unexpected id/type: %+v", got)
	}
	if got.Severity.ValueString() != "LOW" {
		t.Fatalf("unexpected severity: %s", got.Severity.ValueString())
	}
	if got.StartTime.ValueInt64() != 123 || got.EndTime.ValueInt64() != 456 {
		t.Fatalf("unexpected times: start=%d end=%d", got.StartTime.ValueInt64(), got.EndTime.ValueInt64())
	}
	if !got.Ended.ValueBool() {
		t.Fatal("expected ended=true")
	}
	if got.SourceIP.ValueString() != "10.0.0.1" {
		t.Fatalf("unexpected source_ip: %s", got.SourceIP.ValueString())
	}
	if got.RawJSON.IsNull() || got.RawJSON.ValueString() == "" {
		t.Fatal("expected non-empty raw_json")
	}
}

func TestMapIncidentToDataSourceModel_NullsForMissingFields(t *testing.T) {
	got, err := mapIncidentToDataSourceModel("i-2", map[string]any{"time": int64(77)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IncidentType.ValueString() != "" || !got.IncidentType.IsNull() {
		t.Fatal("expected incident_type null")
	}
	if got.Severity.ValueString() != "" || !got.Severity.IsNull() {
		t.Fatal("expected severity null")
	}
	if got.StartTime.ValueInt64() != 77 {
		t.Fatalf("expected start_time 77, got %d", got.StartTime.ValueInt64())
	}
	if !got.EndTime.IsNull() || !got.Ended.IsNull() || !got.SourceIP.IsNull() {
		t.Fatal("expected null end_time/ended/source_ip")
	}
}

func TestAddIncidentDataSourceClientError(t *testing.T) {
	var diags diag.Diagnostics
	addIncidentDataSourceClientError(&diags, "read incident", errors.New("bad"))
	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if diags.Errors()[0].Summary() != "Client Error" {
		t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
	}
}

func TestIncidentDataSource_readModel(t *testing.T) {
	ds := &IncidentDataSource{client: &fakeIncidentDataSourceClient{}}

	_, err := ds.readModel(context.Background(), IncidentDataSourceModel{ID: types.StringValue("i-1")})
	if err == nil || err.Summary != "Not Found" {
		t.Fatalf("expected not found error, got: %+v", err)
	}

	ds.client = &fakeIncidentDataSourceClient{err: errors.New("boom")}
	_, err = ds.readModel(context.Background(), IncidentDataSourceModel{ID: types.StringValue("i-1")})
	if err == nil || err.Summary != "Client Error" {
		t.Fatalf("expected client error, got: %+v", err)
	}

	ds.client = &fakeIncidentDataSourceClient{incident: map[string]any{"type": "Incident", "severity": "LOW", "time": float64(1)}}
	got, err := ds.readModel(context.Background(), IncidentDataSourceModel{ID: types.StringValue("i-1")})
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if got.ID.ValueString() != "i-1" || got.IncidentType.ValueString() != "Incident" {
		t.Fatalf("unexpected mapped model: %+v", got)
	}
}

type fakeIncidentDataSourceClient struct {
	incident map[string]any
	err      error
}

func (f *fakeIncidentDataSourceClient) GetIncident(context.Context, string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.incident, nil
}
