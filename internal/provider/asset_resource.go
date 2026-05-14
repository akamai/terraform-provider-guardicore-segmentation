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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AssetResource{}
var _ resource.ResourceWithImportState = &AssetResource{}

func NewAssetResource() resource.Resource {
	return &AssetResource{}
}

// AssetResource defines the resource implementation.
type AssetResource struct {
	client           *client.Client
	referenceChecker ReferenceChecker
	ignoreCache      *assetLabelIgnoreCache
	createBatcher    *Batcher[*client.AssetCreate, string]
	updateBatcher    *Batcher[*client.AssetBulkUpdateItem, struct{}]
	deleteBatcher    *Batcher[string, struct{}]
}

// NICModel describes a network interface within an asset.
type NICModel struct {
	VifID                types.String `tfsdk:"vif_id"`
	MacAddress           types.String `tfsdk:"mac_address"`
	NetworkID            types.String `tfsdk:"network_id"`
	NetworkName          types.String `tfsdk:"network_name"`
	IsCloudPublic        types.Bool   `tfsdk:"is_cloud_public"`
	IsCorporateInterface types.Bool   `tfsdk:"is_corporate_interface"`
	SwitchID             types.String `tfsdk:"switch_id"`
	IPAddresses          types.List   `tfsdk:"ip_addresses"`
}

// AssetLabelModel describes a label reference on an asset.
type AssetLabelModel struct {
	ID       types.String `tfsdk:"id"`
	Key      types.String `tfsdk:"key"`
	Value    types.String `tfsdk:"value"`
	ReadOnly types.Bool   `tfsdk:"read_only"`
}

// AssetResourceModel describes the resource data model.
type AssetResourceModel struct {
	ID                    types.String      `tfsdk:"id"`
	Name                  types.String      `tfsdk:"name"`
	Nics                  []NICModel        `tfsdk:"nics"`
	OrchestrationObjID    types.String      `tfsdk:"orchestration_obj_id"`
	Status                types.String      `tfsdk:"status"`
	Labels                []AssetLabelModel `tfsdk:"labels"`
	Comments              types.String      `tfsdk:"comments"`
	OrchestrationMetadata types.Object      `tfsdk:"orchestration_metadata"`
	WorksiteID            types.String      `tfsdk:"worksite_id"`
	InstanceID            types.String      `tfsdk:"instance_id"`
	HwUUID                types.String      `tfsdk:"hw_uuid"`
	BiosUUID              types.String      `tfsdk:"bios_uuid"`
	FirstSeen             types.String      `tfsdk:"first_seen"`
	LastSeen              types.String      `tfsdk:"last_seen"`
}

