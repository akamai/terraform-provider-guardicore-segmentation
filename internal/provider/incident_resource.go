package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &IncidentResource{}

func NewIncidentResource() resource.Resource {
	return &IncidentResource{}
}

// IncidentResource defines the resource implementation.
type IncidentResource struct {
	client *client.Client
}

// IncidentResourceModel describes the resource data model.
type IncidentResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Type                     types.String `tfsdk:"type"`
	Severity                 types.String `tfsdk:"severity"`
	AffectedAssetsJSON       types.String `tfsdk:"affected_assets_json"`
	Time                     types.Int64  `tfsdk:"time"`
	Tags                     types.List   `tfsdk:"tags"`
	Description              types.String `tfsdk:"description"`
	Summary                  types.String `tfsdk:"summary"`
	Origin                   types.String `tfsdk:"origin"`
	Mitigation               types.String `tfsdk:"mitigation"`
	CefExtensionsJSON        types.String `tfsdk:"cef_extensions_json"`
	AttachedFiles            types.List   `tfsdk:"attached_files"`
	MapDetailsJSON           types.String `tfsdk:"map_details_json"`
	CustomDefinedObjectsJSON types.String `tfsdk:"custom_defined_objects_json"`
	PropertiesJSON           types.String `tfsdk:"properties_json"`
}

func (r *IncidentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident"
}

func (r *IncidentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a security incident in Akamai Guardicore Segmentation. " +
			"Incidents are **immutable** — they cannot be updated or deleted via the API. " +
			"Any attribute change forces recreation, and `terraform destroy` only removes the incident from Terraform state.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the incident.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The incident type (1-30 characters). For example: `CustomIncident`, `Malware Detection`.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 30),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"severity": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The incident severity. Valid values: `LOW`, `MEDIUM`, `HIGH`.",
				Validators: []validator.String{
					stringvalidator.OneOf("LOW", "MEDIUM", "HIGH"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"affected_assets_json": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "JSON-encoded list of affected assets. Each asset should have `ip` and/or `vm` fields. " +
					"Use `jsonencode()` to provide the value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"time": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Incident occurrence timestamp in milliseconds since epoch.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"tags": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of tags (1-10 items).",
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 10),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Short description of the incident (1-200 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 200),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"summary": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Markdown-formatted incident summary (1-10000 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 10000),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"origin": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Who created the incident (1-25 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 25),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mitigation": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Markdown-formatted incident mitigation information (1-2000 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 2000),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cef_extensions_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON-encoded CEF extensions to add to each CEF message. " +
					"Use `jsonencode()` to provide the value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attached_files": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of attached file IDs (0-6 items).",
				Validators: []validator.List{
					listvalidator.SizeBetween(0, 6),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"map_details_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON-encoded map details for viewing the incident in Akamai Guardicore Segmentation. " +
					"Use `jsonencode()` to provide the value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"custom_defined_objects_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON-encoded list of user-defined objects (max 30 items). " +
					"Use `jsonencode()` to provide the value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "JSON-encoded list of related properties (max 20 items). " +
					"Only for non-Hunt incidents. Use `jsonencode()` to provide the value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *IncidentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IncidentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IncidentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	incident, diags := r.modelToAPI(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateIncident(ctx, incident)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create incident, got error: %s", err))
		return
	}

	data.ID = types.StringValue(id)

	tflog.Trace(ctx, "created incident", map[string]interface{}{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IncidentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IncidentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Incidents are immutable and permanent — they cannot be updated or deleted.
	// The read API (`/api/v3.0/generic-incidents`) returns a very different schema
	// than the create API (`/api/v4.0/incidents`), and many create-time fields
	// (description, summary, origin, mitigation) are not
	// returned. We preserve the state from creation as the source of truth.
	//
	// We still verify existence when possible, but do not remove from state on failure
	// since incidents are guaranteed to persist.
	incident, err := r.client.GetIncident(ctx, data.ID.ValueString())
	if err != nil {
		tflog.Warn(ctx, "unable to verify incident existence, keeping state", map[string]interface{}{
			"id":    data.ID.ValueString(),
			"error": err.Error(),
		})
	} else if incident == nil {
		tflog.Warn(ctx, "incident not found via list API, keeping state (incidents are permanent)", map[string]interface{}{
			"id": data.ID.ValueString(),
		})
	}

	// Keep existing state unchanged — state from Create is authoritative.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IncidentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should never be called because all attributes have RequiresReplace().
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Incidents cannot be updated. All changes require recreation of the resource.",
	)
}

func (r *IncidentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IncidentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The Akamai Guardicore Segmentation API does not support deleting incidents.
	// The incident is removed from Terraform state but continues to exist in Akamai Guardicore Segmentation.
	resp.Diagnostics.AddWarning(
		"Incident Not Deleted",
		fmt.Sprintf("The Akamai Guardicore Segmentation API does not support deleting incidents. "+
			"Incident %s has been removed from Terraform state but still exists in Akamai Guardicore Segmentation.", data.ID.ValueString()),
	)

	tflog.Trace(ctx, "removed incident from state (no API delete)", map[string]interface{}{"id": data.ID.ValueString()})
}

// modelToAPI converts the Terraform model to API struct for create.
func (r *IncidentResource) modelToAPI(ctx context.Context, data *IncidentResourceModel) (*client.IncidentCreate, diag.Diagnostics) { //nolint:unparam
	var diags diag.Diagnostics

	incident := &client.IncidentCreate{
		Type:        data.Type.ValueString(),
		Severity:    data.Severity.ValueString(),
		Time:        data.Time.ValueInt64(),
		Description: data.Description.ValueString(),
		Summary:     data.Summary.ValueString(),
	}

	// Tags
	var tags []string
	diags.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
	if diags.HasError() {
		return nil, diags
	}
	incident.Tags = tags

	// Affected assets (required JSON)
	incident.AffectedAssets = json.RawMessage(data.AffectedAssetsJSON.ValueString())

	// Optional string fields
	if !data.Origin.IsNull() && !data.Origin.IsUnknown() {
		origin := data.Origin.ValueString()
		incident.Origin = &origin
	}
	if !data.Mitigation.IsNull() && !data.Mitigation.IsUnknown() {
		mitigation := data.Mitigation.ValueString()
		incident.Mitigation = &mitigation
	}

	// Optional JSON fields
	if !data.CefExtensionsJSON.IsNull() && !data.CefExtensionsJSON.IsUnknown() {
		incident.CefExtensions = json.RawMessage(data.CefExtensionsJSON.ValueString())
	}
	if !data.MapDetailsJSON.IsNull() && !data.MapDetailsJSON.IsUnknown() {
		incident.MapDetails = json.RawMessage(data.MapDetailsJSON.ValueString())
	}
	if !data.CustomDefinedObjectsJSON.IsNull() && !data.CustomDefinedObjectsJSON.IsUnknown() {
		incident.CustomDefinedObjects = json.RawMessage(data.CustomDefinedObjectsJSON.ValueString())
	}
	if !data.PropertiesJSON.IsNull() && !data.PropertiesJSON.IsUnknown() {
		incident.Properties = json.RawMessage(data.PropertiesJSON.ValueString())
	}

	// Optional attached files
	if !data.AttachedFiles.IsNull() && !data.AttachedFiles.IsUnknown() {
		var files []string
		diags.Append(data.AttachedFiles.ElementsAs(ctx, &files, false)...)
		if diags.HasError() {
			return nil, diags
		}
		incident.AttachedFiles = files
	}

	return incident, diags
}
