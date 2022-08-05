/*
Use this data source to query the detail information of CDN domain.

Example Usage

```hcl
data "tencentcloud_cdn_domains" "foo" {
  domain         	   = "xxxx.com"
  service_type   	   = "web"
  full_url_cache 	   = false
  origin_pull_protocol = "follow"
  https_switch		   = "on"
}
```
*/
package tencentcloud

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	cdn "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
)

func dataSourceTencentCloudCdnDomains() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTencentCloudCdnDomainsRead,
		Schema: map[string]*schema.Schema{
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Acceleration domain name.",
			},
			"service_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateAllowedStringValue(CDN_SERVICE_TYPE),
				Description:  "Service type of acceleration domain name. The available value include `web`, `download` and `media`.",
			},
			"full_url_cache": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to enable full-path cache.",
			},
			"origin_pull_protocol": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateAllowedStringValue(CDN_ORIGIN_PULL_PROTOCOL),
				Description:  "Origin-pull protocol configuration. Valid values: `http`, `https` and `follow`.",
			},
			"https_switch": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateAllowedStringValue(CDN_HTTPS_SWITCH),
				Description:  "HTTPS configuration. Valid values: `on`, `off` and `processing`.",
			},
			"result_output_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Used to save results.",
			},
			"domain_list": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "An information list of cdn domain. Each element contains the following attributes:",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Domain name ID.",
						},
						"domain": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Acceleration domain name.",
						},
						"cname": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "CNAME address of domain name.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Acceleration service status.",
						},
						"create_time": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Domain name creation time.",
						},
						"update_time": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Last modified time of domain name.",
						},
						"service_type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Service type of acceleration domain name.",
						},
						"area": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Acceleration region.",
						},
						"project_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The project CDN belongs to.",
						},
						"full_url_cache": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether to enable full-path cache.",
						},
						"range_origin_switch": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Sharding back to source configuration switch.",
						},
						"request_header": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Request header configuration.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Custom request header configuration switch.",
									},
									"header_rules": {
										Type:        schema.TypeList,
										Computed:    true,
										Description: "Custom request header configuration rules.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"header_mode": {
													Type:        schema.TypeString,
													Computed:    true,
													Description: "Http header setting method.",
												},
												"header_name": {
													Type:        schema.TypeString,
													Computed:    true,
													Description: "Http header name.",
												},
												"header_value": {
													Type:        schema.TypeString,
													Computed:    true,
													Description: "Http header value.",
												},
												"rule_type": {
													Type:        schema.TypeString,
													Computed:    true,
													Description: "Rule type.",
												},
												"rule_paths": {
													Type:        schema.TypeList,
													Computed:    true,
													Elem:        &schema.Schema{Type: schema.TypeString},
													Description: "Rule paths.",
												},
											},
										},
									},
								},
							},
						},
						"rule_cache": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Advanced path cache configuration.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"rule_paths": {
										Type:     schema.TypeList,
										Computed: true,
										Elem: &schema.Schema{
											Type: schema.TypeString,
										},
										Description: "Rule paths.",
									},
									"rule_type": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Rule type.",
									},
									"switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Cache configuration switch.",
									},
									"cache_time": {
										Type:        schema.TypeInt,
										Required:    true,
										Description: "Cache expiration time setting, the unit is second.",
									},
									"compare_max_age": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Advanced cache expiration configuration.",
									},
									"ignore_cache_control": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Force caching. After opening, the no-store and no-cache resources returned by the origin site will also be cached in accordance with the CacheRules rules.",
									},
									"ignore_set_cookie": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Ignore the Set-Cookie header of the origin site.",
									},
									"no_cache_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Cache configuration switch.",
									},
									"re_validate": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Always check back to origin.",
									},
									"follow_origin_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Follow the source station configuration switch.",
									},
								},
							},
						},
						"origin": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Origin server configuration.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"origin_type": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Master origin server type.",
									},
									"origin_list": {
										Type:        schema.TypeList,
										Computed:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "Master origin server list.",
									},
									"backup_origin_type": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Backup origin server type.",
									},
									"backup_origin_list": {
										Type:        schema.TypeList,
										Computed:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "Backup origin server list.",
									},
									"backup_server_name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Host header used when accessing the backup origin server. If left empty, the ServerName of master origin server will be used by default.",
									},
									"server_name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Host header used when accessing the master origin server. If left empty, the acceleration domain name will be used by default.",
									},
									"cos_private_access": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "When OriginType is COS, you can specify if access to private buckets is allowed.",
									},
									"origin_pull_protocol": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Origin-pull protocol configuration.",
									},
								},
							},
						},
						"https_config": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "HTTPS acceleration configuration. It's a list and consist of at most one item.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"https_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "HTTPS configuration switch.",
									},
									"http2_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "HTTP2 configuration switch.",
									},
									"ocsp_stapling_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "OCSP configuration switch.",
									},
									"spdy_switch": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Spdy configuration switch.",
									},
									"verify_client": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Client certificate authentication feature.",
									},
								},
							},
						},
						"tags": {
							Type:        schema.TypeMap,
							Computed:    true,
							Description: "Tags of cdn domain.",
						},
					},
				},
			},
		},
	}
}

