/*
Use this data source to query API gateway throttling APIs.

Example Usage

```hcl
resource "tencentcloud_api_gateway_service" "service" {
  	service_name = "niceservice"
  	protocol     = "http&https"
  	service_desc = "your nice service"
  	net_type     = ["INNER", "OUTER"]
  	ip_version   = "IPv4"
}

resource "tencentcloud_api_gateway_api" "api" {
    service_id            = tencentcloud_api_gateway_service.service.id
    api_name              = "hello_update"
    api_desc              = "my hello api update"
    auth_type             = "SECRET"
    protocol              = "HTTP"
    enable_cors           = true
    request_config_path   = "/user/info"
    request_config_method = "POST"
    request_parameters {
    	name          = "email"
        position      = "QUERY"
        type          = "string"
        desc          = "your email please?"
        default_value = "tom@qq.com"
        required      = true
    }
    service_config_type      = "HTTP"
    service_config_timeout   = 10
    service_config_url       = "http://www.tencent.com"
    service_config_path      = "/user"
    service_config_method    = "POST"
    response_type            = "XML"
    response_success_example = "<note>success</note>"
    response_fail_example    = "<note>fail</note>"
    response_error_codes {
    	code           = 10
        msg            = "system error"
       	desc           = "system error code"
       	converted_code = -10
        need_convert   = true
	}

	release_limit    = 100
	pre_limit        = 100
	test_limit       = 100
}

data "tencentcloud_api_gateway_throttling_apis" "id" {
    service_id = tencentcloud_api_gateway_api.service_id
}

data "tencentcloud_api_gateway_throttling_apis" "foo" {
	service_id        = tencentcloud_api_gateway_api.service.service_id
	environment_names = ["release", "test"]
}
```
*/
package tencentcloud

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	apigateway "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/apigateway/v20180808"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
)

func dataSourceTencentCloudAPIGatewayThrottlingApis() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTencentCloudAPIGatewayThrottlingApisRead,

		Schema: map[string]*schema.Schema{
			"service_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Unique service ID of API.",
			},
			"environment_names": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Environment list.",
			},
			"result_output_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used to save results.",
			},
			//compute
			"list": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "A list of policies bound to API.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"service_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Unique service ID of API.",
						},
						"api_environment_strategies": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "List of throttling policies bound to API.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"api_id": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Unique API ID.",
									},
									"api_name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Custom API name.",
									},
									"path": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "API path.",
									},
									"method": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "API method.",
									},
									"strategy_list": {
										Type:        schema.TypeList,
										Computed:    true,
										Description: "Environment throttling information.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"environment_name": {
													Type:        schema.TypeString,
													Computed:    true,
													Description: "Environment name.",
												},
												"quota": {
													Type:        schema.TypeInt,
													Computed:    true,
													Description: "Throttling value.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceTencentCloudAPIGatewayThrottlingApisRead(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("data_source.tencentcloud_api_gateway_throttling_apis.read")()

	var (
		logId             = getLogId(contextNil)
		ctx               = context.WithValue(context.TODO(), logIdKey, logId)
		apiGatewayService = APIGatewayService{client: meta.(*TencentCloudClient).apiV3Conn}
		infos             []*apigateway.Service
		serviceID         string
		environmentNames  []string
		err               error
		serviceIds        = make([]string, 0)
		resultLists       = make([]map[string]interface{}, 0)
		ids               = make([]string, 0)
	)
	if v, ok := d.GetOk("service_id"); ok {
		serviceID = v.(string)
	}
	if v, ok := d.GetOk("environment_names"); ok {
		environmentNames = helper.InterfacesStrings(v.([]interface{}))
	}

	if serviceID == "" {
		err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			infos, err = apiGatewayService.DescribeServicesStatus(ctx, "", "")
			if err != nil {
				return retryError(err, InternalError)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, result := range infos {
			if result.ServiceId == nil {
				continue
			}
			serviceIds = append(serviceIds, *result.ServiceId)
		}
	} else {
		serviceIds = append(serviceIds, serviceID)
	}

	for _, serviceIdTmp := range serviceIds {
		environmentList, err := apiGatewayService.DescribeApiEnvironmentStrategyList(ctx, serviceIdTmp, environmentNames, "")
		if err != nil {
			return err
		}

		environmentResults := make([]map[string]interface{}, 0, len(environmentList))
		for _, envList := range environmentList {
			environmentSet := envList.EnvironmentStrategySet
			strategyList := make([]map[string]interface{}, 0, len(environmentSet))
			for _, envSet := range environmentSet {
				if envSet == nil {
					continue
				}
				strategyList = append(strategyList, map[string]interface{}{
					"environment_name": envSet.EnvironmentName,
					"quota":            envSet.Quota,
				})
			}

			item := map[string]interface{}{
				"api_id":        envList.ApiId,
				"api_name":      envList.ApiName,
				"path":          envList.Path,
				"method":        envList.Method,
				"strategy_list": strategyList,
			}
			environmentResults = append(environmentResults, item)
		}
		resultLists = append(resultLists, map[string]interface{}{
			"service_id":                 serviceIdTmp,
			"api_environment_strategies": environmentResults,
		})
		ids = append(ids, serviceIdTmp)
	}

	d.SetId(helper.DataResourceIdsHash(ids))
	if err = d.Set("list", resultLists); err != nil {
		log.Printf("[CRITAL]%s provider set list fail, reason:%s", logId, err.Error())
		return err
	}

	output, ok := d.GetOk("result_output_file")
	if ok && output.(string) != "" {
		if err := writeToFile(output.(string), resultLists); err != nil {
			return err
		}
	}
	return nil
}
