package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &LabelDataSource{}

func NewLabelDataSource() datasource.DataSource {
	return &LabelDataSource{}
}

// LabelDataSource defines the data source implementation.
type LabelDataSource struct {
	client *client.Client
}

// LabelDataSourceModel describes the data source data model.
type LabelDataSourceModel struct {
	ID       types.String         `tfsdk:"id"`
	Key      types.String         `tfsdk:"key"`
	Value    types.String         `tfsdk:"value"`
	Criteria []LabelCriteriaModel `tfsdk:"criteria"`
}

func (d *LabelDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (d *LabelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a label from Akamai Guardicore Segmentation. You can look up a label by its ID or by its key and value combination.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the label. Either id or both key and value must be specified.",
			},
			"key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The key (category) of the label. Used together with value to look up a label.",
			},
			"value": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The value of the label. Used together with key to look up a label.",
			},
			"criteria": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Dynamic criteria for automatic label assignment.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The field to match against (e.g., 'name', 'ip', 'os_name', 'process_name').",
						},
						"op": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The operator for matching (e.g., 'EQUALS', 'CONTAINS', 'STARTSWITH', 'ENDSWITH').",
						},
						"argument": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The argument to match against.",
						},
					},
				},
			},
		},
	}
}

func (d *LabelDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *LabelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data LabelDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var label *client.Label
	var err error

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		label, err = d.client.GetLabel(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read label by ID, got error: %s", err))
			return
		}
		if label == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Label with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Key.IsNull() && !data.Value.IsNull() {
		// Look up by key and value
		labels, err := d.client.ListLabels(ctx, data.Key.ValueString(), data.Value.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list labels, got error: %s", err))
			return
		}
		if len(labels) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Label with key=%s and value=%s not found", data.Key.ValueString(), data.Value.ValueString()))
			return
		}
		if len(labels) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d labels matching key=%s and value=%s, expected exactly one", len(labels), data.Key.ValueString(), data.Value.ValueString()))
			return
		}
		label = &labels[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or both key and value must be specified to look up a label.",
		)
		return
	}

	// Map API response to model
	data.ID = types.StringValue(label.ID)
	data.Key = types.StringValue(label.Key)
	data.Value = types.StringValue(label.Value)

	if len(label.DynamicCriteria) > 0 {
		data.Criteria = make([]LabelCriteriaModel, len(label.DynamicCriteria))
		for i, c := range label.DynamicCriteria {
			data.Criteria[i] = LabelCriteriaModel{
				Field:    types.StringValue(c.Field),
				Op:       types.StringValue(c.Op),
				Argument: types.StringValue(c.Argument),
			}
		}
	} else {
		data.Criteria = nil
	}

	tflog.Trace(ctx, "read label data source", map[string]interface{}{"id": label.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
