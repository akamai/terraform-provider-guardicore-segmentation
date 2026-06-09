package worksite

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &WorksiteResource{}
var _ resource.ResourceWithImportState = &WorksiteResource{}

func NewWorksiteResource() resource.Resource {
	return &WorksiteResource{}
}

// WorksiteResource defines the resource implementation.
type WorksiteResource struct {
	client        worksiteResourceClient
	deleteBatcher worksiteDeleteEnqueuer
}

type worksiteResourceClient interface {
	CreateWorksite(ctx context.Context, worksite *client.WorksiteCreate) (string, error)
	GetWorksite(ctx context.Context, id string) (*client.Worksite, error)
	UpdateWorksite(ctx context.Context, worksite *client.WorksiteUpdate) error
}

type worksiteDeleteEnqueuer interface {
	Enqueue(ctx context.Context, in string) (struct{}, error)
}

var _ worksiteResourceClient = (*client.Client)(nil)
var _ worksiteDeleteEnqueuer = (*common.Batcher[string, struct{}])(nil)

// WorksiteResourceModel describes the resource data model.
type WorksiteResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Comment       types.String `tfsdk:"comment"`
	SystemManaged types.Bool   `tfsdk:"system_managed"`
	ManagedBy     types.String `tfsdk:"managed_by"`
}

func (r *WorksiteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worksite"
}

func (r *WorksiteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a worksite in Akamai Guardicore Segmentation. Worksites are used to organize and group agents, assets, and policy rules by physical or logical location.\n\n" +
			"The \"Default\" worksite is system-managed and created by the platform. " +
			"System-managed worksites can be read and referenced by other resources, but cannot be updated or deleted by Terraform.\n\n" +
			"Use this resource for worksites that Terraform should create and manage. " +
			"Use the `guardicore_worksite` data source to reference the system-managed \"Default\" worksite.\n\n" +
			"When `system_managed` is `true`, any attempt to update or delete the worksite will return an error.\n\n" +
			"~> **Note:** The worksites feature must be enabled on the Akamai Guardicore Segmentation instance. If you receive a \"worksites feature is disabled\" error, enable it in the Akamai Guardicore Segmentation management console before using this resource.\n\n" +
			"Assets and policy rules can be assigned to a worksite using the `worksite_id` attribute on `guardicore_asset` and `guardicore_policy_rule` resources.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the worksite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the worksite (1-100 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
			},
			"comment": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A comment for the worksite (up to 2000 characters).",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(2000),
				},
			},
			"system_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this worksite is system-managed. The \"Default\" worksite is system-managed and cannot be updated or deleted by Terraform. Use the `guardicore_worksite` data source to reference it.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"managed_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifies who manages this worksite. `terraform` for user-managed worksites, or `system` for the platform-managed \"Default\" worksite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *WorksiteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData := common.ProviderDataFromRequest(req.ProviderData, &resp.Diagnostics)
	if providerData == nil {
		return
	}

	r.client = providerData.Client
	r.deleteBatcher = providerData.WorksiteDeleteBatcher
}