func dataSourceTencentCloudCdnDomainsRead(d *schema.ResourceData, meta interface{}) error {
	defer logElapsed("data_source.tencentcloud_cdn_domain.read")()
	var (
		logId         = getLogId(contextNil)
		ctx           = context.WithValue(context.TODO(), logIdKey, logId)
		domainConfigs []*cdn.DetailDomain
		err           error

		client     = meta.(*TencentCloudClient).apiV3Conn
		region     = client.Region
		cdnService = CdnService{client: client}
		tagService = TagService{client: client}
	)

	var domainFilterMap = make(map[string]interface{}, 5)
	if v, ok := d.GetOk("domain"); ok {
		domainFilterMap["domain"] = v.(string)
	}
	if v, ok := d.GetOk("service_type"); ok {
		domainFilterMap["serviceType"] = v.(string)
	}
	if v, ok := d.GetOk("https_switch"); ok {
		domainFilterMap["httpsSwitch"] = v.(string)
	}
	if v, ok := d.GetOk("origin_pull_protocol"); ok {
		domainFilterMap["originPullProtocol"] = v.(string)
	}
	if v, ok := d.GetOkExists("full_url_cache"); ok {
		var value string
		if v.(bool) {
			value = "on"
		} else {
			value = "off"
		}

		domainFilterMap["fullUrlCache"] = value
	}

	domainConfigs, err = cdnService.DescribeDomainsConfigByFilters(ctx, domainFilterMap)
	if err != nil {
		log.Printf("[CRITAL]%s describeDomainsConfigByFilters fail, reason:%v ", logId, err)
		return err
	}

	cdnDomainList := make([]map[string]interface{}, 0, len(domainConfigs))
	ids := make([]string, 0, len(domainConfigs))
	for _, detailDomain := range domainConfigs {
		var fullUrlCache bool
		if detailDomain.CacheKey != nil && *detailDomain.CacheKey.FullUrlCache == CDN_SWITCH_ON {
			fullUrlCache = true
		}

		requestHeaders := make([]map[string]interface{}, 0, 1)
		requestHeader := make(map[string]interface{})
		requestHeader["switch"] = detailDomain.RequestHeader.Switch
		if len(detailDomain.RequestHeader.HeaderRules) > 0 {
			headerRules := make([]map[string]interface{}, len(detailDomain.RequestHeader.HeaderRules))
			headerRuleList := detailDomain.RequestHeader.HeaderRules
			for index, value := range headerRuleList {
				headerRule := make(map[string]interface{})
				headerRule["header_mode"] = value.HeaderMode
				headerRule["header_name"] = value.HeaderName
				headerRule["header_value"] = value.HeaderValue
				headerRule["rule_type"] = value.RuleType
				headerRule["rule_paths"] = value.RulePaths
				headerRules[index] = headerRule
			}
			requestHeader["header_rules"] = headerRules
		}
		requestHeaders = append(requestHeaders, requestHeader)

		ruleCaches := make([]map[string]interface{}, len(detailDomain.Cache.RuleCache))
		for index, value := range detailDomain.Cache.RuleCache {
			ruleCache := make(map[string]interface{})
			ruleCache["rule_paths"] = value.RulePaths
			ruleCache["rule_type"] = value.RuleType
			ruleCache["switch"] = value.CacheConfig.Cache.Switch
			ruleCache["cache_time"] = value.CacheConfig.Cache.CacheTime
			ruleCache["compare_max_age"] = value.CacheConfig.Cache.CompareMaxAge
			ruleCache["ignore_cache_control"] = value.CacheConfig.Cache.IgnoreCacheControl
			ruleCache["ignore_set_cookie"] = value.CacheConfig.Cache.IgnoreSetCookie
			ruleCache["no_cache_switch"] = value.CacheConfig.NoCache.Switch
			ruleCache["re_validate"] = value.CacheConfig.NoCache.Revalidate
			ruleCache["follow_origin_switch"] = value.CacheConfig.FollowOrigin.Switch
			ruleCaches[index] = ruleCache
		}

		origins := make([]map[string]interface{}, 0, 1)
		origin := make(map[string]interface{}, 8)
		origin["origin_type"] = detailDomain.Origin.OriginType
		origin["origin_list"] = detailDomain.Origin.Origins
		origin["backup_origin_type"] = detailDomain.Origin.BackupOriginType
		origin["backup_origin_list"] = detailDomain.Origin.BackupOrigins
		origin["backup_server_name"] = detailDomain.Origin.BackupServerName
		origin["server_name"] = detailDomain.Origin.ServerName
		origin["cos_private_access"] = detailDomain.Origin.CosPrivateAccess
		origin["origin_pull_protocol"] = detailDomain.Origin.OriginPullProtocol
		origins = append(origins, origin)

		httpsconfigs := make([]map[string]interface{}, 0, 1)
		if detailDomain.Https != nil {
			httpsConfig := make(map[string]interface{}, 7)
			httpsConfig["https_switch"] = detailDomain.Https.Switch
			httpsConfig["http2_switch"] = detailDomain.Https.Http2
			httpsConfig["ocsp_stapling_switch"] = detailDomain.Https.OcspStapling
			httpsConfig["spdy_switch"] = detailDomain.Https.Spdy
			httpsConfig["verify_client"] = detailDomain.Https.VerifyClient
			httpsconfigs = append(httpsconfigs, httpsConfig)
		}

		tags, errRet := tagService.DescribeResourceTags(ctx, CDN_SERVICE_NAME, CDN_RESOURCE_NAME_DOMAIN, region, *detailDomain.Domain)
		if errRet != nil {
			return errRet
		}

		mapping := map[string]interface{}{
			"id":                  detailDomain.ResourceId,
			"domain":              detailDomain.Domain,
			"cname":               detailDomain.Cname,
			"status":              detailDomain.Status,
			"create_time":         detailDomain.CreateTime,
			"update_time":         detailDomain.UpdateTime,
			"service_type":        detailDomain.ServiceType,
			"area":                detailDomain.Area,
			"project_id":          detailDomain.ProjectId,
			"full_url_cache":      fullUrlCache,
			"range_origin_switch": detailDomain.RangeOriginPull.Switch,
			"request_header":      requestHeaders,
			"rule_cache":          ruleCaches,
			"origin":              origins,
			"https_config":        httpsconfigs,
			"tags":                tags,
		}

		cdnDomainList = append(cdnDomainList, mapping)
		ids = append(ids, *detailDomain.ResourceId)
	}

	d.SetId(helper.DataResourceIdsHash(ids))
	err = d.Set("domain_list", cdnDomainList)
	if err != nil {
		log.Printf("[CRITAL]%s provider set cdn domain list fail, reason:%v ", logId, err)
		return err
	}
	output, ok := d.GetOk("result_output_file")
	if ok && output.(string) != "" {
		if err := writeToFile(output.(string), cdnDomainList); err != nil {
			return err
		}
	}
	return nil
}
