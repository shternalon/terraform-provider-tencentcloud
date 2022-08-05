package tencentcloud

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

var testTkeClusterAsName = "tencentcloud_kubernetes_as_scaling_group"
var testTkeClusterAsResourceKey = testTkeClusterAsName + ".as_test"

// @Deprecated
func testAccTencentCloudTkeAsResource(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckTkeAsDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTkeAsCluster,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTkeAsExists,
					resource.TestCheckResourceAttrSet(testTkeClusterAsResourceKey, "cluster_id"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_group.#", "1"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_config.#", "1"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "labels.test1", "test1"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "labels.test2", "test2"),
				),
			},
			{
				Config: testAccTkeAsClusterUpdate,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTkeAsExists,
					resource.TestCheckResourceAttrSet(testTkeClusterAsResourceKey, "cluster_id"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_group.#", "1"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_config.#", "1"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_group.0.max_size", "6"),
					resource.TestCheckResourceAttr(testTkeClusterAsResourceKey, "auto_scaling_group.0.min_size", "1"),
				),
			},
		},
	})
}

func testAccCheckTkeAsDestroy(s *terraform.State) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	service := AsService{
		client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn,
	}

	rs, ok := s.RootModule().Resources[testTkeClusterAsResourceKey]
	if !ok {
		return fmt.Errorf("tke as group %s is not found", testTkeClusterAsResourceKey)
	}
	if rs.Primary.ID == "" {
		return fmt.Errorf("tke  as group  id is not set")
	}
	items := strings.Split(rs.Primary.ID, ":")
	if len(items) != 2 {
		return fmt.Errorf("resource_tc_kubernetes_as_scaling_group id %s is broken", rs.Primary.ID)
	}
	asGroupId := items[1]

	err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		_, has, err := service.DescribeAutoScalingGroupById(ctx, asGroupId)

		if err != nil {
			return retryError(err)
		}
		if has == 0 {
			return nil
		}

		return resource.RetryableError(fmt.Errorf("tke as group %s still exist", asGroupId))
	})

	if err != nil {
		return err
	}
	return nil
}

func testAccCheckTkeAsExists(s *terraform.State) error {

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	service := AsService{
		client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn,
	}

	rs, ok := s.RootModule().Resources[testTkeClusterAsResourceKey]
	if !ok {
		return fmt.Errorf("tke as group %s is not found", testTkeClusterAsResourceKey)
	}
	if rs.Primary.ID == "" {
		return fmt.Errorf("tke  as group  id is not set")
	}

	items := strings.Split(rs.Primary.ID, ":")
	if len(items) != 2 {
		return fmt.Errorf("resource_tc_kubernetes_as_scaling_group id  %s is broken", rs.Primary.ID)
	}
	asGroupId := items[1]

	_, has, err := service.DescribeAutoScalingGroupById(ctx, asGroupId)
	if err != nil {
		return err
	}
	if has == 1 {
		return nil
	}
	return fmt.Errorf("tke as group %s query fail.", asGroupId)
}

const TkeAsBasic = TkeDataSource + TkeExclusiveNetwork + TkeInstanceType

const testAccTkeAsCluster = TkeAsBasic + `
resource "tencentcloud_kubernetes_as_scaling_group" "as_test" {

  cluster_id = local.cluster_id

  auto_scaling_group {
    scaling_group_name   = "tf-tke-as-group-unit-test"
    max_size             = "5"
    min_size             = "0"
    vpc_id               = local.vpc_id
    subnet_ids           = [local.subnet_id]
    project_id           = 0
    default_cooldown     = 400
    desired_capacity     = "1"
    termination_policies = ["NEWEST_INSTANCE"]
    retry_policy         = "INCREMENTAL_INTERVALS"

    tags = {
      "test" = "test"
    }

  }

  auto_scaling_config {
    configuration_name = "tf-tke-as-config-unit-test"
    instance_type      = local.type1
    project_id         = 0
    system_disk_type   = "CLOUD_PREMIUM"
    system_disk_size   = "50"

    data_disk {
      disk_type = "CLOUD_PREMIUM"
      disk_size = 50
    }

    internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
    internet_max_bandwidth_out = 10
    public_ip_assigned         = true
    password                   = "test123#"
    enhanced_security_service  = false
    enhanced_monitor_service   = false

    instance_tags = {
      tag = "as"
    }

  }
  unschedulable = 0
  labels = {
    "test1" = "test1",
    "test2" = "test2",
  }
  extra_args = [
 	"root-dir=/var/lib/kubelet"
  ]
}

`

const testAccTkeAsClusterUpdate = TkeAsBasic + `
resource "tencentcloud_kubernetes_as_scaling_group" "as_test" {

  cluster_id = local.cluster_id

  auto_scaling_group {
    scaling_group_name   = "tf-tke-as-group-unit-test"
    max_size             = "6"
    min_size             = "1"
    vpc_id               = local.vpc_id
    subnet_ids           = [local.subnet_id]
    project_id           = 0
    default_cooldown     = 400
    desired_capacity     = "1"
    termination_policies = ["NEWEST_INSTANCE"]
    retry_policy         = "INCREMENTAL_INTERVALS"

    tags = {
      "test" = "test"
    }

  }

  auto_scaling_config {
    configuration_name = "tf-tke-as-config-unit-test"
    instance_type      = local.type1
    project_id         = 0
    system_disk_type   = "CLOUD_PREMIUM"
    system_disk_size   = "50"

    data_disk {
      disk_type = "CLOUD_PREMIUM"
      disk_size = 50
    }

    internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
    internet_max_bandwidth_out = 10
    public_ip_assigned         = true
    password                   = "test123#"
    enhanced_security_service  = false
    enhanced_monitor_service   = false

    instance_tags = {
      tag = "as"
    }

  }
  unschedulable = 1
  labels = {
    "test1" = "test1",
    "test2" = "test2",
  }
  extra_args = [
 	"root-dir=/var/lib/kubelet"
  ]
}
`
