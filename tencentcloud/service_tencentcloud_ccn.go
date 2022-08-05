package tencentcloud

import (
	"context"
	"fmt"
	"log"
	"strings"

	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

//Ccn basic information
type CcnBasicInfo struct {
	ccnId             string
	name              string
	description       string
	state             string
	qos               string
	chargeType        string
	bandWithLimitType string
	instanceCount     int64
	createTime        string
}

type CcnAttachedInstanceInfo struct {
	ccnId          string
	instanceType   string
	instanceRegion string
	instanceId     string
	state          string
	attachedTime   string
	cidrBlock      []string
}

type CcnBandwidthLimit struct {
	region string
	limit  int64
}

func (me *VpcService) DescribeCcn(ctx context.Context, ccnId string) (info CcnBasicInfo, has int, errRet error) {
	infos, err := me.DescribeCcns(ctx, ccnId, "")
	if err != nil {
		errRet = err
		return
	}
	has = len(infos)
	if has > 0 {
		info = infos[0]
	}
	return
}

func (me *VpcService) DescribeCcns(ctx context.Context, ccnId, name string) (infos []CcnBasicInfo, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeCcnsRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	infos = make([]CcnBasicInfo, 0, 100)

	var offset uint64 = 0
	var limit uint64 = 100
	var total = -1
	var has = map[string]bool{}

	var filters []*vpc.Filter
	if ccnId != "" {
		filters = me.fillFilter(filters, "ccn-id", ccnId)
	}
	if name != "" {
		filters = me.fillFilter(filters, "ccn-name", name)
	}
	if len(filters) > 0 {
		request.Filters = filters
	}

getMoreData:

	if total >= 0 && int(offset) >= total {
		return
	}
	request.Limit = &limit
	request.Offset = &offset
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeCcns(request)

	if err != nil {
		errRet = err
		responseStr := ""
		if response != nil {
			responseStr = response.ToJsonString()
		}
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
			logId,
			request.GetAction(),
			request.ToJsonString(),
			responseStr,
			errRet.Error())
		return
	}

	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())

	if total < 0 {
		total = int(*response.Response.TotalCount)
	}

	if len(response.Response.CcnSet) > 0 {
		offset += limit
	} else {
		//get empty ,we're done
		return
	}
	for _, item := range response.Response.CcnSet {
		var basicInfo CcnBasicInfo

		basicInfo.ccnId = *item.CcnId
		basicInfo.name = *item.CcnName
		basicInfo.createTime = *item.CreateTime
		basicInfo.description = *item.CcnDescription
		basicInfo.instanceCount = int64(*item.InstanceCount)
		basicInfo.qos = *item.QosLevel
		basicInfo.state = *item.State
		basicInfo.chargeType = *item.InstanceChargeType
		basicInfo.bandWithLimitType = *item.BandwidthLimitType

		if has[basicInfo.ccnId] {
			errRet = fmt.Errorf("get repeated ccn_id[%s] when doing DescribeCcns", basicInfo.ccnId)
			return
		}
		has[basicInfo.ccnId] = true
		infos = append(infos, basicInfo)
	}
	goto getMoreData

}

func (me *VpcService) DescribeCcnRegionBandwidthLimits(ctx context.Context, ccnId string) (infos []CcnBandwidthLimit, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeCcnRegionBandwidthLimitsRequest()

	infos = make([]CcnBandwidthLimit, 0, 100)

	request.CcnId = &ccnId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeCcnRegionBandwidthLimits(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())

	for _, item := range response.Response.CcnRegionBandwidthLimitSet {

		var ccnBandwidthLimit CcnBandwidthLimit
		ccnBandwidthLimit.region = *item.Region
		ccnBandwidthLimit.limit = int64(*item.BandwidthLimit)
		infos = append(infos, ccnBandwidthLimit)
	}
	return
}

func (me *VpcService) CreateCcn(ctx context.Context, name, description,
	qos, chargeType, bandWithLimitType string) (basicInfo CcnBasicInfo, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewCreateCcnRequest()

	request.CcnName = &name
	request.CcnDescription = &description
	request.QosLevel = &qos
	request.InstanceChargeType = &chargeType
	request.BandwidthLimitType = &bandWithLimitType
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateCcn(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())

	if response.Response.Ccn == nil || response.Response.Ccn.CcnId == nil || *response.Response.Ccn.CcnId == "" {
		errRet = fmt.Errorf("CreateCcn return empty response.Response.Ccn ")
		return
	}

	item := response.Response.Ccn
	basicInfo.ccnId = *item.CcnId
	basicInfo.name = *item.CcnName
	basicInfo.createTime = *item.CreateTime

	basicInfo.description = *item.CcnDescription
	basicInfo.instanceCount = int64(*item.InstanceCount)
	basicInfo.qos = *item.QosLevel
	basicInfo.state = *item.State
	return
}

