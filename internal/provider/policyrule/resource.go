package policyrule

import (
	"context"
	"fmt"
	"reflect"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PolicyRuleResource{}
var _ resource.ResourceWithImportState = &PolicyRuleResource{}

func NewPolicyRuleResource() resource.Resource {
	return &PolicyRuleResource{}
}

// PolicyRuleResource defines the resource implementation.
type PolicyRuleResource struct {
	client        *client.Client
	createBatcher *common.Batcher[map[string]any, string]
	updateBatcher *common.Batcher[common.PolicyRuleUpdateReq, struct{}]
	deleteBatcher *common.Batcher[common.PolicyRuleDeleteReq, struct{}]
}

// PolicyRuleResourceModel describes the resource data model.
type PolicyRuleResourceModel struct {
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
	RawSpecJSON       types.String `tfsdk:"raw_spec_json"`
	RawJSON           types.String `tfsdk:"raw_json"`
	WorksiteID        types.String `tfsdk:"worksite_id"`
}

func (r *PolicyRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_rule"
}

func (r *PolicyRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a policy rule in Akamai Guardicore Segmentation. This resource exposes the stable, commonly used policy rule fields as typed Terraform attributes. When ` + "`source`" + ` or ` + "`destination`" + ` is omitted, Terraform treats that endpoint as "any" and the provider sends an empty object because the API requires both endpoint objects.

	Use ` + "`raw_spec_json`" + ` only for unsupported top-level extras that do not yet have a typed Terraform attribute. The full API response is exposed as ` + "`raw_json`" + `.
	The API may represent worksite assignment inside policy rule attributes, but Terraform manages it as the top-level ` + "`worksite_id`" + ` attribute.
	The ` + "`icmp_matches.version`" + ` field is treated as a pass-through string so environments can keep using the representation their API returns.
	Each ` + "`icmp_matches[*].icmp_codes`" + ` value is always imported and sent to the API; when omitted, the provider defaults it to an empty list.

Example:

` + "```hcl" + `
	resource "guardicore_policy_rule" "example" {
	  action           = "ALLOW"
	  section_position = "ALLOW"
	  enabled          = true
	  comments         = "Allow HTTPS to production web"

	  destination = {
	    label_group_ids = [guardicore_label_group.production_web.id]
	  }

	  ports        = [443]
	  ip_protocols = ["TCP"]
}
` + "```" + `

~> **Reference Validation:** Label, label group, user group, asset, policy group, and worksite IDs referenced by typed attributes are validated for existence in Akamai Guardicore Segmentation during plan and apply. Non-existent IDs will produce a clear error before any API call is made.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the policy rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"action": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The policy rule action.",
				Validators: []validator.String{
					stringvalidator.OneOf("ALLOW", "BLOCK", "ALERT", "BLOCK_AND_ALERT", "ALLOW_AND_ENCRYPT"),
				},
			},
			"section_position": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The policy section where the rule is placed.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether the policy rule is enabled.",
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comments attached to the policy rule.",
			},
			"ruleset_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The policy ruleset name.",
			},
			"priority": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Rule priority, when supported by the API.",
			},
			"ip_protocols": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IP protocols for the rule.",
			},
			"ports": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Explicit ports for the rule.",
			},
			"port_ranges": policyRuleRangeSchemaAttribute(),
			"exclude_ports": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Ports excluded from the rule.",
			},
			"exclude_port_ranges": policyRuleRangeSchemaAttribute(),
			"icmp_matches":        policyRuleICMPMatchSchemaAttribute(),
			"network_profile": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Network profile associated with the rule.",
			},
			"scope": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional label IDs that scope the rule.",
			},
			"schedule":    policyRuleScheduleSchemaAttribute(),
			"source":      policyRuleEndpointSchemaAttribute(),
			"destination": policyRuleEndpointSchemaAttribute(),
			"raw_spec_json": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional JSON overlay for unsupported top-level extras. Typed attributes take precedence.",
			},
			"raw_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The full policy rule object returned by the API, normalized as JSON.",
			},
			"worksite_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The worksite ID to assign this policy rule to. Use `guardicore_worksite.example.id` to reference a managed worksite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *PolicyRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData := common.ProviderDataFromRequest(req.ProviderData, &resp.Diagnostics)
	if providerData == nil {
		return
	}

	r.client = providerData.Client
	r.createBatcher = providerData.PolicyRuleCreateBatcher
	r.updateBatcher = providerData.PolicyRuleUpdateBatcher
	r.deleteBatcher = providerData.PolicyRuleDeleteBatcher
}

