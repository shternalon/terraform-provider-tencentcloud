/*
Use this resource to create postgresql readonly instance.

Example Usage

```hcl
resource "tencentcloud_postgresql_readonly_instance" "foo" {
  auto_renew_flag       = 0
  db_version            = "10.4"
  instance_charge_type  = "POSTPAID_BY_HOUR"
  master_db_instance_id = "postgres-j4pm65id"
  memory                = 4
  name                  = "hello"
  need_support_ipv6     = 0
  project_id            = 0
  security_groups_ids   = [
    "sg-fefj5n6r",
  ]
  storage               = 250
  subnet_id             = "subnet-enm92y0m"
  vpc_id                = "vpc-86v957zb"
  zone                  = "ap-guangzhou-6"
}
```

Import

postgresql readonly instance can be imported using the id, e.g.

```
$ terraform import tencentcloud_postgresql_readonly_instance.foo pgro-bcqx8b9a
```
*/
package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"strings"

	postgresql "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/postgres/v20170312"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
)

func resourceTencentCloudPostgresqlReadonlyInstance() *schema.Resource {
	return &schema.Resource{
		Create: resourceTencentCloudPostgresqlReadOnlyInstanceCreate,
		Read:   resourceTencentCloudPostgresqlReadOnlyInstanceRead,
		Update: resourceTencentCloudPostgresqlReadOnlyInstanceUpdate,
		Delete: resourceTencentCLoudPostgresqlReadOnlyInstanceDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"db_version": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "PostgreSQL kernel version, which must be the same as that of the primary instance.",
			},
			"storage": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Instance storage capacity in GB.",
			},
			"memory": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Memory size(in GB). Allowed value must be larger than `memory` that data source `tencentcloud_postgresql_specinfos` provides.",
			},
			"master_db_instance_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "ID of the primary instance to which the read-only replica belongs.",
			},
			"zone": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "Availability zone ID, which can be obtained through the Zone field in the returned value of the DescribeZones API.",
			},
			"project_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Project ID.",
			},
			"vpc_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
				Description: "VPC ID.",
			},
			"subnet_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "VPC subnet ID.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Instance name.",
			},
			"security_groups_ids": {
				Type:        schema.TypeSet,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "ID of security group.",
			},
			"instance_charge_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      COMMON_PAYTYPE_POSTPAID,
				ForceNew:     true,
				ValidateFunc: validateAllowedStringValue(POSTGRESQL_PAYTYPE),
				Description:  "instance billing mode. Valid values: PREPAID (monthly subscription), POSTPAID_BY_HOUR (pay-as-you-go).",
			},
			"auto_renew_flag": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				ForceNew:    true,
				Description: "Renewal flag. Valid values: 0 (manual renewal), 1 (auto-renewal). Default value: 0.",
			},
			"need_support_ipv6": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Whether to support IPv6 address access. Valid values: 1 (yes), 0 (no).",
			},
			//"tag_list": {
			//	Type:        schema.TypeMap,
			//	Optional:    true,
			//	Description: "The information of tags to be associated with instances. This parameter is left empty by default..",
			//},
			//"read_only_group_id": {
			//	Type:        schema.TypeString,
			//	ForceNew:    true,
			//	Optional:    true,
			//	Description: "RO group ID.",
			//},
			// Computed values
			"create_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Create time of the postgresql instance.",
			},
		},
	}
}

