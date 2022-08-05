package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func init() {
	// go test -v ./tencentcloud -sweep=ap-guangzhou -sweep-run=tencentcloud_tcr_instance
	resource.AddTestSweepers("tencentcloud_tcr_instance", &resource.Sweeper{
		Name: "tencentcloud_tcr_instance",
		F: func(r string) error {
			logId := getLogId(contextNil)
			ctx := context.WithValue(context.TODO(), logIdKey, logId)
			cli, _ := sharedClientForRegion(r)
			client := cli.(*TencentCloudClient).apiV3Conn
			service := TCRService{client}

			instances, err := service.DescribeTCRInstances(ctx, "", nil)

			if err != nil {
				return err
			}

			for i := range instances {
				ins := instances[i]
				id := *ins.RegistryId
				name := *ins.RegistryName
				created, err := time.Parse(time.RFC3339, *ins.CreatedAt)
				if err != nil {
					created = time.Time{}
				}
				if isResourcePersist(name, &created) {
					continue
				}
				log.Printf("instance %s:%s will delete", id, name)
				err = service.DeleteTCRInstance(ctx, id, true)
				if err != nil {
					continue
				}
			}

			return nil
		},
	})
}

func TestAccTencentCloudTCRInstance_basic_and_update(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckTCRInstanceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTCRInstance_basic,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "name", "testacctcrinstance1"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "instance_type", "basic"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "tags.test", "test"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "delete_bucket", "true"),
					resource.TestCheckResourceAttrSet("tencentcloud_tcr_instance.mytcr_instance", "internal_end_point"),
					resource.TestCheckResourceAttrSet("tencentcloud_tcr_instance.mytcr_instance", "status"),
					resource.TestCheckResourceAttrSet("tencentcloud_tcr_instance.mytcr_instance", "public_domain"),
				),
				Destroy: false,
			},
			{
				ResourceName:            "tencentcloud_tcr_instance.mytcr_instance",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_bucket"},
			},
			{
				Config: testAccTCRInstance_basic_update_remark,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckTCRInstanceExists("tencentcloud_tcr_instance.mytcr_instance"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "tags.test", "test"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "delete_bucket", "true"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "open_public_operation", "true"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "security_policy.#", "2"),
					//resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "security_policy.0.cidr_block", "192.168.1.1/24"),
					//resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "security_policy.1.cidr_block", "10.0.0.1/16"),
				),
				Destroy: false,
			},
			{
				Config: testAccTCRInstance_basic_update_security,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckTCRInstanceExists("tencentcloud_tcr_instance.mytcr_instance"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "open_public_operation", "true"),
					resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "security_policy.#", "1"),
					//resource.TestCheckResourceAttr("tencentcloud_tcr_instance.mytcr_instance", "security_policy.0.cidr_block", "192.168.1.1/24"),
				),
			},
		},
	})
}

func testAccCheckTCRInstanceDestroy(s *terraform.State) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)
	tcrService := TCRService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "tencentcloud_tcr_instance" {
			continue
		}
		_, has, err := tcrService.DescribeTCRInstanceById(ctx, rs.Primary.ID)
		if has {
			return fmt.Errorf("TCR instance still exists")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func testAccCheckTCRInstanceExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("TCR instance %s is not found", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("TCR instance id is not set")
		}

		tcrService := TCRService{client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn}
		_, has, err := tcrService.DescribeTCRInstanceById(ctx, rs.Primary.ID)
		if !has {
			return fmt.Errorf("TCR instance %s is not found", rs.Primary.ID)
		}
		if err != nil {
			return err
		}

		return nil
	}
}

const testAccTCRInstance_basic = `
resource "tencentcloud_tcr_instance" "mytcr_instance" {
  name        = "testacctcrinstance1"
  instance_type = "basic"
  delete_bucket = true

  tags ={
	test = "test"
  }
}`

const testAccTCRInstance_basic_update_remark = `
resource "tencentcloud_tcr_instance" "mytcr_instance" {
  name        = "testacctcrinstance1"
  instance_type = "basic"
  delete_bucket = true
  open_public_operation = true
  security_policy {
    cidr_block = "192.168.1.1/24"
  }
  security_policy {
    cidr_block = "10.0.0.1/16"
  }

  tags ={
	test = "test"
  }
}`

const testAccTCRInstance_basic_update_security = `
resource "tencentcloud_tcr_instance" "mytcr_instance" {
  name        = "testacctcrinstance1"
  instance_type = "basic"
  delete_bucket = true
  open_public_operation = true

  security_policy {
    cidr_block = "192.168.1.1/24"
  }

  tags ={
	test = "test"
  }
}
`
