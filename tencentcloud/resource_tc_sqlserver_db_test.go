package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

const testDbName = "testAccSqlserverDB"

func init() {
	// go test -v ./tencentcloud -sweep=ap-guangzhou -sweep-run=tencentcloud_sqlserver_db
	resource.AddTestSweepers("tencentcloud_sqlserver_db", &resource.Sweeper{
		Name: "tencentcloud_sqlserver_db",
		F: func(r string) error {
			logId := getLogId(contextNil)
			ctx := context.WithValue(context.TODO(), logIdKey, logId)
			cli, _ := sharedClientForRegion(r)
			client := cli.(*TencentCloudClient).apiV3Conn
			service := SqlserverService{client}

			instances, err := service.DescribeSqlserverInstances(ctx, "", "", -1, "", "", -1)

			if err != nil {
				return err
			}

			var (
				insId    string
				subInsId string
			)

			for _, v := range instances {
				if *v.Name == defaultSQLServerName {
					insId = *v.InstanceId
				}
				if *v.Name == defaultSubSQLServerName {
					subInsId = *v.InstanceId
				}
			}

			dbs, err := service.DescribeDBsOfInstance(ctx, insId)

			if err != nil {
				return err
			}

			for i := range dbs {
				db := dbs[i]
				if !strings.HasPrefix(*db.Name, "test") {
					continue
				}
				err := service.DeleteSqlserverDB(ctx, insId, []*string{db.Name})
				if err != nil {
					continue
				}
			}

			// Clear sub instance db
			subDbs, err := service.DescribeDBsOfInstance(ctx, subInsId)

			for i := range subDbs {
				db := subDbs[i]
				if *db.Name == defaultSQLServerPubSubDB {
					err = service.DeleteSqlserverDB(ctx, subInsId, []*string{db.Name})
					break
				}
			}

			if err != nil {
				log.Printf("Delete sub instance DB fail: %s", err.Error())
			}
			return nil
		},
	})
}

func TestAccTencentCloudSqlserverDB_basic_and_update(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckSqlserverDBDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSqlserverDB_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tencentcloud_sqlserver_db.mysqlserver_db", "name", testDbName),
					resource.TestCheckResourceAttr("tencentcloud_sqlserver_db.mysqlserver_db", "charset", "Chinese_PRC_BIN"),
					resource.TestCheckResourceAttr("tencentcloud_sqlserver_db.mysqlserver_db", "remark", "testACC-remark"),
					resource.TestCheckResourceAttrSet("tencentcloud_sqlserver_db.mysqlserver_db", "create_time"),
					resource.TestCheckResourceAttrSet("tencentcloud_sqlserver_db.mysqlserver_db", "status"),
					resource.TestCheckResourceAttrSet("tencentcloud_sqlserver_db.mysqlserver_db", "instance_id"),
				),
				Destroy: false,
			},
			{
				ResourceName:      "tencentcloud_sqlserver_db.mysqlserver_db",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccSqlserverDB_basic_update_remark,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSqlserverDBExists("tencentcloud_sqlserver_db.mysqlserver_db"),
					resource.TestCheckResourceAttr("tencentcloud_sqlserver_db.mysqlserver_db", "remark", "testACC-remark_update"),
				),
			},
		},
	})
}

func testAccCheckSqlserverDBDestroy(s *terraform.State) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)
	sqlserverService := SqlserverService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "tencentcloud_sqlserver_db" {
			continue
		}
		_, has, err := sqlserverService.DescribeDBDetailsById(ctx, rs.Primary.ID)
		if has {
			return fmt.Errorf("SQL Server DB still exists")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func testAccCheckSqlserverDBExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("SQL Server DB %s is not found", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("SQL Server DB id is not set")
		}

		sqlserverService := SqlserverService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
		_, has, err := sqlserverService.DescribeDBDetailsById(ctx, rs.Primary.ID)
		if !has {
			return fmt.Errorf("SQL Server DB %s is not found", rs.Primary.ID)
		}
		if err != nil {
			return err
		}

		return nil
	}
}

const testAccSqlserverDB_basic = CommonPresetSQLServer + `
resource "tencentcloud_sqlserver_db" "mysqlserver_db" {
  instance_id = local.sqlserver_id
  name        = "` + testDbName + `"
  charset     = "Chinese_PRC_BIN"
  remark      = "testACC-remark"
}`

const testAccSqlserverDB_basic_update_remark = CommonPresetSQLServer + `
resource "tencentcloud_sqlserver_db" "mysqlserver_db" {
  instance_id = local.sqlserver_id
  name        = "` + testDbName + `"
  charset     = "Chinese_PRC_BIN"
  remark      = "testACC-remark_update"
}`
