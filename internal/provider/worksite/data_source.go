package worksite

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	client worksiteDataSourceClient
}

type worksiteDataSourceClient interface {
	GetWorksite(ctx context.Context, id string) (*client.Worksite, error)
	ListWorksites(ctx context.Context, search string) ([]client.Worksite, error)
}

var _ worksiteDataSourceClient = (*client.Client)(nil)

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

	providerData := common.ProviderDataFromRequest(req.ProviderData, &resp.Diagnostics)
	if providerData == nil {
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

	result, err := d.readModel(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func findExactWorksiteByName(worksites []client.Worksite, name string) (*client.Worksite, int) {
	var match *client.Worksite
	count := 0
	for i := range worksites {
		if worksites[i].Name != name {
			continue
		}
		count++
		if match == nil {
			match = &worksites[i]
		}
	}

	return match, count
}

func addWorksiteDataSourceClientError(diags *diag.Diagnostics, action string, err error) {
	if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
		diags.AddError("Worksites Feature Disabled", err.Error())
		return
	}

	diags.AddError("Client Error", fmt.Sprintf("Unable to %s, got error: %s", action, err))
}

type worksiteReadError struct {
	Summary string
	Detail  string
}

func (d *WorksiteDataSource) readModel(ctx context.Context, data WorksiteDataSourceModel) (WorksiteDataSourceModel, *worksiteReadError) {
	var worksite *client.Worksite

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		ws, err := d.client.GetWorksite(ctx, data.ID.ValueString())
		if err != nil {
			return WorksiteDataSourceModel{}, toWorksiteReadError("read worksite by ID", err)
		}
		if ws == nil {
			return WorksiteDataSourceModel{}, &worksiteReadError{
				Summary: "Not Found",
				Detail:  fmt.Sprintf("Worksite with ID %s not found", data.ID.ValueString()),
			}
		}
		worksite = ws
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		worksites, err := d.client.ListWorksites(ctx, data.Name.ValueString())
		if err != nil {
			return WorksiteDataSourceModel{}, toWorksiteReadError("list worksites", err)
		}

		match, count := findExactWorksiteByName(worksites, data.Name.ValueString())
		if count == 0 {
			return WorksiteDataSourceModel{}, &worksiteReadError{
				Summary: "Not Found",
				Detail:  fmt.Sprintf("Worksite with name %q not found", data.Name.ValueString()),
			}
		}
		if count > 1 {
			return WorksiteDataSourceModel{}, &worksiteReadError{
				Summary: "Multiple Results",
				Detail:  fmt.Sprintf("Found %d worksites matching name %q, expected exactly one", count, data.Name.ValueString()),
			}
		}
		worksite = match
	} else {
		return WorksiteDataSourceModel{}, &worksiteReadError{
			Summary: "Invalid Configuration",
			Detail:  "Either id or name must be specified to look up a worksite.",
		}
	}

	data.ID = types.StringValue(worksite.ID)
	data.Name = types.StringValue(worksite.Name)
	data.Comment = types.StringValue(worksite.Comment)

	sm, mb := common.WorksiteIsSystemManaged(worksite)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	tflog.Trace(ctx, "read worksite data source", map[string]any{"id": worksite.ID})

	return data, nil
}

func toWorksiteReadError(action string, err error) *worksiteReadError {
	if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
		return &worksiteReadError{Summary: "Worksites Feature Disabled", Detail: err.Error()}
	}

	return &worksiteReadError{Summary: "Client Error", Detail: fmt.Sprintf("Unable to %s, got error: %s", action, err)}
}
