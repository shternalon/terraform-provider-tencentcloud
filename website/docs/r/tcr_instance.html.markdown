---
subcategory: "Tencent Container Registry(TCR)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_tcr_instance"
sidebar_current: "docs-tencentcloud-resource-tcr_instance"
description: |-
  Use this resource to create tcr instance.
---

# tencentcloud_tcr_instance

Use this resource to create tcr instance.

## Example Usage

```hcl
resource "tencentcloud_tcr_instance" "foo" {
  name          = "example"
  instance_type = "basic"

  tags = {
    test = "tf"
  }
}
```

Using public network access whitelist

```hcl
resource "tencentcloud_tcr_instance" "foo" {
  name                  = "example"
  instance_type         = "basic"
  open_public_operation = true
  security_policy {
    cidr_block = "10.0.0.1/24"
  }
  security_policy {
    cidr_block = "192.168.1.1"
  }
}
```

## Argument Reference

The following arguments are supported:

* `instance_type` - (Required, String, ForceNew) TCR types. Valid values are: `standard`, `basic`, `premium`.
* `name` - (Required, String, ForceNew) Name of the TCR instance.
* `delete_bucket` - (Optional, Bool) Indicate to delete the COS bucket which is auto-created with the instance or not.
* `open_public_operation` - (Optional, Bool) Control public network access.
* `security_policy` - (Optional, Set) Public network access allowlist policies of the TCR instance. Only available when `open_public_operation` is `true`.
* `tags` - (Optional, Map) The available tags within this TCR instance.

The `security_policy` object supports the following:

* `cidr_block` - (Optional, String) The public network IP address of the access source.
* `description` - (Optional, String) Remarks of policy.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the resource.
* `internal_end_point` - Internal address for access of the TCR instance.
* `public_domain` - Public address for access of the TCR instance.
* `public_status` - Status of the TCR instance public network access.
* `status` - Status of the TCR instance.


## Import

tcr instance can be imported using the id, e.g.

```
$ terraform import tencentcloud_tcr_instance.foo cls-cda1iex1
```