func (me *VpcService) DeleteCcn(ctx context.Context, ccnId string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDeleteCcnRequest()
	request.CcnId = &ccnId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DeleteCcn(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())
	return
}

func (me *VpcService) ModifyCcnAttribute(ctx context.Context, ccnId, name, description string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewModifyCcnAttributeRequest()
	request.CcnId = &ccnId

	if name != "" {
		request.CcnName = &name
	}
	if description != "" {
		request.CcnDescription = &description
	}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ModifyCcnAttribute(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())
	return
}

func (me *VpcService) DescribeCcnAttachedInstance(ctx context.Context, ccnId,
	instanceRegion, instanceType, instanceId string) (info CcnAttachedInstanceInfo, has int, errRet error) {

	infos, err := me.DescribeCcnAttachedInstances(ctx, ccnId)

	if err != nil {
		errRet = err
		return
	}

	for _, item := range infos {
		if item.instanceId == instanceId &&
			item.instanceRegion == instanceRegion &&
			strings.EqualFold(item.instanceType, instanceType) {
			has = 1
			info = item
			return
		}
	}
	return
}

func (me *VpcService) DescribeCcnAttachedInstances(ctx context.Context, ccnId string) (infos []CcnAttachedInstanceInfo, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeCcnAttachedInstancesRequest()
	request.CcnId = &ccnId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeCcnAttachedInstances(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())

	infos = make([]CcnAttachedInstanceInfo, 0, len(response.Response.InstanceSet))

	for _, item := range response.Response.InstanceSet {

		var info CcnAttachedInstanceInfo

		info.attachedTime = *item.AttachedTime
		info.cidrBlock = make([]string, 0, len(item.CidrBlock))

		for _, v := range item.CidrBlock {
			info.cidrBlock = append(info.cidrBlock, *v)
		}

		info.ccnId = ccnId
		info.instanceId = *item.InstanceId
		info.instanceRegion = *item.InstanceRegion
		info.instanceType = *item.InstanceType
		info.state = *item.State
		infos = append(infos, info)
	}
	return
}

func (me *VpcService) DescribeCcnAttachmentsByInstance(ctx context.Context, instanceType string, instanceId string, instanceRegion string) (infos []vpc.CcnAttachedInstance, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeCcnAttachedInstancesRequest()
	request.Filters = make([]*vpc.Filter, 0, 3)
	request.Filters = append(request.Filters, &vpc.Filter{Name: helper.String("instance-type"), Values: []*string{&instanceType}})
	request.Filters = append(request.Filters, &vpc.Filter{Name: helper.String("instance-id"), Values: []*string{&instanceId}})
	request.Filters = append(request.Filters, &vpc.Filter{Name: helper.String("instance-region"), Values: []*string{&instanceRegion}})

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeCcnAttachedInstances(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())

	infos = make([]vpc.CcnAttachedInstance, 0, len(response.Response.InstanceSet))

	for _, item := range response.Response.InstanceSet {
		infos = append(infos, *item)
	}
	return
}

func (me *VpcService) AttachCcnInstances(ctx context.Context, ccnId, instanceRegion, instanceType, instanceId string, ccnUin string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewAttachCcnInstancesRequest()
	request.CcnId = &ccnId

	if ccnUin != "" {
		request.CcnUin = &ccnUin
	}
	var ccnInstance vpc.CcnInstance
	ccnInstance.InstanceId = &instanceId
	ccnInstance.InstanceRegion = &instanceRegion
	ccnInstance.InstanceType = &instanceType

	request.Instances = []*vpc.CcnInstance{&ccnInstance}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().AttachCcnInstances(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())
	return
}

func (me *VpcService) DetachCcnInstances(ctx context.Context, ccnId, instanceRegion, instanceType, instanceId string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDetachCcnInstancesRequest()
	request.CcnId = &ccnId

	var ccnInstance vpc.CcnInstance
	ccnInstance.InstanceId = &instanceId
	ccnInstance.InstanceRegion = &instanceRegion
	ccnInstance.InstanceType = &instanceType

	request.Instances = []*vpc.CcnInstance{&ccnInstance}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DetachCcnInstances(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())
	return

}

