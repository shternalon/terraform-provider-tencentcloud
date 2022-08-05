package tencentcloud

import (
	"context"
	"fmt"
	"testing"

	cdb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdb/v20170320"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

const TestAccTencentCloudMysqlMasterInstance_availability_zone = "ap-guangzhou-3"
const TestAccTencentCloudMysqlInstanceName = "testAccMysql"

func init() {
	// go test -v ./tencentcloud -sweep=ap-guangzhou -sweep-run=tencentcloud_mysql_instance
	resource.AddTestSweepers("tencentcloud_mysql_instance", &resource.Sweeper{
		Name: "tencentcloud_mysql_instance",
		F:    testSweepMySQLInstance,
	})
}

func testSweepMySQLInstance(region string) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)
	cli, err := sharedClientForRegion(region)
	if err != nil {
		return err
	}
	client := cli.(*TencentCloudClient).apiV3Conn
	service := MysqlService{client: client}

	request := cdb.NewDescribeDBInstancesRequest()
	request.Limit = helper.IntUint64(2000)

	response, err := client.UseMysqlClient().DescribeDBInstances(request)
	if err != nil {
		return err
	}

	if len(response.Response.Items) == 0 {
		return nil
	}

	for _, v := range response.Response.Items {
		id := *v.InstanceId
		name := *v.InstanceName
		if isResourcePersist(name, nil) {
			continue
		}
		err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			_, err := service.IsolateDBInstance(ctx, id)
			if err != nil {
				//for the pay order wait
				return retryError(err, InternalError)
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = resource.Retry(7*readRetryTimeout, func() *resource.RetryError {
			mysqlInfo, err := service.DescribeDBInstanceById(ctx, id)

			if err != nil {
				if _, ok := err.(*errors.TencentCloudSDKError); !ok {
					return resource.RetryableError(err)
				} else {
					return resource.NonRetryableError(err)
				}
			}
			if mysqlInfo == nil {
				return nil
			}
			if *mysqlInfo.Status == MYSQL_STATUS_ISOLATING || *mysqlInfo.Status == MYSQL_STATUS_RUNNING {
				return resource.RetryableError(fmt.Errorf("mysql isolating."))
			}
			if *mysqlInfo.Status == MYSQL_STATUS_ISOLATED {
				return nil
			}
			return resource.NonRetryableError(fmt.Errorf("after IsolateDBInstance mysql Status is %d", *mysqlInfo.Status))
		})

		err = service.OfflineIsolatedInstances(ctx, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestAccTencentCloudMysqlDeviceType(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckMysqlMasterInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMySQLDeviceType,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_exclusive"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_exclusive", "device_type", "EXCLUSIVE"),
				),
			},
			{
				ResourceName:            "tencentcloud_mysql_instance.mysql_exclusive",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"root_password", "prepaid_period", "first_slave_zone", "force_delete", "param_template_id", "fast_upgrade"},
			},
			{
				Config: testAccMySQLDeviceTypeUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_exclusive"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_exclusive", "device_type", "EXCLUSIVE"),
				),
			},
		},
	})
}

func TestAccTencentCloudMysqlMasterInstance_fullslave(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckMysqlMasterInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMysqlMasterInstance_fullslave(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "instance_name", TestAccTencentCloudMysqlInstanceName),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "slave_deploy_mode", "0"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "slave_sync_mode", "2"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "availability_zone", TestAccTencentCloudMysqlMasterInstance_availability_zone),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "first_slave_zone", TestAccTencentCloudMysqlMasterInstance_availability_zone),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "second_slave_zone", TestAccTencentCloudMysqlMasterInstance_availability_zone),
				),
			},
		},
	})
}

