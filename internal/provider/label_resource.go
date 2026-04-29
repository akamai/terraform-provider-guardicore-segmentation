package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
var _ resource.Resource = &LabelResource{}
var _ resource.ResourceWithImportState = &LabelResource{}

func NewLabelResource() resource.Resource {
	return &LabelResource{}
}

// LabelResource defines the resource implementation.
type LabelResource struct {
	client                *client.Client
	validateRefsOnDestroy bool
	strictRefsOnDestroy   bool
}

// LabelResourceModel describes the resource data model.
type LabelResourceModel struct {
	ID       types.String         `tfsdk:"id"`
	Key      types.String         `tfsdk:"key"`
	Value    types.String         `tfsdk:"value"`
	Criteria []LabelCriteriaModel `tfsdk:"criteria"`
}

// LabelCriteriaModel describes the criteria data model.
type LabelCriteriaModel struct {
	Field    types.String `tfsdk:"field"`
	Op       types.String `tfsdk:"op"`
	Argument types.String `tfsdk:"argument"`
}

func (r *LabelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *LabelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a label in Akamai Guardicore Segmentation. Labels are used to categorize and organize assets.",

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
			"criteria": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Dynamic criteria for automatic label assignment.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The field to match against (e.g., 'name', 'ip', 'os_name', 'process_name').",
						},
						"op": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The operator for matching (e.g., 'EQUALS', 'CONTAINS', 'STARTSWITH', 'ENDSWITH', 'REGEX').",
						},
						"argument": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The argument to match against.",
						},
					},
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
	r.validateRefsOnDestroy = providerData.ValidateRefsOnDestroy
	r.strictRefsOnDestroy = providerData.StrictRefsOnDestroy
}

func (r *LabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := r.modelToAPI(&data)

	createdLabel, err := r.client.CreateLabel(ctx, label)
	if err != nil {
		var apiErr *client.APIError
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
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create label, got error: %s", err))
		}
		return
	}

	data.ID = types.StringValue(createdLabel.ID)

	tflog.Trace(ctx, "created label", map[string]interface{}{"id": createdLabel.ID})

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

	r.apiToModel(label, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := r.modelToAPI(&data)

	updateLabel := &client.LabelUpdate{
		Key:   label.Key,
		Value: label.Value,
	}
	if len(label.Criteria) > 0 {
		updateLabel.Criteria = label.Criteria
	}

	_, err := r.client.UpdateLabel(ctx, data.ID.ValueString(), updateLabel)
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LabelResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
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

	err := r.client.DeleteLabel(ctx, data.ID.ValueString())
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

	if len(label.DynamicCriteria) > 0 {
		// Sort criteria to ensure consistent ordering (API may return in different order)
		sortedCriteria := make([]client.LabelCriteria, len(label.DynamicCriteria))
		copy(sortedCriteria, label.DynamicCriteria)
		sort.Slice(sortedCriteria, func(i, j int) bool {
			if sortedCriteria[i].Field != sortedCriteria[j].Field {
				return sortedCriteria[i].Field < sortedCriteria[j].Field
			}
			if sortedCriteria[i].Op != sortedCriteria[j].Op {
				return sortedCriteria[i].Op < sortedCriteria[j].Op
			}
			return sortedCriteria[i].Argument < sortedCriteria[j].Argument
		})

		data.Criteria = make([]LabelCriteriaModel, len(sortedCriteria))
		for i, c := range sortedCriteria {
			data.Criteria[i] = LabelCriteriaModel{
				Field:    types.StringValue(c.Field),
				Op:       types.StringValue(c.Op),
				Argument: types.StringValue(c.Argument),
			}
		}
	} else {
		data.Criteria = nil
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