func resourceTencentCloudPostgresqlReadOnlyInstanceCreate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_postgresql_readonly_instance.create")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	var (
		request           = postgresql.NewCreateReadOnlyDBInstanceRequest()
		response          *postgresql.CreateReadOnlyDBInstanceResponse
		postgresqlService = PostgresqlService{client: meta.(*TencentCloudClient).apiV3Conn}
		zone              string
		dbVersion         string
		memory            int
	)
	if v, ok := d.GetOk("db_version"); ok {
		dbVersion = v.(string)
		request.DBVersion = helper.String(dbVersion)
	}
	if v, ok := d.GetOk("storage"); ok {
		request.Storage = helper.IntUint64(v.(int))
	}
	if v, ok := d.GetOk("memory"); ok {
		memory = v.(int)
	}
	if v, ok := d.GetOk("master_db_instance_id"); ok {
		request.MasterDBInstanceId = helper.String(v.(string))
	}
	if v, ok := d.GetOk("zone"); ok {
		zone = v.(string)
		request.Zone = helper.String(zone)
	}
	if v, ok := d.GetOk("project_id"); ok {
		request.ProjectId = helper.IntUint64(v.(int))
	}
	if v, ok := d.GetOk("instance_charge_type"); ok {
		request.InstanceChargeType = helper.String(v.(string))
	}
	if v, ok := d.GetOk("auto_renew_flag"); ok {
		request.AutoRenewFlag = helper.IntInt64(v.(int))
	}
	if v, ok := d.GetOk("vpc_id"); ok {
		request.VpcId = helper.String(v.(string))
	}
	if v, ok := d.GetOk("subnet_id"); ok {
		request.SubnetId = helper.String(v.(string))
	}
	if v, ok := d.GetOk("name"); ok {
		request.Name = helper.String(v.(string))
	}
	if v, ok := d.GetOk("need_support_ipv6"); ok {
		request.NeedSupportIpv6 = helper.IntUint64(v.(int))
	}
	if v, ok := d.GetOk("read_only_group_id"); ok {
		request.ReadOnlyGroupId = helper.String(v.(string))
	}
	if v, ok := d.GetOk("security_groups_ids"); ok {
		securityGroupsIds := v.(*schema.Set).List()
		request.SecurityGroupIds = make([]*string, 0, len(securityGroupsIds))
		for _, item := range securityGroupsIds {
			request.SecurityGroupIds = append(request.SecurityGroupIds, helper.String(item.(string)))
		}
	}
	//if tags := helper.GetTags(d, "tag_list"); len(tags) > 0 {
	//	for k, v := range tags {
	//		request.TagList = &postgresql.Tag{
	//			TagKey:   &k,
	//			TagValue: &v,
	//		}
	//	}
	//}

	// get specCode with db_version and memory
	var allowVersion, allowMemory []string
	var specVersion, specCode string
	err := resource.Retry(readRetryTimeout*5, func() *resource.RetryError {
		speccodes, inErr := postgresqlService.DescribeSpecinfos(ctx, zone)
		if inErr != nil {
			return retryError(inErr)
		}
		for _, info := range speccodes {
			if !IsContains(allowVersion, *info.Version) {
				allowVersion = append(allowVersion, *info.Version)
			}
			if *info.Version == dbVersion {
				specVersion = *info.Version
				memoryString := fmt.Sprintf("%d", int(*info.Memory)/1024)
				if !IsContains(allowMemory, memoryString) {
					allowMemory = append(allowMemory, memoryString)
				}
				if int(*info.Memory)/1024 == memory {
					specCode = *info.SpecCode
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if specVersion == "" {
		return fmt.Errorf(`The "db_version" value: "%s" is invalid, Valid values are one of: "%s"`, dbVersion, strings.Join(allowVersion, `", "`))
	}
	if specCode == "" {
		return fmt.Errorf(`The "storage" value: %d is invalid, Valid values are one of: %s`, memory, strings.Join(allowMemory, `, `))
	}
	request.SpecCode = helper.String(specCode)

	request.InstanceCount = helper.IntUint64(1)
	request.Period = helper.IntUint64(1)

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		result, e := meta.(*TencentCloudClient).apiV3Conn.UsePostgresqlClient().CreateReadOnlyDBInstance(request)
		if e != nil {
			return retryError(e)
		} else {
			log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
				logId, request.GetAction(), request.ToJsonString(), result.ToJsonString())
		}
		response = result
		return nil

	})
	if err != nil {
		return err
	}
	instanceId := *response.Response.DBInstanceIdSet[0]
	d.SetId(instanceId)

	// check creation done
	err = resource.Retry(5*readRetryTimeout, func() *resource.RetryError {
		instance, has, err := postgresqlService.DescribePostgresqlInstanceById(ctx, instanceId)
		if err != nil {
			return retryError(err)
		} else if has && *instance.DBInstanceStatus == "running" {
			return nil
		} else if !has {
			return resource.NonRetryableError(fmt.Errorf("create postgresql instance fail"))
		} else {
			return resource.RetryableError(fmt.Errorf("creating readonly postgresql instance %s , status %s ", instanceId, *instance.DBInstanceStatus))
		}
	})

	if err != nil {
		return err
	}

	return resourceTencentCloudPostgresqlReadOnlyInstanceRead(d, meta)
}

func resourceTencentCloudPostgresqlReadOnlyInstanceRead(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_postgresql_readonly_instance.read")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	postgresqlService := PostgresqlService{client: meta.(*TencentCloudClient).apiV3Conn}
	instance, has, err := postgresqlService.DescribePostgresqlInstanceById(ctx, d.Id())
	if err != nil {
		return err
	}
	if !has {
		d.SetId("")
		return nil
	}

	_ = d.Set("db_version", instance.DBVersion)
	_ = d.Set("storage", instance.DBInstanceStorage)
	_ = d.Set("memory", instance.DBInstanceMemory)
	_ = d.Set("master_db_instance_id", instance.MasterDBInstanceId)
	_ = d.Set("zone", instance.Zone)
	_ = d.Set("project_id", instance.ProjectId)

	if *instance.PayType == POSTGRESQL_PAYTYPE_PREPAID || *instance.PayType == COMMON_PAYTYPE_PREPAID {
		_ = d.Set("instance_charge_type", COMMON_PAYTYPE_PREPAID)
	} else {
		_ = d.Set("instance_charge_type", COMMON_PAYTYPE_POSTPAID)
	}

	_ = d.Set("auto_renew_flag", instance.AutoRenew)
	_ = d.Set("vpc_id", instance.VpcId)
	_ = d.Set("subnet_id", instance.SubnetId)
	_ = d.Set("name", instance.DBInstanceName)
	_ = d.Set("need_support_ipv6", instance.SupportIpv6)

	// security groups
	// Only redis service support modify Generic DB instance security groups
	redisService := RedisService{client: meta.(*TencentCloudClient).apiV3Conn}
	sg, err := redisService.DescribeDBSecurityGroups(ctx, "postgres", d.Id())
	if err != nil {
		return err
	}
	if len(sg) > 0 {
		_ = d.Set("security_groups_ids", sg)
	}

	//tags := make(map[string]string, len(instance.TagList))
	//for _, tag := range instance.TagList {
	//	tags[*tag.TagKey] = *tag.TagValue
	//}
	//_ = d.Set("tag_list", tags)

	// computed
	_ = d.Set("create_time", instance.CreateTime)
	_ = d.Set("status", instance.DBInstanceStatus)

	return nil
}

func resourceTencentCloudPostgresqlReadOnlyInstanceUpdate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_postgresql_readonly_instance.update")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	postgresqlService := PostgresqlService{client: meta.(*TencentCloudClient).apiV3Conn}
	instanceId := d.Id()
	d.Partial(true)

	var outErr, inErr, checkErr error
	// update name
	if d.HasChange("name") {
		name := d.Get("name").(string)
		outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			inErr = postgresqlService.ModifyPostgresqlInstanceName(ctx, instanceId, name)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
		if outErr != nil {
			return outErr
		}
		// check update name done
		checkErr = postgresqlService.CheckDBInstanceStatus(ctx, instanceId)
		if checkErr != nil {
			return checkErr
		}
		d.SetPartial("name")
	}

	// upgrade storage and memory size
	if d.HasChange("memory") || d.HasChange("storage") {
		memory := d.Get("memory").(int)
		storage := d.Get("storage").(int)
		outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			inErr = postgresqlService.UpgradePostgresqlInstance(ctx, instanceId, memory, storage)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
		if outErr != nil {
			return outErr
		}
		// check update storage and memory done
		checkErr = postgresqlService.CheckDBInstanceStatus(ctx, instanceId)
		if checkErr != nil {
			return checkErr
		}
		d.SetPartial("memory")
		d.SetPartial("storage")
	}

	// update project id
	if d.HasChange("project_id") {
		projectId := d.Get("project_id").(int)
		outErr = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			inErr = postgresqlService.ModifyPostgresqlInstanceProjectId(ctx, instanceId, projectId)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
		if outErr != nil {
			return outErr
		}

		// check update project id done
		checkErr = postgresqlService.CheckDBInstanceStatus(ctx, instanceId)
		if checkErr != nil {
			return checkErr
		}
		d.SetPartial("project_id")
	}

	if d.HasChange("security_groups_ids") {

		// Only redis service support modify Generic DB instance security groups
		service := RedisService{client: meta.(*TencentCloudClient).apiV3Conn}
		ids := d.Get("security_groups_ids").(*schema.Set).List()
		var sgIds []*string
		for _, id := range ids {
			sgIds = append(sgIds, helper.String(id.(string)))
		}
		err := service.ModifyDBInstanceSecurityGroups(ctx, "postgres", d.Id(), sgIds)
		if err != nil {
			return err
		}
		d.SetPartial("security_groups_ids")
	}

	//if d.HasChange("tags") {
	//
	//	oldValue, newValue := d.GetChange("tags")
	//	replaceTags, deleteTags := diffTags(oldValue.(map[string]interface{}), newValue.(map[string]interface{}))
	//
	//	tcClient := meta.(*TencentCloudClient).apiV3Conn
	//	tagService := &TagService{client: tcClient}
	//	resourceName := BuildTagResourceName("postgres", "DBInstanceId", tcClient.Region, d.Id())
	//	err := tagService.ModifyTags(ctx, resourceName, replaceTags, deleteTags)
	//	if err != nil {
	//		return err
	//	}
	//	d.SetPartial("tags")
	//}

	d.Partial(false)

	return resourceTencentCloudPostgresqlReadOnlyInstanceRead(d, meta)
}

func resourceTencentCLoudPostgresqlReadOnlyInstanceDelete(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_postgresql_readonly_instance.delete")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	instanceId := d.Id()
	postgresqlService := PostgresqlService{client: meta.(*TencentCloudClient).apiV3Conn}

	// isolate
	err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		e := postgresqlService.IsolatePostgresqlInstance(ctx, instanceId)
		if e != nil {
			return retryError(e)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// delete
	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		e := postgresqlService.DeletePostgresqlInstance(ctx, instanceId)
		if e != nil {
			return retryError(e)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
