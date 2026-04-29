package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/normalize"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &PolicyGroupResource{}
var _ resource.ResourceWithImportState = &PolicyGroupResource{}

func NewPolicyGroupResource() resource.Resource {
	return &PolicyGroupResource{}
}

// PolicyGroupResource defines the resource implementation.
type PolicyGroupResource struct {
	client                *client.Client
	validateRefsOnDestroy bool
	strictRefsOnDestroy   bool
}

// PolicyGroupResourceModel describes the resource data model.
type PolicyGroupResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	Comments       types.String `tfsdk:"comments"`
	MembersJSON    types.String `tfsdk:"members_json"`
	ExcludeMembers types.String `tfsdk:"exclude_members_json"`
}

func (r *PolicyGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_group"
}

func (r *PolicyGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Akamai Guardicore Segmentation policy group. Policy groups allow you to define collections of labels, FQDNs, or IP addresses for use in policy rules. New policy groups are published after creation.\n\n" +
			"~> **Reference Validation:** For LABEL type groups, label IDs in `members_json` and `exclude_members_json` are validated for existence in Akamai Guardicore Segmentation during plan and apply. " +
			"Non-existent label IDs will produce a clear error before any API call is made.\n\n" +
			"~> **Member Limit:** Policy groups support a maximum of 100 members. Exceeding this limit will result in an error.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the policy group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the policy group (1-100 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of members in the group. Must be one of: LABEL, FQDN, IP_ADDRESS. Changing this requires resource replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf("LABEL", "FQDN", "IP_ADDRESS"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comments or description for the policy group (max 200 characters).",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(200),
				},
			},
			"members_json": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "JSON string defining members to include. Format depends on type:\n" +
					"  - LABEL: nested array `[[\"label-id-1\", \"label-id-2\"], [\"label-id-3\"]]` (OR of ANDs)\n" +
					"  - FQDN: string array `[\"example.com\", \"*.example.com\"]`\n" +
					"  - IP_ADDRESS: object array `[{\"subnet\": \"10.0.0.0/8\"}, {\"range\": {\"start\": \"192.168.1.1\", \"end\": \"192.168.1.254\"}}]`\n\n" +
					"Use `jsonencode()` for type safety.",
				Validators: []validator.String{
					PolicyGroupMembersValidator("type"),
				},
			},
			"exclude_members_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON string defining members to exclude. Only valid for LABEL type groups. " +
					"Format: nested array `[[\"label-id-1\"], [\"label-id-2\"]]`",
				Validators: []validator.String{
					PolicyGroupExcludeMembersValidator("type"),
					PolicyGroupMembersValidator("type"),
				},
			},
		},
	}
}

func (r *PolicyGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.validateRefsOnDestroy = providerData.ValidateRefsOnDestroy
	r.strictRefsOnDestroy = providerData.StrictRefsOnDestroy
}

