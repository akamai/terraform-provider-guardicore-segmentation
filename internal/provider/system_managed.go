package provider

import (
	"fmt"
	"strings"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// LabelIsSystemManaged determines if a label is system-managed based on API fields.
func LabelIsSystemManaged(label *client.Label) (systemManaged bool, managedBy string) {
	if label.ReadOnly != nil && *label.ReadOnly {
		managedBy = "system"
		if label.Origin != nil && *label.Origin != "" {
			managedBy = *label.Origin
		}
		return true, managedBy
	}
	return false, "terraform"
}

// UserGroupIsSystemManaged determines if a user group is system-managed.
// A user group is system-managed when all orchestration IDs are "local" or when
// no orchestration groups exist.
func UserGroupIsSystemManaged(ug *client.UserGroup) (systemManaged bool, managedBy string) {
	if len(ug.GroupsByDomainName) > 0 {
		for _, domainInfo := range ug.GroupsByDomainName {
			if domainInfo.OrchestrationID != "local" {
				return false, "terraform"
			}
		}
		return true, "system"
	}

	if len(ug.OrchestrationsGroups) > 0 {
		for _, og := range ug.OrchestrationsGroups {
			if og.OrchestrationID != "local" {
				return false, "terraform"
			}
		}
		return true, "system"
	}

	return true, "system"
}

// WorksiteIsSystemManaged determines if a worksite is system-managed.
// The "Default" worksite is created by the platform and cannot be modified.
func WorksiteIsSystemManaged(ws *client.Worksite) (systemManaged bool, managedBy string) {
	if ws.Name == "Default" {
		return true, "system"
	}
	return false, "terraform"
}

// DnsBlocklistTypeIsSystemManaged determines if a DNS blocklist type is
// system-managed and therefore not creatable via Terraform resource blocks.
func DnsBlocklistTypeIsSystemManaged(listType string) bool {
	switch strings.ToUpper(listType) {
	case "AKAMAI_INTELLIGENCE", "WEB_CATEGORY":
		return true
	default:
		return false
	}
}

// DiagnoseSystemManagedMutation appends an error diagnostic when a system-managed
// resource is being updated or deleted. The resourceType should be the Terraform
// resource suffix (e.g., "label", "user_group"). The verb is the action being
// attempted (e.g., "update", "delete").
func DiagnoseSystemManagedMutation(systemManaged types.Bool, resourceType, resourceName, verb string, diags *diag.Diagnostics) {
	if !systemManaged.ValueBool() {
		return
	}
	diags.AddError(
		fmt.Sprintf("System-Managed Resource Cannot Be %sd", verb),
		fmt.Sprintf(
			"The %s %q is system-managed and cannot be %sd by Terraform. "+
				"To reference it, use the data source instead:\n\n"+
				"  data \"guardicore_%s\" \"example\" {\n"+
				"    id = %q\n"+
				"  }\n\n"+
				"Then reference it as data.guardicore_%s.example.id in your configuration.",
			resourceType, resourceName, verb,
			resourceType, resourceName, resourceType,
		),
	)
}
