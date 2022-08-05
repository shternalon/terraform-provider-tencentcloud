package tencentcloud

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccTencentCloudDataSourceImagesBase(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccTencentCloudDataSourceImagesBase,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_images.foo"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_images.foo", "images.#"),
				),
			},
			{
				Config: testAccTencentCloudDataSourceImagesBaseWithFilter,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_images.foo"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_images.foo", "images.#"),
				),
			},
			{
				Config: testAccTencentCloudDataSourceImagesBaseWithOsName,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_images.foo"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_images.foo", "images.#"),
				),
			},
			{
				Config: testAccTencentCloudDataSourceImagesBaseWithImageNameRegex,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_images.foo"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_images.foo", "images.#"),
				),
			},
			{
				Config: testAccTencentCloudDataSourceImagesBaseWithInstanceType,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_images.foo"),
					resource.TestCheckResourceAttrSet("data.tencentcloud_images.foo", "images.#"),
				),
			},
		},
	})
}

const testAccTencentCloudDataSourceImagesBase = `
data "tencentcloud_images" "foo" {
	result_output_file = "data_source_tc_images_test.txt"
}
`

const testAccTencentCloudDataSourceImagesBaseWithFilter = `
data "tencentcloud_images" "foo" {
	image_type = ["PRIVATE_IMAGE"]
}
`

const testAccTencentCloudDataSourceImagesBaseWithOsName = `
data "tencentcloud_images" "foo" {
  image_type = ["PUBLIC_IMAGE"]
  os_name = "CentOS 7.5"
}
`

const testAccTencentCloudDataSourceImagesBaseWithImageNameRegex = `
data "tencentcloud_images" "foo" {
  image_type = ["PUBLIC_IMAGE"]
  image_name_regex = "^CentOS\\s+7\\.5\\s+64\\w*"
}
`

const testAccTencentCloudDataSourceImagesBaseWithInstanceType = `
data "tencentcloud_images" "foo" {
  instance_type = "S1.SMALL1"
}
`
