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
var _ datasource.DataSource = &LabelGroupDataSource{}

func NewLabelGroupDataSource() datasource.DataSource {
	return &LabelGroupDataSource{}
}

// LabelGroupDataSource defines the data source implementation.
type LabelGroupDataSource struct {
	client *client.Client
}

// LabelGroupDataSourceModel describes the data source data model.
type LabelGroupDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Value       types.String `tfsdk:"value"`
	Comments    types.String `tfsdk:"comments"`
	Include     types.Object `tfsdk:"include"`
	Exclude     types.Object `tfsdk:"exclude"`
	IncludeJSON types.String `tfsdk:"include_json"`
	ExcludeJSON types.String `tfsdk:"exclude_json"`
}

func (d *LabelGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label_group"
}

func (d *LabelGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a label group from Akamai Guardicore Segmentation. You can look up a label group by its ID or by its key and value combination.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the label group. Either id or both key and value must be specified.",
			},
			"key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The key (category) of the label group. Used together with value to look up a label group.",
			},
			"value": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The value of the label group. Used together with key to look up a label group.",
			},
			"comments": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Comments or description for the label group.",
			},
			"include": labelGroupSelectorDataSourceAttribute("Typed selector defining labels to include using `or_groups[*].label_ids`."),
			"exclude": labelGroupSelectorDataSourceAttribute("Typed selector defining labels to exclude using `or_groups[*].label_ids`."),
			"include_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The normalized include selector JSON returned by the API.",
			},
			"exclude_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The normalized exclude selector JSON returned by the API.",
			},
		},
	}
}

func (d *LabelGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *LabelGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data LabelGroupDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var labelGroup *client.LabelGroup
	var err error

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		labelGroup, err = d.client.GetLabelGroup(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read label group by ID, got error: %s", err))
			return
		}
		if labelGroup == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Label group with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Key.IsNull() && !data.Value.IsNull() {
		// Look up by key and value
		labelGroups, err := d.client.ListLabelGroups(ctx, data.Key.ValueString(), data.Value.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list label groups, got error: %s", err))
			return
		}
		if len(labelGroups) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Label group with key=%s and value=%s not found", data.Key.ValueString(), data.Value.ValueString()))
			return
		}
		if len(labelGroups) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d label groups matching key=%s and value=%s, expected exactly one", len(labelGroups), data.Key.ValueString(), data.Value.ValueString()))
			return
		}
		labelGroup = &labelGroups[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or both key and value must be specified to look up a label group.",
		)
		return
	}

	// Map API response to model
	data.ID = types.StringValue(labelGroup.ID)
	data.Key = types.StringValue(labelGroup.Key)
	data.Value = types.StringValue(labelGroup.Value)

	if labelGroup.Comments != "" {
		data.Comments = types.StringValue(labelGroup.Comments)
	} else {
		data.Comments = types.StringNull()
	}

	includeObject, diags := labelGroupSelectorObjectFromRead(ctx, labelGroup.IncludeLabels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Include = includeObject
	includeJSON, err := normalizeLabelGroupSelectorJSON(convertOrLabelsReadToCreate(labelGroup.IncludeLabels))
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to normalize include_json: %s", err))
		return
	}
	data.IncludeJSON = includeJSON

	excludeObject, diags := labelGroupSelectorObjectFromRead(ctx, labelGroup.ExcludeLabels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Exclude = excludeObject
	excludeJSON, err := normalizeLabelGroupSelectorJSON(convertOrLabelsReadToCreate(labelGroup.ExcludeLabels))
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to normalize exclude_json: %s", err))
		return
	}
	data.ExcludeJSON = excludeJSON

	tflog.Trace(ctx, "read label group data source", map[string]interface{}{"id": labelGroup.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
