package tencentcloud

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccDataSourceTencentCloudSecurityGroups_basic(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccDataSourceTencentCloudSecurityGroupsConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.foo"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.#", "1"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.name", "ci-temp-security-groups-test"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.description", "ci-temp-security-groups-test"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.be_associate_count", "0"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.ingress.#", "0"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.egress.#", "0"),
				),
			},
		},
	})
}

func TestAccDataSourceTencentCloudSecurityGroups_searchByName(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccDataSourceTencentCloudSecurityGroupsConfigSearchByName,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.foo"),
					resource.TestMatchResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.#", regexp.MustCompile(`^[1-9]\d*$`)),
				),
			},
		},
	})
}

func TestAccDataSourceTencentCloudSecurityGroups_emptyResult(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccDataSourceTencentCloudSecurityGroupsConfigEmptyResult,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.foo"),
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.bar"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.#", "0"),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.bar", "security_groups.#", "0"),
				),
			},
		},
	})
}

func TestAccDataSourceTencentCloudSecurityGroups_tags(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccDataSourceTencentCloudSecurityGroupsTags,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.foo"),
					resource.TestMatchResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.tags.test", "test"),
				),
			},
		},
	})
}

func TestAccDataSourceTencentCloudSecurityGroups_searchByProjectId(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: TestAccDataSourceTencentCloudSecurityGroupsConfigSearchByProjectId,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTencentCloudDataSourceID("data.tencentcloud_security_groups.foo"),
					resource.TestMatchResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.#", regexp.MustCompile(`^[1-9]\d*$`)),
					resource.TestCheckResourceAttr("data.tencentcloud_security_groups.foo", "security_groups.0.project_id", "0"),
				),
			},
		},
	})
}

const TestAccDataSourceTencentCloudSecurityGroupsConfig = `
resource "tencentcloud_security_group" "foo" {
  name        = "ci-temp-security-groups-test"
  description = "ci-temp-security-groups-test"
}

data "tencentcloud_security_groups" "foo" {
  security_group_id = tencentcloud_security_group.foo.id
}
`

const TestAccDataSourceTencentCloudSecurityGroupsConfigSearchByName = `
resource "tencentcloud_security_group" "foo" {
  name        = "ci-temp-security-groups-test"
  description = "ci-temp-security-groups-test"
}

data "tencentcloud_security_groups" "foo" {
  name = tencentcloud_security_group.foo.name
}
`

const TestAccDataSourceTencentCloudSecurityGroupsConfigEmptyResult = `
data "tencentcloud_security_groups" "foo" {
  name = "lkagjlajtanvzvzmga"
}

data "tencentcloud_security_groups" "bar" {
  security_group_id = "sg-00000000"
}
`

const TestAccDataSourceTencentCloudSecurityGroupsTags = `
resource "tencentcloud_security_group" "foo" {
  name        = "ci-temp-security-groups-test"
  description = "ci-temp-security-groups-test"

  tags = {
    "test" = "test"
  }
}

data "tencentcloud_security_groups" "foo" {
  tags = tencentcloud_security_group.foo.tags
}
`

const TestAccDataSourceTencentCloudSecurityGroupsConfigSearchByProjectId = `
resource "tencentcloud_security_group" "foo" {
  name        = "ci-temp-security-groups-test"
  description = "ci-temp-security-groups-test"
  project_id  = 0
}

data "tencentcloud_security_groups" "foo" {
  project_id = tencentcloud_security_group.foo.project_id
}
`
