package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &WorksiteDataSource{}

func NewWorksiteDataSource() datasource.DataSource {
	return &WorksiteDataSource{}
}

// WorksiteDataSource defines the data source implementation.
type WorksiteDataSource struct {
	client *client.Client
}

// WorksiteDataSourceModel describes the data source data model.
type WorksiteDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Comment       types.String `tfsdk:"comment"`
	SystemManaged types.Bool   `tfsdk:"system_managed"`
	ManagedBy     types.String `tfsdk:"managed_by"`
}

func (d *WorksiteDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worksite"
}

func (d *WorksiteDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a worksite from Akamai Guardicore Segmentation. You can look up a worksite by its ID or by name.\n\n" +
			"Use this data source to reference existing worksites, including the system-managed \"Default\" worksite " +
			"that cannot be modified by Terraform. The `system_managed` attribute indicates whether the worksite is managed by the platform.\n\n" +
			"~> **Note:** The worksites feature must be enabled on the Akamai Guardicore Segmentation instance.\n\n" +
			"Assets and policy rules can be assigned to a worksite using the `worksite_id` attribute on `guardicore_asset` and `guardicore_policy_rule` resources.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the worksite. Either id or name must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the worksite. Used to look up a worksite by name.",
			},
			"comment": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A comment for the worksite.",
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this worksite is system-managed. The \"Default\" worksite is system-managed and cannot be updated or deleted by Terraform. Use the `guardicore_worksite` data source to reference it.",
			},
			"managed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifies who manages this worksite. `terraform` for user-managed worksites, or `system` for the platform-managed \"Default\" worksite.",
			},
		},
	}
}

func (d *WorksiteDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorksiteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorksiteDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var worksite *client.Worksite

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		var err error
		worksite, err = d.client.GetWorksite(ctx, data.ID.ValueString())
		if err != nil {
			if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
				resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
			} else {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read worksite by ID, got error: %s", err))
			}
			return
		}
		if worksite == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Worksite with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		// Look up by name
		worksites, err := d.client.ListWorksites(ctx, data.Name.ValueString())
		if err != nil {
			if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
				resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
			} else {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list worksites, got error: %s", err))
			}
			return
		}

		// Filter for exact name match
		var matches []client.Worksite
		for _, w := range worksites {
			if w.Name == data.Name.ValueString() {
				matches = append(matches, w)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Worksite with name %q not found", data.Name.ValueString()))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d worksites matching name %q, expected exactly one", len(matches), data.Name.ValueString()))
			return
		}
		worksite = &matches[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or name must be specified to look up a worksite.",
		)
		return
	}

	// Map API response to model
	data.ID = types.StringValue(worksite.ID)
	data.Name = types.StringValue(worksite.Name)
	data.Comment = types.StringValue(worksite.Comment)

	sm, mb := WorksiteIsSystemManaged(worksite)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	tflog.Trace(ctx, "read worksite data source", map[string]any{"id": worksite.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
