package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &AgentAggregatorDataSource{}

func NewAgentAggregatorDataSource() datasource.DataSource {
	return &AgentAggregatorDataSource{}
}

type AgentAggregatorDataSource struct {
	client *client.Client
}

type AgentAggregatorDataSourceModel struct {
	ID                          types.String                     `tfsdk:"id"`
	InternalID                  types.String                     `tfsdk:"internal_id"`
	Cls                         types.String                     `tfsdk:"cls"`
	ComponentID                 types.String                     `tfsdk:"component_id"`
	AgentID                     types.String                     `tfsdk:"agent_id"`
	Version                     types.String                     `tfsdk:"version"`
	FullVersion                 *AgentAggregatorFullVersionModel `tfsdk:"full_version"`
	BuildCommit                 types.String                     `tfsdk:"build_commit"`
	BuildDate                   types.String                     `tfsdk:"build_date"`
	InstallDate                 types.String                     `tfsdk:"install_date"`
	IPAddress                   types.String                     `tfsdk:"ip_address"`
	Hostname                    types.String                     `tfsdk:"hostname"`
	Interfaces                  []AgentAggregatorInterfaceModel  `tfsdk:"interfaces"`
	FirstSeen                   types.String                     `tfsdk:"first_seen"`
	AssociatedMgmtConfiguration types.String                     `tfsdk:"associated_mgmt_configuration"`
	DocVersion                  types.Int64                      `tfsdk:"doc_version"`
	LastSeen                    types.String                     `tfsdk:"last_seen"`
	DisplayStatus               types.String                     `tfsdk:"display_status"`
	IsMissing                   types.Bool                       `tfsdk:"is_missing"`
	State                       types.String                     `tfsdk:"state"`
	AggregatorType              types.String                     `tfsdk:"aggregator_type"`
	AggregatorFeatures          types.List                       `tfsdk:"aggregator_features"`
	ClusterID                   types.String                     `tfsdk:"cluster_id"`
	ZookeeperID                 types.Int64                      `tfsdk:"zookeeper_id"`
	SubComponents               types.List                       `tfsdk:"sub_components"`
	NetworkDevices              types.String                     `tfsdk:"network_devices"`
	IntegrationSDKCapabilities  types.List                       `tfsdk:"integration_sdk_capabilities"`
	ManagementHosts             types.List                       `tfsdk:"management_hosts"`
	ExternalFQDNAddresses       types.List                       `tfsdk:"external_fqdn_addresses"`
	AggrCertSerialNumber        types.String                     `tfsdk:"aggr_cert_serial_number"`
	CollectorType               types.String                     `tfsdk:"collector_type"`
	LegacyComponentID           types.String                     `tfsdk:"legacy_component_id"`
	EnforcementID               types.String                     `tfsdk:"enforcement_id"`
	EventletVersion             types.String                     `tfsdk:"eventlet_version"`
	ExternalAddress             types.String                     `tfsdk:"external_address"`
	HostIPs                     types.List                       `tfsdk:"host_ips"`
	InternalAddress             types.String                     `tfsdk:"internal_address"`
	ManagementHost              types.String                     `tfsdk:"management_host"`
	SystemUptime                types.String                     `tfsdk:"system_uptime"`
	GuestInstallationDetails    types.String                     `tfsdk:"guest_installation_details"`
	MitigationID                types.String                     `tfsdk:"mitigation_id"`
	TenantName                  types.String                     `tfsdk:"tenant_name"`
	IsConfigurationDirty        types.Bool                       `tfsdk:"is_configuration_dirty"`
}

type AgentAggregatorFullVersionModel struct {
	Major types.String `tfsdk:"major"`
	Minor types.String `tfsdk:"minor"`
	Tag   types.String `tfsdk:"tag"`
}

type AgentAggregatorInterfaceModel struct {
	InterfaceName types.String `tfsdk:"interface_name"`
	IPAddress     types.String `tfsdk:"ip_address"`
	Netmask       types.String `tfsdk:"netmask"`
}

func (d *AgentAggregatorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_aggregator"
}

