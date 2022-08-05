package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	monitor "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/monitor/v20180724"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

type MonitorService struct {
	client *connectivity.TencentCloudClient
}

func (me *MonitorService) CheckCanCreateMysqlROInstance(ctx context.Context, mysqlId string) (can bool, errRet error) {

	logId := getLogId(ctx)

	loc, err := time.LoadLocation("Asia/Chongqing")
	if err != nil {
		errRet = fmt.Errorf("Can not load  time zone `Asia/Chongqing`, reason %s", err.Error())
		return
	}

	request := monitor.NewGetMonitorDataRequest()

	request.Namespace = helper.String("QCE/CDB")
	request.MetricName = helper.String("RealCapacity")
	request.Period = helper.Uint64(60)

	now := time.Now()
	request.StartTime = helper.String(now.Add(-5 * time.Minute).In(loc).Format("2006-01-02T15:04:05+08:00"))
	request.EndTime = helper.String(now.In(loc).Format("2006-01-02T15:04:05+08:00"))

	request.Instances = []*monitor.Instance{
		{
			Dimensions: []*monitor.Dimension{{
				Name:  helper.String("InstanceId"),
				Value: &mysqlId,
			}},
		},
	}
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().GetMonitorData(request)
	if err != nil {
		errRet = err
		return
	}

	if len(response.Response.DataPoints) == 0 {
		return
	}
	dataPoint := response.Response.DataPoints[0]
	if len(dataPoint.Values) == 0 {
		return
	}
	can = true
	return
}

func (me *MonitorService) FullRegions() (regions []string, errRet error) {
	request := cvm.NewDescribeRegionsRequest()
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		if response, err := me.client.UseCvmClient().DescribeRegions(request); err != nil {
			return retryError(err, InternalError)
		} else {
			for _, region := range response.Response.RegionSet {
				regions = append(regions, *region.Region)
			}
		}
		return nil
	}); err != nil {
		errRet = err
		return
	}
	return
}

func (me *MonitorService) DescribePolicyGroupDetailInfo(ctx context.Context, groupId int64) (response *monitor.DescribePolicyGroupInfoResponse, errRet error) {

	var (
		request = monitor.NewDescribePolicyGroupInfoRequest()
		err     error
	)
	request.GroupId = &groupId
	request.Module = helper.String("monitor")

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		if response, err = me.client.UseMonitorClient().DescribePolicyGroupInfo(request); err != nil {
			return retryError(err, InternalError)
		}
		return nil
	}); err != nil {
		errRet = err
		return
	}
	return
}

func (me *MonitorService) DescribeAlarmPolicyById(ctx context.Context, policyId string) (info *monitor.AlarmPolicy, errRet error) {

	var (
		request = monitor.NewDescribeAlarmPolicyRequest()
	)
	logId := getLogId(ctx)
	request.Module = helper.String("monitor")
	request.PolicyId = &policyId

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DescribeAlarmPolicy(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	if response.Response.Policy == nil {
		return
	}
	info = response.Response.Policy
	return
}

func (me *MonitorService) DescribePolicyGroup(ctx context.Context, groupId int64) (info *monitor.DescribePolicyGroupListGroup, errRet error) {

	var (
		request       = monitor.NewDescribePolicyGroupListRequest()
		offset  int64 = 0
		limit   int64 = 20
		finish  bool
	)
	request.Module = helper.String("monitor")
	request.Offset = &offset
	request.Limit = &limit

	for {
		if finish {
			break
		}
		if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())
			response, err := me.client.UseMonitorClient().DescribePolicyGroupList(request)
			if err != nil {
				return retryError(err, InternalError)
			}
			if len(response.Response.GroupList) < int(limit) {
				finish = true
			}
			for _, v := range response.Response.GroupList {
				if *v.GroupId == groupId {
					info = v
					return nil
				}
			}
			return nil
		}); err != nil {
			errRet = err
			return
		}
		if info != nil {
			return
		}
		offset = offset + limit
	}
	return
}

