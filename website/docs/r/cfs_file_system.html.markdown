---
subcategory: "Cloud File Storage(CFS)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_cfs_file_system"
sidebar_current: "docs-tencentcloud-resource-cfs_file_system"
description: |-
  Provides a resource to create a cloud file system(CFS).
---

# tencentcloud_cfs_file_system

Provides a resource to create a cloud file system(CFS).

## Example Usage

```hcl
resource "tencentcloud_cfs_file_system" "foo" {
  name              = "test_file_system"
  availability_zone = "ap-guangzhou-3"
  access_group_id   = "pgroup-7nx89k7l"
  protocol          = "NFS"
  vpc_id            = "vpc-ah9fbkap"
  subnet_id         = "subnet-9mu2t9iw"
}
```

## Argument Reference

The following arguments are supported:

* `access_group_id` - (Required, String) ID of a access group.
* `availability_zone` - (Required, String, ForceNew) The available zone that the file system locates at.
* `subnet_id` - (Required, String, ForceNew) ID of a subnet.
* `vpc_id` - (Required, String, ForceNew) ID of a VPC network.
* `mount_ip` - (Optional, String, ForceNew) IP of mount point.
* `name` - (Optional, String) Name of a file system.
* `protocol` - (Optional, String, ForceNew) File service protocol. Valid values are `NFS` and `CIFS`. and the default is `NFS`.
* `storage_type` - (Optional, String, ForceNew) File service StorageType. Valid values are `SD` and `HP`. and the default is `SD`.
* `tags` - (Optional, Map) Instance tags.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the resource.
* `create_time` - Create time of the file system.


## Import

Cloud file system can be imported using the id, e.g.

```
$ terraform import tencentcloud_cfs_file_system.foo cfs-6hgquxmj
```