func (r *WorksiteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.createModel(ctx, data)
	if err != nil {
		addWorksiteClientError(&resp.Diagnostics, "create", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

func (r *WorksiteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, remove, err := r.readModel(ctx, data)
	if err != nil {
		addWorksiteClientError(&resp.Diagnostics, "read", err)
		return
	}
	if remove {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

func (r *WorksiteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorksiteResourceModel
	var stateData WorksiteResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	common.DiagnoseSystemManagedMutation(stateData.SystemManaged, "worksite", data.ID.ValueString(), "update", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, gone, err := r.updateModel(ctx, data, stateData)
	if err != nil {
		addWorksiteClientError(&resp.Diagnostics, "update", err)
		return
	}
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

func (r *WorksiteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	common.DiagnoseSystemManagedMutation(data.SystemManaged, "worksite", data.ID.ValueString(), "delete", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.deleteModel(ctx, data)
	if err != nil {
		addWorksiteClientError(&resp.Diagnostics, "delete", err)
		return
	}
}

func (r *WorksiteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct for create.
func (r *WorksiteResource) modelToAPI(data *WorksiteResourceModel) *client.WorksiteCreate {
	return &client.WorksiteCreate{
		Name:    data.Name.ValueString(),
		Comment: data.Comment.ValueString(),
	}
}

// apiToModel converts the API struct to Terraform model.
func (r *WorksiteResource) apiToModel(worksite *client.Worksite, data *WorksiteResourceModel) {
	data.ID = types.StringValue(worksite.ID)
	data.Name = types.StringValue(worksite.Name)
	data.Comment = types.StringValue(worksite.Comment)
}

func (r *WorksiteResource) createModel(ctx context.Context, data WorksiteResourceModel) (WorksiteResourceModel, error) {
	worksite := r.modelToAPI(&data)

	id, err := r.client.CreateWorksite(ctx, worksite)
	if err != nil {
		return WorksiteResourceModel{}, err
	}

	data.ID = types.StringValue(id)

	created, err := r.client.GetWorksite(ctx, id)
	if err != nil {
		return WorksiteResourceModel{}, fmt.Errorf("unable to read worksite after creation, got error: %w", err)
	}
	if created != nil {
		r.apiToModel(created, &data)
	}

	data.SystemManaged = types.BoolValue(false)
	data.ManagedBy = types.StringValue("terraform")

	tflog.Trace(ctx, "created worksite", map[string]any{"id": id})

	return data, nil
}

func (r *WorksiteResource) readModel(ctx context.Context, data WorksiteResourceModel) (WorksiteResourceModel, bool, error) {
	worksite, err := r.client.GetWorksite(ctx, data.ID.ValueString())
	if err != nil {
		return WorksiteResourceModel{}, false, err
	}

	if worksite == nil {
		return WorksiteResourceModel{}, true, nil
	}

	r.apiToModel(worksite, &data)

	sm, mb := common.WorksiteIsSystemManaged(worksite)
	data.SystemManaged = types.BoolValue(sm)
	data.ManagedBy = types.StringValue(mb)

	return data, false, nil
}

func (r *WorksiteResource) updateModel(ctx context.Context, data WorksiteResourceModel, stateData WorksiteResourceModel) (WorksiteResourceModel, bool, error) {
	update := &client.WorksiteUpdate{
		ID:      data.ID.ValueString(),
		Name:    data.Name.ValueString(),
		Comment: data.Comment.ValueString(),
	}

	err := r.client.UpdateWorksite(ctx, update)
	if err != nil {
		return WorksiteResourceModel{}, false, err
	}

	updated, err := r.client.GetWorksite(ctx, data.ID.ValueString())
	if err != nil {
		return WorksiteResourceModel{}, false, fmt.Errorf("unable to read worksite after update, got error: %w", err)
	}
	if updated == nil {
		return WorksiteResourceModel{}, true, nil
	}
	r.apiToModel(updated, &data)
	data.SystemManaged = stateData.SystemManaged
	data.ManagedBy = stateData.ManagedBy

	tflog.Trace(ctx, "updated worksite", map[string]any{"id": data.ID.ValueString()})

	return data, false, nil
}

func (r *WorksiteResource) deleteModel(ctx context.Context, data WorksiteResourceModel) error {
	_, err := r.deleteBatcher.Enqueue(ctx, data.ID.ValueString())
	if err != nil {
		return err
	}

	tflog.Trace(ctx, "deleted worksite", map[string]any{"id": data.ID.ValueString()})
	return nil
}

func addWorksiteClientError(diags *diag.Diagnostics, action string, err error) {
	if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
		diags.AddError("Worksites Feature Disabled", err.Error())
		return
	}

	diags.AddError("Client Error", fmt.Sprintf("Unable to %s worksite, got error: %s", action, err))
}
