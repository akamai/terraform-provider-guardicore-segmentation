package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DnsSecurityResource{}
var _ resource.ResourceWithImportState = &DnsSecurityResource{}

func NewDnsSecurityResource() resource.Resource {
	return &DnsSecurityResource{}
}

// DnsSecurityResource defines the resource implementation.
type DnsSecurityResource struct {
	client        *client.Client
	createBatcher *Batcher[*client.DnsBlocklistCreate, string]
	updateBatcher *Batcher[dnsSecurityUpdateReq, struct{}]
	deleteBatcher *Batcher[string, struct{}]
}

// DnsSecurityResourceModel describes the resource data model.
type DnsSecurityResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Domains types.Set    `tfsdk:"domains"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *DnsSecurityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_security"
}

func (r *DnsSecurityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS security blocklist in Akamai Guardicore Segmentation. DNS blocklists allow you to block or exclude specific domains from DNS resolution.\n\n" +
			"~> **Note:** The DNS Security feature must be enabled on the Akamai Guardicore Segmentation instance. If you receive a \"DNS Security is not enabled\" error, enable it in the Akamai Guardicore Segmentation management console before using this resource.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the DNS blocklist.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the DNS blocklist.",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of the DNS blocklist. Valid values: `AKAMAI_INTELLIGENCE`, `CUSTOM_BLOCK`, `CUSTOM_EXCLUSION`, `WEB_CATEGORY`, `CUSTOM_BLOCKLIST`, `EXCLUSION_LIST`. `AKAMAI_INTELLIGENCE` and `WEB_CATEGORY` are system-managed and cannot be used when creating this resource (use the `guardicore_dns_security` data source to reference them). Changing this requires resource replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"AKAMAI_INTELLIGENCE",
						"CUSTOM_BLOCK",
						"CUSTOM_EXCLUSION",
						"WEB_CATEGORY",
						"CUSTOM_BLOCKLIST",
						"EXCLUSION_LIST",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domains": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The set of domains in the blocklist.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the DNS blocklist is enabled. Defaults to `true`.",
			},
		},
	}
}

func (r *DnsSecurityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.createBatcher = providerData.DnsSecurityCreateBatcher
	r.updateBatcher = providerData.DnsSecurityUpdateBatcher
	r.deleteBatcher = providerData.DnsSecurityDeleteBatcher
}

func (r *DnsSecurityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DnsSecurityResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	blocklist := r.modelToAPI(ctx, &data)

	if DnsBlocklistTypeIsSystemManaged(blocklist.Type) {
		resp.Diagnostics.AddError(
			"System-Managed DNS Blocklist Type",
			fmt.Sprintf(
				"DNS blocklist type %q is system-managed and cannot be used when creating guardicore_dns_security resources. Use data \"guardicore_dns_security\" to reference existing system-managed blocklists.",
				blocklist.Type,
			),
		)
		return
	}

	id, err := r.createBatcher.Enqueue(ctx, blocklist)
	if err != nil {
		if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
			resp.Diagnostics.AddError("DNS Security Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create DNS blocklist, got error: %s", err))
		}
		return
	}

	data.ID = types.StringValue(id)

	// Apply enabled via PATCH since the create endpoint doesn't support it
	if !data.Enabled.IsNull() && !data.Enabled.ValueBool() {
		enabled := data.Enabled.ValueBool()
		edit := &client.DnsBlocklistEdit{Enabled: &enabled}
		err = r.client.UpdateDnsBlocklist(ctx, id, edit)
		if err != nil {
			if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
				resp.Diagnostics.AddError("DNS Security Feature Disabled", err.Error())
			} else {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update DNS blocklist after creation, got error: %s", err))
			}
			return
		}
	}

	created, err := waitForReadAfterCreate(ctx, "dns blocklist", func(ctx context.Context) (*client.DnsBlocklist, error) {
		return r.client.GetDnsBlocklist(ctx, id)
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read DNS blocklist after creation, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(r.apiToModel(created, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created DNS blocklist", map[string]interface{}{"id": id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsSecurityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DnsSecurityResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	blocklist, err := r.client.GetDnsBlocklist(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
			resp.Diagnostics.AddError("DNS Security Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read DNS blocklist, got error: %s", err))
		}
		return
	}

	if blocklist == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.apiToModel(blocklist, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsSecurityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DnsSecurityResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	enabled := data.Enabled.ValueBool()

	edit := &client.DnsBlocklistEdit{
		Name:    &name,
		Enabled: &enabled,
	}

	// Handle domains
	if !data.Domains.IsNull() {
		var domains []string
		diags := data.Domains.ElementsAs(ctx, &domains, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		edit.Domains = domains
	} else {
		edit.Domains = []string{}
	}

	_, err := r.updateBatcher.Enqueue(ctx, dnsSecurityUpdateReq{id: data.ID.ValueString(), edit: edit})
	if err != nil {
		if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
			resp.Diagnostics.AddError("DNS Security Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update DNS blocklist, got error: %s", err))
		}
		return
	}

	tflog.Trace(ctx, "updated DNS blocklist", map[string]interface{}{"id": data.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsSecurityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DnsSecurityResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.deleteBatcher.Enqueue(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrDnsSecurityFeatureDisabled) {
			resp.Diagnostics.AddError("DNS Security Feature Disabled", err.Error())
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete DNS blocklist, got error: %s", err))
		}
		return
	}

	tflog.Trace(ctx, "deleted DNS blocklist", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *DnsSecurityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelToAPI converts the Terraform model to API struct for create.
func (r *DnsSecurityResource) modelToAPI(ctx context.Context, data *DnsSecurityResourceModel) *client.DnsBlocklistCreate {
	blocklist := &client.DnsBlocklistCreate{
		Name: data.Name.ValueString(),
		Type: data.Type.ValueString(),
	}

	if !data.Domains.IsNull() {
		var domains []string
		data.Domains.ElementsAs(ctx, &domains, false)
		blocklist.Domains = domains
	} else {
		blocklist.Domains = []string{}
	}

	return blocklist
}

// apiToModel converts the API struct to Terraform model.
func (r *DnsSecurityResource) apiToModel(blocklist *client.DnsBlocklist, data *DnsSecurityResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(blocklist.ID)
	data.Name = types.StringValue(blocklist.Name)
	data.Type = types.StringValue(blocklist.Type)
	data.Enabled = types.BoolValue(blocklist.Enabled)

	setValue, setDiags := dnsDomainsSetValue(blocklist.Domains)
	diags.Append(setDiags...)
	data.Domains = setValue

	return diags
}
