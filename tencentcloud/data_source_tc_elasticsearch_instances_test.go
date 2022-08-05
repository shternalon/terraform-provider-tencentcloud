package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccTencentCloudElasticsearchInstancesDataSource(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckElasticsearchInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccElasticsearchInstancesDataSource,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.tencentcloud_elasticsearch_instances.foo", "instance_list.#", "1"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.instance_id"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.instance_name"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.availability_zone"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.vpc_id"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.subnet_id"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.version"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.license_type"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.charge_type"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.tags.test"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.node_info_list.#"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.node_info_list.0.node_num"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.node_info_list.0.node_type"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.node_info_list.0.encrypt"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_elasticsearch_instances.foo", "instance_list.0.create_time"),
				),
			},
		},
	})
}

const testAccElasticsearchInstancesDataSource = defaultVpcVariable + `
  
data "tencentcloud_elasticsearch_instances" "foo" {
	instance_name = "keep"
}
`
