package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccTencentCloudSqlserverPublishSubscribeDataSource(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSqlserverPublishSubscribeDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTencentCloudSqlServerPublishSubscribeDataSourceConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.tencentcloud_sqlserver_publish_subscribes.publish_subscribes", "publish_subscribe_list.#"),
				),
			},
		},
	})
}

const testAccTencentCloudSqlServerPublishSubscribeDataSourceConfig = CommonPresetSQLServer + `

data "tencentcloud_sqlserver_publish_subscribes" "publish_subscribes" {
	instance_id                     = local.sqlserver_id
}
`