func (d *AgentAggregatorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an agent aggregator from Akamai Guardicore Segmentation. " +
			"You can look up an agent aggregator by its ID or by hostname.\n\n" +
			"Agent aggregators are system-managed infrastructure components that cannot be created, updated, or deleted via Terraform. " +
			"This data source provides read-only access to aggregator details.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the agent aggregator. Either `id` or `hostname` must be specified.",
			},
			"hostname": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The hostname of the agent aggregator. Used to look up an aggregator by hostname.",
			},
			"internal_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The internal database identifier (`_id`).",
			},
			"cls": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The class type of the component (e.g., `SystemComponent.Aggregator`).",
			},
			"component_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The component identifier (e.g., `AGR-<uuid>`).",
			},
			"agent_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The agent identifier.",
			},
			"version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The major version string.",
			},
			"full_version": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The full version details of the aggregator.",
				Attributes: map[string]schema.Attribute{
					"major": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The major version number.",
					},
					"minor": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The minor version number.",
					},
					"tag": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The version tag (e.g., `v53.5`).",
					},
				},
			},
			"build_commit": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The build commit hash.",
			},
			"build_date": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The build date as a Unix timestamp in milliseconds.",
			},
			"install_date": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The installation date as a Unix timestamp in milliseconds.",
			},
			"ip_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The primary IP address of the aggregator.",
			},
			"interfaces": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The network interfaces of the aggregator.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"interface_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The interface name (e.g., `eth0`, `docker0`).",
						},
						"ip_address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The IP address on this interface.",
						},
						"netmask": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The netmask (CIDR prefix length).",
						},
					},
				},
			},
			"first_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the aggregator was first seen, as a Unix timestamp in milliseconds. Server-computed.",
			},
			"associated_mgmt_configuration": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The associated management configuration as a JSON string.",
			},
			"doc_version": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The document version number.",
			},
			"last_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the aggregator was last seen, as a Unix timestamp in milliseconds.",
			},
			"display_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display status of the aggregator (e.g., `UP`).",
			},
			"is_missing": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the aggregator is missing.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The state of the aggregator (e.g., `On`).",
			},
			"aggregator_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The type of aggregator (e.g., `AGENT_AGGREGATOR`).",
			},
			"aggregator_features": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The features supported by the aggregator.",
			},
			"cluster_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The cluster identifier.",
			},
			"zookeeper_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The ZooKeeper node identifier.",
			},
			"sub_components": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The sub-components running on the aggregator.",
			},
			"network_devices": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network devices information as a JSON string.",
			},
			"integration_sdk_capabilities": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The integration SDK capabilities.",
			},
			"management_hosts": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The management host addresses.",
			},
			"external_fqdn_addresses": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The external FQDN addresses.",
			},
			"aggr_cert_serial_number": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The aggregator certificate serial number.",
			},
			"collector_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The collector type (may be null).",
			},
			"legacy_component_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The legacy component ID (hyphenated API field `component-id`).",
			},
			"enforcement_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The enforcement identifier.",
			},
			"eventlet_version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The eventlet library version.",
			},
			"external_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external address.",
			},
			"host_ips": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "All host IP addresses.",
			},
			"internal_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The internal address.",
			},
			"management_host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The primary management host address.",
			},
			"system_uptime": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The system uptime string. Server-computed.",
			},
			"guest_installation_details": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Guest installation details as a JSON string.",
			},
			"mitigation_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The mitigation identifier.",
			},
			"tenant_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The tenant name (may be null).",
			},
			"is_configuration_dirty": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the aggregator configuration has unsaved changes.",
			},
		},
	}
}

func (d *AgentAggregatorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = providerData.Client
}

