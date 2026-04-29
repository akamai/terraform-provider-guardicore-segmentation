package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DnsSecurityDataSource{}

func NewDnsSecurityDataSource() datasource.DataSource {
	return &DnsSecurityDataSource{}
}

// DnsSecurityDataSource defines the data source implementation.
type DnsSecurityDataSource struct {
	client *client.Client
}

// DnsSecurityDataSourceModel describes the data source data model.
type DnsSecurityDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Domains types.List   `tfsdk:"domains"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (d *DnsSecurityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_security"
}

func (d *DnsSecurityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a DNS security blocklist. You can look up a blocklist by its ID or by name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the DNS blocklist. Either id or name must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the DNS blocklist. Used to look up a blocklist by name.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The type of the DNS blocklist.",
			},
			"domains": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The list of domains in the blocklist.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the DNS blocklist is enabled.",
			},
		},
	}
}

func (d *DnsSecurityDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DnsSecurityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DnsSecurityDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var blocklist *client.DnsBlocklist

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		var err error
		blocklist, err = d.client.GetDnsBlocklist(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read DNS blocklist by ID, got error: %s", err))
			return
		}
		if blocklist == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("DNS blocklist with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		// Look up by name
		blocklists, err := d.client.ListDnsBlocklists(ctx, data.Name.ValueString(), "")
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list DNS blocklists, got error: %s", err))
			return
		}

		// Filter for exact name match
		var matches []client.DnsBlocklist
		for _, b := range blocklists {
			if b.Name == data.Name.ValueString() {
				matches = append(matches, b)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("DNS blocklist with name %q not found", data.Name.ValueString()))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d DNS blocklists matching name %q, expected exactly one", len(matches), data.Name.ValueString()))
			return
		}
		blocklist = &matches[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or name must be specified to look up a DNS blocklist.",
		)
		return
	}

	// Map API response to model
	data.ID = types.StringValue(blocklist.ID)
	data.Name = types.StringValue(blocklist.Name)
	data.Type = types.StringValue(blocklist.Type)
	data.Enabled = types.BoolValue(blocklist.Enabled)

	if len(blocklist.Domains) > 0 {
		sortedDomains := make([]string, len(blocklist.Domains))
		copy(sortedDomains, blocklist.Domains)
		sort.Strings(sortedDomains)
		data.Domains, _ = types.ListValueFrom(ctx, types.StringType, sortedDomains)
	} else {
		data.Domains = types.ListNull(types.StringType)
	}

	tflog.Trace(ctx, "read DNS security data source", map[string]interface{}{"id": blocklist.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
