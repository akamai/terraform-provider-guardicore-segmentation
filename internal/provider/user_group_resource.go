package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
var _ resource.Resource = &UserGroupResource{}
var _ resource.ResourceWithImportState = &UserGroupResource{}

func NewUserGroupResource() resource.Resource {
	return &UserGroupResource{}
}

// UserGroupResource defines the resource implementation.
type UserGroupResource struct {
	client                *client.Client
	validateRefsOnDestroy bool
	strictRefsOnDestroy   bool
}

// OrchestrationGroupModel describes an orchestration group within a user group.
type OrchestrationGroupModel struct {
	OrchestrationID types.String `tfsdk:"orchestration_id"`
	Groups          types.List   `tfsdk:"groups"`
}

// UserGroupResourceModel describes the resource data model.
type UserGroupResourceModel struct {
	ID                   types.String              `tfsdk:"id"`
	Title                types.String              `tfsdk:"title"`
	OrchestrationsGroups []OrchestrationGroupModel `tfsdk:"orchestrations_groups"`
}

func (r *UserGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (r *UserGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a user group in Akamai Guardicore Segmentation. User groups associate Active Directory orchestration groups for use in visibility policies.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the user group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The title of the user group.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"orchestrations_groups": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "List of orchestration groups to include in the user group.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"orchestration_id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The orchestration ID.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"groups": schema.ListAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "List of group IDs within the orchestration.",
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
						},
					},
				},
			},
		},
	}
}

func (r *UserGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := r.modelToAPI(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateUserGroup(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user group, got error: %s", err))
		return
	}

	data.ID = types.StringValue(id)

	// Read back to populate server-computed fields
	created, err := r.client.GetUserGroup(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user group after creation, got error: %s", err))
		return
	}
	if created != nil {
		r.apiToModel(ctx, created, &data, &resp.Diagnostics)
	}

	tflog.Trace(ctx, "created user group", map[string]any{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userGroup, err := r.client.GetUserGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user group, got error: %s", err))
		return
	}

	if userGroup == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.apiToModel(ctx, userGroup, &data, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := r.modelToAPI(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.UpdateUserGroup(ctx, data.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update user group, got error: %s", err))
		return
	}

	// Read back to populate server-computed fields
	updated, err := r.client.GetUserGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user group after update, got error: %s", err))
		return
	}
	if updated != nil {
		r.apiToModel(ctx, updated, &data, &resp.Diagnostics)
	}

	tflog.Trace(ctx, "updated user group", map[string]any{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for references from policy rules before destroying
	if r.validateRefsOnDestroy {
		r.checkUserGroupReferencesOnDestroy(ctx, data.ID.ValueString(), r.strictRefsOnDestroy, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	err := r.client.DeleteUserGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user group, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted user group", map[string]any{"id": data.ID.ValueString()})
}

func (r *UserGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct.
func (r *UserGroupResource) modelToAPI(ctx context.Context, data *UserGroupResourceModel, diags *diag.Diagnostics) *client.UserGroupCreate {
	orchGroups := make([]client.OrchestrationGroup, len(data.OrchestrationsGroups))
	for i, og := range data.OrchestrationsGroups {
		var groups []string
		diags.Append(og.Groups.ElementsAs(ctx, &groups, false)...)
		orchGroups[i] = client.OrchestrationGroup{
			OrchestrationID: og.OrchestrationID.ValueString(),
			Groups:          groups,
		}
	}

	return &client.UserGroupCreate{
		Title:                data.Title.ValueString(),
		OrchestrationsGroups: orchGroups,
	}
}

// apiToModel converts the API struct to Terraform model.
func (r *UserGroupResource) apiToModel(ctx context.Context, userGroup *client.UserGroup, data *UserGroupResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(userGroup.ID)
	data.Title = types.StringValue(userGroup.Title)

	orchGroups := make([]OrchestrationGroupModel, len(userGroup.OrchestrationsGroups))
	for i, og := range userGroup.OrchestrationsGroups {
		groupsList, d := types.ListValueFrom(ctx, types.StringType, og.Groups)
		diags.Append(d...)
		orchGroups[i] = OrchestrationGroupModel{
			OrchestrationID: types.StringValue(og.OrchestrationID),
			Groups:          groupsList,
		}
	}
	data.OrchestrationsGroups = orchGroups
}

// checkUserGroupReferencesOnDestroy checks if any policy rules reference this
// user group and emits warning or error diagnostics depending on strict mode.
func (r *UserGroupResource) checkUserGroupReferencesOnDestroy(ctx context.Context, userGroupID string, strict bool, diags *diag.Diagnostics) {
	addDiag := diags.AddWarning
	if strict {
		addDiag = diags.AddError
	}

	rules, err := r.client.ListPolicyRules(ctx)
	if err != nil {
		tflog.Warn(ctx, "unable to check user group references on destroy", map[string]any{"error": err.Error()})
		return
	}

	for _, rule := range rules {
		ruleID, _ := rule["id"].(string)
		if policyRuleReferencesUserGroup(rule, userGroupID) {
			addDiag(
				"User Group Referenced by Policy Rule",
				fmt.Sprintf("User group %q is referenced by policy rule %q. "+
					"Destroying this user group may leave the policy rule in an inconsistent state.",
					userGroupID, ruleID),
			)
		}
	}
}

// policyRuleReferencesUserGroup checks if a policy rule spec references
// a specific user group ID in its source or destination endpoints.
func policyRuleReferencesUserGroup(rule map[string]any, userGroupID string) bool {
	for _, endpointKey := range []string{"source", "destination"} {
		endpoint, ok := rule[endpointKey]
		if !ok {
			continue
		}
		endpointMap, ok := endpoint.(map[string]any)
		if !ok {
			continue
		}
		refs, ok := endpointMap["user_group_ids"]
		if !ok {
			continue
		}
		refSlice, ok := refs.([]any)
		if !ok {
			continue
		}
		for _, ref := range refSlice {
			if refID, ok := ref.(string); ok && refID == userGroupID {
				return true
			}
		}
	}
	return false
}
