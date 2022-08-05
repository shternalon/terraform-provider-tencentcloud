/*
Use this resource to create SQL Server account DB attachment

Example Usage

```hcl
resource "tencentcloud_sqlserver_account_db_attachment" "foo" {
  instance_id = "mssql-3cdq7kx5"
  account_name = tencentcloud_sqlserver_account.example.name
  db_name = tencentcloud_sqlserver_db.example.name
  privilege = "ReadWrite"
}
```

Import

SQL Server account DB attachment can be imported using the id, e.g.

```
$ terraform import tencentcloud_sqlserver_account_db_attachment.foo mssql-3cdq7kx5#tf_sqlserver_account#test111
```
*/
package tencentcloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceTencentCloudSqlserverAccountDBAttachment() *schema.Resource {
	return &schema.Resource{
		Create: resourceTencentCloudSqlserverAccountDBAttachmentCreate,
		Read:   resourceTencentCloudSqlserverAccountDBAttachmentRead,
		Update: resourceTencentCloudSqlserverAccountDBAttachmentUpdate,
		Delete: resourceTencentCLoudSqlserverAccountDBAttachmentDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"instance_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "SQL Server instance ID that the account belongs to.",
			},
			"account_name": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "SQL Server account name.",
			},
			"db_name": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "SQL Server DB name.",
			},
			"privilege": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Privilege of the account on DB. Valid values: `ReadOnly`, `ReadWrite`.",
			},
		},
	}
}

func resourceTencentCloudSqlserverAccountDBAttachmentCreate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_sqlserver_account_db_attachment.create")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	sqlserverService := SqlserverService{client: meta.(*TencentCloudClient).apiV3Conn}

	var (
		accountName = d.Get("account_name").(string)
		dbName      = d.Get("db_name").(string)
		instanceId  = d.Get("instance_id").(string)
		privilege   = d.Get("privilege").(string)
	)

	var outErr, inErr error

	//check account exists
	outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		_, has, inErr := sqlserverService.DescribeSqlserverAccountById(ctx, instanceId, accountName)
		if inErr != nil {
			return retryError(inErr)
		}
		if !has {
			return resource.NonRetryableError(fmt.Errorf(" SQL Server account %s, instance ID %s is not exist", accountName, instanceId))
		}
		return nil
	})
	if outErr != nil {
		return outErr
	}

	//check db exists
	outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		_, has, inErr := sqlserverService.DescribeDBDetailsById(ctx, instanceId+FILED_SP+dbName)
		if inErr != nil {
			return retryError(inErr)
		}
		if !has {
			return resource.NonRetryableError(fmt.Errorf(" SQL Server DB %s, instance ID %s is not exist", dbName, instanceId))
		}
		return nil
	})
	if outErr != nil {
		return outErr
	}

	outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		inErr = sqlserverService.ModifyAccountDBAttachment(ctx, instanceId, accountName, dbName, privilege)
		if inErr != nil {
			return retryError(inErr)
		}
		return nil
	})
	if outErr != nil {
		return outErr
	}

	d.SetId(instanceId + FILED_SP + accountName + FILED_SP + dbName)

	return resourceTencentCloudSqlserverAccountDBAttachmentRead(d, meta)
}

func resourceTencentCloudSqlserverAccountDBAttachmentUpdate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_sqlserver_account_db_attachment.update")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	sqlserverService := SqlserverService{client: meta.(*TencentCloudClient).apiV3Conn}
	id := d.Id()
	idStrs := strings.Split(id, FILED_SP)
	if len(idStrs) != 3 {
		return fmt.Errorf("invalid SQL Server account DB attachment %s", id)
	}
	instanceId := idStrs[0]
	accountName := idStrs[1]
	dbName := idStrs[2]

	var outErr, inErr error

	//update privilege
	if d.HasChange("privilege") {
		privilege := d.Get("privilege").(string)
		outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			inErr = sqlserverService.ModifyAccountDBAttachment(ctx, instanceId, accountName, dbName, privilege)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
		if outErr != nil {
			return outErr
		}
	}

	return resourceTencentCloudSqlserverAccountDBAttachmentRead(d, meta)
}

func resourceTencentCloudSqlserverAccountDBAttachmentRead(d *schema.ResourceData, meta interface{}) error {

	defer logElapsed("resource.tencentcloud_sqlserver_account_db_attachment.read")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	id := d.Id()
	idStrs := strings.Split(id, FILED_SP)
	if len(idStrs) != 3 {
		return fmt.Errorf("invalid SQL Server account DB attachment ID %s", id)
	}
	instanceId := idStrs[0]
	accountName := idStrs[1]
	dbName := idStrs[2]

	var outErr, inErr error

	sqlserverService := SqlserverService{client: meta.(*TencentCloudClient).apiV3Conn}
	attachment, has, outErr := sqlserverService.DescribeAccountDBAttachmentById(ctx, instanceId, accountName, dbName)
	if outErr != nil {
		inErr = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			attachment, has, inErr = sqlserverService.DescribeAccountDBAttachmentById(ctx, instanceId, accountName, dbName)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
	}

	if !has {
		d.SetId("")
		return nil
	}

	_ = d.Set("instance_id", instanceId)
	_ = d.Set("account_name", accountName)
	_ = d.Set("db_name", dbName)
	_ = d.Set("privilege", attachment["privilege"])

	return nil
}

func resourceTencentCLoudSqlserverAccountDBAttachmentDelete(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_sqlserver_account_db_attachment.delete")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	id := d.Id()
	idStrs := strings.Split(id, FILED_SP)
	if len(idStrs) != 3 {
		return fmt.Errorf("invalid SQL Server account DB attachment id %s", id)
	}
	instanceId := idStrs[0]
	accountName := idStrs[1]
	dbName := idStrs[2]

	sqlserverService := SqlserverService{client: meta.(*TencentCloudClient).apiV3Conn}

	var outErr, inErr error
	var has bool
	privilege := "Delete"

	outErr = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		_, has, inErr = sqlserverService.DescribeAccountDBAttachmentById(ctx, instanceId, accountName, dbName)
		if inErr != nil {
			return retryError(inErr)
		}
		return nil
	})

	if outErr != nil {
		return outErr
	}

	if !has {
		return nil
	}

	outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		inErr = sqlserverService.ModifyAccountDBAttachment(ctx, instanceId, accountName, dbName, privilege)
		if inErr != nil {
			return retryError(inErr)
		}
		return nil
	})

	if outErr != nil {
		return outErr
	}

	outErr = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		_, has, inErr = sqlserverService.DescribeAccountDBAttachmentById(ctx, instanceId, accountName, dbName)
		if inErr != nil {
			return retryError(inErr)
		}
		if has {
			inErr = fmt.Errorf("delete SQL Server account DB attachment %s fail, account still exists from SDK DescribeSqlserverAccountDBAttachmentById", id)
			return resource.RetryableError(inErr)
		}
		return nil
	})

	if outErr != nil {
		return outErr
	}
	return nil
}
