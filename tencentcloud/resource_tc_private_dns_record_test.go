package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccTencentCloudPrivateDnsRecord_basic(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccPrivateDnsRecord_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("tencentcloud_private_dns_record.record", "weight", "1"),
				),
			},
			{
				ResourceName:      "tencentcloud_private_dns_record.record",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testAccPrivateDnsRecord_basic = defaultInstanceVariable + `
resource "tencentcloud_private_dns_zone" "zone" {
  dns_forward_status = "DISABLED"
  domain             = "domain.com"
  remark             = "test_record"
  tags = {
    "created-by" : "terraform",
  }
}

resource "tencentcloud_private_dns_record" "record" {
  mx           = 0
  record_type  = "A"
  record_value = "192.168.1.2"
  sub_domain   = "www"
  ttl          = 300
  weight       = 1
  zone_id      = tencentcloud_private_dns_zone.zone.id
}
`
