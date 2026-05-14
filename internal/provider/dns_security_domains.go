package provider

import (
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func dnsDomainsSetValue(domains []string) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(domains) == 0 {
		return types.SetNull(types.StringType), diags
	}

	normalized := normalizeDNSDomains(domains)
	values := make([]attr.Value, 0, len(normalized))
	for _, domain := range normalized {
		values = append(values, types.StringValue(domain))
	}

	setValue, setDiags := types.SetValue(types.StringType, values)
	diags.Append(setDiags...)

	return setValue, diags
}

func normalizeDNSDomains(domains []string) []string {
	unique := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		unique[domain] = struct{}{}
	}

	normalized := make([]string, 0, len(unique))
	for domain := range unique {
		normalized = append(normalized, domain)
	}

	sort.Strings(normalized)

	return normalized
}
