package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

var testDataSqlserverBasicInstancesName = "data.tencentcloud_sqlserver_basic_instances.id_test"

func TestAccDataSourceTencentCloudSqlserverBasicInstances(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSqlserverBasicInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTencentCloudDataSqlserverBasicInstancesBasic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(testDataSqlserverBasicInstancesName, "name"),
					resource.TestCheckResourceAttrSet(testDataSqlserverBasicInstancesName, "instance_list.#"),
				),
			},
		},
	})
}

const testAccTencentCloudDataSqlserverBasicInstancesBasic = testAccSqlserverAZ + `
data "tencentcloud_sqlserver_basic_instances" "id_test"{
	name = "keep"
}
`
