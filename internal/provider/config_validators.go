package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Ensure resources implement ResourceWithConfigValidators.
var _ resource.ResourceWithConfigValidators = &LabelGroupResource{}
var _ resource.ResourceWithConfigValidators = &PolicyRuleResource{}
var _ resource.ResourceWithConfigValidators = &PolicyGroupResource{}
var _ resource.ResourceWithConfigValidators = &AssetResource{}

// ConfigValidators returns validators that run during plan for LabelGroupResource.
func (r *LabelGroupResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	var checker ReferenceChecker
	if r.client != nil {
		checker = r.client
	}
	return []resource.ConfigValidator{
		&labelGroupRefValidator{client: checker},
	}
}

// ConfigValidators returns validators that run during plan for PolicyRuleResource.
func (r *PolicyRuleResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	var checker ReferenceChecker
	if r.client != nil {
		checker = r.client
	}
	return []resource.ConfigValidator{
		&policyRuleRefValidator{client: checker},
	}
}

// ConfigValidators returns validators that run during plan for PolicyGroupResource.
func (r *PolicyGroupResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	var checker ReferenceChecker
	if r.client != nil {
		checker = r.client
	}
	return []resource.ConfigValidator{
		&policyGroupRefValidator{client: checker},
	}
}

// ConfigValidators returns validators that run during plan for AssetResource.
func (r *AssetResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	var checker ReferenceChecker
	if r.client != nil {
		checker = r.client
	}
	return []resource.ConfigValidator{
		&assetRefValidator{client: checker},
	}
}

// labelGroupRefValidator validates that label IDs in include/exclude labels exist.
type labelGroupRefValidator struct {
	client ReferenceChecker
}

func (v *labelGroupRefValidator) Description(_ context.Context) string {
	return "Validates that label IDs referenced by typed and raw label group selectors exist in Akamai Guardicore Segmentation."
}

func (v *labelGroupRefValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *labelGroupRefValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v.client == nil {
		return // Client not available during initial plan; CRUD validation will catch errors
	}

	var data LabelGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateLabelGroupSelectors(ctx, v.client, &data)...)
}

// policyRuleRefValidator validates that policy rule references exist.
type policyRuleRefValidator struct {
	client ReferenceChecker
}

func (v *policyRuleRefValidator) Description(_ context.Context) string {
	return "Validates that typed and JSON policy rule references exist in Akamai Guardicore Segmentation."
}

func (v *policyRuleRefValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *policyRuleRefValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v.client == nil {
		return
	}

	var data PolicyRuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := buildPolicyRuleSpecFromModel(ctx, &data)
	if spec == nil && !diags.HasError() {
		return
	}
	if containsUnknownSentinel(spec) {
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if spec != nil {
		resp.Diagnostics.Append(validatePolicyRuleRefsMap(ctx, v.client, spec)...)
	}

	if !data.WorksiteID.IsNull() && !data.WorksiteID.IsUnknown() {
		resp.Diagnostics.Append(validateWorksiteExists(ctx, v.client, data.WorksiteID.ValueString())...)
	}

}

// policyGroupRefValidator validates that label IDs in LABEL type policy group members exist.
type policyGroupRefValidator struct {
	client ReferenceChecker
}

func (v *policyGroupRefValidator) Description(_ context.Context) string {
	return "Validates that label IDs referenced in members_json and exclude_members_json exist in Akamai Guardicore Segmentation for LABEL type policy groups."
}

func (v *policyGroupRefValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *policyGroupRefValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v.client == nil {
		return
	}

	var data PolicyGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only validate label references for LABEL type groups
	if !data.Type.IsNull() && !data.Type.IsUnknown() && data.Type.ValueString() == "LABEL" {
		if !data.MembersJSON.IsNull() && !data.MembersJSON.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, v.client, data.MembersJSON.ValueString(), "members_json")...)
		}
		if !data.ExcludeMembers.IsNull() && !data.ExcludeMembers.IsUnknown() {
			resp.Diagnostics.Append(validatePolicyGroupLabelRefs(ctx, v.client, data.ExcludeMembers.ValueString(), "exclude_members_json")...)
		}
	}
}

// assetRefValidator validates that label IDs in the asset labels block exist.
type assetRefValidator struct {
	client ReferenceChecker
}

func (v *assetRefValidator) Description(_ context.Context) string {
	return "Validates that label IDs referenced in the labels block exist and are directly assignable in Akamai Guardicore Segmentation."
}

func (v *assetRefValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *assetRefValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v.client == nil {
		return
	}

	var data AssetResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateAssetOrchestrationMetadataConfig(data.OrchestrationMetadata, path.Root("orchestration_metadata"), &resp.Diagnostics)

	if len(data.Labels) > 0 {
		resp.Diagnostics.Append(validateAssetLabelRefs(ctx, v.client, data.Labels)...)
	}

	if !data.WorksiteID.IsNull() && !data.WorksiteID.IsUnknown() {
		resp.Diagnostics.Append(validateWorksiteExists(ctx, v.client, data.WorksiteID.ValueString())...)
	}
}
