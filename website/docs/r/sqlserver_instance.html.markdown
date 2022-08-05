---
subcategory: "SQLServer"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_sqlserver_instance"
sidebar_current: "docs-tencentcloud-resource-sqlserver_instance"
description: |-
  Use this resource to create SQL Server instance
---

# tencentcloud_sqlserver_instance

Use this resource to create SQL Server instance

## Example Usage

```hcl
resource "tencentcloud_sqlserver_instance" "foo" {
  name              = "example"
  availability_zone = var.availability_zone
  charge_type       = "POSTPAID_BY_HOUR"
  vpc_id            = "vpc-409mvdvv"
  subnet_id         = "subnet-nf9n81ps"
  project_id        = 123
  memory            = 2
  storage           = 100
}
```

## Argument Reference

The following arguments are supported:

* `memory` - (Required, Int) Memory size (in GB). Allowed value must be larger than `memory` that data source `tencentcloud_sqlserver_specinfos` provides.
* `name` - (Required, String) Name of the SQL Server instance.
* `storage` - (Required, Int) Disk size (in GB). Allowed value must be a multiple of 10. The storage must be set with the limit of `storage_min` and `storage_max` which data source `tencentcloud_sqlserver_specinfos` provides.
* `auto_renew` - (Optional, Int) Automatic renewal sign. 0 for normal renewal, 1 for automatic renewal (Default). Only valid when purchasing a prepaid instance.
* `auto_voucher` - (Optional, Int) Whether to use the voucher automatically; 1 for yes, 0 for no, the default is 0.
* `availability_zone` - (Optional, String, ForceNew) Availability zone.
* `charge_type` - (Optional, String, ForceNew) Pay type of the SQL Server instance. Available values `PREPAID`, `POSTPAID_BY_HOUR`.
* `engine_version` - (Optional, String, ForceNew) Version of the SQL Server database engine. Allowed values are `2008R2`(SQL Server 2008 Enterprise), `2012SP3`(SQL Server 2012 Enterprise), `2016SP1` (SQL Server 2016 Enterprise), `201602`(SQL Server 2016 Standard) and `2017`(SQL Server 2017 Enterprise). Default is `2008R2`.
* `ha_type` - (Optional, String, ForceNew) Instance type. `DUAL` (dual-server high availability), `CLUSTER` (cluster). Default is `DUAL`.
* `maintenance_start_time` - (Optional, String) Start time of the maintenance in one day, format like `HH:mm`.
* `maintenance_time_span` - (Optional, Int) The timespan of maintenance in one day, unit is hour.
* `maintenance_week_set` - (Optional, Set: [`Int`]) A list of integer indicates weekly maintenance. For example, [2,7] presents do weekly maintenance on every Tuesday and Sunday.
* `multi_zones` - (Optional, Bool, ForceNew) Indicate whether to deploy across availability zones.
* `period` - (Optional, Int) Purchase instance period in month. The value does not exceed 48.
* `project_id` - (Optional, Int) Project ID, default value is 0.
* `security_groups` - (Optional, Set: [`String`]) Security group bound to the instance.
* `subnet_id` - (Optional, String, ForceNew) ID of subnet.
* `tags` - (Optional, Map) The tags of the SQL Server.
* `voucher_ids` - (Optional, Set: [`String`]) An array of voucher IDs, currently only one can be used for a single order.
* `vpc_id` - (Optional, String, ForceNew) ID of VPC.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the resource.
* `create_time` - Create time of the SQL Server instance.
* `ro_flag` - Readonly flag. `RO` (read-only instance), `MASTER` (primary instance with read-only instances). If it is left empty, it refers to an instance which is not read-only and has no RO group.
* `status` - Status of the SQL Server instance. 1 for applying, 2 for running, 3 for running with limit, 4 for isolated, 5 for recycling, 6 for recycled, 7 for running with task, 8 for off-line, 9 for expanding, 10 for migrating, 11 for readonly, 12 for rebooting.
* `vip` - IP for private access.
* `vport` - Port for private access.


## Import

SQL Server instance can be imported using the id, e.g.

```
$ terraform import tencentcloud_sqlserver_instance.foo mssql-3cdq7kx5
```

