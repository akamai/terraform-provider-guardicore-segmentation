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
var _ datasource.DataSource = &UserGroupDataSource{}

func NewUserGroupDataSource() datasource.DataSource {
	return &UserGroupDataSource{}
}

// UserGroupDataSource defines the data source implementation.
type UserGroupDataSource struct {
	client *client.Client
}

// UserGroupDataSourceModel describes the data source data model.
type UserGroupDataSourceModel struct {
	ID                   types.String              `tfsdk:"id"`
	Title                types.String              `tfsdk:"title"`
	OrchestrationsGroups []OrchestrationGroupModel `tfsdk:"orchestrations_groups"`
	SystemManaged        types.Bool                `tfsdk:"system_managed"`
	ManagedBy            types.String              `tfsdk:"managed_by"`
}

func (d *UserGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (d *UserGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a user group from Akamai Guardicore Segmentation.\n\n" +
			"Use this data source to reference existing user groups, including system-managed groups that cannot be modified by Terraform. " +
			"The `system_managed` attribute indicates whether the group is managed by the platform.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the user group. Either id or title must be specified.",
			},
			"title": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The title of the user group. Used to look up a user group by title.",
			},
			"orchestrations_groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of orchestration groups included in the user group.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"orchestration_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The orchestration ID.",
						},
						"groups": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "List of group IDs within the orchestration.",
						},
					},
				},
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this user group is system-managed. System-managed groups cannot be updated or deleted by Terraform. Use the `guardicore_user_group` data source to reference them.",
			},
			"managed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifies who manages this user group. `terraform` for user-managed groups, or `system` for platform-managed groups.",
			},
		},
	}
}

func (d *UserGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserGroupDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var userGroup *client.UserGroup

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		var err error
		userGroup, err = d.client.GetUserGroup(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user group by ID, got error: %s", err))
			return
		}
		if userGroup == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("User group with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Title.IsNull() && data.Title.ValueString() != "" {
		// Look up by title
		userGroups, err := d.client.ListUserGroups(ctx, data.Title.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list user groups, got error: %s", err))
			return
		}

		// Filter for exact title match
		var matches []client.UserGroup
		for _, ug := range userGroups {
			if ug.Title == data.Title.ValueString() {
				matches = append(matches, ug)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("User group with title %q not found", data.Title.ValueString()))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d user groups matching title %q, expected exactly one", len(matches), data.Title.ValueString()))
			return
		}
		userGroup = &matches[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or title must be specified to look up a user group.",
		)
		return
	}

	// Map API response to model
	data.ID = types.StringValue(userGroup.ID)
	data.Title = types.StringValue(userGroup.Title)
	data.OrchestrationsGroups = resolveOrchestrationGroups(ctx, userGroup, &resp.Diagnostics)

	sm, mb := UserGroupIsSystemManaged(userGroup)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	tflog.Trace(ctx, "read user group data source", map[string]any{"id": userGroup.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
