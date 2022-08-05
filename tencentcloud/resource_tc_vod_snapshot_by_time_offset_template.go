/*
Provide a resource to create a VOD snapshot by time offset template.

Example Usage

```hcl
resource "tencentcloud_vod_snapshot_by_time_offset_template" "foo" {
  name                = "tf-snapshot"
  width               = 130
  height              = 128
  resolution_adaptive = false
  format              = "png"
  comment             = "test"
  fill_type           = "white"
}
```

Import

VOD snapshot by time offset template can be imported using the id, e.g.

```
$ terraform import tencentcloud_vod_snapshot_by_time_offset_template.foo 46906
```
*/
package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	vod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vod/v20180717"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

func resourceTencentCloudVodSnapshotByTimeOffsetTemplate() *schema.Resource {
	return &schema.Resource{
		Create: resourceTencentCloudVodSnapshotByTimeOffsetTemplateCreate,
		Read:   resourceTencentCloudVodSnapshotByTimeOffsetTemplateRead,
		Update: resourceTencentCloudVodSnapshotByTimeOffsetTemplateUpdate,
		Delete: resourceTencentCloudVodSnapshotByTimeOffsetTemplateDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateStringLengthInRange(1, 64),
				Description:  "Name of a time point screen capturing template. Length limit: 64 characters.",
			},
			"width": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Maximum value of the `width` (or long side) of a screenshot in px. Value range: 0 and [128, 4,096]. If both `width` and `height` are `0`, the resolution will be the same as that of the source video; If `width` is `0`, but `height` is not `0`, width will be proportionally scaled; If `width` is not `0`, but `height` is `0`, `height` will be proportionally scaled; If both `width` and `height` are not `0`, the custom resolution will be used. Default value: `0`.",
			},
			"height": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Maximum value of the `height` (or short side) of a screenshot in px. Value range: 0 and [128, 4,096]. If both `width` and `height` are `0`, the resolution will be the same as that of the source video; If `width` is `0`, but `height` is not `0`, `width` will be proportionally scaled; If `width` is not `0`, but `height` is `0`, `height` will be proportionally scaled; If both `width` and `height` are not `0`, the custom resolution will be used. Default value: `0`.",
			},
			"resolution_adaptive": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Resolution adaption. Valid values: `true`,`false`. `true`: enabled. In this case, `width` represents the long side of a video, while `height` the short side; `false`: disabled. In this case, `width` represents the width of a video, while `height` the height. Default value: `true`.",
			},
			"format": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Image format. Valid values: `jpg`, `png`. Default value: `jpg`.",
			},
			"comment": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateStringLengthInRange(1, 256),
				Description:  "Template description. Length limit: 256 characters.",
			},
			"sub_app_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Subapplication ID in VOD. If you need to access a resource in a subapplication, enter the subapplication ID in this field; otherwise, leave it empty.",
			},
			"fill_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "black",
				Description: "Fill refers to the way of processing a screenshot when its aspect ratio is different from that of the source video. The following fill types are supported: `stretch`: stretch. The screenshot will be stretched frame by frame to match the aspect ratio of the source video, which may make the screenshot `shorter` or `longer`; `black`: fill with black. This option retains the aspect ratio of the source video for the screenshot and fills the unmatched area with black color blocks. `white`: fill with white. This option retains the aspect ratio of the source video for the screenshot and fills the unmatched area with white color blocks. `gauss`: fill with Gaussian blur. This option retains the aspect ratio of the source video for the screenshot and fills the unmatched area with Gaussian blur. Default value: `black`.",
			},
			// computed
			"create_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Creation time of template in ISO date format.",
			},
			"update_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Last modified time of template in ISO date format.",
			},
		},
	}
}