func (me *MonitorService) DescribeBindingPolicyObjectList(ctx context.Context, groupId int64) (objects []*monitor.DescribeBindingPolicyObjectListInstance, errRet error) {

	var (
		requestList  = monitor.NewDescribeBindingPolicyObjectListRequest()
		responseList *monitor.DescribeBindingPolicyObjectListResponse
		offset       int64 = 0
		limit        int64 = 100
		finish       bool
		err          error
	)

	requestList.GroupId = &groupId
	requestList.Module = helper.String("monitor")
	requestList.Offset = &offset
	requestList.Limit = &limit

	for {
		if finish {
			break
		}
		if err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(requestList.GetAction())
			if responseList, err = me.client.UseMonitorClient().DescribeBindingPolicyObjectList(requestList); err != nil {
				return retryError(err, InternalError)
			}
			objects = append(objects, responseList.Response.List...)
			if len(responseList.Response.List) < int(limit) {
				finish = true
			}
			return nil
		}); err != nil {
			errRet = err
			return
		}
		offset = offset + limit
	}

	return
}

func (me *MonitorService) DescribeBindingAlarmPolicyObjectList(ctx context.Context, policyId string) (
	objects []*monitor.DescribeBindingPolicyObjectListInstance, errRet error) {

	var (
		requestList  = monitor.NewDescribeBindingPolicyObjectListRequest()
		responseList *monitor.DescribeBindingPolicyObjectListResponse
		offset       int64 = 0
		limit        int64 = 100
		finish       bool
		err          error
	)
	requestList.GroupId = helper.Int64(0)
	requestList.PolicyId = &policyId
	requestList.Module = helper.String("monitor")
	requestList.Offset = &offset
	requestList.Limit = &limit

	for {
		if finish {
			break
		}
		if err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(requestList.GetAction())
			if responseList, err = me.client.UseMonitorClient().DescribeBindingPolicyObjectList(requestList); err != nil {
				return retryError(err, InternalError)
			}
			objects = append(objects, responseList.Response.List...)
			if len(responseList.Response.List) < int(limit) {
				finish = true
			}
			return nil
		}); err != nil {
			errRet = err
			return
		}
		offset = offset + limit
	}

	return
}

// tmp
func (me *MonitorService) DescribeMonitorTmpInstance(ctx context.Context, tmpInstanceId string) (tmpInstance *monitor.PrometheusInstancesItem, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribePrometheusInstancesRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()
	request.InstanceIds = []*string{&tmpInstanceId}

	response, err := me.client.UseMonitorClient().DescribePrometheusInstances(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.InstanceSet) < 1 {
		return
	}
	tmpInstance = response.Response.InstanceSet[0]
	return
}

