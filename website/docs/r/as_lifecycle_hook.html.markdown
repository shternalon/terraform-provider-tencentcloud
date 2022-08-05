---
subcategory: "Auto Scaling(AS)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_as_lifecycle_hook"
sidebar_current: "docs-tencentcloud-resource-as_lifecycle_hook"
description: |-
  Provides a resource for an AS (Auto scaling) lifecycle hook.
---

# tencentcloud_as_lifecycle_hook

Provides a resource for an AS (Auto scaling) lifecycle hook.

## Example Usage

```hcl
resource "tencentcloud_as_lifecycle_hook" "lifecycle_hook" {
  scaling_group_id         = "sg-12af45"
  lifecycle_hook_name      = "tf-as-lifecycle-hook"
  lifecycle_transition     = "INSTANCE_LAUNCHING"
  default_result           = "CONTINUE"
  heartbeat_timeout        = 500
  notification_metadata    = "tf test"
  notification_target_type = "CMQ_QUEUE"
  notification_queue_name  = "lifcyclehook"
}
```

## Argument Reference

The following arguments are supported:

* `lifecycle_hook_name` - (Required, String) The name of the lifecycle hook.
* `lifecycle_transition` - (Required, String) The instance state to which you want to attach the lifecycle hook. Valid values: `INSTANCE_LAUNCHING` and `INSTANCE_TERMINATING`.
* `scaling_group_id` - (Required, String, ForceNew) ID of a scaling group.
* `default_result` - (Optional, String) Defines the action the AS group should take when the lifecycle hook timeout elapses or if an unexpected failure occurs. Valid values: `CONTINUE` and `ABANDON`. The default value is `CONTINUE`.
* `heartbeat_timeout` - (Optional, Int) Defines the amount of time, in seconds, that can elapse before the lifecycle hook times out. Valid value ranges: (30~7200). and default value is `300`.
* `notification_metadata` - (Optional, String) Contains additional information that you want to include any time AS sends a message to the notification target.
* `notification_queue_name` - (Optional, String) For CMQ_QUEUE type, a name of queue must be set.
* `notification_target_type` - (Optional, String) Target type. Valid values: `CMQ_QUEUE`, `CMQ_TOPIC`.
* `notification_topic_name` - (Optional, String) For CMQ_TOPIC type, a name of topic must be set.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the resource.



