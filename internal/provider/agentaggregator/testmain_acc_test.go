package agentaggregator_test

import (
	"os"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/acctest"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestMain(m *testing.M) {
	acctest.SetProviderFactories(map[string]func() (tfprotov6.ProviderServer, error){
		"guardicore": providerserver.NewProtocol6WithError(provider.New("test")()),
	})
	acctest.InitTestConfig()
	os.Exit(m.Run())
}
