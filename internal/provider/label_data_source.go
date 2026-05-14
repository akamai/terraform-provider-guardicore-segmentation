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
	ID            types.String         `tfsdk:"id"`
	Key           types.String         `tfsdk:"key"`
	Value         types.String         `tfsdk:"value"`
	Criteria      []LabelCriteriaModel `tfsdk:"criteria"`
	SystemManaged types.Bool           `tfsdk:"system_managed"`
	ManagedBy     types.String         `tfsdk:"managed_by"`
}

func (d *LabelDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (d *LabelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a label from Akamai Guardicore Segmentation.\n\n" +
			"Use this data source to reference existing labels, including system-managed labels that cannot be modified by Terraform. " +
			"The `system_managed` attribute indicates whether the label is managed by the platform.",

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
			"criteria": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Dynamic criteria for automatic label assignment. Criteria are emitted as an unordered set to avoid drift from API ordering differences.",
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
						"compound_criteria": schema.SetNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Nested criteria matched as an unordered compound group.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"field": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The field to match against.",
									},
									"op": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The operator for matching.",
									},
									"argument": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The argument to match against.",
									},
								},
							},
						},
					},
				},
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this label is system-managed. System-managed labels cannot be updated or deleted by Terraform. Use the `guardicore_label` data source to reference them.",
			},
			"managed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifies who manages this label. `terraform` for user-managed labels, or the system origin (e.g., `system`).",
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
		// The single-GET endpoint may not return the read_only field;
		// supplement via the list endpoint which does.
		if label.ReadOnly == nil {
			listLabels, listErr := d.client.ListLabels(ctx, label.Key, label.Value)
			if listErr == nil {
				for _, candidate := range listLabels {
					if candidate.ID == label.ID {
						label.ReadOnly = candidate.ReadOnly
						break
					}
				}
			}
		}
	} else if !data.Key.IsNull() && !data.Value.IsNull() {
		// Look up by key and value
		labels, listErr := d.client.ListLabels(ctx, data.Key.ValueString(), data.Value.ValueString())
		if listErr != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list labels, got error: %s", listErr))
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
			if len(c.CompoundCriteria) > 0 {
				compound := make([]LabelCompoundCriteriaModel, len(c.CompoundCriteria))
				for j, cc := range c.CompoundCriteria {
					compound[j] = LabelCompoundCriteriaModel{
						Field:    types.StringValue(cc.Field),
						Op:       types.StringValue(cc.Op),
						Argument: types.StringValue(cc.Argument),
					}
				}

				data.Criteria[i] = LabelCriteriaModel{
					Field:            types.StringNull(),
					Op:               types.StringNull(),
					Argument:         types.StringNull(),
					CompoundCriteria: compound,
				}
				continue
			}

			data.Criteria[i] = LabelCriteriaModel{
				Field:            types.StringValue(c.Field),
				Op:               types.StringValue(c.Op),
				Argument:         types.StringValue(c.Argument),
				CompoundCriteria: nil,
			}
		}
	} else {
		data.Criteria = nil
	}

	sm, mb := LabelIsSystemManaged(label)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	tflog.Trace(ctx, "read label data source", map[string]interface{}{"id": label.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
