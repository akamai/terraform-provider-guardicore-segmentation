package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyGroupDataSource{}

func NewPolicyGroupDataSource() datasource.DataSource {
	return &PolicyGroupDataSource{}
}

// PolicyGroupDataSource defines the data source implementation.
type PolicyGroupDataSource struct {
	client *client.Client
}

// PolicyGroupDataSourceModel describes the data source data model.
type PolicyGroupDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	Comments       types.String `tfsdk:"comments"`
	MembersJSON    types.String `tfsdk:"members_json"`
	ExcludeMembers types.String `tfsdk:"exclude_members_json"`
}

func (d *PolicyGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_group"
}

func (d *PolicyGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves an Akamai Guardicore Segmentation policy group by ID or by name and type combination.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the policy group. Either `id` or both `name` and `type` must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the policy group. Required when looking up by name.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The type of the policy group (LABEL, FQDN, or IP_ADDRESS). Required when looking up by name.",
			},
			"comments": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Comments or description for the policy group.",
			},
			"members_json": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "JSON string containing the included members. Format depends on type:\n" +
					"  - LABEL: nested array of label IDs\n" +
					"  - FQDN: string array of domains\n" +
					"  - IP_ADDRESS: object array with subnet/range definitions",
			},
			"exclude_members_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "JSON string containing excluded members. Only populated for LABEL type groups.",
			},
		},
	}
}

func (d *PolicyGroupDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("type"),
		),
		datasourcevalidator.RequiredTogether(
			path.MatchRoot("name"),
			path.MatchRoot("type"),
		),
		datasourcevalidator.AtLeastOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *PolicyGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PolicyGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyGroupDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var policyGroup *client.PolicyGroup
	var err error

	if !data.ID.IsNull() {
		// Lookup by ID
		policyGroup, err = d.client.GetPolicyGroup(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy group, got error: %s", err))
			return
		}
	} else {
		// Lookup by name and type
		groups, err := d.client.ListPolicyGroups(ctx, data.Name.ValueString(), data.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list policy groups, got error: %s", err))
			return
		}

		// Find exact match
		for _, g := range groups {
			if g.Name == data.Name.ValueString() && g.Type == data.Type.ValueString() {
				policyGroup = &g
				break
			}
		}
	}

	if policyGroup == nil {
		resp.Diagnostics.AddError(
			"Policy Group Not Found",
			"No policy group found with the specified criteria.",
		)
		return
	}

	// Convert API response to model
	data.ID = types.StringValue(policyGroup.ID)
	data.Name = types.StringValue(policyGroup.Name)
	data.Type = types.StringValue(policyGroup.Type)

	if policyGroup.Comments != "" {
		data.Comments = types.StringValue(policyGroup.Comments)
	} else {
		data.Comments = types.StringNull()
	}

	if len(policyGroup.IncludeMembers) > 0 {
		// Normalize JSON to match Terraform's jsonencode() format
		normalized, err := normalize.NormalizeJSONString(string(policyGroup.IncludeMembers))
		if err != nil {
			resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to normalize include_members JSON: %s", err))
			return
		}
		data.MembersJSON = types.StringValue(normalized)
	} else {
		data.MembersJSON = types.StringNull()
	}

	if len(policyGroup.ExcludeMembers) > 0 {
		// Normalize JSON to match Terraform's jsonencode() format
		normalized, err := normalize.NormalizeJSONString(string(policyGroup.ExcludeMembers))
		if err != nil {
			resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Failed to normalize exclude_members JSON: %s", err))
			return
		}
		data.ExcludeMembers = types.StringValue(normalized)
	} else {
		data.ExcludeMembers = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
