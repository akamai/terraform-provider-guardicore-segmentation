package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LabelResource{}
var _ resource.ResourceWithImportState = &LabelResource{}

func NewLabelResource() resource.Resource {
	return &LabelResource{}
}

// LabelResource defines the resource implementation.
type LabelResource struct {
	client                *client.Client
	createBatcher         *Batcher[*client.LabelCreate, string]
	updateBatcher         *Batcher[labelUpdateReq, struct{}]
	deleteBatcher         *Batcher[string, struct{}]
	validateRefsOnDestroy bool
	strictRefsOnDestroy   bool
}

// LabelResourceModel describes the resource data model.
type LabelResourceModel struct {
	ID            types.String         `tfsdk:"id"`
	Key           types.String         `tfsdk:"key"`
	Value         types.String         `tfsdk:"value"`
	Criteria      []LabelCriteriaModel `tfsdk:"criteria"`
	SystemManaged types.Bool           `tfsdk:"system_managed"`
	ManagedBy     types.String         `tfsdk:"managed_by"`
}

// LabelCriteriaModel describes the criteria data model.
type LabelCriteriaModel struct {
	Field            types.String                 `tfsdk:"field"`
	Op               types.String                 `tfsdk:"op"`
	Argument         types.String                 `tfsdk:"argument"`
	CompoundCriteria []LabelCompoundCriteriaModel `tfsdk:"compound_criteria"`
}

type LabelCompoundCriteriaModel struct {
	Field    types.String `tfsdk:"field"`
	Op       types.String `tfsdk:"op"`
	Argument types.String `tfsdk:"argument"`
}

func (r *LabelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *LabelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a label in Akamai Guardicore Segmentation.\n\n" +
			"Some labels are system-managed (e.g., automatically created by the platform for worksites or orchestration). " +
			"System-managed labels can be read and referenced by other resources, but cannot be updated or deleted by Terraform.\n\n" +
			"Use this resource for labels that Terraform should create and manage. " +
			"Use the `guardicore_label` data source to reference existing system-managed labels.\n\n" +
			"When `system_managed` is `true`, any attempt to update or delete the label will return an error.\n\n" +
			"~> **Update behavior:** Key/value updates use `PUT /api/v4.0/labels/{id}` so the label ID remains stable and downstream references by ID stay valid. Dynamic criteria updates use `POST /api/v3.0/visibility/labels/{id}/dynamic-criteria/changes` with add/modify/delete semantics for top-level OR criteria (including compound criteria groups).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the label.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The key (category) of the label (e.g., 'Environment', 'Application').",
			},
			"value": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The value of the label (e.g., 'Production', 'Web Server').",
			},
			"criteria": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Dynamic criteria for automatic label assignment. Criteria are treated as an unordered unique set. Ordering is intentionally canonicalized in state so equivalent API responses with different list ordering do not create drift. On updates, criteria diffs are applied through the dynamic criteria changes endpoint (added/modified/deleted top-level criteria).",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The field to match against (e.g., 'name', 'ip', 'os_name', 'process_name').",
						},
						"op": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The operator for matching (e.g., 'EQUALS', 'CONTAINS', 'STARTSWITH', 'ENDSWITH', 'REGEX').",
						},
						"argument": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The argument to match against.",
						},
						"compound_criteria": schema.SetNestedAttribute{
							Optional:            true,
							MarkdownDescription: "Nested criteria matched as an unordered unique compound group.",
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"field": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The field to match against.",
									},
									"op": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The operator for matching.",
									},
									"argument": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The argument to match against.",
									},
								},
							},
						},
					},
				},
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this label is system-managed. System-managed labels cannot be updated or deleted by Terraform. Use the `guardicore_label` data source to reference them.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"managed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifies who manages this label. `terraform` for user-managed labels, or the system origin (e.g., `system`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *LabelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.createBatcher = providerData.LabelCreateBatcher
	r.updateBatcher = providerData.LabelUpdateBatcher
	r.deleteBatcher = providerData.LabelDeleteBatcher
	r.validateRefsOnDestroy = providerData.ValidateRefsOnDestroy
	r.strictRefsOnDestroy = providerData.StrictRefsOnDestroy
}

