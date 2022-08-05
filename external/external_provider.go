package external_provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	provider "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud"
)

// Provider returns a *schema.Provider.
func Provider() *schema.Provider {
	return provider.Provider().(*schema.Provider)
}