func resourceTencentCloudVodSnapshotByTimeOffsetTemplateCreate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_vod_snapshot_by_time_offset_template.create")()

	var (
		logId   = getLogId(contextNil)
		request = vod.NewCreateSnapshotByTimeOffsetTemplateRequest()
	)

	request.Name = helper.String(d.Get("name").(string))
	request.Width = helper.IntUint64(d.Get("width").(int))
	request.Height = helper.IntUint64(d.Get("height").(int))
	request.ResolutionAdaptive = helper.String(RESOLUTION_ADAPTIVE_TO_STRING[d.Get("resolution_adaptive").(bool)])
	if v, ok := d.GetOk("format"); ok {
		request.Format = helper.String(v.(string))
	}
	if v, ok := d.GetOk("comment"); ok {
		request.Comment = helper.String(v.(string))
	}
	if v, ok := d.GetOk("sub_app_id"); ok {
		request.SubAppId = helper.IntUint64(v.(int))
	}
	request.FillType = helper.String(d.Get("fill_type").(string))

	var response *vod.CreateSnapshotByTimeOffsetTemplateResponse
	var err error
	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, err = meta.(*TencentCloudClient).apiV3Conn.UseVodClient().CreateSnapshotByTimeOffsetTemplate(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, reason:%s", logId, request.GetAction(), err.Error())
			return retryError(err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if response == nil || response.Response == nil {
		return fmt.Errorf("for vod snapshot by time offset template creation, response is nil")
	}
	d.SetId(strconv.FormatUint(*response.Response.Definition, 10))

	return resourceTencentCloudVodSnapshotByTimeOffsetTemplateRead(d, meta)
}

func resourceTencentCloudVodSnapshotByTimeOffsetTemplateRead(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_vod_snapshot_by_time_offset_template.read")()
	defer inconsistentCheck(d, meta)()

	var (
		logId      = getLogId(contextNil)
		ctx        = context.WithValue(context.TODO(), logIdKey, logId)
		id         = d.Id()
		subAppId   = d.Get("sub_app_id").(int)
		client     = meta.(*TencentCloudClient).apiV3Conn
		vodService = VodService{client: client}
	)
	// waiting for refreshing cache
	time.Sleep(30 * time.Second)
	template, has, err := vodService.DescribeSnapshotByTimeOffsetTemplatesById(ctx, id, subAppId)
	if err != nil {
		return err
	}
	if !has {
		d.SetId("")
		return nil
	}

	_ = d.Set("name", template.Name)
	_ = d.Set("width", template.Width)
	_ = d.Set("height", template.Height)
	_ = d.Set("resolution_adaptive", *template.ResolutionAdaptive == "open")
	_ = d.Set("format", template.Format)
	_ = d.Set("comment", template.Comment)
	_ = d.Set("fill_type", template.FillType)
	_ = d.Set("create_time", template.CreateTime)
	_ = d.Set("update_time", template.UpdateTime)

	return nil
}

func resourceTencentCloudVodSnapshotByTimeOffsetTemplateUpdate(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_vod_snapshot_by_time_offset_template.update")()

	var (
		logId      = getLogId(contextNil)
		request    = vod.NewModifySnapshotByTimeOffsetTemplateRequest()
		id         = d.Id()
		changeFlag = false
	)

	idUint, _ := strconv.ParseUint(id, 0, 64)
	request.Definition = &idUint
	if d.HasChange("name") {
		changeFlag = true
		request.Name = helper.String(d.Get("name").(string))
	}
	if d.HasChange("width") || d.HasChange("height") || d.HasChange("resolution_adaptive") {
		changeFlag = true
		request.Width = helper.IntUint64(d.Get("width").(int))
		request.Height = helper.IntUint64(d.Get("height").(int))
		request.ResolutionAdaptive = helper.String(RESOLUTION_ADAPTIVE_TO_STRING[d.Get("resolution_adaptive").(bool)])
	}
	if d.HasChange("format") {
		changeFlag = true
		request.Format = helper.String(d.Get("format").(string))
	}
	if d.HasChange("comment") {
		changeFlag = true
		request.Comment = helper.String(d.Get("comment").(string))
	}
	if d.HasChange("sub_app_id") {
		changeFlag = true
		request.SubAppId = helper.IntUint64(d.Get("sub_app_id").(int))
	}
	if d.HasChange("fill_type") {
		changeFlag = true
		request.FillType = helper.String(d.Get("fill_type").(string))
	}

	if changeFlag {
		var err error
		err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())
			_, err = meta.(*TencentCloudClient).apiV3Conn.UseVodClient().ModifySnapshotByTimeOffsetTemplate(request)
			if err != nil {
				log.Printf("[CRITAL]%s api[%s] fail, reason:%s", logId, request.GetAction(), err.Error())
				return retryError(err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		return resourceTencentCloudVodSnapshotByTimeOffsetTemplateRead(d, meta)
	}

	return nil
}

func resourceTencentCloudVodSnapshotByTimeOffsetTemplateDelete(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("resource.tencentcloud_vod_snapshot_by_time_offset_template.delete")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	id := d.Id()
	vodService := VodService{
		client: meta.(*TencentCloudClient).apiV3Conn,
	}

	if err := vodService.DeleteSnapshotByTimeOffsetTemplate(ctx, id, uint64(d.Get("sub_app_id").(int))); err != nil {
		return err
	}

	return nil
}
