---
subcategory: "EMR"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_emr_cluster"
sidebar_current: "docs-tencentcloud-resource-emr_cluster"
description: |-
  Provide a resource to create a emr cluster.
---

# tencentcloud_emr_cluster

Provide a resource to create a emr cluster.

## Example Usage

```hcl
resource "tencentcloud_emr_cluster" "emrrrr" {
  product_id       = 4
  display_strategy = "clusterList"
  vpc_settings = {
    vpc_id = "vpc-fuwly8x5"
    subnet_id : "subnet-d830wfso"
  }
  softwares     = ["hadoop-2.8.4", "zookeeper-3.4.9"]
  support_ha    = 0
  instance_name = "emr-test"
  resource_spec {
    master_resource_spec {
      mem_size     = 8192
      cpu          = 4
      disk_size    = 100
      disk_type    = "CLOUD_PREMIUM"
      spec         = "CVM.S2"
      storage_type = 5
    }
    core_resource_spec {
      mem_size     = 8192
      cpu          = 4
      disk_size    = 100
      disk_type    = "CLOUD_PREMIUM"
      spec         = "CVM.S2"
      storage_type = 5
    }
    master_count = 1
    core_count   = 2
  }
  login_settings = {
    password = "Tencent@cloud123"
  }
  time_span = 1
  time_unit = "m"
  pay_mode  = 1
  placement = {
    zone       = "ap-guangzhou-3"
    project_id = 0
  }
}
```

## Argument Reference

The following arguments are supported:

* `display_strategy` - (Required, String, ForceNew) Display strategy of EMR instance.
* `instance_name` - (Required, String, ForceNew) Name of the instance, which can contain 6 to 36 English letters, Chinese characters, digits, dashes(-), or underscores(_).
* `login_settings` - (Required, Map, ForceNew) Instance login settings.
* `pay_mode` - (Required, Int) The pay mode of instance. 0 is pay on an annual basis, 1 is pay on a measure basis.
* `placement` - (Required, Map, ForceNew) The location of the instance.
* `product_id` - (Required, Int, ForceNew) The product id of EMR instance.
* `softwares` - (Required, List: [`String`], ForceNew) The softwares of a EMR instance.
* `support_ha` - (Required, Int, ForceNew) The flag whether the instance support high availability.(0=>not support, 1=>support).
* `time_span` - (Required, Int) The length of time the instance was purchased. Use with TimeUnit.When TimeUnit is s, the parameter can only be filled in at 3600, representing a metered instance.
When TimeUnit is m, the number filled in by this parameter indicates the length of purchase of the monthly instance of the package year, such as 1 for one month of purchase.
* `time_unit` - (Required, String) The unit of time in which the instance was purchased. When PayMode is 0, TimeUnit can only take values of s(second). When PayMode is 1, TimeUnit can only take the value m(month).
* `vpc_settings` - (Required, Map, ForceNew) The private net config of EMR instance.
* `extend_fs_field` - (Optional, String) Access the external file system.
* `need_master_wan` - (Optional, String, ForceNew) Whether to enable the cluster Master node public network. Value range:
				- NEED_MASTER_WAN: Indicates that the cluster Master node public network is enabled.
				- NOT_NEED_MASTER_WAN: Indicates that it is not turned on.
				By default, the cluster Master node internet is enabled.
* `resource_spec` - (Optional, List) Resource specification of EMR instance.
* `sg_id` - (Optional, String, ForceNew) The ID of the security group to which the instance belongs, in the form of sg-xxxxxxxx.

The `resource_spec` object supports the following:

* `common_count` - (Optional, Int, ForceNew) The number of common node.
* `common_resource_spec` - (Optional, List, ForceNew) 
* `core_count` - (Optional, Int) The number of core node.
* `core_resource_spec` - (Optional, List, ForceNew) 
* `master_count` - (Optional, Int) The number of master node.
* `master_resource_spec` - (Optional, List, ForceNew) 
* `task_count` - (Optional, Int) The number of core node.
* `task_resource_spec` - (Optional, List, ForceNew) 

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the resource.
* `instance_id` - Created EMR instance id.