func (r *PolicyRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := buildPolicyRuleSpecFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(common.ValidatePolicyRuleRefsMap(ctx, r.client, spec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.createBatcher == nil {
		resp.Diagnostics.AddError("Client Error", "Policy rule batcher is not configured")
		return
	}

	id, err := r.createBatcher.Enqueue(ctx, spec)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create policy rule, got error: %s", err))
		return
	}

	data.ID = types.StringValue(id)
	resp.Diagnostics.Append(updatePolicyRuleModelFromAPI(ctx, &data, spec, spec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Assign worksite if specified
	if common.IsSet(data.WorksiteID) {
		if err := r.client.MovePolicyRulesToWorksite(ctx, data.WorksiteID.ValueString(), []string{id}); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to assign policy rule to worksite, got error: %s", err))
			return
		}
	} else {
		data.WorksiteID = types.StringNull()
	}

	if data.WorksiteID.IsUnknown() {
		data.WorksiteID = types.StringNull()
	}

	tflog.Trace(ctx, "created policy rule", map[string]any{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetPolicyRule(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy rule, got error: %s", err))
		return
	}

	if rule == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// The API does not return worksite in the rule GET response, so preserve state.
	// worksite_id is managed via the separate bulk-move endpoint.
	if wsID := extractWorksiteIDFromRule(rule); !wsID.IsNull() {
		data.WorksiteID = wsID
	}

	resp.Diagnostics.Append(updatePolicyRuleModelFromAPI(ctx, &data, rule, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var stateData PolicyRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := buildPolicyRuleSpecFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(common.ValidatePolicyRuleRefsMap(ctx, r.client, spec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateSpec, _ := buildPolicyRuleSpecFromModel(ctx, &stateData)
	if !reflect.DeepEqual(spec, stateSpec) {
		if _, err := r.updateBatcher.Enqueue(ctx, common.PolicyRuleUpdateReq{
			ID:   data.ID.ValueString(),
			Spec: spec,
		}); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update policy rule, got error: %s", err))
			return
		}
	}

	oldWorksite := stateData.WorksiteID
	newWorksite := data.WorksiteID

	if !newWorksite.Equal(oldWorksite) {
		if common.IsSet(newWorksite) {
			// Move to new worksite
			if err := r.client.MovePolicyRulesToWorksite(ctx, newWorksite.ValueString(), []string{data.ID.ValueString()}); err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to assign policy rule to worksite, got error: %s", err))
				return
			}
		} else {
			// Unassign from worksite by moving to "all_worksites"
			if err := r.client.MovePolicyRulesToWorksite(ctx, "all_worksites", []string{data.ID.ValueString()}); err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to unassign policy rule from worksite, got error: %s", err))
				return
			}
		}
	}

	if data.WorksiteID.IsUnknown() {
		data.WorksiteID = types.StringNull()
	}
	resp.Diagnostics.Append(updatePolicyRuleModelFromAPI(ctx, &data, spec, spec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "updated policy rule", map[string]any{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// RulesetName comes from prior state: the rule is gone from the API by the
	// time the batch publishes, so the publish scope cannot be read back.
	deleteReq := common.PolicyRuleDeleteReq{
		ID:          data.ID.ValueString(),
		RulesetName: data.RulesetName.ValueString(),
	}

	if _, err := r.deleteBatcher.Enqueue(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete policy rule, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted policy rule", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *PolicyRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// extractWorksiteIDFromRule extracts the worksite ID from a policy rule API response.
// The worksite is returned as a nested object: {"worksite": {"id": "...", "name": "..."}}.
func extractWorksiteIDFromRule(rule map[string]any) types.String {
	ws, ok := rule["worksite"]
	if !ok {
		return types.StringNull()
	}

	wsMap, ok := ws.(map[string]any)
	if !ok {
		return types.StringNull()
	}

	id, ok := wsMap["id"]
	if !ok {
		return types.StringNull()
	}

	idStr, ok := id.(string)
	if !ok || idStr == "" {
		return types.StringNull()
	}

	return types.StringValue(idStr)
}

func preserveEmptyList(original types.List, elementType attr.Type) types.List {
	if common.IsKnown(original) && len(original.Elements()) == 0 {
		return original
	}
	return types.ListNull(elementType)
}

func updatePolicyRuleModelFromAPI(ctx context.Context, data *PolicyRuleResourceModel, apiRule map[string]any, effectiveSpec map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics
	originalSource := data.Source
	originalDestination := data.Destination
	originalIPProtocols := data.IPProtocols
	originalPorts := data.Ports
	originalExcludePorts := data.ExcludePorts
	originalPortRanges := data.PortRanges
	originalExcludePortRanges := data.ExcludePortRanges
	originalScope := data.Scope

	normalizedRule := normalize.NormalizePolicyRuleSpec(apiRule)
	rawJSON, err := normalize.NormalizeJSON(normalizedRule)
	if err != nil {
		diags.AddError("Conversion Error", fmt.Sprintf("Unable to normalize policy rule JSON: %s", err))
		return diags
	}
	data.RawJSON = types.StringValue(rawJSON)

	if effectiveSpec == nil {
		effectiveSpec = normalizedRule
	} else {
		effectiveSpec = normalize.NormalizePolicyRuleSpec(effectiveSpec)
	}
	_, err = normalize.NormalizeJSON(effectiveSpec)
	if err != nil {
		diags.AddError("Conversion Error", fmt.Sprintf("Unable to normalize effective policy rule JSON: %s", err))
		return diags
	}
	if value, ok := effectiveSpec["action"].(string); ok {
		data.Action = types.StringValue(value)
	} else {
		data.Action = types.StringNull()
	}
	if value, ok := effectiveSpec["section_position"].(string); ok {
		data.SectionPosition = types.StringValue(value)
	} else {
		data.SectionPosition = types.StringNull()
	}
	if value, ok := effectiveSpec["enabled"].(bool); ok {
		data.Enabled = types.BoolValue(value)
	} else {
		data.Enabled = types.BoolNull()
	}
	if value, ok := effectiveSpec["comments"].(string); ok {
		data.Comments = types.StringValue(value)
	} else {
		data.Comments = types.StringNull()
	}
	if value, ok := effectiveSpec["ruleset_name"].(string); ok {
		data.RulesetName = types.StringValue(value)
	} else {
		data.RulesetName = types.StringNull()
	}
	if value, ok := effectiveSpec["priority"].(float64); ok {
		data.Priority = types.Int64Value(int64(value))
	} else if value, ok := effectiveSpec["priority"].(int64); ok {
		data.Priority = types.Int64Value(value)
	} else {
		data.Priority = types.Int64Null()
	}
	if value, d := listStringsFromAny(ctx, effectiveSpec["ip_protocols"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.IPProtocols = value
	} else {
		data.IPProtocols = preserveEmptyList(originalIPProtocols, types.StringType)
	}
	if value, d := listIntsFromAny(ctx, effectiveSpec["ports"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.Ports = value
	} else {
		data.Ports = preserveEmptyList(originalPorts, types.Int64Type)
	}
	if value, d := listIntsFromAny(ctx, effectiveSpec["exclude_ports"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.ExcludePorts = value
	} else {
		data.ExcludePorts = preserveEmptyList(originalExcludePorts, types.Int64Type)
	}

	if value, d := policyRuleRangeListFromAny(effectiveSpec["port_ranges"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.PortRanges = value
	} else {
		data.PortRanges = preserveEmptyList(originalPortRanges, types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()})
	}
	if value, d := policyRuleRangeListFromAny(effectiveSpec["exclude_port_ranges"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.ExcludePortRanges = value
	} else {
		data.ExcludePortRanges = preserveEmptyList(originalExcludePortRanges, types.ObjectType{AttrTypes: policyRuleRangeAttrTypes()})
	}
	if value, d := policyRuleICMPMatchListFromAny(ctx, effectiveSpec["icmp_matches"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.ICMPMatches = value
	} else {
		data.ICMPMatches = types.ListNull(types.ObjectType{AttrTypes: policyRuleICMPMatchAttrTypes()})
	}
	if value, ok := effectiveSpec["network_profile"].(string); ok {
		data.NetworkProfile = types.StringValue(value)
	} else {
		data.NetworkProfile = types.StringNull()
	}
	if value, d := listStringsFromAny(ctx, effectiveSpec["scope"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.Scope = value
	} else if common.IsKnown(originalScope) {
		data.Scope = originalScope
	} else {
		data.Scope = types.ListNull(types.StringType)
	}
	if value, d := scheduleObjectFromAny(ctx, effectiveSpec["schedule"]); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.Schedule = value
	} else {
		data.Schedule = types.ObjectNull(policyRuleScheduleAttrTypes())
	}
	if value, d := endpointObjectFromAny(ctx, effectiveSpec["source"], policyRuleEndpointAttrTypes()); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.Source = value
	} else {
		if common.IsKnown(originalSource) {
			data.Source = originalSource
		} else {
			data.Source = types.ObjectNull(policyRuleEndpointAttrTypes())
		}
	}
	if value, d := endpointObjectFromAny(ctx, effectiveSpec["destination"], policyRuleEndpointAttrTypes()); !value.IsNull() || d.HasError() {
		diags.Append(d...)
		data.Destination = value
	} else {
		if common.IsKnown(originalDestination) {
			data.Destination = originalDestination
		} else {
			data.Destination = types.ObjectNull(policyRuleEndpointAttrTypes())
		}
	}

	if data.RawSpecJSON.IsUnknown() {
		data.RawSpecJSON = types.StringNull()
	}

	return diags
}
