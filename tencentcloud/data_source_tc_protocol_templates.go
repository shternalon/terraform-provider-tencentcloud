/*
Use this data source to query detailed information of protocol templates.

Example Usage

```hcl
data "tencentcloud_protocol_templates" "name" {
  name       = "test"
}
```
*/
package tencentcloud

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
)

func dataSourceTencentCloudProtocolTemplates() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTencentCloudProtocolTemplatesRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the protocol template to query.",
			},
			"id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the protocol template to query.",
			},
			"result_output_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used to save results.",
			},

			// Computed values
			"template_list": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Information list of the dedicated protocol templates.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "ID of the protocol template.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of protocol template.",
						},
						"protocols": {
							Type: schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Computed:    true,
							Description: "Set of the protocols.",
						},
					},
				},
			},
		},
	}
}

func dataSourceTencentCloudProtocolTemplatesRead(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("data_source.tencentcloud_protocol_templates.read")()

	logId := getLogId(contextNil)
	ctx := context.WithValue(context.TODO(), logIdKey, logId)

	var name, templateId string
	var filters = make([]*vpc.Filter, 0)
	if v, ok := d.GetOk("name"); ok {
		name = v.(string)
		filters = append(filters, &vpc.Filter{Name: helper.String("service-template-name"), Values: []*string{&name}})
	}

	if v, ok := d.GetOk("id"); ok {
		templateId = v.(string)
		filters = append(filters, &vpc.Filter{Name: helper.String("service-template-id"), Values: []*string{&templateId}})
	}

	vpcService := VpcService{client: meta.(*TencentCloudClient).apiV3Conn}
	var outErr, inErr error
	templates, outErr := vpcService.DescribeServiceTemplates(ctx, filters)
	if outErr != nil {
		outErr = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			templates, inErr = vpcService.DescribeServiceTemplates(ctx, filters)
			if inErr != nil {
				return retryError(inErr)
			}
			return nil
		})
	}

	if outErr != nil {
		return outErr
	}

	ids := make([]string, 0, len(templates))
	templateList := make([]map[string]interface{}, 0, len(templates))
	for _, ins := range templates {
		mapping := map[string]interface{}{
			"id":        ins.ServiceTemplateId,
			"name":      ins.ServiceTemplateName,
			"protocols": ins.ServiceSet,
		}
		templateList = append(templateList, mapping)
		ids = append(ids, *ins.ServiceTemplateId)
	}
	d.SetId(helper.DataResourceIdsHash(ids))
	if e := d.Set("template_list", templateList); e != nil {
		log.Printf("[CRITAL]%s provider set protocol template list fail, reason:%s\n", logId, e)
		return e
	}

	output, ok := d.GetOk("result_output_file")
	if ok && output.(string) != "" {
		if e := writeToFile(output.(string), templateList); e != nil {
			return e
		}
	}

	return nil

}
