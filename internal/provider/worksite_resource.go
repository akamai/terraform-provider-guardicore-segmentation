package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &WorksiteResource{}
var _ resource.ResourceWithImportState = &WorksiteResource{}

func NewWorksiteResource() resource.Resource {
	return &WorksiteResource{}
}

// WorksiteResource defines the resource implementation.
type WorksiteResource struct {
	client *client.Client
}

// WorksiteResourceModel describes the resource data model.
type WorksiteResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Comment types.String `tfsdk:"comment"`
}

func (r *WorksiteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_worksite"
}

func (r *WorksiteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a worksite in Akamai Guardicore Segmentation. Worksites are used to organize and group agents, assets, and policy rules by physical or logical location.\n\n" +
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
		},
	}
}

func (r *WorksiteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

func (r *WorksiteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	worksite := r.modelToAPI(&data)

	id, err := r.client.CreateWorksite(ctx, worksite)
	if err != nil {
		if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
			resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create worksite, got error: %s", err))
		}
		return
	}

	data.ID = types.StringValue(id)

	// Read back to populate server-computed fields
	created, err := r.client.GetWorksite(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read worksite after creation, got error: %s", err))
		return
	}
	if created != nil {
		r.apiToModel(created, &data)
	}

	tflog.Trace(ctx, "created worksite", map[string]any{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorksiteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	worksite, err := r.client.GetWorksite(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
			resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read worksite, got error: %s", err))
		}
		return
	}

	if worksite == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.apiToModel(worksite, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorksiteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	update := &client.WorksiteUpdate{
		ID:      data.ID.ValueString(),
		Name:    data.Name.ValueString(),
		Comment: data.Comment.ValueString(),
	}

	err := r.client.UpdateWorksite(ctx, update)
	if err != nil {
		if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
			resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update worksite, got error: %s", err))
		}
		return
	}

	// Read back to populate server-computed fields
	updated, err := r.client.GetWorksite(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read worksite after update, got error: %s", err))
		return
	}
	if updated != nil {
		r.apiToModel(updated, &data)
	}

	tflog.Trace(ctx, "updated worksite", map[string]any{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorksiteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorksiteResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWorksite(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrWorksitesFeatureDisabled) {
			resp.Diagnostics.AddError("Worksites Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete worksite, got error: %s", err))
		}
		return
	}

	tflog.Trace(ctx, "deleted worksite", map[string]any{"id": data.ID.ValueString()})
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
