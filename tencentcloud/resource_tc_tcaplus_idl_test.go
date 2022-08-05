package tencentcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

var testTcaplusIdlResourceName = "tencentcloud_tcaplus_idl"
var testTcaplusIdlResourceNameResourceKey = testTcaplusIdlResourceName + ".test_idl"

func TestAccTencentCloudTcaplusIdlResource(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckTcaplusIdlDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTcaplusIdl,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTcaplusIdlExists(testTcaplusIdlResourceNameResourceKey),
					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "cluster_id"),
					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "file_name", "tf_idl_test_guagua"),
					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "file_type", "PROTO"),
					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "file_ext_type", "proto"),

					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "table_infos.#", "1"),

					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "table_infos.0.key_fields"),
					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "table_infos.0.sum_key_field_size"),
					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "table_infos.0.value_fields"),
					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "table_infos.0.sum_value_field_size"),
					resource.TestCheckResourceAttrSet(testTcaplusIdlResourceNameResourceKey, "table_infos.0.index_key_set"),
					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "table_infos.0.table_name", "tb_idle_test"),
					resource.TestCheckResourceAttr(testTcaplusIdlResourceNameResourceKey, "table_infos.0.error", ""),
				),
			},
		},
	})
}
func testAccCheckTcaplusIdlDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != testTcaplusIdlResourceName {
			continue
		}
		service := TcaplusService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		var tcaplusIdlId TcaplusIdlId

		if err := json.Unmarshal([]byte(rs.Primary.ID), &tcaplusIdlId); err != nil {
			return fmt.Errorf("idl id is broken,%s", err.Error())
		}
		parseTableInfos, err := service.DesOldIdlFiles(ctx, tcaplusIdlId)
		if err != nil {
			parseTableInfos, err = service.DesOldIdlFiles(ctx, tcaplusIdlId)
		}
		if err != nil {
			return err
		}
		if len(parseTableInfos) == 0 {
			return nil
		}
		return fmt.Errorf("delete tcaplus idl %s fail, still on server", rs.Primary.ID)
	}
	return nil
}

func testAccCheckTcaplusIdlExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("resource %s is not found", n)
		}
		service := TcaplusService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		var tcaplusIdlId TcaplusIdlId

		if err := json.Unmarshal([]byte(rs.Primary.ID), &tcaplusIdlId); err != nil {
			return fmt.Errorf("idl id is broken,%s", err.Error())
		}
		parseTableInfos, err := service.DesOldIdlFiles(ctx, tcaplusIdlId)
		if err != nil {
			parseTableInfos, err = service.DesOldIdlFiles(ctx, tcaplusIdlId)
		}
		if err != nil {
			return err
		}
		if len(parseTableInfos) != 0 {
			return nil
		}
		return fmt.Errorf("tcaplus idl %s not found on server", rs.Primary.ID)
	}
}

const testAccTcaplusIdl = defaultTcaPlusData + `
resource "tencentcloud_tcaplus_idl" "test_idl" {
  cluster_id     = local.tcaplus_id
  tablegroup_id  = local.tcaplus_table_group_id
  file_name      = "tf_idl_test_guagua"
  file_type      = "PROTO"
  file_ext_type  = "proto"
  file_content   = <<EOF
    syntax = "proto2";
    package myTcaplusTable;
    import "tcaplusservice.optionv1.proto";
    message tb_idle_test {
        option(tcaplusservice.tcaplus_primary_key) = "uin,name,region";
        required int64 uin = 1;
        required string name = 2;
        required int32 region = 3;
        required int32 gamesvrid = 4;
        optional int32 logintime = 5 [default = 1];
        repeated int64 lockid = 6 [packed = true];
        optional bool is_available = 7 [default = false];
        optional pay_info pay = 8;
    }

    message pay_info {
        required int64 pay_id = 1;
        optional uint64 total_money = 2;
        optional uint64 pay_times = 3;
        optional pay_auth_info auth = 4;
        message pay_auth_info {
            required string pay_keys = 1;
            optional int64 update_time = 2;
        }
    }
    EOF
}
`
