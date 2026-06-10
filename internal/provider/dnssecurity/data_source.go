package dnssecurity

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
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
	client dnsSecurityDataSourceClient
}

type dnsSecurityDataSourceClient interface {
	GetDnsBlocklist(ctx context.Context, id string) (*client.DnsBlocklist, error)
	ListDnsBlocklists(ctx context.Context, name, listType string) ([]client.DnsBlocklist, error)
}

var _ dnsSecurityDataSourceClient = (*client.Client)(nil)

// DnsSecurityDataSourceModel describes the data source data model.
type DnsSecurityDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Domains types.Set    `tfsdk:"domains"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (d *DnsSecurityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_security"
}

func (d *DnsSecurityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a DNS security blocklist. You can look up a blocklist by its ID or by name.\n\n" +
			"~> **Note:** The DNS Security feature must be enabled on the Akamai Guardicore Segmentation instance. If you receive a \"DNS Security is not enabled\" error, enable it in the Akamai Guardicore Segmentation management console before using this data source.",

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
			"domains": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The set of domains in the blocklist.",
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

	providerData := common.ProviderDataFromRequest(req.ProviderData, &resp.Diagnostics)
	if providerData == nil {
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

	updated, readErr := d.readModel(ctx, data)
	if readErr != nil {
		resp.Diagnostics.AddError(readErr.Summary, readErr.Detail)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

type dnsSecurityReadError struct {
	Summary string
	Detail  string
}

func (d *DnsSecurityDataSource) readModel(ctx context.Context, data DnsSecurityDataSourceModel) (DnsSecurityDataSourceModel, *dnsSecurityReadError) {
	var blocklist *client.DnsBlocklist

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		var err error
		blocklist, err = d.client.GetDnsBlocklist(ctx, data.ID.ValueString())
		if err != nil {
			if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
				return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "DNS Security Feature Disabled", Detail: err.Error()}
			}
			return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Client Error", Detail: fmt.Sprintf("Unable to read DNS blocklist by ID, got error: %s", err)}
		}
		if blocklist == nil {
			return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Not Found", Detail: fmt.Sprintf("DNS blocklist with ID %s not found", data.ID.ValueString())}
		}
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		blocklists, err := d.client.ListDnsBlocklists(ctx, data.Name.ValueString(), "")
		if err != nil {
			if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
				return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "DNS Security Feature Disabled", Detail: err.Error()}
			}
			return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Client Error", Detail: fmt.Sprintf("Unable to list DNS blocklists, got error: %s", err)}
		}

		var matches []client.DnsBlocklist
		for _, b := range blocklists {
			if b.Name == data.Name.ValueString() {
				matches = append(matches, b)
			}
		}

		if len(matches) == 0 {
			return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Not Found", Detail: fmt.Sprintf("DNS blocklist with name %q not found", data.Name.ValueString())}
		}
		if len(matches) > 1 {
			return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Multiple Results", Detail: fmt.Sprintf("Found %d DNS blocklists matching name %q, expected exactly one", len(matches), data.Name.ValueString())}
		}
		blocklist = &matches[0]
	} else {
		return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Invalid Configuration", Detail: "Either id or name must be specified to look up a DNS blocklist."}
	}

	setValue, setDiags := dnsDomainsSetValue(blocklist.Domains)
	if setDiags.HasError() {
		return DnsSecurityDataSourceModel{}, &dnsSecurityReadError{Summary: "Conversion Error", Detail: setDiags.Errors()[0].Detail()}
	}

	updated := DnsSecurityDataSourceModel{
		ID:      types.StringValue(blocklist.ID),
		Name:    types.StringValue(blocklist.Name),
		Type:    types.StringValue(blocklist.Type),
		Domains: setValue,
		Enabled: types.BoolValue(blocklist.Enabled),
	}

	tflog.Trace(ctx, "read DNS security data source", map[string]interface{}{"id": blocklist.ID})

	return updated, nil
}
