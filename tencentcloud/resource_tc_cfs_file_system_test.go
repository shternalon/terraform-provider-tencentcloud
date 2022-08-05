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
	// go test -v ./tencentcloud -sweep=ap-guangzhou -sweep-run=tencentcloud_cfs_file_system
	resource.AddTestSweepers("tencentcloud_cfs_file_system", &resource.Sweeper{
		Name: "tencentcloud_cfs_file_system",
		F: func(r string) error {
			logId := getLogId(contextNil)
			ctx := context.WithValue(context.TODO(), logIdKey, logId)
			cli, _ := sharedClientForRegion(r)
			client := cli.(*TencentCloudClient).apiV3Conn

			service := CfsService{client}

			fsList, err := service.DescribeFileSystem(ctx, "", "", "")
			if err != nil {
				return err
			}
			for i := range fsList {
				item := fsList[i]
				id := *item.FileSystemId
				name := *item.FsName
				created := time.Time{}
				if item.CreationTime != nil {
					if result, err := time.Parse(time.RFC3339, *item.CreationTime); err != nil {
						created = result
					}
				}
				if isResourcePersist(name, &created) {
					continue
				}
				log.Printf("%s -> %s will be sweep", id, name)
				err = service.DeleteFileSystem(ctx, id)
				if err != nil {
					continue
				}
			}

			return nil
		},
	})
}

func TestAccTencentCloudCfsFileSystem(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCfsFileSystemDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCfsFileSystem,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCfsFileSystemExists("tencentcloud_cfs_file_system.foo"),
					resource.TestCheckResourceAttr("tencentcloud_cfs_file_system.foo", "name", "test_cfs_file_system"),
					resource.TestCheckResourceAttr("tencentcloud_cfs_file_system.foo", "availability_zone", "ap-guangzhou-3"),
					resource.TestCheckResourceAttrSet("tencentcloud_cfs_file_system.foo", "access_group_id"),
					resource.TestCheckResourceAttr("tencentcloud_cfs_file_system.foo", "protocol", "NFS"),
				),
			},
			// add tag
			{
				Config: testAccCfsMasterInstance_multiTags("master"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCfsFileSystemExists("tencentcloud_cfs_file_system.foo"),
					resource.TestCheckResourceAttr("tencentcloud_cfs_file_system.foo", "tags.role", "master"),
				),
			},
			// update tag
			{
				Config: testAccCfsMasterInstance_multiTags("master-version2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCfsFileSystemExists("tencentcloud_cfs_file_system.foo"),
					resource.TestCheckResourceAttr("tencentcloud_cfs_file_system.foo", "tags.role", "master-version2"),
				),
			},
			// remove tag
			{
				Config: testAccCfsFileSystem,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckCfsFileSystemExists("tencentcloud_cfs_file_system.foo"),
					resource.TestCheckNoResourceAttr("tencentcloud_cfs_file_system.foo", "tags.role"),
				),
			},
		},
	})
}

func testAccCheckCfsFileSystemDestroy(s *terraform.State) error {
	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)
	cfsService := CfsService{
		client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn,
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "tencentcloud_cfs_file_system" {
			continue
		}

		fileSystems, err := cfsService.DescribeFileSystem(ctx, rs.Primary.ID, "", "")
		if err != nil {
			err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
				fileSystems, err = cfsService.DescribeFileSystem(ctx, rs.Primary.ID, "", "")
				if err != nil {
					return retryError(err)
				}
				return nil
			})
		}
		if err != nil {
			return err
		}
		if len(fileSystems) > 0 {
			return fmt.Errorf("cfs file system still exist: %s", rs.Primary.ID)
		}
	}
	return nil
}

func testAccCheckCfsFileSystemExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		logId := getLogId(contextNil)
		ctx := context.WithValue(context.TODO(), logIdKey, logId)

		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("cfs file system %s is not found", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("cfs file system id is not set")
		}
		cfsService := CfsService{
			client: testAccProvider.Meta().(*TencentCloudClient).apiV3Conn,
		}
		fileSystems, err := cfsService.DescribeFileSystem(ctx, rs.Primary.ID, "", "")
		if err != nil {
			err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
				fileSystems, err = cfsService.DescribeFileSystem(ctx, rs.Primary.ID, "", "")
				if err != nil {
					return retryError(err)
				}
				return nil
			})
		}
		if err != nil {
			return err
		}
		if len(fileSystems) < 1 {
			return fmt.Errorf("cfs file system is not found")
		}
		return nil
	}
}

const testAccCfsFileSystem = defaultCfsAccessGroup + `
resource "tencentcloud_vpc" "vpc" {
  name       = "test-cfs-vpc"
  cidr_block = "10.2.0.0/16"
}

resource "tencentcloud_subnet" "subnet" {
  vpc_id            = tencentcloud_vpc.vpc.id
  name              = "test-cfs-subnet"
  cidr_block        = "10.2.11.0/24"
  availability_zone = "ap-guangzhou-3"
}

resource "tencentcloud_cfs_file_system" "foo" {
  name = "test_cfs_file_system"
  availability_zone = "ap-guangzhou-3"
  access_group_id = local.cfs_access_group_id
  protocol = "NFS"
  vpc_id = tencentcloud_vpc.vpc.id
  subnet_id = tencentcloud_subnet.subnet.id
  storage_type = "SD"
}
`

func testAccCfsMasterInstance_multiTags(value string) string {
	return fmt.Sprintf(
		`
%s
resource "tencentcloud_vpc" "vpc" {
  name       = "test-cfs-vpc"
  cidr_block = "10.2.0.0/16"
}

resource "tencentcloud_subnet" "subnet" {
  vpc_id            = tencentcloud_vpc.vpc.id
  name              = "test-cfs-subnet"
  cidr_block        = "10.2.11.0/24"
  availability_zone = "ap-guangzhou-3"
}


resource "tencentcloud_cfs_file_system" "foo" {
  name = "test_cfs_file_system"
  availability_zone = "ap-guangzhou-3"
  access_group_id = local.cfs_access_group_id
  protocol = "NFS"
  vpc_id = tencentcloud_vpc.vpc.id
  subnet_id = tencentcloud_subnet.subnet.id
  storage_type = "SD"
  
  tags = {
	  test = "test-tf"
	  role = "%s"
  }
}
`, defaultCfsAccessGroup, value)
}