func (r *LabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateLabelCriteria(data.Criteria, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	label := r.modelToAPI(&data)

	id, err := r.createBatcher.Enqueue(ctx, label)
	if err != nil {
		var apiErr *client.APIError
		var itemErr *client.BulkItemError
		if errors.As(err, &apiErr) && apiErr.IsAlreadyExists() {
			resp.Diagnostics.AddError(
				"Label Already Exists",
				fmt.Sprintf(
					"A label with key=%q value=%q already exists in Akamai Guardicore Segmentation. "+
						"To manage it with Terraform, import it:\n\n"+
						"  terraform import guardicore_label.<NAME> <LABEL_ID>\n\n"+
						"Or use a different key/value combination.",
					data.Key.ValueString(), data.Value.ValueString(),
				),
			)
		} else if errors.As(err, &itemErr) {
			lines := make([]string, 0, len(itemErr.Messages))
			for _, msg := range itemErr.Messages {
				lines = append(lines, "  * "+msg)
			}
			resp.Diagnostics.AddError(
				"Label Validation Error",
				fmt.Sprintf(
					"Unable to create label key=%q value=%q due to validation errors:\n%s",
					data.Key.ValueString(), data.Value.ValueString(),
					strings.Join(lines, "\n"),
				),
			)
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create label, got error: %s", err))
		}
		return
	}

	data.ID = types.StringValue(id)
	data.SystemManaged = types.BoolValue(false)
	data.ManagedBy = types.StringValue("terraform")

	tflog.Trace(ctx, "created label", map[string]interface{}{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label, err := r.client.GetLabel(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read label, got error: %s", err))
		return
	}

	if label == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if label.ReadOnly == nil {
		listLabels, listErr := r.client.ListLabels(ctx, label.Key, label.Value)
		if listErr == nil {
			for _, candidate := range listLabels {
				if candidate.ID == label.ID {
					label.ReadOnly = candidate.ReadOnly
					break
				}
			}
		}
	}

	r.apiToModel(label, &data)

	sm, mb := LabelIsSystemManaged(label)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LabelResourceModel
	var stateData LabelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateLabelCriteria(data.Criteria, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	DiagnoseSystemManagedMutation(stateData.SystemManaged, "label", data.ID.ValueString(), "update", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if strings.EqualFold(data.Key.ValueString(), "Worksite") {
		if worksiteLabelEditableChange(stateData, data) {
			resp.Diagnostics.AddError(
				"Worksite Label Is Read-Only",
				"Imported labels with key \"Worksite\" are system-managed and cannot be edited by Terraform. Remove this change, or remove the resource from Terraform configuration if you do not want it tracked.",
			)
			return
		}

		tflog.Trace(ctx, "skipped update for immutable Worksite label", map[string]interface{}{"id": data.ID.ValueString()})
		data.SystemManaged = stateData.SystemManaged
		data.ManagedBy = stateData.ManagedBy
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	label := r.modelToAPI(&data)

	updateLabel := &client.LabelUpdate{
		Key:      label.Key,
		Value:    label.Value,
		Criteria: label.Criteria,
	}

	_, err := r.updateBatcher.Enqueue(ctx, labelUpdateReq{id: data.ID.ValueString(), label: updateLabel})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsAlreadyExists() {
			resp.Diagnostics.AddError(
				"Label Already Exists",
				fmt.Sprintf(
					"Cannot rename label to key=%q value=%q because a label with that "+
						"key/value combination already exists in Akamai Guardicore Segmentation.",
					data.Key.ValueString(), data.Value.ValueString(),
				),
			)
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update label, got error: %s", err))
		}
		return
	}

	tflog.Trace(ctx, "updated label", map[string]interface{}{"id": data.ID.ValueString()})
	data.SystemManaged = stateData.SystemManaged
	data.ManagedBy = stateData.ManagedBy

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func worksiteLabelEditableChange(state, plan LabelResourceModel) bool {
	if state.Key.ValueString() != plan.Key.ValueString() {
		return true
	}

	if state.Value.ValueString() != plan.Value.ValueString() {
		return true
	}

	return !labelCriteriaEqual(state.Criteria, plan.Criteria)
}

func labelCriteriaEqual(a, b []LabelCriteriaModel) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Field.ValueString() != b[i].Field.ValueString() {
			return false
		}
		if a[i].Op.ValueString() != b[i].Op.ValueString() {
			return false
		}
		if a[i].Argument.ValueString() != b[i].Argument.ValueString() {
			return false
		}
		if len(a[i].CompoundCriteria) != len(b[i].CompoundCriteria) {
			return false
		}
		for j := range a[i].CompoundCriteria {
			if a[i].CompoundCriteria[j].Field.ValueString() != b[i].CompoundCriteria[j].Field.ValueString() {
				return false
			}
			if a[i].CompoundCriteria[j].Op.ValueString() != b[i].CompoundCriteria[j].Op.ValueString() {
				return false
			}
			if a[i].CompoundCriteria[j].Argument.ValueString() != b[i].CompoundCriteria[j].Argument.ValueString() {
				return false
			}
		}
	}

	return true
}

func (r *LabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	DiagnoseSystemManagedMutation(data.SystemManaged, "label", data.ID.ValueString(), "delete", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for references from label groups and assets before destroying
	if r.validateRefsOnDestroy {
		r.checkLabelReferencesOnDestroy(ctx, data.ID.ValueString(), r.strictRefsOnDestroy, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	_, err := r.deleteBatcher.Enqueue(ctx, data.ID.ValueString())
	if err != nil {
		if r.validateRefsOnDestroy && !r.strictRefsOnDestroy && strings.Contains(err.Error(), "is used in a label group") {
			resp.Diagnostics.AddWarning(
				"Label Deletion Blocked by Server",
				fmt.Sprintf("The API refused to delete the label because it is still referenced: %s. "+
					"The label has been removed from Terraform state but still exists in Akamai Guardicore Segmentation.", err),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete label, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted label", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *LabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct for create/update.
func (r *LabelResource) modelToAPI(data *LabelResourceModel) *client.LabelCreate {
	label := &client.LabelCreate{
		Key:   data.Key.ValueString(),
		Value: data.Value.ValueString(),
	}

	if len(data.Criteria) == 0 {
		label.Criteria = []client.LabelCriteria{}
		return label
	}

	label.Criteria = make([]client.LabelCriteria, len(data.Criteria))
	for i, c := range data.Criteria {
		if len(c.CompoundCriteria) > 0 {
			compound := make([]client.LabelCriteria, len(c.CompoundCriteria))
			for j, cc := range c.CompoundCriteria {
				compound[j] = client.LabelCriteria{
					Field:    cc.Field.ValueString(),
					Op:       cc.Op.ValueString(),
					Argument: cc.Argument.ValueString(),
				}
			}
			label.Criteria[i] = client.LabelCriteria{CompoundCriteria: compound}
			continue
		}

		label.Criteria[i] = client.LabelCriteria{
			Field:    c.Field.ValueString(),
			Op:       c.Op.ValueString(),
			Argument: c.Argument.ValueString(),
		}
	}

	return label
}

// apiToModel converts the API struct to Terraform model.
// Note: The API returns criteria in "dynamic_criteria" field, not "criteria".
func (r *LabelResource) apiToModel(label *client.Label, data *LabelResourceModel) {
	data.ID = types.StringValue(label.ID)
	data.Key = types.StringValue(label.Key)
	data.Value = types.StringValue(label.Value)

	importableCriteria := make([]client.LabelCriteria, 0, len(label.DynamicCriteria))
	for _, c := range label.DynamicCriteria {
		if c.IsReadOnlyWorksiteGenerated() {
			continue
		}
		importableCriteria = append(importableCriteria, c)
	}

	if len(importableCriteria) > 0 {
		data.Criteria = make([]LabelCriteriaModel, len(importableCriteria))
		for i, c := range importableCriteria {
			if len(c.CompoundCriteria) > 0 {
				compound := make([]LabelCompoundCriteriaModel, len(c.CompoundCriteria))
				for j, cc := range c.CompoundCriteria {
					compound[j] = LabelCompoundCriteriaModel{
						Field:    types.StringValue(cc.Field),
						Op:       types.StringValue(cc.Op),
						Argument: types.StringValue(cc.Argument),
					}
				}
				data.Criteria[i] = LabelCriteriaModel{
					Field:            types.StringNull(),
					Op:               types.StringNull(),
					Argument:         types.StringNull(),
					CompoundCriteria: compound,
				}
				continue
			}

			data.Criteria[i] = LabelCriteriaModel{
				Field:            types.StringValue(c.Field),
				Op:               types.StringValue(c.Op),
				Argument:         types.StringValue(c.Argument),
				CompoundCriteria: nil,
			}
		}
		canonicalizeLabelCriteria(data.Criteria)
	} else {
		data.Criteria = nil
	}
}

func canonicalizeLabelCriteria(criteria []LabelCriteriaModel) {
	for i := range criteria {
		if len(criteria[i].CompoundCriteria) <= 1 {
			continue
		}

		sort.SliceStable(criteria[i].CompoundCriteria, func(left, right int) bool {
			leftKey := labelCompoundCriteriaSortKey(criteria[i].CompoundCriteria[left])
			rightKey := labelCompoundCriteriaSortKey(criteria[i].CompoundCriteria[right])
			return leftKey < rightKey
		})
	}

	sort.SliceStable(criteria, func(i, j int) bool {
		leftKey := labelCriteriaSortKey(criteria[i])
		rightKey := labelCriteriaSortKey(criteria[j])
		return leftKey < rightKey
	})
}

func labelCriteriaSortKey(criteria LabelCriteriaModel) string {
	isCompound := len(criteria.CompoundCriteria) > 0
	kind := "0"
	if isCompound {
		kind = "1"
	}

	compoundKey := ""
	for _, c := range criteria.CompoundCriteria {
		compoundKey += labelCompoundCriteriaSortKey(c) + "\x00"
	}

	return kind + "\x00" + criteria.Field.ValueString() + "\x00" + criteria.Op.ValueString() + "\x00" + criteria.Argument.ValueString() + "\x00" + compoundKey
}

func labelCompoundCriteriaSortKey(criteria LabelCompoundCriteriaModel) string {
	return criteria.Field.ValueString() + "\x00" + criteria.Op.ValueString() + "\x00" + criteria.Argument.ValueString()
}

func validateLabelCriteria(criteria []LabelCriteriaModel, diags *diag.Diagnostics) {
	flatSeen := make(map[string]struct{})
	compoundGroupSeen := make(map[string]struct{})

	for i, c := range criteria {
		flatField := !c.Field.IsNull() && c.Field.ValueString() != ""
		flatOp := !c.Op.IsNull() && c.Op.ValueString() != ""
		flatArg := !c.Argument.IsNull() && c.Argument.ValueString() != ""
		flatAny := flatField || flatOp || flatArg
		flatAll := flatField && flatOp && flatArg
		compoundSet := len(c.CompoundCriteria) > 0

		if compoundSet && flatAny {
			diags.AddError(
				"Invalid Label Criteria",
				fmt.Sprintf("criteria[%d] cannot set both flat fields (field/op/argument) and compound_criteria", i),
			)
			continue
		}

		if compoundSet {
			compoundSeen := make(map[string]struct{})
			for j, cc := range c.CompoundCriteria {
				if cc.Field.IsNull() || cc.Field.ValueString() == "" || cc.Op.IsNull() || cc.Op.ValueString() == "" || cc.Argument.IsNull() || cc.Argument.ValueString() == "" {
					diags.AddError(
						"Invalid Label Criteria",
						fmt.Sprintf("criteria[%d].compound_criteria[%d] requires non-empty field, op, and argument", i, j),
					)
					continue
				}

				key := cc.Field.ValueString() + "\x00" + cc.Op.ValueString() + "\x00" + cc.Argument.ValueString()
				if _, exists := compoundSeen[key]; exists {
					diags.AddError(
						"Invalid Label Criteria",
						fmt.Sprintf("criteria[%d].compound_criteria contains duplicate criterion field=%q op=%q argument=%q", i, cc.Field.ValueString(), cc.Op.ValueString(), cc.Argument.ValueString()),
					)
					continue
				}
				compoundSeen[key] = struct{}{}
			}

			groupKey := labelCriteriaSortKey(c)
			if _, exists := compoundGroupSeen[groupKey]; exists {
				diags.AddError(
					"Invalid Label Criteria",
					fmt.Sprintf("criteria[%d] duplicates an existing compound_criteria group", i),
				)
				continue
			}
			compoundGroupSeen[groupKey] = struct{}{}
			continue
		}

		if flatAny && !flatAll {
			diags.AddError(
				"Invalid Label Criteria",
				fmt.Sprintf("criteria[%d] must set all of field, op, and argument for a flat criterion", i),
			)
			continue
		}

		if !flatAny {
			diags.AddError(
				"Invalid Label Criteria",
				fmt.Sprintf("criteria[%d] must define either flat field/op/argument or compound_criteria", i),
			)
			continue
		}

		key := c.Field.ValueString() + "\x00" + c.Op.ValueString() + "\x00" + c.Argument.ValueString()
		if _, exists := flatSeen[key]; exists {
			diags.AddError(
				"Invalid Label Criteria",
				fmt.Sprintf("criteria[%d] duplicates an existing flat criterion field=%q op=%q argument=%q", i, c.Field.ValueString(), c.Op.ValueString(), c.Argument.ValueString()),
			)
			continue
		}
		flatSeen[key] = struct{}{}
	}
}

// checkLabelReferencesOnDestroy checks if any label groups or assets reference this label
// and emits warning or error diagnostics depending on strict mode.
func (r *LabelResource) checkLabelReferencesOnDestroy(ctx context.Context, labelID string, strict bool, diags *diag.Diagnostics) {
	addDiag := diags.AddWarning
	if strict {
		addDiag = diags.AddError
	}

	groups, err := r.client.ListLabelGroups(ctx, "", "")
	if err != nil {
		tflog.Warn(ctx, "unable to check label group references on destroy", map[string]any{"error": err.Error()})
	} else {
		for _, group := range groups {
			if labelGroupReferencesLabel(&group, labelID) {
				addDiag(
					"Label Referenced by Label Group",
					fmt.Sprintf("Label %q is referenced by label group %q (%s). "+
						"Destroying this label may leave the label group in an inconsistent state.",
						labelID, group.Value, group.ID),
				)
			}
		}
	}

	assets, err := r.client.ListAssets(ctx, "")
	if err != nil {
		tflog.Warn(ctx, "unable to check asset label references on destroy", map[string]any{"error": err.Error()})
	} else {
		for _, asset := range assets {
			for _, label := range asset.Labels {
				if label.ID == labelID {
					addDiag(
						"Label Referenced by Asset",
						fmt.Sprintf("Label %q is referenced by asset %q (%s). "+
							"Destroying this label may leave the asset in an inconsistent state.",
							labelID, asset.Name, asset.ID),
					)
				}
			}
		}
	}
}

// labelGroupReferencesLabel checks if a label group's include or exclude labels
// reference a specific label ID.
func labelGroupReferencesLabel(group *client.LabelGroup, labelID string) bool {
	if group.IncludeLabels != nil {
		for _, orLabel := range group.IncludeLabels.OrLabels {
			for _, andLabel := range orLabel.AndLabels {
				if andLabel.ID == labelID {
					return true
				}
			}
		}
	}
	if group.ExcludeLabels != nil {
		for _, orLabel := range group.ExcludeLabels.OrLabels {
			for _, andLabel := range orLabel.AndLabels {
				if andLabel.ID == labelID {
					return true
				}
			}
		}
	}
	return false
}