func (me *MonitorService) IsolateMonitorTmpInstanceById(ctx context.Context, tmpInstanceId string) (errRet error) {
	logId := getLogId(ctx)

	request := monitor.NewTerminatePrometheusInstancesRequest()
	request.InstanceIds = []*string{&tmpInstanceId}

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().TerminatePrometheusInstances(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *MonitorService) DeleteMonitorTmpInstanceById(ctx context.Context, tmpInstanceId string) (errRet error) {
	logId := getLogId(ctx)

	request := monitor.NewDestroyPrometheusInstanceRequest()
	request.InstanceId = &tmpInstanceId

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DestroyPrometheusInstance(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *MonitorService) DescribeMonitorTmpCvmAgent(ctx context.Context, instanceId string, tmpCvmAgentId string) (tmpCvmAgent *monitor.PrometheusAgent, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribePrometheusAgentsRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()
	request.InstanceId = &instanceId
	request.AgentIds = []*string{&tmpCvmAgentId}

	response, err := me.client.UseMonitorClient().DescribePrometheusAgents(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.AgentSet) < 1 {
		return
	}
	tmpCvmAgent = response.Response.AgentSet[0]
	return
}

func (me *MonitorService) DescribeMonitorTmpScrapeJob(ctx context.Context, tmpScrapeJobId string) (tmpScrapeJob *monitor.PrometheusScrapeJob, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribePrometheusScrapeJobsRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()

	ids := strings.Split(tmpScrapeJobId, FILED_SP)

	request.JobIds = []*string{&ids[0]}
	request.InstanceId = &ids[1]
	request.AgentId = &ids[2]

	response, err := me.client.UseMonitorClient().DescribePrometheusScrapeJobs(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.ScrapeJobSet) < 1 {
		return
	}
	tmpScrapeJob = response.Response.ScrapeJobSet[0]
	return
}

func (me *MonitorService) DeleteMonitorTmpScrapeJobById(ctx context.Context, tmpScrapeJobId string) (errRet error) {
	logId := getLogId(ctx)

	ids := strings.Split(tmpScrapeJobId, FILED_SP)
	request := monitor.NewDeletePrometheusScrapeJobsRequest()
	request.JobIds = []*string{&ids[0]}
	request.InstanceId = &ids[1]
	request.AgentId = &ids[2]

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DeletePrometheusScrapeJobs(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *MonitorService) DescribeMonitorTmpExporterIntegration(ctx context.Context, tmpExporterIntegrationId string) (tmpExporterIntegration *monitor.IntegrationConfiguration, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribeExporterIntegrationsRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()

	ids := strings.Split(tmpExporterIntegrationId, FILED_SP)
	request.Name = &ids[0]
	request.InstanceId = &ids[1]
	kubeType, _ := strconv.Atoi(ids[2])
	request.KubeType = helper.IntInt64(kubeType)
	request.ClusterId = &ids[3]

	response, err := me.client.UseMonitorClient().DescribeExporterIntegrations(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.IntegrationSet) < 1 {
		return
	}
	tmpExporterIntegration = response.Response.IntegrationSet[0]
	return
}

func (me *MonitorService) DeleteMonitorTmpExporterIntegrationById(ctx context.Context, tmpExporterIntegrationId string) (errRet error) {
	logId := getLogId(ctx)

	request := monitor.NewDeleteExporterIntegrationRequest()
	ids := strings.Split(tmpExporterIntegrationId, FILED_SP)
	request.Name = &ids[0]
	request.InstanceId = &ids[1]
	kubeType, _ := strconv.Atoi(ids[2])
	request.KubeType = helper.IntInt64(kubeType)
	request.ClusterId = &ids[3]

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DeleteExporterIntegration(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *MonitorService) DescribeMonitorTmpAlertRuleById(ctx context.Context, instanceId string, tmpAlertRuleId string) (instance *monitor.PrometheusRuleSet, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribeAlertRulesRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()

	request.InstanceId = &instanceId
	request.RuleId = &tmpAlertRuleId

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DescribeAlertRules(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.AlertRuleSet) < 1 {
		return
	}
	instance = response.Response.AlertRuleSet[0]

	return
}

func (me *MonitorService) DeleteMonitorTmpAlertRule(ctx context.Context, instanceId string, ruleId string) (errRet error) {
	logId := getLogId(ctx)

	request := monitor.NewDeleteAlertRulesRequest()
	request.InstanceId = &instanceId
	request.RuleIds = []*string{&ruleId}

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DeleteAlertRules(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *MonitorService) DescribeMonitorRecordingRuleById(ctx context.Context, instanceId string, recordingRuleId string) (instance *monitor.RecordingRuleSet, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = monitor.NewDescribeRecordingRulesRequest()
	)

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "query object", request.ToJsonString(), errRet.Error())
		}
	}()

	request.InstanceId = &instanceId
	request.RuleId = &recordingRuleId

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DescribeRecordingRules(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.RecordingRuleSet) < 1 {
		return
	}
	instance = response.Response.RecordingRuleSet[0]

	return
}

func (me *MonitorService) DeleteMonitorRecordingRule(ctx context.Context, instanceId string, recordingRuleId string) (errRet error) {
	logId := getLogId(ctx)

	request := monitor.NewDeleteRecordingRulesRequest()
	request.InstanceId = &instanceId
	request.RuleIds = []*string{&recordingRuleId}

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, "delete object", request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseMonitorClient().DeleteRecordingRules(request)
	if err != nil {
		errRet = err
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}
