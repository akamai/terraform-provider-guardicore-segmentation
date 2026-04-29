package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyRuleDataSource{}

func NewPolicyRuleDataSource() datasource.DataSource {
	return &PolicyRuleDataSource{}
}

// PolicyRuleDataSource defines the data source implementation.
type PolicyRuleDataSource struct {
	client *client.Client
}

// PolicyRuleDataSourceModel describes the data source data model.
type PolicyRuleDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Action            types.String `tfsdk:"action"`
	SectionPosition   types.String `tfsdk:"section_position"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	Comments          types.String `tfsdk:"comments"`
	RulesetName       types.String `tfsdk:"ruleset_name"`
	Priority          types.Int64  `tfsdk:"priority"`
	IPProtocols       types.List   `tfsdk:"ip_protocols"`
	Ports             types.List   `tfsdk:"ports"`
	PortRanges        types.List   `tfsdk:"port_ranges"`
	ExcludePorts      types.List   `tfsdk:"exclude_ports"`
	ExcludePortRanges types.List   `tfsdk:"exclude_port_ranges"`
	ICMPMatches       types.List   `tfsdk:"icmp_matches"`
	NetworkProfile    types.String `tfsdk:"network_profile"`
	Scope             types.List   `tfsdk:"scope"`
	Schedule          types.Object `tfsdk:"schedule"`
	Source            types.Object `tfsdk:"source"`
	Destination       types.Object `tfsdk:"destination"`
	SpecJSON          types.String `tfsdk:"spec_json"`
	RawJSON           types.String `tfsdk:"raw_json"`
	WorksiteID        types.String `tfsdk:"worksite_id"`
}

func (d *PolicyRuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_rule"
}

func (d *PolicyRuleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a policy rule from Akamai Guardicore Segmentation by its ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the policy rule.",
			},
			"action":              schema.StringAttribute{Computed: true, MarkdownDescription: "The policy rule action."},
			"section_position":    schema.StringAttribute{Computed: true, MarkdownDescription: "The policy section where the rule is placed."},
			"enabled":             schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the policy rule is enabled."},
			"comments":            schema.StringAttribute{Computed: true, MarkdownDescription: "Comments attached to the policy rule."},
			"ruleset_name":        schema.StringAttribute{Computed: true, MarkdownDescription: "The policy ruleset name."},
			"priority":            schema.Int64Attribute{Computed: true, MarkdownDescription: "Rule priority, when returned by the API."},
			"ip_protocols":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IP protocols for the rule."},
			"ports":               schema.ListAttribute{Computed: true, ElementType: types.Int64Type, MarkdownDescription: "Explicit ports for the rule."},
			"port_ranges":         policyRuleRangeDataSourceAttribute(),
			"exclude_ports":       schema.ListAttribute{Computed: true, ElementType: types.Int64Type, MarkdownDescription: "Ports excluded from the rule."},
			"exclude_port_ranges": policyRuleRangeDataSourceAttribute(),
			"icmp_matches":        policyRuleICMPMatchDataSourceAttribute(),
			"network_profile":     schema.StringAttribute{Computed: true, MarkdownDescription: "Network profile associated with the rule."},
			"scope":               schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Optional label IDs that scope the rule."},
			"schedule":            policyRuleScheduleDataSourceAttribute(),
			"source":              policyRuleEndpointDataSourceAttribute(),
			"destination":         policyRuleEndpointDataSourceAttribute(),
			"spec_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The effective policy rule specification as normalized JSON.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The full policy rule object returned by the API, normalized as JSON.",
			},
			"worksite_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The worksite ID assigned to this policy rule.",
			},
		},
	}
}

func (d *PolicyRuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PolicyRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyRuleDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := d.client.GetPolicyRule(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy rule, got error: %s", err))
		return
	}

	if rule == nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Policy rule with ID %s not found", data.ID.ValueString()))
		return
	}

	// Extract worksite before normalization strips it
	data.WorksiteID = extractWorksiteIDFromRule(rule)

	resourceModel := PolicyRuleResourceModel{
		ID:                data.ID,
		Action:            data.Action,
		SectionPosition:   data.SectionPosition,
		Enabled:           data.Enabled,
		Comments:          data.Comments,
		RulesetName:       data.RulesetName,
		Priority:          data.Priority,
		IPProtocols:       data.IPProtocols,
		Ports:             data.Ports,
		PortRanges:        data.PortRanges,
		ExcludePorts:      data.ExcludePorts,
		ExcludePortRanges: data.ExcludePortRanges,
		ICMPMatches:       data.ICMPMatches,
		NetworkProfile:    data.NetworkProfile,
		Scope:             data.Scope,
		Schedule:          data.Schedule,
		Source:            data.Source,
		Destination:       data.Destination,
		RawJSON:           data.RawJSON,
		WorksiteID:        data.WorksiteID,
	}
	resp.Diagnostics.Append(updatePolicyRuleModelFromAPI(ctx, &resourceModel, rule, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if value, d := endpointObjectFromAny(ctx, rule["source"], policyRuleEndpointDataSourceAttrTypes()); !value.IsNull() || d.HasError() {
		resp.Diagnostics.Append(d...)
		data.Source = value
	} else {
		data.Source = types.ObjectNull(policyRuleEndpointDataSourceAttrTypes())
	}
	if value, d := endpointObjectFromAny(ctx, rule["destination"], policyRuleEndpointDataSourceAttrTypes()); !value.IsNull() || d.HasError() {
		resp.Diagnostics.Append(d...)
		data.Destination = value
	} else {
		data.Destination = types.ObjectNull(policyRuleEndpointDataSourceAttrTypes())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	data.Action = resourceModel.Action
	data.SectionPosition = resourceModel.SectionPosition
	data.Enabled = resourceModel.Enabled
	data.Comments = resourceModel.Comments
	data.RulesetName = resourceModel.RulesetName
	data.Priority = resourceModel.Priority
	data.IPProtocols = resourceModel.IPProtocols
	data.Ports = resourceModel.Ports
	data.PortRanges = resourceModel.PortRanges
	data.ExcludePorts = resourceModel.ExcludePorts
	data.ExcludePortRanges = resourceModel.ExcludePortRanges
	data.ICMPMatches = resourceModel.ICMPMatches
	data.NetworkProfile = resourceModel.NetworkProfile
	data.Scope = resourceModel.Scope
	data.Schedule = resourceModel.Schedule
	specJSON, err := normalize.NormalizeJSON(normalize.NormalizePolicyRuleSpec(rule))
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to normalize policy rule spec JSON: %s", err))
		return
	}
	data.SpecJSON = types.StringValue(specJSON)
	data.RawJSON = resourceModel.RawJSON

	tflog.Trace(ctx, "read policy rule data source", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