func (me *VpcService) DescribeCcnRegionBandwidthLimit(ctx context.Context, ccnId,
	region string) (bandwidth int64, errRet error) {

	infos, err := me.DescribeCcnRegionBandwidthLimits(ctx, ccnId)
	if err != nil {
		errRet = err
		return
	}
	for _, v := range infos {
		if v.region == region {
			bandwidth = v.limit
			break
		}
	}
	return
}

func (me *VpcService) GetCcnRegionBandwidthLimit(ctx context.Context, ccnId,
	region, dstRegion, limitType string) (int64, error) {
	infos, err := me.GetCcnRegionBandwidthLimits(ctx, ccnId)
	if err != nil {
		return 0, err
	}
	for _, v := range infos {
		if v.Region != nil {
			switch limitType {
			case OuterRegionLimit:
				if *v.Region == region {
					return int64(*v.BandwidthLimit), nil
				}
			case InterRegionLimit:
				if v.DstRegion != nil && *v.DstRegion == dstRegion && *v.Region == region {
					return int64(*v.BandwidthLimit), nil
				}
			default:
				return 0, fmt.Errorf("unknown type of band with limit type")
			}
		}
	}
	return 0, nil
}

func (me *VpcService) GetCcnRegionBandwidthLimits(ctx context.Context,
	ccnID string) (infos []vpc.CcnRegionBandwidthLimit, errRet error) {
	var (
		request  = vpc.NewGetCcnRegionBandwidthLimitsRequest()
		response *vpc.GetCcnRegionBandwidthLimitsResponse
		err      error
		limit    uint64 = 100
		offset   uint64 = 0
	)
	request.CcnId = &ccnID
	request.Limit = &limit
	request.Offset = &offset

	ratelimit.Check(request.GetAction())
	for {
		response, err = me.client.UseVpcClient().GetCcnRegionBandwidthLimits(request)
		if err != nil {
			errRet = err
			return
		}
		if response.Response == nil || response.Response.CcnBandwidthSet == nil {
			errRet = fmt.Errorf("TencentCloud SDK %s return empty response", request.GetAction())
			return
		}

		for _, item := range response.Response.CcnBandwidthSet {
			if item.CcnRegionBandwidthLimit != nil {
				infos = append(infos, *item.CcnRegionBandwidthLimit)
			}
		}
		if len(response.Response.CcnBandwidthSet) < int(limit) {
			break
		}
		offset += limit
	}
	return
}

func (me *VpcService) SetCcnRegionBandwidthLimits(ctx context.Context, ccnId, region, dstRegion string,
	bandwidth int64) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewSetCcnRegionBandwidthLimitsRequest()
	request.CcnId = &ccnId

	var uint64bandwidth = uint64(bandwidth)
	var ccnRegionBandwidthLimit vpc.CcnRegionBandwidthLimit
	ccnRegionBandwidthLimit.BandwidthLimit = &uint64bandwidth
	ccnRegionBandwidthLimit.Region = &region
	if dstRegion != "" {
		ccnRegionBandwidthLimit.DstRegion = &dstRegion
	}

	request.CcnRegionBandwidthLimits = []*vpc.CcnRegionBandwidthLimit{&ccnRegionBandwidthLimit}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().SetCcnRegionBandwidthLimits(request)

	defer func() {
		if errRet != nil {
			responseStr := ""
			if response != nil {
				responseStr = response.ToJsonString()
			}
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s],response body [%s], reason[%s]\n",
				logId,
				request.GetAction(),
				request.ToJsonString(),
				responseStr,
				errRet.Error())
		}
	}()

	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId,
		request.GetAction(),
		request.ToJsonString(),
		response.ToJsonString())
	return
}

func (me *VpcService) ModifyCcnRegionBandwidthLimitsType(ctx context.Context, ccnID, limitType string) error {
	request := vpc.NewModifyCcnRegionBandwidthLimitsTypeRequest()
	request.CcnId = &ccnID
	request.BandwidthLimitType = &limitType
	_, err := me.client.UseVpcClient().ModifyCcnRegionBandwidthLimitsType(request)
	if err != nil {
		return err
	}
	return nil
}