func TestAccTencentCloudMysqlMasterInstance_basic_and_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckMysqlMasterInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMysqlMasterInstance_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "instance_name", TestAccTencentCloudMysqlInstanceName),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "mem_size", "1000"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "volume_size", "50"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "intranet_port", "3360"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "engine_version", "5.7"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "internet_service", "0"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "slave_deploy_mode", "0"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "slave_sync_mode", "0"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "project_id", "0"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "availability_zone", TestAccTencentCloudMysqlMasterInstance_availability_zone),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "first_slave_zone", TestAccTencentCloudMysqlMasterInstance_availability_zone),

					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "intranet_ip"),
					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "status"),
					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "task_status"),
					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "gtid"),
				),
			},
			{
				ResourceName:            "tencentcloud_mysql_instance.mysql_master",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"root_password", "prepaid_period", "first_slave_zone", "force_delete"},
			},
			// add tag
			{
				Config: testAccMysqlMasterInstance_multiTags("master"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "tags.role", "master"),
				),
			},
			// update tag
			{
				Config: testAccMysqlMasterInstance_multiTags("master-version2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "tags.role", "master-version2"),
				),
			},
			// remove tag
			{
				Config: testAccMysqlMasterInstance_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckNoResourceAttr("tencentcloud_mysql_instance.mysql_master", "tags.role"),
				),
			},

			// open internet service
			{
				Config: testAccMysqlMasterInstance_internet_service(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "internet_service", "1"),
					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "internet_host"),
					resource.TestCheckResourceAttrSet("tencentcloud_mysql_instance.mysql_master", "internet_port"),
				),
			},

			//close internet  service
			{
				Config: testAccMysqlMasterInstance_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "internet_service", "0")),
			},

			//modify  parameters
			{
				Config: testAccMysqlMasterInstance_parameters(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "parameters.max_connections", "1000")),
			},
			//remove parameters and  restore
			{
				Config: testAccMysqlMasterInstance_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master")),
			},
			// update instance_name
			{
				Config: testAccMysqlMasterInstance_update("testAccMysql-version1", "3360"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "instance_name", "testAccMysql-version1"),
				),
			},
			// update intranet_port
			{
				Config: testAccMysqlMasterInstance_update("testAccMysql-version1", "3361"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMysqlMasterInstanceExists("tencentcloud_mysql_instance.mysql_master"),
					resource.TestCheckResourceAttr("tencentcloud_mysql_instance.mysql_master", "intranet_port", "3361"),
				),
			},
		},
	})
}

func testAccCheckMysqlMasterInstanceDestroy(s *terraform.State) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)
	mysqlService := MysqlService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "tencentcloud_mysql_instance" {
			continue
		}
		instance, err := mysqlService.DescribeRunningDBInstanceById(ctx, rs.Primary.ID)
		if instance != nil {
			return fmt.Errorf("mysql instance still exist")
		}
		if err != nil {
			sdkErr, ok := err.(*errors.TencentCloudSDKError)
			if ok && sdkErr.Code == MysqlInstanceIdNotFound {
				continue
			}
			return err
		}
	}
	return nil
}

func testAccCheckMysqlMasterInstanceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("mysql instance %s is not found", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("mysql instance id is not set")
		}

		mysqlService := MysqlService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
		instance, err := mysqlService.DescribeDBInstanceById(ctx, rs.Primary.ID)
		if instance == nil {
			return fmt.Errorf("mysql instance %s is not found", rs.Primary.ID)
		}
		if err != nil {
			return err
		}
		return nil
	}
}

const testAccMySQLDeviceType = `
variable "temporary_param_tmpl_id" {
	default = 16954
}

resource "tencentcloud_mysql_instance" "mysql_exclusive" {
  charge_type       = "POSTPAID"
  mem_size          = 16000
  cpu               = 2
  volume_size       = 50
  instance_name     = "testAccMysqlBasic"
  engine_version    = "5.7"
  intranet_port     = 3360
  root_password     = "test1234"
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
  device_type       = "EXCLUSIVE"
  param_template_id = var.temporary_param_tmpl_id
}
`

const testAccMySQLDeviceTypeUpdate = `
variable "temporary_param_tmpl_id" {
	default = 16954
}

resource "tencentcloud_mysql_instance" "mysql_exclusive" {
  charge_type       = "POSTPAID"
  mem_size          = 16000
  cpu               = 2
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  intranet_port     = 3360
  root_password     = "test1234"
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
  device_type       = "EXCLUSIVE"
  fast_upgrade      = 1
  param_template_id = var.temporary_param_tmpl_id
}
`

func testAccMysqlMasterInstance_basic() string {
	return `
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = 3360
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
}`
}

func testAccMysqlMasterInstance_fullslave() string {
	return `
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = 3360
  availability_zone = "ap-guangzhou-3"
  slave_deploy_mode = 0
  first_slave_zone  = "ap-guangzhou-3"
  second_slave_zone = "ap-guangzhou-3"
  slave_sync_mode   = 2
  force_delete      = true
}`
}

func testAccMysqlMasterInstance_internet_service(open bool) string {
	tag := "0"
	if open {
		tag = "1"
	}
	return `
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = 3360
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  internet_service  = ` + tag + `
  force_delete      = true
}`

}

func testAccMysqlMasterInstance_parameters() string {
	return `
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = 3360
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
  
  parameters = {
    max_connections = "1000"
  }
}`
}

func testAccMysqlMasterInstance_multiTags(value string) string {
	return fmt.Sprintf(`
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = 3360
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
  tags = {
    test = "test-tf"
    role = "%s"
  }
}
	`, value)
}

func testAccMysqlMasterInstance_update(instance_name, instranet_port string) string {
	tpl := `
resource "tencentcloud_mysql_instance" "mysql_master" {
  charge_type       = "POSTPAID"
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "%s"
  engine_version    = "5.7"
  root_password     = "test1234"
  intranet_port     = %s
  availability_zone = "ap-guangzhou-3"
  first_slave_zone  = "ap-guangzhou-3"
  force_delete      = true
}`
	return fmt.Sprintf(tpl, instance_name, instranet_port)
}