func (r *AssetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (r *AssetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an asset in Akamai Guardicore Segmentation. Assets represent network entities with network interfaces (NICs), labels, and orchestration metadata.\n\n" +
			"~> **Note:** The API DELETE operation deactivates the asset (sets status to `deleted`) rather than permanently removing it. " +
			"Deactivated assets may still appear in the Akamai Guardicore Segmentation management UI.\n\n" +
			"~> **Labels Behavior:** The `labels` attribute manages only explicitly configured, assignable labels for this asset. " +
			"Labels marked `read_only = true` and labels with dynamic criteria are not assignable and will be rejected during plan/apply validation. " +
			"Omit `labels` to opt out of Terraform label management for an asset. Set `labels = []` to explicitly manage zero assignable labels.\n\n" +
			"~> **Reference Validation:** Label IDs in the `labels` block and the `worksite_id` are validated for existence in Akamai Guardicore Segmentation during plan and apply. " +
			"Non-existent IDs will produce a clear error before any API call is made.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the asset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the asset.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"nics": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "List of network interfaces for the asset.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"vif_id": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Virtual interface ID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"mac_address": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "MAC address.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"network_id": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Network ID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"network_name": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Network name.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"is_cloud_public": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this is a cloud public network.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"is_corporate_interface": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this is a corporate network interface.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"switch_id": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Switch ID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"ip_addresses": schema.ListAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "List of IP addresses for this NIC.",
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
						},
					},
				},
			},
			"orchestration_obj_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Primary key of the asset from the customer's database or playbook. Cannot be changed after creation.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Current asset status (`on`, `off`, or `deleted`).",
				Validators: []validator.String{
					stringvalidator.OneOf("on", "off", "deleted"),
				},
			},
			"labels": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Set of Terraform-managed labels assigned to the asset. Each label requires `id`, `key`, and `value` — the API needs all three. Reference a managed label via `guardicore_label.<name>.id`, `.key`, and `.value`. Use the `guardicore_label` data source for labels not managed by Terraform. Labels marked `read_only = true` and labels with dynamic criteria are system-managed and cannot be assigned in `guardicore_asset.labels` (validation error). Omit this attribute to avoid Terraform label management for the asset; set `labels = []` to manage zero assignable labels.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The label ID. Use `guardicore_label.<name>.id` to reference a managed label.",
						},
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The label key. Use `guardicore_label.<name>.key` to reference a managed label.",
						},
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The label value. Use `guardicore_label.<name>.value` to reference a managed label.",
						},
						"read_only": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the label is read-only and managed by the server. Read-only labels and labels with dynamic criteria cannot be assigned in `guardicore_asset.labels`.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Additional asset comments.",
			},
			"orchestration_metadata": assetOrchestrationMetadataResourceSchemaAttribute(),
			"worksite_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The worksite ID to assign this asset to. Use `guardicore_worksite.example.id` to reference a managed worksite.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Instance ID generated by AWS/Azure/GCP. Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"hw_uuid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Hardware UUID. Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bios_uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BIOS UUID reported by the API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"first_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the asset was first seen.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"last_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the asset was last seen.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AssetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.referenceChecker = providerData.Client
	r.ignoreCache = providerData.AssetLabelIgnoreCache
	r.createBatcher = providerData.AssetCreateBatcher
	r.updateBatcher = providerData.AssetUpdateBatcher
	r.deleteBatcher = providerData.AssetDeleteBatcher
}