func (d *AgentAggregatorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentAggregatorDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var aggregator *client.AgentAggregator

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		var err error
		aggregator, err = d.client.GetAgentAggregator(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read agent aggregator by ID, got error: %s", err))
			return
		}
		if aggregator == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Agent aggregator with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Hostname.IsNull() && data.Hostname.ValueString() != "" {
		aggregators, err := d.client.ListAgentAggregators(ctx, data.Hostname.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list agent aggregators, got error: %s", err))
			return
		}

		var matches []client.AgentAggregator
		for _, a := range aggregators {
			if a.Hostname == data.Hostname.ValueString() {
				matches = append(matches, a)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Agent aggregator with hostname %q not found", data.Hostname.ValueString()))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d agent aggregators matching hostname %q, expected exactly one", len(matches), data.Hostname.ValueString()))
			return
		}
		aggregator = &matches[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or hostname must be specified to look up an agent aggregator.",
		)
		return
	}

	d.apiToModel(ctx, aggregator, &data, &resp.Diagnostics)

	tflog.Trace(ctx, "read agent aggregator data source", map[string]any{"id": aggregator.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *AgentAggregatorDataSource) apiToModel(ctx context.Context, a *client.AgentAggregator, data *AgentAggregatorDataSourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(a.ID)
	data.InternalID = types.StringValue(a.InternalID)
	data.Cls = types.StringValue(a.Cls)
	data.ComponentID = types.StringValue(a.ComponentID)
	data.AgentID = types.StringValue(a.AgentID)
	data.Version = types.StringValue(a.Version)
	data.BuildCommit = types.StringValue(a.BuildCommit)
	data.IPAddress = types.StringValue(a.IPAddress)
	data.Hostname = types.StringValue(a.Hostname)
	data.DisplayStatus = types.StringValue(a.DisplayStatus)
	data.IsMissing = types.BoolValue(a.IsMissing)
	data.State = types.StringValue(a.State)
	data.AggregatorType = types.StringValue(a.AggregatorType)
	data.ClusterID = types.StringValue(a.ClusterID)
	data.ZookeeperID = types.Int64Value(int64(a.ZookeeperID))
	data.DocVersion = types.Int64Value(int64(a.DocVersion))
	data.AggrCertSerialNumber = types.StringValue(a.AggrCertSerialNumber)
	data.LegacyComponentID = types.StringValue(a.LegacyComponentID)
	data.EnforcementID = types.StringValue(a.EnforcementID)
	data.EventletVersion = types.StringValue(a.EventletVersion)
	data.ExternalAddress = types.StringValue(a.ExternalAddress)
	data.InternalAddress = types.StringValue(a.InternalAddress)
	data.ManagementHost = types.StringValue(a.ManagementHost)
	data.SystemUptime = types.StringValue(a.SystemUptime)
	data.MitigationID = types.StringValue(a.MitigationID)
	data.IsConfigurationDirty = types.BoolValue(a.IsConfigurationDirty)

	// Timestamps (any type → string)
	data.BuildDate = anyToStringValue(a.BuildDate)
	data.InstallDate = anyToStringValue(a.InstallDate)
	data.FirstSeen = anyToStringValue(a.FirstSeen)
	data.LastSeen = anyToStringValue(a.LastSeen)

	// Nullable strings
	if a.CollectorType != nil {
		data.CollectorType = types.StringValue(*a.CollectorType)
	} else {
		data.CollectorType = types.StringNull()
	}
	if a.TenantName != nil {
		data.TenantName = types.StringValue(*a.TenantName)
	} else {
		data.TenantName = types.StringNull()
	}

	// JSON RawMessage fields
	data.AssociatedMgmtConfiguration = rawJSONToStringValue(a.AssociatedMgmtConfiguration)
	data.NetworkDevices = rawJSONToStringValue(a.NetworkDevices)
	data.GuestInstallationDetails = rawJSONToStringValue(a.GuestInstallationDetails)

	// Full version
	if a.FullVersion != nil {
		data.FullVersion = &AgentAggregatorFullVersionModel{
			Major: types.StringValue(a.FullVersion.Major),
			Minor: types.StringValue(a.FullVersion.Minor),
			Tag:   types.StringValue(a.FullVersion.Tag),
		}
	}

	// Interfaces
	if a.Interfaces != nil {
		ifaces := make([]AgentAggregatorInterfaceModel, len(a.Interfaces))
		for i, iface := range a.Interfaces {
			ifaces[i] = AgentAggregatorInterfaceModel{
				InterfaceName: types.StringValue(iface.Interface),
				IPAddress:     types.StringValue(iface.IPAddress),
				Netmask:       types.StringValue(iface.Netmask),
			}
		}
		data.Interfaces = ifaces
	}

	// String list attributes
	data.AggregatorFeatures = stringSliceToListValue(ctx, a.AggregatorFeatures, diags)
	data.SubComponents = stringSliceToListValue(ctx, a.SubComponents, diags)
	data.IntegrationSDKCapabilities = stringSliceToListValue(ctx, a.IntegrationSDKCapabilities, diags)
	data.ManagementHosts = stringSliceToListValue(ctx, a.ManagementHosts, diags)
	data.ExternalFQDNAddresses = stringSliceToListValue(ctx, a.ExternalFQDNAddresses, diags)
	data.HostIPs = stringSliceToListValue(ctx, a.HostIPs, diags)
}

func anyToStringValue(v any) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(fmt.Sprintf("%v", v))
}

func rawJSONToStringValue(raw []byte) types.String {
	if raw == nil {
		return types.StringValue("{}")
	}
	return types.StringValue(string(raw))
}

func stringSliceToListValue(ctx context.Context, s []string, diags *diag.Diagnostics) types.List {
	if s == nil {
		s = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, s)
	diags.Append(d...)
	return list
}