func (r *PolicyGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate label references for LABEL type groups
	if data.Type.ValueString() == "LABEL" {
		if !data.MembersJSON.IsNull() && !data.MembersJSON.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, r.client, data.MembersJSON.ValueString(), "members_json")...)
		}
		if !data.ExcludeMembers.IsNull() && !data.ExcludeMembers.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, r.client, data.ExcludeMembers.ValueString(), "exclude_members_json")...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Validate member count limit (100 members max)
	includeCount, err := countPolicyGroupMembers(data.MembersJSON.ValueString(), data.Type.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Validation Error", fmt.Sprintf("Unable to count members: %s", err))
		return
	}
	excludeCount := 0
	if !data.ExcludeMembers.IsNull() {
		excludeCount, err = countPolicyGroupMembers(data.ExcludeMembers.ValueString(), data.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Validation Error", fmt.Sprintf("Unable to count exclude_members: %s", err))
			return
		}
	}
	totalCount := includeCount + excludeCount
	if totalCount > 100 {
		resp.Diagnostics.AddError(
			"Member Count Limit Exceeded",
			fmt.Sprintf("Policy groups support a maximum of 100 members. This configuration has %d members (%d include + %d exclude).",
				totalCount, includeCount, excludeCount),
		)
		return
	}

	policyGroup, err := r.modelToAPI(&data)
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert model to API: %s", err))
		return
	}

	createdID, err := r.client.CreatePolicyGroup(ctx, policyGroup)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create policy group, got error: %s", err))
		return
	}

	if err := r.client.PublishPolicyGroups(ctx); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to publish policy groups, got error: %s", err))
		return
	}

	data.ID = types.StringValue(createdID)

	tflog.Trace(ctx, "created policy group", map[string]interface{}{"id": createdID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyGroup, err := r.client.GetPolicyGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read policy group, got error: %s", err))
		return
	}

	if policyGroup == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.apiToModel(policyGroup, &data); err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert API to model: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate label references for LABEL type groups
	if data.Type.ValueString() == "LABEL" {
		if !data.MembersJSON.IsNull() && !data.MembersJSON.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, r.client, data.MembersJSON.ValueString(), "members_json")...)
		}
		if !data.ExcludeMembers.IsNull() && !data.ExcludeMembers.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, r.client, data.ExcludeMembers.ValueString(), "exclude_members_json")...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Validate member count limit (100 members max)
	includeCount, err := countPolicyGroupMembers(data.MembersJSON.ValueString(), data.Type.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Validation Error", fmt.Sprintf("Unable to count members: %s", err))
		return
	}
	excludeCount := 0
	if !data.ExcludeMembers.IsNull() {
		excludeCount, err = countPolicyGroupMembers(data.ExcludeMembers.ValueString(), data.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Validation Error", fmt.Sprintf("Unable to count exclude_members: %s", err))
			return
		}
	}
	totalCount := includeCount + excludeCount
	if totalCount > 100 {
		resp.Diagnostics.AddError(
			"Member Count Limit Exceeded",
			fmt.Sprintf("Policy groups support a maximum of 100 members. This configuration has %d members (%d include + %d exclude).",
				totalCount, includeCount, excludeCount),
		)
		return
	}

	policyGroup, err := r.modelToAPI(&data)
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert model to API: %s", err))
		return
	}

	err = r.client.UpdatePolicyGroup(ctx, data.ID.ValueString(), policyGroup)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update policy group, got error: %s", err))
		return
	}

	if err := r.client.PublishPolicyGroups(ctx); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to publish policy groups, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "updated policy group", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for references from policy rules before destroying
	if r.validateRefsOnDestroy {
		r.checkPolicyGroupReferencesOnDestroy(ctx, data.ID.ValueString(), r.strictRefsOnDestroy, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	err := r.client.DeletePolicyGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete policy group, got error: %s", err))
		return
	}

	if err := r.client.PublishPolicyGroups(ctx); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to publish policy groups, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted policy group", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *PolicyGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct for create/update.
func (r *PolicyGroupResource) modelToAPI(data *PolicyGroupResourceModel) (*client.PolicyGroupCreate, error) {
	policyGroup := &client.PolicyGroupCreate{
		Name: data.Name.ValueString(),
		Type: data.Type.ValueString(),
	}

	if !data.Comments.IsNull() {
		policyGroup.Comments = data.Comments.ValueString()
	}

	if !data.MembersJSON.IsNull() && data.MembersJSON.ValueString() != "" {
		// Normalize JSON before storing
		normalized, err := normalize.NormalizeJSONString(data.MembersJSON.ValueString())
		if err != nil {
			return nil, fmt.Errorf("failed to normalize members_json: %w", err)
		}
		policyGroup.IncludeMembers = json.RawMessage(normalized)
	}

	if !data.ExcludeMembers.IsNull() && data.ExcludeMembers.ValueString() != "" {
		// Normalize JSON before storing
		normalized, err := normalize.NormalizeJSONString(data.ExcludeMembers.ValueString())
		if err != nil {
			return nil, fmt.Errorf("failed to normalize exclude_members_json: %w", err)
		}
		policyGroup.ExcludeMembers = json.RawMessage(normalized)
	}

	return policyGroup, nil
}

// apiToModel converts the API struct to Terraform model.
func (r *PolicyGroupResource) apiToModel(policyGroup *client.PolicyGroup, data *PolicyGroupResourceModel) error {
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
			return fmt.Errorf("failed to normalize include_members JSON: %w", err)
		}
		data.MembersJSON = types.StringValue(normalized)
	} else {
		data.MembersJSON = types.StringNull()
	}

	if len(policyGroup.ExcludeMembers) > 0 {
		// Normalize JSON to match Terraform's jsonencode() format
		normalized, err := normalize.NormalizeJSONString(string(policyGroup.ExcludeMembers))
		if err != nil {
			return fmt.Errorf("failed to normalize exclude_members JSON: %w", err)
		}
		data.ExcludeMembers = types.StringValue(normalized)
	} else {
		data.ExcludeMembers = types.StringNull()
	}

	return nil
}

// checkPolicyGroupReferencesOnDestroy checks if any policy rules reference this
// policy group and emits warning or error diagnostics depending on strict mode.
func (r *PolicyGroupResource) checkPolicyGroupReferencesOnDestroy(ctx context.Context, policyGroupID string, strict bool, diags *diag.Diagnostics) {
	addDiag := diags.AddWarning
	if strict {
		addDiag = diags.AddError
	}

	rules, err := r.client.ListPolicyRules(ctx)
	if err != nil {
		tflog.Warn(ctx, "unable to check policy group references on destroy", map[string]any{"error": err.Error()})
		return
	}

	for _, rule := range rules {
		ruleID, _ := rule["id"].(string)
		if policyRuleReferencesPolicyGroup(rule, policyGroupID) {
			addDiag(
				"Policy Group Referenced by Policy Rule",
				fmt.Sprintf("Policy group %q is referenced by policy rule %q. "+
					"Destroying this policy group may leave the policy rule in an inconsistent state.",
					policyGroupID, ruleID),
			)
		}
	}
}

// policyRuleReferencesPolicyGroup checks if a policy rule spec references
// a specific policy group ID in its source or destination endpoints.
func policyRuleReferencesPolicyGroup(rule map[string]any, policyGroupID string) bool {
	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := rule[endpointKey]
		if !ok {
			continue
		}
		endpointMap, ok := endpoint.(map[string]any)
		if !ok {
			continue
		}
		// Check policy_groups key
		refs, ok := endpointMap["policy_groups"]
		if !ok {
			continue
		}
		refSlice, ok := refs.([]any)
		if !ok {
			continue
		}
		for _, ref := range refSlice {
			if refID, ok := ref.(string); ok && refID == policyGroupID {
				return true
			}
		}
	}
	return false
}
