package provider

import (
	"context"
	"encoding/json"
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
	client *client.Client
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
	ID    types.String `tfsdk:"id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// AssetResourceModel describes the resource data model.
type AssetResourceModel struct {
	ID                        types.String      `tfsdk:"id"`
	Name                      types.String      `tfsdk:"name"`
	Nics                      []NICModel        `tfsdk:"nics"`
	OrchestrationObjID        types.String      `tfsdk:"orchestration_obj_id"`
	Status                    types.String      `tfsdk:"status"`
	Labels                    []AssetLabelModel `tfsdk:"labels"`
	Comments                  types.String      `tfsdk:"comments"`
	OrchestrationMetadataJSON types.String      `tfsdk:"orchestration_metadata_json"`
	WorksiteID                types.String      `tfsdk:"worksite_id"`
	InstanceID                types.String      `tfsdk:"instance_id"`
	HwUUID                    types.String      `tfsdk:"hw_uuid"`
	BiosUUID                  types.String      `tfsdk:"bios_uuid"`
	FirstSeen                 types.String      `tfsdk:"first_seen"`
	LastSeen                  types.String      `tfsdk:"last_seen"`
}

func (r *AssetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (r *AssetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an asset in Akamai Guardicore Segmentation. Assets represent network entities with network interfaces (NICs), labels, and orchestration metadata.\n\n" +
			"~> **Note:** The API DELETE operation deactivates the asset (sets status to `deleted`) rather than permanently removing it. " +
			"Deactivated assets may still appear in the Akamai Guardicore Segmentation management UI.\n\n" +
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
			"labels": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of labels assigned to the asset. Each label requires `id`, `key`, and `value` — the API needs all three. Reference a managed label via `guardicore_label.<name>.id`, `.key`, and `.value`. Use the `guardicore_label` data source for labels not managed by Terraform. Note: the server may automatically assign additional labels (e.g., \"Agent: Not Installed\"), but only user-specified labels are tracked in Terraform state.",
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
					},
				},
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Additional asset comments.",
			},
			"orchestration_metadata_json": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Orchestration metadata as a JSON string. Use `jsonencode()` to encode. " +
					"Known fields: `asset_type` (e.g., `F5`), `f5_device_hostname`, `partition`, `vs_name`.",
			},
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
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "BIOS UUID.",
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

	id, err := r.client.CreateAsset(ctx, apiReq)
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

	// Read back to populate server-computed fields
	created, err := r.client.GetAsset(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read asset after creation, got error: %s", err))
		return
	}
	if created != nil {
		r.apiToModel(ctx, created, &data, &resp.Diagnostics)
	}

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

	err := r.client.UpdateAsset(ctx, data.ID.ValueString(), apiReq)
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

	err := r.client.BulkDeactivateAssets(ctx, []string{data.ID.ValueString()})
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
	labels := r.modelLabelsToAPI(data.Labels)

	result := &client.AssetCreate{
		Name:               data.Name.ValueString(),
		Nics:               nics,
		OrchestrationObjID: data.OrchestrationObjID.ValueString(),
		Labels:             labels,
	}

	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		result.Status = data.Status.ValueString()
	}
	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		result.Comments = data.Comments.ValueString()
	}
	if !data.OrchestrationMetadataJSON.IsNull() && !data.OrchestrationMetadataJSON.IsUnknown() {
		result.OrchestrationMetadata = json.RawMessage(data.OrchestrationMetadataJSON.ValueString())
	}
	if !data.InstanceID.IsNull() && !data.InstanceID.IsUnknown() {
		result.InstanceID = data.InstanceID.ValueString()
	}
	if !data.HwUUID.IsNull() && !data.HwUUID.IsUnknown() {
		result.HwUUID = data.HwUUID.ValueString()
	}
	if !data.BiosUUID.IsNull() && !data.BiosUUID.IsUnknown() {
		result.BiosUUID = data.BiosUUID.ValueString()
	}
	return result
}

// modelToUpdateAPI converts the Terraform model to API struct for update.
// NOTE: orchestration_obj_id, instance_id, hw_uuid, bios_uuid are NOT in the edit body.
func (r *AssetResource) modelToUpdateAPI(ctx context.Context, data *AssetResourceModel, diags *diag.Diagnostics) *client.AssetUpdate {
	nics := r.modelNicsToAPI(ctx, data.Nics, diags)
	labels := r.modelLabelsToAPI(data.Labels)

	result := &client.AssetUpdate{
		Name:   data.Name.ValueString(),
		Nics:   nics,
		Labels: labels,
	}

	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		result.Status = data.Status.ValueString()
	}
	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		result.Comments = data.Comments.ValueString()
	}
	if !data.OrchestrationMetadataJSON.IsNull() && !data.OrchestrationMetadataJSON.IsUnknown() {
		result.OrchestrationMetadata = json.RawMessage(data.OrchestrationMetadataJSON.ValueString())
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
			apiNic.IsCorporateInterface = nic.IsCorporateInterface.ValueBool()
		}
		if !nic.SwitchID.IsNull() && !nic.SwitchID.IsUnknown() {
			apiNic.SwitchID = nic.SwitchID.ValueString()
		}

		apiNics[i] = apiNic
	}
	return apiNics
}

// modelLabelsToAPI converts label models to API structs.
func (r *AssetResource) modelLabelsToAPI(labels []AssetLabelModel) []client.AssetLabelRef {
	if labels == nil {
		return nil
	}
	apiLabels := make([]client.AssetLabelRef, len(labels))
	for i, l := range labels {
		apiLabels[i] = client.AssetLabelRef{
			ID:    l.ID.ValueString(),
			Key:   l.Key.ValueString(),
			Value: l.Value.ValueString(),
		}
	}
	return apiLabels
}

// apiToModel converts the API struct to Terraform model.
// All Computed fields must be set to known values (not left unknown) after apply.
func (r *AssetResource) apiToModel(ctx context.Context, asset *client.Asset, data *AssetResourceModel, diags *diag.Diagnostics) {
	// Detect import: during ImportState, only ID is set in state — Name is null.
	isImport := data.Name.IsNull()

	data.ID = types.StringValue(asset.ID)
	data.Name = types.StringValue(asset.Name)
	data.Status = types.StringValue(asset.Status)

	// For optional create-only fields, only set if API returns a non-empty value.
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

	// Convert NICs — always set to a known value
	nics := make([]NICModel, len(asset.Nics))
	for i, nic := range asset.Nics {
		ipList, d := types.ListValueFrom(ctx, types.StringType, nic.IPAddresses)
		diags.Append(d...)

		nics[i] = NICModel{
			VifID:                types.StringValue(nic.VifID),
			MacAddress:           types.StringValue(nic.MacAddress),
			NetworkID:            types.StringValue(nic.NetworkID),
			NetworkName:          types.StringValue(nic.NetworkName),
			IsCorporateInterface: types.BoolValue(nic.IsCorporateInterface),
			SwitchID:             types.StringValue(nic.SwitchID),
			IPAddresses:          ipList,
		}

		if nic.IsCloudPublic != nil {
			nics[i].IsCloudPublic = types.BoolValue(*nic.IsCloudPublic)
		} else {
			nics[i].IsCloudPublic = types.BoolValue(false)
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

		// Iterate in plan order, keeping only labels confirmed by the API.
		// This preserves the user's declared ordering (the API may return
		// labels in a different order, e.g., by creation timestamp).
		var filtered []AssetLabelModel
		for _, pl := range data.Labels {
			if apiByID[pl.ID.ValueString()] {
				filtered = append(filtered, pl)
			}
		}
		if filtered != nil {
			data.Labels = filtered
		} else {
			data.Labels = []AssetLabelModel{}
		}
	} else if len(asset.Labels) > 0 && isImport {
		// Import scenario: data.Labels is nil and data.Name was null before
		// apiToModel (ImportState only sets the ID). Populate labels from the
		// API response so they appear in state, allowing Terraform to reconcile
		// with config. We do NOT populate labels during normal Read when the
		// user didn't configure labels — that would track server-assigned labels.
		imported := make([]AssetLabelModel, len(asset.Labels))
		for i, l := range asset.Labels {
			imported[i] = AssetLabelModel{
				ID:    types.StringValue(l.ID),
				Key:   types.StringValue(l.Key),
				Value: types.StringValue(l.Value),
			}
		}
		data.Labels = imported
	}
	// When data.Labels is nil (user didn't configure labels) and this is not
	// an import, keep nil. Server-assigned labels are not tracked.

	// Convert OrchestrationMetadata — only update if user explicitly set it in config.
	// The API may populate this server-side, but we should not reflect that back
	// if the user didn't configure it (it would cause inconsistent result errors).
	if !data.OrchestrationMetadataJSON.IsNull() && !data.OrchestrationMetadataJSON.IsUnknown() {
		// User explicitly set it — update from API response
		metaStr := string(asset.OrchestrationMetadata)
		if len(asset.OrchestrationMetadata) > 0 && metaStr != "null" {
			data.OrchestrationMetadataJSON = types.StringValue(metaStr)
		}
	} else {
		// User didn't set it — keep null
		data.OrchestrationMetadataJSON = types.StringNull()
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
	if asset.LastSeen != nil {
		data.LastSeen = types.StringValue(fmt.Sprintf("%v", asset.LastSeen))
	} else {
		data.LastSeen = types.StringValue("")
	}
}
