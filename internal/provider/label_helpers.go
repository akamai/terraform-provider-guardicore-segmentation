package provider

import "github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"

func labelIsReadOnly(label *client.Label) bool {
	return label != nil && label.ReadOnly != nil && *label.ReadOnly
}
