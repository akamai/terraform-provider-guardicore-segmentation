package usergroup

import (
	"context"
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
var _ datasource.DataSource = &UserGroupDataSource{}

func NewUserGroupDataSource() datasource.DataSource {
	return &UserGroupDataSource{}
}

// UserGroupDataSource defines the data source implementation.
type UserGroupDataSource struct {
	client userGroupDataSourceClient
}

type userGroupDataSourceClient interface {
	GetUserGroup(ctx context.Context, id string) (*client.UserGroup, error)
	ListUserGroups(ctx context.Context, search string) ([]client.UserGroup, error)
}

var _ userGroupDataSourceClient = (*client.Client)(nil)

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

	providerData := common.ProviderDataFromRequest(req.ProviderData, &resp.Diagnostics)
	if providerData == nil {
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

	result, err := d.readModel(ctx, data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func findExactUserGroupByTitle(userGroups []client.UserGroup, title string) (*client.UserGroup, int) {
	var match *client.UserGroup
	count := 0
	for i := range userGroups {
		if userGroups[i].Title != title {
			continue
		}
		count++
		if match == nil {
			match = &userGroups[i]
		}
	}

	return match, count
}

func addUserGroupDataSourceClientError(diags *diag.Diagnostics, action string, err error) {
	diags.AddError("Client Error", fmt.Sprintf("Unable to %s, got error: %s", action, err))
}

type userGroupReadError struct {
	Summary string
	Detail  string
}

func (d *UserGroupDataSource) readModel(ctx context.Context, data UserGroupDataSourceModel, diags *diag.Diagnostics) (UserGroupDataSourceModel, *userGroupReadError) {
	var userGroup *client.UserGroup

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		ug, err := d.client.GetUserGroup(ctx, data.ID.ValueString())
		if err != nil {
			return UserGroupDataSourceModel{}, &userGroupReadError{
				Summary: "Client Error",
				Detail:  fmt.Sprintf("Unable to read user group by ID, got error: %s", err),
			}
		}
		if ug == nil {
			return UserGroupDataSourceModel{}, &userGroupReadError{
				Summary: "Not Found",
				Detail:  fmt.Sprintf("User group with ID %s not found", data.ID.ValueString()),
			}
		}
		userGroup = ug
	} else if !data.Title.IsNull() && data.Title.ValueString() != "" {
		userGroups, err := d.client.ListUserGroups(ctx, data.Title.ValueString())
		if err != nil {
			return UserGroupDataSourceModel{}, &userGroupReadError{
				Summary: "Client Error",
				Detail:  fmt.Sprintf("Unable to list user groups, got error: %s", err),
			}
		}

		match, count := findExactUserGroupByTitle(userGroups, data.Title.ValueString())
		if count == 0 {
			return UserGroupDataSourceModel{}, &userGroupReadError{
				Summary: "Not Found",
				Detail:  fmt.Sprintf("User group with title %q not found", data.Title.ValueString()),
			}
		}
		if count > 1 {
			return UserGroupDataSourceModel{}, &userGroupReadError{
				Summary: "Multiple Results",
				Detail:  fmt.Sprintf("Found %d user groups matching title %q, expected exactly one", count, data.Title.ValueString()),
			}
		}
		userGroup = match
	} else {
		return UserGroupDataSourceModel{}, &userGroupReadError{
			Summary: "Invalid Configuration",
			Detail:  "Either id or title must be specified to look up a user group.",
		}
	}

	data.ID = types.StringValue(userGroup.ID)
	data.Title = types.StringValue(userGroup.Title)
	data.OrchestrationsGroups = resolveOrchestrationGroups(ctx, userGroup, diags)

	sm, mb := common.UserGroupIsSystemManaged(userGroup)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	tflog.Trace(ctx, "read user group data source", map[string]any{"id": userGroup.ID})

	return data, nil
}
