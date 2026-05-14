package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
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
var _ resource.Resource = &LabelGroupResource{}
var _ resource.ResourceWithImportState = &LabelGroupResource{}

func NewLabelGroupResource() resource.Resource {
	return &LabelGroupResource{}
}

// LabelGroupResource defines the resource implementation.
type LabelGroupResource struct {
	client                *client.Client
	createBatcher         *Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate]
	updateBatcher         *Batcher[labelGroupUpdateReq, *client.LabelGroupCreate]
	deleteBatcher         *Batcher[string, struct{}]
	validateRefsOnDestroy bool
	strictRefsOnDestroy   bool
}

// LabelGroupResourceModel describes the resource data model.
type LabelGroupResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Comments       types.String `tfsdk:"comments"`
	Include        types.Object `tfsdk:"include"`
	Exclude        types.Object `tfsdk:"exclude"`
	RawIncludeJSON types.String `tfsdk:"raw_include_json"`
	RawExcludeJSON types.String `tfsdk:"raw_exclude_json"`
	IncludeJSON    types.String `tfsdk:"include_json"`
	ExcludeJSON    types.String `tfsdk:"exclude_json"`
}

func (r *LabelGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label_group"
}

func (r *LabelGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a label group in Akamai Guardicore Segmentation. Label groups allow you to define logical groupings of labels using AND/OR logic for policy rules. New label groups are published after creation.\n\n" +
			"Typed `include` and `exclude` attributes are the primary Terraform interface. Optional `raw_include_json` and `raw_exclude_json` attributes act as JSON escape hatches for advanced cases. Typed attributes take precedence over overlapping raw JSON fields. The normalized effective selectors are exposed as `include_json` and `exclude_json`.\n\n" +
			"~> **Reference Validation:** Label IDs referenced by typed selector fields and raw JSON overlays are validated for existence in Akamai Guardicore Segmentation during plan and apply. Non-existent label IDs will produce a clear error before any API call is made.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the label group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The key (category) of the label group.",
			},
			"value": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The value of the label group.",
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comments or description for the label group.",
			},
			"include": labelGroupSelectorSchemaAttribute("Typed selector defining labels to include. Use `or_groups[*].label_ids` to model OR-of-AND label matching."),
			"exclude": labelGroupSelectorSchemaAttribute("Typed selector defining labels to exclude. Use `or_groups[*].label_ids` to model OR-of-AND label matching."),
			"raw_include_json": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional JSON overlay for advanced include selector fields. Typed `include` takes precedence.",
				Validators:          []validator.String{NonEmptyLabelsJSON()},
			},
			"raw_exclude_json": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional JSON overlay for advanced exclude selector fields. Typed `exclude` takes precedence.",
				Validators:          []validator.String{NonEmptyLabelsJSON()},
			},
			"include_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The normalized effective include selector after merging typed attributes and raw JSON.",
			},
			"exclude_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The normalized effective exclude selector after merging typed attributes and raw JSON.",
			},
		},
	}
}

func (r *LabelGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.createBatcher = providerData.LabelGroupCreateBatcher
	r.updateBatcher = providerData.LabelGroupUpdateBatcher
	r.deleteBatcher = providerData.LabelGroupDeleteBatcher
	r.validateRefsOnDestroy = providerData.ValidateRefsOnDestroy
	r.strictRefsOnDestroy = providerData.StrictRefsOnDestroy
}

