package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AssetDataSource{}

func NewAssetDataSource() datasource.DataSource {
	return &AssetDataSource{}
}

// AssetDataSource defines the data source implementation.
type AssetDataSource struct {
	client *client.Client
}

// AssetDataSourceModel describes the data source data model.
type AssetDataSourceModel struct {
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

func (d *AssetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (d *AssetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an asset from Akamai Guardicore Segmentation. You can look up an asset by its ID or by name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The unique identifier of the asset. Either `id` or `name` must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the asset. Used to look up an asset by name.",
			},
			"nics": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of network interfaces for the asset.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"vif_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Virtual interface ID.",
						},
						"mac_address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "MAC address.",
						},
						"network_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Network ID.",
						},
						"network_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Network name.",
						},
						"is_cloud_public": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is a cloud public network.",
						},
						"is_corporate_interface": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is a corporate network interface.",
						},
						"switch_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Switch ID.",
						},
						"ip_addresses": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "List of IP addresses for this NIC.",
						},
					},
				},
			},
			"orchestration_obj_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Primary key of the asset from the customer's database or playbook.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current asset status (`on`, `off`, or `deleted`).",
			},
			"labels": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of labels assigned to the asset.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The label ID.",
						},
						"key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The label key.",
						},
						"value": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The label value.",
						},
					},
				},
			},
			"comments": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Additional asset comments.",
			},
			"orchestration_metadata_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Orchestration metadata as a JSON string.",
			},
			"worksite_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The worksite ID assigned to this asset.",
			},
			"instance_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance ID generated by AWS/Azure/GCP.",
			},
			"hw_uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Hardware UUID.",
			},
			"bios_uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BIOS UUID.",
			},
			"first_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the asset was first seen.",
			},
			"last_seen": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the asset was last seen.",
			},
		},
	}
}

func (d *AssetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = providerData.Client
}

func (d *AssetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AssetDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var asset *client.Asset

	if !data.ID.IsNull() && data.ID.ValueString() != "" {
		// Look up by ID
		var err error
		asset, err = d.client.GetAsset(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read asset by ID, got error: %s", err))
			return
		}
		if asset == nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Asset with ID %s not found", data.ID.ValueString()))
			return
		}
	} else if !data.Name.IsNull() && data.Name.ValueString() != "" {
		// Look up by name
		assets, err := d.client.ListAssets(ctx, data.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list assets, got error: %s", err))
			return
		}

		// Filter for exact name match
		var matches []client.Asset
		for _, a := range assets {
			if a.Name == data.Name.ValueString() {
				matches = append(matches, a)
			}
		}

		if len(matches) == 0 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Asset with name %q not found", data.Name.ValueString()))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Multiple Results", fmt.Sprintf("Found %d assets matching name %q, expected exactly one", len(matches), data.Name.ValueString()))
			return
		}
		asset = &matches[0]
	} else {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either id or name must be specified to look up an asset.",
		)
		return
	}

	// Map API response to model
	d.apiToModel(ctx, asset, &data, &resp.Diagnostics)

	tflog.Trace(ctx, "read asset data source", map[string]any{"id": asset.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// apiToModel converts the API struct to the data source model.
func (d *AssetDataSource) apiToModel(ctx context.Context, asset *client.Asset, data *AssetDataSourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(asset.ID)
	data.Name = types.StringValue(asset.Name)
	data.Status = types.StringValue(asset.Status)

	// Convert NICs
	if asset.Nics != nil {
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
				nics[i].IsCloudPublic = types.BoolNull()
			}
		}
		data.Nics = nics
	}

	// Convert Labels
	if asset.Labels != nil {
		labels := make([]AssetLabelModel, len(asset.Labels))
		for i, l := range asset.Labels {
			labels[i] = AssetLabelModel{
				ID:    types.StringValue(l.ID),
				Key:   types.StringValue(l.Key),
				Value: types.StringValue(l.Value),
			}
		}
		data.Labels = labels
	}

	data.Comments = types.StringValue(asset.Comments)
	data.OrchestrationObjID = types.StringValue(asset.OrchestrationObjID)
	data.BiosUUID = types.StringValue(asset.BiosUUID)
	data.InstanceID = types.StringValue(asset.InstanceID)
	data.HwUUID = types.StringValue(asset.HwUUID)

	if len(asset.OrchestrationMetadata) > 0 && string(asset.OrchestrationMetadata) != "null" {
		// Compact JSON for consistent output
		var raw json.RawMessage
		if err := json.Unmarshal(asset.OrchestrationMetadata, &raw); err == nil {
			data.OrchestrationMetadataJSON = types.StringValue(string(raw))
		}
	} else {
		data.OrchestrationMetadataJSON = types.StringValue("")
	}

	// Convert worksite
	if asset.ScopingDetails != nil && asset.ScopingDetails.Worksite != nil && asset.ScopingDetails.Worksite.ID != "" {
		data.WorksiteID = types.StringValue(asset.ScopingDetails.Worksite.ID)
	} else {
		data.WorksiteID = types.StringValue("")
	}

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
