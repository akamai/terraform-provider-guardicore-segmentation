package agentaggregator

import (
	"context"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAgentAggregatorDataSource_MetadataAndSchema(t *testing.T) {
	ds := &AgentAggregatorDataSource{}

	metaResp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_agent_aggregator" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	for _, key := range []string{"id", "hostname", "full_version", "interfaces", "tenant_name", "is_configuration_dirty"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}
}

func TestAgentAggregatorDataSource_Configure(t *testing.T) {
	t.Run("nil provider data", func(t *testing.T) {
		ds := &AgentAggregatorDataSource{}
		resp := &datasource.ConfigureResponse{}
		ds.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("unexpected provider data type", func(t *testing.T) {
		ds := &AgentAggregatorDataSource{}
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

		ds := &AgentAggregatorDataSource{}
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

func TestAgentAggregatorDataSource_apiToModel(t *testing.T) {
	ctx := context.Background()

	collectorType := "collector"
	tenantName := "tenant-a"
	agg := &client.AgentAggregator{
		ID:                          "ag-1",
		InternalID:                  "internal-1",
		Cls:                         "SystemComponent.Aggregator",
		ComponentID:                 "AGR-1",
		AgentID:                     "agent-1",
		Version:                     "53",
		FullVersion:                 &client.AgentAggregatorFullVersion{Major: "53", Minor: "5", Tag: "v53.5"},
		BuildCommit:                 "abc123",
		BuildDate:                   123,
		InstallDate:                 "456",
		IPAddress:                   "10.0.0.1",
		Hostname:                    "agg.local",
		Interfaces:                  []client.AgentAggregatorInterface{{Interface: "eth0", IPAddress: "10.0.0.1", Netmask: "24"}},
		FirstSeen:                   float64(789),
		AssociatedMgmtConfiguration: []byte(`{"mode":"default"}`),
		DocVersion:                  2,
		LastSeen:                    int64(1000),
		DisplayStatus:               "UP",
		IsMissing:                   false,
		State:                       "On",
		AggregatorType:              "AGENT_AGGREGATOR",
		AggregatorFeatures:          []string{"f1", "f2"},
		ClusterID:                   "cluster-1",
		ZookeeperID:                 7,
		SubComponents:               []string{"svc1"},
		NetworkDevices:              []byte(`{"count":1}`),
		IntegrationSDKCapabilities:  []string{"cap1"},
		ManagementHosts:             []string{"mgmt-1"},
		ExternalFQDNAddresses:       []string{"agg.example.com"},
		AggrCertSerialNumber:        "cert-1",
		CollectorType:               &collectorType,
		LegacyComponentID:           "legacy-1",
		EnforcementID:               "enf-1",
		EventletVersion:             "0.33",
		ExternalAddress:             "203.0.113.10",
		HostIPs:                     []string{"10.0.0.1", "10.0.0.2"},
		InternalAddress:             "10.0.0.1",
		ManagementHost:              "mgmt.local",
		SystemUptime:                "1 day",
		GuestInstallationDetails:    []byte(`{"guest":true}`),
		MitigationID:                "mit-1",
		TenantName:                  &tenantName,
		IsConfigurationDirty:        true,
	}

	data := &AgentAggregatorDataSourceModel{}
	var diags diag.Diagnostics

	ds := &AgentAggregatorDataSource{}
	ds.apiToModel(ctx, agg, data, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.ID.ValueString() != "ag-1" || data.Hostname.ValueString() != "agg.local" {
		t.Fatalf("unexpected mapped id/hostname: id=%s hostname=%s", data.ID.ValueString(), data.Hostname.ValueString())
	}
	if data.FullVersion == nil || data.FullVersion.Tag.ValueString() != "v53.5" {
		t.Fatalf("unexpected full_version mapping: %+v", data.FullVersion)
	}
	if len(data.Interfaces) != 1 || data.Interfaces[0].InterfaceName.ValueString() != "eth0" {
		t.Fatalf("unexpected interfaces mapping: %+v", data.Interfaces)
	}
	if data.BuildDate.ValueString() != "123" || data.InstallDate.ValueString() != "456" || data.FirstSeen.ValueString() != "789" {
		t.Fatalf("unexpected timestamp mapping: build=%s install=%s first_seen=%s", data.BuildDate.ValueString(), data.InstallDate.ValueString(), data.FirstSeen.ValueString())
	}
	if data.AssociatedMgmtConfiguration.ValueString() != `{"mode":"default"}` {
		t.Fatalf("unexpected associated_mgmt_configuration: %s", data.AssociatedMgmtConfiguration.ValueString())
	}
	if data.CollectorType.ValueString() != "collector" || data.TenantName.ValueString() != "tenant-a" {
		t.Fatalf("unexpected nullable fields: collector=%s tenant=%s", data.CollectorType.ValueString(), data.TenantName.ValueString())
	}
	if data.AggregatorFeatures.IsNull() || data.AggregatorFeatures.IsUnknown() {
		t.Fatal("expected aggregator_features list value")
	}
	if data.IsConfigurationDirty.ValueBool() != true {
		t.Fatal("expected is_configuration_dirty=true")
	}
}

func TestAgentAggregatorDataSource_apiToModel_NullablesAndDefaults(t *testing.T) {
	ctx := context.Background()
	agg := &client.AgentAggregator{}

	data := &AgentAggregatorDataSourceModel{}
	var diags diag.Diagnostics

	ds := &AgentAggregatorDataSource{}
	ds.apiToModel(ctx, agg, data, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !data.CollectorType.IsNull() || !data.TenantName.IsNull() {
		t.Fatalf("expected nullable strings to be null: collector=%v tenant=%v", data.CollectorType, data.TenantName)
	}
	if data.AssociatedMgmtConfiguration.ValueString() != "{}" || data.NetworkDevices.ValueString() != "{}" || data.GuestInstallationDetails.ValueString() != "{}" {
		t.Fatalf("expected nil raw json fields to map to {}: assoc=%s network=%s guest=%s", data.AssociatedMgmtConfiguration.ValueString(), data.NetworkDevices.ValueString(), data.GuestInstallationDetails.ValueString())
	}
	if data.FullVersion != nil {
		t.Fatalf("expected full_version to remain nil, got: %+v", data.FullVersion)
	}
	if data.Interfaces != nil {
		t.Fatalf("expected interfaces to remain nil, got: %+v", data.Interfaces)
	}
	if data.AggregatorFeatures.IsNull() || data.SubComponents.IsNull() || data.HostIPs.IsNull() {
		t.Fatal("expected list attributes to be non-null empty lists")
	}
}

func TestAnyToStringValue(t *testing.T) {
	if !anyToStringValue(nil).IsNull() {
		t.Fatal("expected null for nil")
	}
	if anyToStringValue(42).ValueString() != "42" {
		t.Fatalf("expected 42, got %s", anyToStringValue(42).ValueString())
	}
	if anyToStringValue("x").ValueString() != "x" {
		t.Fatalf("expected x, got %s", anyToStringValue("x").ValueString())
	}
}

func TestRawJSONToStringValue(t *testing.T) {
	if rawJSONToStringValue(nil).ValueString() != "{}" {
		t.Fatalf("expected {} for nil raw json, got %s", rawJSONToStringValue(nil).ValueString())
	}
	if rawJSONToStringValue([]byte(`{"a":1}`)).ValueString() != `{"a":1}` {
		t.Fatalf("unexpected mapped raw json: %s", rawJSONToStringValue([]byte(`{"a":1}`)).ValueString())
	}
}

func TestStringSliceToListValue(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	list := stringSliceToListValue(ctx, nil, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for nil slice: %v", diags)
	}
	if list.IsNull() || list.IsUnknown() {
		t.Fatal("expected non-null known list")
	}
	if len(list.Elements()) != 0 {
		t.Fatalf("expected empty list, got %d elements", len(list.Elements()))
	}

	list = stringSliceToListValue(ctx, []string{"a", "b"}, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for non-empty slice: %v", diags)
	}
	if len(list.Elements()) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements()))
	}
	first, ok := list.Elements()[0].(types.String)
	if !ok {
		t.Fatalf("expected first element to be types.String, got %T", list.Elements()[0])
	}
	second, ok := list.Elements()[1].(types.String)
	if !ok {
		t.Fatalf("expected second element to be types.String, got %T", list.Elements()[1])
	}
	if first.ValueString() != "a" || second.ValueString() != "b" {
		t.Fatalf("unexpected list elements: %+v", list.Elements())
	}
}