func (r *LabelGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LabelGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateLabelGroupSelectors(ctx, r.client, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.checkDuplicateLabelGroup(ctx, data.Key.ValueString(), data.Value.ValueString(), "")...)
	if resp.Diagnostics.HasError() {
		return
	}

	labelGroup, err := r.modelToAPI(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert model to API: %s", err))
		return
	}

	createdLabelGroup, err := r.createBatcher.Enqueue(ctx, labelGroup)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create label group, got error: %s", err))
		return
	}

	if err := r.apiCreateToModel(ctx, createdLabelGroup, &data); err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert API to model: %s", err))
		return
	}

	tflog.Trace(ctx, "created label group", map[string]interface{}{"id": createdLabelGroup.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LabelGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labelGroup, err := r.client.GetLabelGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read label group, got error: %s", err))
		return
	}

	if labelGroup == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.apiToModel(ctx, labelGroup, &data); err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert API to model: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LabelGroupResourceModel
	var state LabelGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateLabelGroupSelectors(ctx, r.client, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Key.ValueString() != state.Key.ValueString() || data.Value.ValueString() != state.Value.ValueString() {
		resp.Diagnostics.Append(r.checkDuplicateLabelGroup(ctx, data.Key.ValueString(), data.Value.ValueString(), data.ID.ValueString())...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	labelGroup, err := r.modelToAPI(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert model to API: %s", err))
		return
	}

	updatedLabelGroup, err := r.updateBatcher.Enqueue(ctx, labelGroupUpdateReq{
		id:         data.ID.ValueString(),
		labelGroup: labelGroup,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update label group, got error: %s", err))
		return
	}

	if err := r.apiCreateToModel(ctx, updatedLabelGroup, &data); err != nil {
		resp.Diagnostics.AddError("Conversion Error", fmt.Sprintf("Unable to convert API to model: %s", err))
		return
	}

	tflog.Trace(ctx, "updated label group", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LabelGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for references from policy rules before destroying
	if r.validateRefsOnDestroy {
		r.checkLabelGroupReferencesOnDestroy(ctx, data.ID.ValueString(), r.strictRefsOnDestroy, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if _, err := r.deleteBatcher.Enqueue(ctx, data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete label group, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted label group", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *LabelGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct for create/update.
func (r *LabelGroupResource) modelToAPI(ctx context.Context, data *LabelGroupResourceModel) (*client.LabelGroupCreate, error) {
	labelGroup := &client.LabelGroupCreate{
		Key:   data.Key.ValueString(),
		Value: data.Value.ValueString(),
	}

	if !data.Comments.IsNull() {
		labelGroup.Comments = data.Comments.ValueString()
	}

	includeLabels, diags := resolveLabelGroupSelector(ctx, data.Include, data.RawIncludeJSON, "include", "raw_include_json")
	if diags.HasError() {
		return nil, fmt.Errorf("failed to resolve include selector: %s", diags.Errors()[0].Detail())
	}
	if includeLabels != nil {
		labelGroup.IncludeLabels = includeLabels
	}

	excludeLabels, diags := resolveLabelGroupSelector(ctx, data.Exclude, data.RawExcludeJSON, "exclude", "raw_exclude_json")
	if diags.HasError() {
		return nil, fmt.Errorf("failed to resolve exclude selector: %s", diags.Errors()[0].Detail())
	}
	if excludeLabels != nil {
		labelGroup.ExcludeLabels = excludeLabels
	}

	return labelGroup, nil
}

// apiToModel converts the API struct to Terraform model.
// Note: The API returns full label objects, but we store only IDs in state.
func (r *LabelGroupResource) apiToModel(ctx context.Context, labelGroup *client.LabelGroup, data *LabelGroupResourceModel) error {
	data.ID = types.StringValue(labelGroup.ID)
	data.Key = types.StringValue(labelGroup.Key)
	data.Value = types.StringValue(labelGroup.Value)

	if labelGroup.Comments != "" {
		data.Comments = types.StringValue(labelGroup.Comments)
	} else {
		data.Comments = types.StringNull()
	}

	includeObject, diags := labelGroupSelectorObjectFromRead(ctx, labelGroup.IncludeLabels)
	if diags.HasError() {
		return fmt.Errorf("failed to map include selector: %s", diags.Errors()[0].Detail())
	}
	data.Include = includeObject
	includeJSON, err := normalizeLabelGroupSelectorJSON(convertOrLabelsReadToCreate(labelGroup.IncludeLabels))
	if err != nil {
		return fmt.Errorf("failed to normalize include_json: %w", err)
	}
	data.IncludeJSON = includeJSON

	excludeObject, diags := labelGroupSelectorObjectFromRead(ctx, labelGroup.ExcludeLabels)
	if diags.HasError() {
		return fmt.Errorf("failed to map exclude selector: %s", diags.Errors()[0].Detail())
	}
	data.Exclude = excludeObject
	excludeJSON, err := normalizeLabelGroupSelectorJSON(convertOrLabelsReadToCreate(labelGroup.ExcludeLabels))
	if err != nil {
		return fmt.Errorf("failed to normalize exclude_json: %w", err)
	}
	data.ExcludeJSON = excludeJSON

	return nil
}

func (r *LabelGroupResource) apiCreateToModel(ctx context.Context, labelGroup *client.LabelGroupCreate, data *LabelGroupResourceModel) error {
	data.ID = types.StringValue(labelGroup.ID)
	data.Key = types.StringValue(labelGroup.Key)
	data.Value = types.StringValue(labelGroup.Value)

	if labelGroup.Comments != "" {
		data.Comments = types.StringValue(labelGroup.Comments)
	} else {
		data.Comments = types.StringNull()
	}

	includeObject, diags := labelGroupSelectorObjectFromCreate(ctx, labelGroup.IncludeLabels)
	if diags.HasError() {
		return fmt.Errorf("failed to map include selector: %s", diags.Errors()[0].Detail())
	}
	data.Include = includeObject
	includeJSON, err := normalizeLabelGroupSelectorJSON(labelGroup.IncludeLabels)
	if err != nil {
		return fmt.Errorf("failed to normalize include_json: %w", err)
	}
	data.IncludeJSON = includeJSON

	excludeObject, diags := labelGroupSelectorObjectFromCreate(ctx, labelGroup.ExcludeLabels)
	if diags.HasError() {
		return fmt.Errorf("failed to map exclude selector: %s", diags.Errors()[0].Detail())
	}
	data.Exclude = excludeObject
	excludeJSON, err := normalizeLabelGroupSelectorJSON(labelGroup.ExcludeLabels)
	if err != nil {
		return fmt.Errorf("failed to normalize exclude_json: %w", err)
	}
	data.ExcludeJSON = excludeJSON

	return nil
}

// checkDuplicateLabelGroup checks if a label group with the same key+value already exists.
func (r *LabelGroupResource) checkDuplicateLabelGroup(ctx context.Context, key, value, excludeID string) diag.Diagnostics {
	var diags diag.Diagnostics

	existing, err := r.client.ListLabelGroups(ctx, key, value)
	if err != nil {
		tflog.Warn(ctx, "unable to check for duplicate label groups", map[string]any{"error": err.Error()})
		return diags
	}

	for _, group := range existing {
		if group.ID == excludeID {
			continue
		}
		if group.Key == key && group.Value == value {
			diags.AddError(
				"Label Group Already Exists",
				fmt.Sprintf(
					"A label group with key=%q value=%q already exists (ID: %s). "+
						"To manage it with Terraform, import it:\n\n"+
						"  terraform import guardicore_label_group.<NAME> %s\n\n"+
						"Or use a different key/value combination.",
					key, value, group.ID, group.ID,
				),
			)
			return diags
		}
	}

	return diags
}

// checkLabelGroupReferencesOnDestroy checks if any policy rules reference this
// label group and emits warning or error diagnostics depending on strict mode.
func (r *LabelGroupResource) checkLabelGroupReferencesOnDestroy(ctx context.Context, labelGroupID string, strict bool, diags *diag.Diagnostics) {
	addDiag := diags.AddWarning
	if strict {
		addDiag = diags.AddError
	}

	rules, err := r.client.ListPolicyRules(ctx)
	if err != nil {
		tflog.Warn(ctx, "unable to check label group references on destroy", map[string]any{"error": err.Error()})
		return
	}

	for _, rule := range rules {
		ruleID, _ := rule["id"].(string)
		if policyRuleReferencesLabelGroup(rule, labelGroupID) {
			addDiag(
				"Label Group Referenced by Policy Rule",
				fmt.Sprintf("Label group %q is referenced by policy rule %q. "+
					"Destroying this label group may leave the policy rule in an inconsistent state.",
					labelGroupID, ruleID),
			)
		}
	}
}

// policyRuleReferencesLabelGroup checks if a policy rule spec references
// a specific label group ID in its source or destination endpoints.
func policyRuleReferencesLabelGroup(rule map[string]any, labelGroupID string) bool {
	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := rule[endpointKey]
		if !ok {
			continue
		}
		endpointMap, ok := endpoint.(map[string]any)
		if !ok {
			continue
		}
		for _, refKey := range []string{"label_group_ids"} {
			refs, ok := endpointMap[refKey]
			if !ok {
				continue
			}
			refSlice, ok := refs.([]any)
			if !ok {
				continue
			}
			for _, ref := range refSlice {
				if refID, ok := ref.(string); ok && refID == labelGroupID {
					return true
				}
			}
		}
	}
	return false
}
