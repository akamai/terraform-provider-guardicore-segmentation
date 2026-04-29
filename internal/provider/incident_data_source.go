package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &IncidentDataSource{}

func NewIncidentDataSource() datasource.DataSource {
	return &IncidentDataSource{}
}

// IncidentDataSource defines the data source implementation.
type IncidentDataSource struct {
	client *client.Client
}

// IncidentDataSourceModel describes the data source data model.
type IncidentDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	IncidentType types.String `tfsdk:"incident_type"`
	Severity     types.String `tfsdk:"severity"`
	StartTime    types.Int64  `tfsdk:"start_time"`
	EndTime      types.Int64  `tfsdk:"end_time"`
	Ended        types.Bool   `tfsdk:"ended"`
	SourceIP     types.String `tfsdk:"source_ip"`
	RawJSON      types.String `tfsdk:"raw_json"`
}

func (d *IncidentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident"
}

func (d *IncidentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an incident by its ID. " +
			"The `raw_json` attribute contains the full incident object for fields not exposed as typed attributes.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the incident.",
			},
			"incident_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The type of the incident (e.g., `Incident`, `Deception`, `Network Scan`, `Reveal`, `Experimental`).",
			},
			"severity": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The severity of the incident. For example: `LOW`, `MEDIUM`, `HIGH`.",
			},
			"start_time": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The incident start time as a Unix timestamp in milliseconds.",
			},
			"end_time": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The incident end time as a Unix timestamp in milliseconds.",
			},
			"ended": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the incident has ended.",
			},
			"source_ip": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The source IP address of the incident.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The full incident object as a JSON string. Contains all fields from the API response.",
			},
		},
	}
}

func (d *IncidentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IncidentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IncidentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	incident, err := d.client.GetIncident(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read incident, got error: %s", err))
		return
	}

	if incident == nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Incident with ID %s not found. "+
			"Note: incidents created via the v4.0 API may not be immediately queryable via the v3.0 generic-incidents API. "+
			"If this incident was just created, retry after a short delay.", data.ID.ValueString()))
		return
	}

	// Map API response to model
	data.ID = types.StringValue(data.ID.ValueString())

	if v, ok := incident["type"].(string); ok {
		data.IncidentType = types.StringValue(v)
	} else {
		data.IncidentType = types.StringNull()
	}

	if v, ok := incident["severity"].(string); ok {
		data.Severity = types.StringValue(v)
	} else {
		data.Severity = types.StringNull()
	}

	if v, ok := incident["time"].(float64); ok {
		data.StartTime = types.Int64Value(int64(v))
	} else if v, ok := incident["time"].(int64); ok {
		data.StartTime = types.Int64Value(v)
	} else {
		data.StartTime = types.Int64Null()
	}

	if v, ok := incident["end_time"].(float64); ok {
		data.EndTime = types.Int64Value(int64(v))
	} else {
		data.EndTime = types.Int64Null()
	}

	if v, ok := incident["ended"].(bool); ok {
		data.Ended = types.BoolValue(v)
	} else {
		data.Ended = types.BoolNull()
	}

	if v, ok := incident["source_ip"].(string); ok {
		data.SourceIP = types.StringValue(v)
	} else {
		data.SourceIP = types.StringNull()
	}

	// Marshal full response as raw JSON
	rawBytes, err := json.Marshal(incident)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to serialize incident to JSON: %s", err))
		return
	}
	data.RawJSON = types.StringValue(string(rawBytes))

	tflog.Trace(ctx, "read incident data source", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