func (r *AssetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AssetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate label references exist before creating
	if len(data.Labels) > 0 {
		resp.Diagnostics.Append(validateAssetLabelRefs(ctx, r.client, data.Labels)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	apiReq := r.modelToCreateAPI(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.createBatcher.Enqueue(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create asset, got error: %s", err))
		return
	}

	data.ID = types.StringValue(id)

	// Assign worksite via separate endpoint if specified
	if !data.WorksiteID.IsNull() && !data.WorksiteID.IsUnknown() {
		err := r.client.AssignWorksite(ctx, &client.WorksiteAssignRequest{
			ID:         data.WorksiteID.ValueString(),
			EntityType: "asset",
			EntityIDs:  []string{id},
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to assign asset to worksite, got error: %s", err))
			return
		}
	}

	// Read back to populate server-computed fields (with eventual consistency retries)
	created, err := waitForReadAfterCreate(ctx, "asset", func(ctx context.Context) (*client.Asset, error) {
		return r.client.GetAsset(ctx, id)
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read asset after creation, got error: %s", err))
		return
	}
	r.apiToModel(ctx, created, &data, &resp.Diagnostics)

	tflog.Trace(ctx, "created asset", map[string]any{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AssetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asset, err := r.client.GetAsset(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read asset, got error: %s", err))
		return
	}

	if asset == nil || asset.Status == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}

	r.apiToModel(ctx, asset, &data, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AssetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate label references exist before updating
	if len(data.Labels) > 0 {
		resp.Diagnostics.Append(validateAssetLabelRefs(ctx, r.client, data.Labels)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	apiReq := r.modelToUpdateAPI(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.updateBatcher.Enqueue(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update asset, got error: %s", err))
		return
	}

	// Handle worksite assignment changes via separate endpoint
	var stateData AssetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldWS := stateData.WorksiteID.ValueString()
	newWS := data.WorksiteID.ValueString()
	wantWorksite := !data.WorksiteID.IsNull() && !data.WorksiteID.IsUnknown()

	if wantWorksite && newWS != oldWS {
		err := r.client.AssignWorksite(ctx, &client.WorksiteAssignRequest{
			ID:         newWS,
			EntityType: "asset",
			EntityIDs:  []string{data.ID.ValueString()},
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to assign asset to worksite, got error: %s", err))
			return
		}
	} else if !wantWorksite && oldWS != "" {
		err := r.client.AssignWorksite(ctx, &client.WorksiteAssignRequest{
			ID:         "all_worksites",
			EntityType: "asset",
			EntityIDs:  []string{data.ID.ValueString()},
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to unassign asset from worksite, got error: %s", err))
			return
		}
	}

	// Read back to populate server-computed fields
	updated, err := r.client.GetAsset(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read asset after update, got error: %s", err))
		return
	}
	if updated != nil {
		r.apiToModel(ctx, updated, &data, &resp.Diagnostics)
	}

	tflog.Trace(ctx, "updated asset", map[string]any{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AssetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.deleteBatcher.Enqueue(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to deactivate asset, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deactivated asset", map[string]any{"id": data.ID.ValueString()})
}

func (r *AssetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToCreateAPI converts the Terraform model to API struct for create.
func (r *AssetResource) modelToCreateAPI(ctx context.Context, data *AssetResourceModel, diags *diag.Diagnostics) *client.AssetCreate {
	nics := r.modelNicsToAPI(ctx, data.Nics, diags)
	labels := r.modelLabelsToAPI(ctx, data.Labels, diags)

	result := &client.AssetCreate{
		Name:               data.Name.ValueString(),
		Nics:               nics,
		OrchestrationObjID: data.OrchestrationObjID.ValueString(),
	}
	if labels != nil {
		result.Labels = &labels
	}

	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		result.Status = data.Status.ValueString()
	}
	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		result.Comments = data.Comments.ValueString()
	}
	if !data.OrchestrationMetadata.IsNull() && !data.OrchestrationMetadata.IsUnknown() {
		metadata, d := buildAssetOrchestrationMetadataJSON(ctx, data.OrchestrationMetadata)
		diags.Append(d...)
		if diags.HasError() {
			return result
		}
		result.OrchestrationMetadata = metadata
	}
	if !data.InstanceID.IsNull() && !data.InstanceID.IsUnknown() {
		result.InstanceID = data.InstanceID.ValueString()
	}
	if !data.HwUUID.IsNull() && !data.HwUUID.IsUnknown() {
		result.HwUUID = data.HwUUID.ValueString()
	}
	return result
}

// modelToUpdateAPI converts the Terraform model to API struct for bulk update.
// NOTE: orchestration_obj_id, instance_id, and hw_uuid are NOT in the edit body.
func (r *AssetResource) modelToUpdateAPI(ctx context.Context, data *AssetResourceModel, diags *diag.Diagnostics) *client.AssetBulkUpdateItem {
	nics := r.modelNicsToAPI(ctx, data.Nics, diags)
	labels := r.modelLabelsToAPI(ctx, data.Labels, diags)

	result := &client.AssetBulkUpdateItem{
		AssetID: data.ID.ValueString(),
		Name:    data.Name.ValueString(),
		Nics:    nics,
	}
	if labels != nil {
		result.Labels = &labels
	}

	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		result.Status = data.Status.ValueString()
	}
	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		result.Comments = data.Comments.ValueString()
	}
	if !data.OrchestrationMetadata.IsNull() && !data.OrchestrationMetadata.IsUnknown() {
		metadata, d := buildAssetOrchestrationMetadataJSON(ctx, data.OrchestrationMetadata)
		diags.Append(d...)
		if diags.HasError() {
			return result
		}
		result.OrchestrationMetadata = metadata
	}
	return result
}

// modelNicsToAPI converts NIC models to API structs.
func (r *AssetResource) modelNicsToAPI(ctx context.Context, nics []NICModel, diags *diag.Diagnostics) []client.AssetNIC {
	apiNics := make([]client.AssetNIC, len(nics))
	for i, nic := range nics {
		var ips []string
		diags.Append(nic.IPAddresses.ElementsAs(ctx, &ips, false)...)

		apiNic := client.AssetNIC{
			IPAddresses: ips,
		}

		if !nic.VifID.IsNull() && !nic.VifID.IsUnknown() {
			apiNic.VifID = nic.VifID.ValueString()
		}
		if !nic.MacAddress.IsNull() && !nic.MacAddress.IsUnknown() {
			apiNic.MacAddress = nic.MacAddress.ValueString()
		}
		if !nic.NetworkID.IsNull() && !nic.NetworkID.IsUnknown() {
			apiNic.NetworkID = nic.NetworkID.ValueString()
		}
		if !nic.NetworkName.IsNull() && !nic.NetworkName.IsUnknown() {
			apiNic.NetworkName = nic.NetworkName.ValueString()
		}
		if !nic.IsCloudPublic.IsNull() && !nic.IsCloudPublic.IsUnknown() {
			v := nic.IsCloudPublic.ValueBool()
			apiNic.IsCloudPublic = &v
		}
		if !nic.IsCorporateInterface.IsNull() && !nic.IsCorporateInterface.IsUnknown() {
			v := nic.IsCorporateInterface.ValueBool()
			apiNic.IsCorporateInterface = &v
		}
		if !nic.SwitchID.IsNull() && !nic.SwitchID.IsUnknown() {
			apiNic.SwitchID = nic.SwitchID.ValueString()
		}

		apiNics[i] = apiNic
	}
	return apiNics
}

// modelLabelsToAPI converts label models to API structs.
func (r *AssetResource) modelLabelsToAPI(ctx context.Context, labels []AssetLabelModel, diags *diag.Diagnostics) []client.AssetLabelRef {
	if labels == nil {
		return nil
	}
	apiLabels := make([]client.AssetLabelRef, 0, len(labels))
	for i, l := range labels {
		if l.ID.IsNull() || l.ID.IsUnknown() || l.ID.ValueString() == "" ||
			l.Key.IsNull() || l.Key.IsUnknown() || l.Key.ValueString() == "" ||
			l.Value.IsNull() || l.Value.IsUnknown() || l.Value.ValueString() == "" {
			diags.AddError(
				"Invalid Asset Label Configuration",
				fmt.Sprintf("labels[%d] must include non-empty id, key, and value.", i),
			)
			continue
		}

		if r.isAssetIgnoredLabel(ctx, l, diags) {
			diags.AddError(
				"Invalid Asset Label Configuration",
				fmt.Sprintf("Label %q in labels[%d] is read-only or dynamic and cannot be managed in guardicore_asset.labels. Remove it from configuration; server-managed labels are preserved automatically.", l.ID.ValueString(), i),
			)
			continue
		}

		apiLabels = append(apiLabels, client.AssetLabelRef{
			ID:    l.ID.ValueString(),
			Key:   l.Key.ValueString(),
			Value: l.Value.ValueString(),
		})
	}
	return apiLabels
}

func (r *AssetResource) isAssetIgnoredLabel(ctx context.Context, label AssetLabelModel, diags *diag.Diagnostics) bool {
	if isAssetReadOnlyLabel(label) {
		return true
	}

	if label.ID.IsNull() || label.ID.IsUnknown() || label.ID.ValueString() == "" || r.referenceChecker == nil {
		return false
	}

	labelID := label.ID.ValueString()
	if r.ignoreCache != nil {
		if ignored, ok := r.ignoreCache.Get(labelID); ok {
			return ignored
		}
	}

	apiLabel, err := r.referenceChecker.GetLabel(ctx, labelID)
	if err != nil {
		diags.AddWarning(
			"Unable to Determine Asset Label Ignore Status",
			fmt.Sprintf("Unable to check whether label %q should be ignored for asset payload reconciliation: %s", labelID, err),
		)
		return false
	}

	if apiLabel == nil {
		return false
	}

	if apiLabel.ReadOnly == nil && apiLabel.Key != "" && apiLabel.Value != "" {
		labels, listErr := r.referenceChecker.ListLabels(ctx, apiLabel.Key, apiLabel.Value)
		if listErr != nil {
			diags.AddWarning(
				"Unable to Determine Asset Label Ignore Status",
				fmt.Sprintf("Unable to list labels while resolving read-only status for label %q: %s", labelID, listErr),
			)
		} else {
			for _, candidate := range labels {
				if candidate.ID == labelID {
					apiLabel.ReadOnly = candidate.ReadOnly
					if len(apiLabel.DynamicCriteria) == 0 && len(candidate.DynamicCriteria) > 0 {
						apiLabel.DynamicCriteria = candidate.DynamicCriteria
					}
					break
				}
			}
		}
	}

	ignored := (apiLabel.ReadOnly != nil && *apiLabel.ReadOnly) || len(apiLabel.DynamicCriteria) > 0

	if r.ignoreCache != nil {
		r.ignoreCache.Set(labelID, ignored)
	}

	return ignored
}

func assetLabelModelFromAPI(apiLabel client.AssetLabelRef, existing *AssetLabelModel) AssetLabelModel {
	model := AssetLabelModel{
		ID:       types.StringValue(apiLabel.ID),
		Key:      types.StringValue(apiLabel.Key),
		Value:    types.StringValue(apiLabel.Value),
		ReadOnly: boolPointerToTerraform(apiLabel.ReadOnly),
	}
	if existing != nil {
		model = *existing
		if model.ID.IsNull() || model.ID.IsUnknown() || model.ID.ValueString() == "" {
			model.ID = types.StringValue(apiLabel.ID)
		}
		if model.Key.IsNull() || model.Key.IsUnknown() || model.Key.ValueString() == "" {
			model.Key = types.StringValue(apiLabel.Key)
		}
		if model.Value.IsNull() || model.Value.IsUnknown() || model.Value.ValueString() == "" {
			model.Value = types.StringValue(apiLabel.Value)
		}
		if model.ReadOnly.IsNull() || model.ReadOnly.IsUnknown() {
			model.ReadOnly = boolPointerToTerraform(apiLabel.ReadOnly)
		}
	}

	return model
}

func hasCompleteAssetLabelFields(label AssetLabelModel) bool {
	return !label.ID.IsNull() && !label.ID.IsUnknown() && label.ID.ValueString() != "" &&
		!label.Key.IsNull() && !label.Key.IsUnknown() && label.Key.ValueString() != "" &&
		!label.Value.IsNull() && !label.Value.IsUnknown() && label.Value.ValueString() != ""
}

func boolPointerToTerraform(value *bool) types.Bool {
	if value == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*value)
}

func isAssetReadOnlyLabel(label AssetLabelModel) bool {
	return !label.ReadOnly.IsNull() && !label.ReadOnly.IsUnknown() && label.ReadOnly.ValueBool()
}

// apiToModel converts the API struct to Terraform model.
// All Computed fields must be set to known values (not left unknown) after apply.
func (r *AssetResource) apiToModel(ctx context.Context, asset *client.Asset, data *AssetResourceModel, diags *diag.Diagnostics) {
	// Detect import: during ImportState, only ID is set in state — Name is null.
	isImport := data.Name.IsNull()

	data.ID = types.StringValue(asset.ID)
	data.Name = types.StringValue(asset.Name)
	data.Status = types.StringValue(asset.Status)

	// For fields that are API-populated or create-only, only set if API returns a non-empty value.
	// Otherwise preserve the existing plan value (null if user didn't set it).
	if asset.BiosUUID != "" {
		data.BiosUUID = types.StringValue(asset.BiosUUID)
	} else if data.BiosUUID.IsUnknown() {
		data.BiosUUID = types.StringNull()
	}
	orchObjID := asset.OrchestrationObjID
	if orchObjID == "" && len(asset.OrchestrationDetails) > 0 {
		orchObjID = asset.OrchestrationDetails[0].OrchestrationObjID
	}
	if orchObjID != "" {
		data.OrchestrationObjID = types.StringValue(orchObjID)
	}
	if asset.InstanceID != "" {
		data.InstanceID = types.StringValue(asset.InstanceID)
	} else if data.InstanceID.IsUnknown() {
		data.InstanceID = types.StringNull()
	}
	if asset.HwUUID != "" {
		data.HwUUID = types.StringValue(asset.HwUUID)
	} else if data.HwUUID.IsUnknown() {
		data.HwUUID = types.StringNull()
	}
	if asset.Comments != "" {
		data.Comments = types.StringValue(asset.Comments)
	} else if data.Comments.IsUnknown() {
		data.Comments = types.StringNull()
	}

	// Filter out NICs with empty ip_addresses before converting. The API may
	// return such NICs for agent-reported assets (e.g., vSphere). They cannot
	// be represented in the schema (ip_addresses requires SizeAtLeast(1)) and
	// would be rejected by the API on the next update.
	validAPInicsList := make([]client.AssetNIC, 0, len(asset.Nics))
	for _, nic := range asset.Nics {
		if len(nic.IPAddresses) > 0 {
			validAPInicsList = append(validAPInicsList, nic)
		} else {
			tflog.Warn(ctx, "dropping NIC with empty ip_addresses from state",
				map[string]any{"asset_id": asset.ID, "mac_address": nic.MacAddress, "vif_id": nic.VifID})
			diags.AddWarning(
				"NIC with Empty IP Addresses Excluded",
				fmt.Sprintf("Asset %q has a NIC (MAC: %s) with no IP addresses. "+
					"This NIC was excluded from Terraform state because the provider requires at least one IP address per NIC. "+
					"The NIC will not be managed by Terraform. If NICs are updated, the API replaces all NICs, "+
					"which will remove this unmanaged NIC from the server.",
					asset.ID, nic.MacAddress),
			)
		}
	}

	if len(validAPInicsList) == 0 && len(asset.Nics) > 0 {
		diags.AddWarning(
			"Asset Has No Valid NICs",
			fmt.Sprintf("Asset %q has %d NIC(s) but none have valid IP addresses. "+
				"This asset cannot be fully managed by Terraform until at least one NIC has a valid IP address.",
				asset.ID, len(asset.Nics)),
		)
	}

	nics := make([]NICModel, len(validAPInicsList))
	for i, nic := range validAPInicsList {
		ipList, d := types.ListValueFrom(ctx, types.StringType, nic.IPAddresses)
		diags.Append(d...)

		nics[i] = NICModel{
			VifID:       types.StringValue(nic.VifID),
			MacAddress:  types.StringValue(nic.MacAddress),
			NetworkID:   types.StringValue(nic.NetworkID),
			NetworkName: types.StringValue(nic.NetworkName),
			SwitchID:    types.StringValue(nic.SwitchID),
			IPAddresses: ipList,
		}

		if nic.IsCloudPublic != nil {
			nics[i].IsCloudPublic = types.BoolValue(*nic.IsCloudPublic)
		} else {
			nics[i].IsCloudPublic = types.BoolValue(false)
		}

		if nic.IsCorporateInterface != nil {
			nics[i].IsCorporateInterface = types.BoolValue(*nic.IsCorporateInterface)
		} else {
			nics[i].IsCorporateInterface = types.BoolValue(false)
		}
	}
	data.Nics = nics

	// Convert Labels — only track user-specified labels in state.
	// The server may auto-assign labels (e.g., "Agent: Not Installed"),
	// but reflecting them would cause "inconsistent result after apply"
	// errors because the plan didn't include them.
	//
	// The API list endpoint returns labels with only the ID populated
	// (key and value are empty strings). We match by ID (primary) and
	// fall back to key+value when the API does return those fields.
	// The user's planned key+value are preserved in state since the API
	// doesn't return them.
	if data.Labels != nil {
		// Build a set of label IDs confirmed by the API response.
		apiByID := make(map[string]bool, len(asset.Labels))
		for _, l := range asset.Labels {
			apiByID[l.ID] = true
		}

		// Iterate over configured labels, keeping only labels confirmed by the API.
		// labels is a set attribute, so ordering is intentionally not relied on.
		filtered := make([]AssetLabelModel, 0, len(data.Labels))
		for _, pl := range data.Labels {
			id := pl.ID.ValueString()
			if apiByID[id] {
				apiLabel := client.AssetLabelRef{ID: id}
				for _, l := range asset.Labels {
					if l.ID == id {
						apiLabel = l
						break
					}
				}
				filtered = append(filtered, assetLabelModelFromAPI(apiLabel, &pl))
			}
		}
		data.Labels = filtered
	} else if len(asset.Labels) > 0 && isImport {
		// Import scenario: data.Labels is nil and data.Name was null before
		// apiToModel (ImportState only sets the ID). Populate labels from the
		// API response so they appear in state, allowing Terraform to reconcile
		// with config. We do NOT populate labels during normal Read when the
		// user didn't configure labels — that would track server-assigned labels.
		//
		// Intentionally do NOT hydrate missing label details via GetLabel during
		// import. Import should stay aligned with importer output and include only
		// labels that are fully present in the asset API response.
		imported := make([]AssetLabelModel, 0, len(asset.Labels))
		for _, l := range asset.Labels {
			label := assetLabelModelFromAPI(l, nil)

			if r.isAssetIgnoredLabel(ctx, label, diags) {
				continue
			}

			if !hasCompleteAssetLabelFields(label) {
				diags.AddWarning(
					"Skipping Incomplete Imported Asset Label",
					fmt.Sprintf("Skipping imported asset label %q because key/value could not be resolved from the API. Add this label manually in configuration if it should be managed by Terraform.", label.ID.ValueString()),
				)
				continue
			}

			imported = append(imported, label)
		}
		if len(imported) > 0 {
			data.Labels = imported
		} else {
			data.Labels = nil
		}
	}
	// When data.Labels is nil (user didn't configure labels) and this is not
	// an import, keep nil. Server-assigned labels are not tracked.

	// Convert OrchestrationMetadata — only update if user explicitly set it in config,
	// except during import where we hydrate API values into state.
	if !data.OrchestrationMetadata.IsNull() && !data.OrchestrationMetadata.IsUnknown() {
		obj, d := assetOrchestrationMetadataObjectFromAPI(asset.OrchestrationMetadata)
		diags.Append(d...)
		if !d.HasError() {
			data.OrchestrationMetadata = obj
		}
	} else {
		data.OrchestrationMetadata = types.ObjectNull(assetOrchestrationMetadataAttrTypes())
	}

	// Convert worksite — from scoping_details.worksite.id
	if asset.ScopingDetails != nil && asset.ScopingDetails.Worksite != nil && asset.ScopingDetails.Worksite.ID != "" {
		data.WorksiteID = types.StringValue(asset.ScopingDetails.Worksite.ID)
	} else {
		data.WorksiteID = types.StringNull()
	}

	// Convert first_seen and last_seen — always set to known values
	if asset.FirstSeen != nil {
		data.FirstSeen = types.StringValue(fmt.Sprintf("%v", asset.FirstSeen))
	} else {
		data.FirstSeen = types.StringValue("")
	}
	if data.LastSeen.IsNull() || data.LastSeen.IsUnknown() || data.LastSeen.ValueString() == "" {
		if asset.LastSeen != nil {
			data.LastSeen = types.StringValue(fmt.Sprintf("%v", asset.LastSeen))
		} else {
			data.LastSeen = types.StringValue("")
		}
	}
}
