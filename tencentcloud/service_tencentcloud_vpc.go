package tencentcloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	sdkErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

var eipUnattachLocker = &sync.Mutex{}

// VPC basic information
type VpcBasicInfo struct {
	vpcId          string
	name           string
	cidr           string
	isMulticast    bool
	isDefault      bool
	dnsServers     []string
	createTime     string
	tags           []*vpc.Tag
	assistantCidrs []string
}

// subnet basic information
type VpcSubnetBasicInfo struct {
	vpcId            string
	subnetId         string
	routeTableId     string
	name             string
	cidr             string
	isMulticast      bool
	isDefault        bool
	zone             string
	availableIpCount int64
	createTime       string
}

// route entry basic information
type VpcRouteEntryBasicInfo struct {
	routeEntryId    int64
	destinationCidr string
	nextType        string
	nextBub         string
	description     string
	entryType       string
	enabled         bool
}

// route table basic information
type VpcRouteTableBasicInfo struct {
	routeTableId string
	name         string
	vpcId        string
	isDefault    bool
	subnetIds    []string
	entryInfos   []VpcRouteEntryBasicInfo
	createTime   string
}

type VpcSecurityGroupLiteRule struct {
	action          string
	cidrIp          string
	port            string
	protocol        string
	addressId       string
	addressGroupId  string
	securityGroupId string
}

var securityGroupIdRE = regexp.MustCompile("^sg-\\w{8}$")
var ipAddressIdRE = regexp.MustCompile("^ipm-\\w{8}$")
var ipAddressGroupIdRE = regexp.MustCompile("^ipmg-\\w{8}$")
var portRE = regexp.MustCompile(`^(\d{1,5},)*\d{1,5}$|^\d{1,5}-\d{1,5}$`)

// acl rule
type VpcACLRule struct {
	action   string
	cidrIp   string
	port     string
	protocol string
}

type VpcEniIP struct {
	ip      net.IP
	primary bool
	desc    *string
}

func (rule VpcSecurityGroupLiteRule) String() string {

	var source string

	if rule.cidrIp != "" {
		source = rule.cidrIp
	}
	if rule.securityGroupId != "" {
		source = rule.securityGroupId
	}
	if rule.addressId != "" {
		source = rule.addressId
	}
	if rule.addressGroupId != "" {
		source = rule.addressGroupId
	}

	return fmt.Sprintf("%s#%s#%s#%s", rule.action, source, rule.port, rule.protocol)
}

func getSecurityGroupPolicies(rules []VpcSecurityGroupLiteRule) []*vpc.SecurityGroupPolicy {
	policies := make([]*vpc.SecurityGroupPolicy, 0)

	for i := range rules {
		rule := rules[i]
		policy := &vpc.SecurityGroupPolicy{
			Protocol: &rule.protocol,
			Action:   &rule.action,
		}

		if rule.securityGroupId != "" {
			policy.SecurityGroupId = &rule.securityGroupId
		} else if rule.addressId != "" || rule.addressGroupId != "" {
			policy.AddressTemplate = &vpc.AddressTemplateSpecification{}
			if rule.addressId != "" {
				policy.AddressTemplate.AddressId = &rule.addressId
			}
			if rule.addressGroupId != "" {
				policy.AddressTemplate.AddressGroupId = &rule.addressGroupId
			}
		} else {
			policy.CidrBlock = &rule.cidrIp
		}

		if rule.port != "" {
			policy.Port = &rule.port
		}

		policies = append(policies, policy)
	}
	return policies
}

type VpcService struct {
	client *connectivity.TencentCloudClient
}

// ///////common
func (me *VpcService) fillFilter(ins []*vpc.Filter, key, value string) (outs []*vpc.Filter) {
	if ins == nil {
		ins = make([]*vpc.Filter, 0, 2)
	}

	var filter = vpc.Filter{Name: &key, Values: []*string{&value}}
	ins = append(ins, &filter)
	outs = ins
	return
}

// ////////api
func (me *VpcService) CreateVpc(ctx context.Context, name, cidr string,
	isMulticast bool, dnsServers []string) (vpcId string, isDefault bool, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewCreateVpcRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	request.VpcName = &name
	request.CidrBlock = &cidr

	var enableMulticast = map[bool]string{true: "true", false: "false"}[isMulticast]
	request.EnableMulticast = &enableMulticast

	if len(dnsServers) > 0 {
		request.DnsServers = make([]*string, 0, len(dnsServers))
		for index := range dnsServers {
			request.DnsServers = append(request.DnsServers, &dnsServers[index])
		}
	}
	var response *vpc.CreateVpcResponse
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		result, err := me.client.UseVpcClient().CreateVpc(request)
		if err != nil {
			return retryError(err)
		}
		response = result
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s create vpc failed, reason: %v", logId, err)
		errRet = err
		return
	}
	vpcId, isDefault = *response.Response.Vpc.VpcId, *response.Response.Vpc.IsDefault
	return
}

func (me *VpcService) DescribeVpc(ctx context.Context,
	vpcId string,
	tagKey string,
	cidrBlock string) (info VpcBasicInfo, has int, errRet error) {
	infos, err := me.DescribeVpcs(ctx, vpcId, "", nil, nil, tagKey, cidrBlock)
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

func (me *VpcService) DescribeVpcs(ctx context.Context,
	vpcId, name string,
	tags map[string]string,
	isDefaultPtr *bool,
	tagKey string,
	cidrBlock string) (infos []VpcBasicInfo, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeVpcsRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	infos = make([]VpcBasicInfo, 0, 100)

	var (
		offset  = 0
		limit   = 100
		total   = -1
		hasVpc  = map[string]bool{}
		filters []*vpc.Filter
	)

	if vpcId != "" {
		filters = me.fillFilter(filters, "vpc-id", vpcId)
	}

	if name != "" {
		filters = me.fillFilter(filters, "vpc-name", name)
	}

	if tagKey != "" {
		filters = me.fillFilter(filters, "tag-key", tagKey)
	}

	if cidrBlock != "" {
		filters = me.fillFilter(filters, "cidr-block", cidrBlock)
	}

	if isDefaultPtr != nil {
		filters = me.fillFilter(filters, "is-default", map[bool]string{true: "true", false: "false"}[*isDefaultPtr])
	}

	for k, v := range tags {
		filters = me.fillFilter(filters, "tag:"+k, v)
	}

	if len(filters) > 0 {
		request.Filters = filters
	}

getMoreData:

	if total >= 0 {
		if offset >= total {
			return
		}
	}
	var strLimit = fmt.Sprintf("%d", limit)
	request.Limit = &strLimit

	var strOffset = fmt.Sprintf("%d", offset)
	request.Offset = &strOffset
	var response *vpc.DescribeVpcsResponse
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		result, err := me.client.UseVpcClient().DescribeVpcs(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		response = result
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s read vpc failed, reason: %v", logId, err)
		return nil, err
	}

	if total < 0 {
		total = int(*response.Response.TotalCount)
	}

	if len(response.Response.VpcSet) > 0 {
		offset += limit
	} else {
		// get empty VpcInfo, we're done
		return
	}
	for _, item := range response.Response.VpcSet {
		var basicInfo VpcBasicInfo
		basicInfo.cidr = *item.CidrBlock
		basicInfo.createTime = *item.CreatedTime
		basicInfo.dnsServers = make([]string, 0, len(item.DnsServerSet))

		for _, v := range item.DnsServerSet {
			basicInfo.dnsServers = append(basicInfo.dnsServers, *v)
		}
		basicInfo.isDefault = *item.IsDefault
		basicInfo.isMulticast = *item.EnableMulticast
		basicInfo.name = *item.VpcName
		basicInfo.vpcId = *item.VpcId

		if hasVpc[basicInfo.vpcId] {
			errRet = fmt.Errorf("get repeated vpc_id[%s] when doing DescribeVpcs", basicInfo.vpcId)
			return
		}
		hasVpc[basicInfo.vpcId] = true

		if len(item.AssistantCidrSet) > 0 {
			for i := range item.AssistantCidrSet {
				cidr := item.AssistantCidrSet[i].CidrBlock
				basicInfo.assistantCidrs = append(basicInfo.assistantCidrs, *cidr)
			}
		}

		if len(item.TagSet) > 0 {
			basicInfo.tags = item.TagSet
		}

		infos = append(infos, basicInfo)
	}
	goto getMoreData

}
func (me *VpcService) DescribeSubnet(ctx context.Context,
	subnetId string,
	isRemoteVpcSNAT *bool,
	tagKey,
	cidrBlock string) (info VpcSubnetBasicInfo, has int, errRet error) {
	infos, err := me.DescribeSubnets(ctx, subnetId, "", "", "", nil, nil, isRemoteVpcSNAT, tagKey, cidrBlock)
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

func (me *VpcService) DescribeSubnets(ctx context.Context,
	subnetId,
	vpcId,
	subnetName,
	zone string,
	tags map[string]string,
	isDefaultPtr *bool,
	isRemoteVpcSNAT *bool,
	tagKey,
	cidrBlock string) (infos []VpcSubnetBasicInfo, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeSubnetsRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	var (
		offset    = 0
		limit     = 100
		total     = -1
		hasSubnet = map[string]bool{}
		filters   []*vpc.Filter
	)

	if subnetId != "" {
		filters = me.fillFilter(filters, "subnet-id", subnetId)
	}
	if vpcId != "" {
		filters = me.fillFilter(filters, "vpc-id", vpcId)
	}
	if subnetName != "" {
		filters = me.fillFilter(filters, "subnet-name", subnetName)
	}
	if zone != "" {
		filters = me.fillFilter(filters, "zone", zone)
	}

	if isDefaultPtr != nil {
		filters = me.fillFilter(filters, "is-default", map[bool]string{true: "true", false: "false"}[*isDefaultPtr])
	}

	if isRemoteVpcSNAT != nil {
		filters = me.fillFilter(filters, "is-remote-vpc-snat", map[bool]string{true: "true", false: "false"}[*isRemoteVpcSNAT])
	}

	if tagKey != "" {
		filters = me.fillFilter(filters, "tag-key", tagKey)
	}
	if cidrBlock != "" {
		filters = me.fillFilter(filters, "cidr-block", cidrBlock)
	}

	for k, v := range tags {
		filters = me.fillFilter(filters, "tag:"+k, v)
	}

	if len(filters) > 0 {
		request.Filters = filters
	}

getMoreData:
	if total >= 0 {
		if offset >= total {
			return
		}
	}
	var strLimit = fmt.Sprintf("%d", limit)
	request.Limit = &strLimit

	var strOffset = fmt.Sprintf("%d", offset)
	request.Offset = &strOffset
	var response *vpc.DescribeSubnetsResponse
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		result, err := me.client.UseVpcClient().DescribeSubnets(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		response = result
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s read subnets failed, reason: %v", logId, err)
		return nil, err
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if total < 0 {
		total = int(*response.Response.TotalCount)
	}

	if len(response.Response.SubnetSet) > 0 {
		offset += limit
	} else {
		// get empty subnet, we're done
		return
	}
	for _, item := range response.Response.SubnetSet {
		var basicInfo VpcSubnetBasicInfo

		basicInfo.cidr = *item.CidrBlock
		basicInfo.createTime = *item.CreatedTime
		basicInfo.vpcId = *item.VpcId
		basicInfo.subnetId = *item.SubnetId
		basicInfo.routeTableId = *item.RouteTableId

		basicInfo.name = *item.SubnetName
		basicInfo.isDefault = *item.IsDefault
		basicInfo.isMulticast = *item.EnableBroadcast

		basicInfo.zone = *item.Zone
		basicInfo.availableIpCount = int64(*item.AvailableIpAddressCount)

		if hasSubnet[basicInfo.subnetId] {
			errRet = fmt.Errorf("get repeated subnetId[%s] when doing DescribeSubnets", basicInfo.subnetId)
			return
		}
		hasSubnet[basicInfo.subnetId] = true
		infos = append(infos, basicInfo)
	}
	goto getMoreData
}

func (me *VpcService) ModifyVpcAttribute(ctx context.Context, vpcId, name string, isMulticast bool, dnsServers []string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyVpcAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	request.VpcId = &vpcId
	request.VpcName = &name

	if len(dnsServers) > 0 {
		request.DnsServers = make([]*string, 0, len(dnsServers))
		for index := range dnsServers {
			request.DnsServers = append(request.DnsServers, &dnsServers[index])
		}
	}
	var enableMulticast = map[bool]string{true: "true", false: "false"}[isMulticast]
	request.EnableMulticast = &enableMulticast

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err := me.client.UseVpcClient().ModifyVpcAttribute(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s modify vpc failed, reason: %v", logId, err)
		return err
	}

	return
}

func (me *VpcService) DeleteVpc(ctx context.Context, vpcId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteVpcRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()
	if vpcId == "" {
		errRet = fmt.Errorf("DeleteVpc can not delete empty vpc_id.")
		return
	}

	request.VpcId = &vpcId

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err := me.client.UseVpcClient().DeleteVpc(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s delete vpc failed, reason: %v", logId, err)
		return err
	}
	return

}

func (me *VpcService) CreateSubnet(ctx context.Context, vpcId, name, cidr, zone string) (subnetId string, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateSubnetRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if vpcId == "" {
		errRet = fmt.Errorf("CreateSubnet can not invoke by empty vpc_id.")
		return
	}
	request.VpcId = &vpcId
	request.SubnetName = &name
	request.CidrBlock = &cidr
	request.Zone = &zone
	var response *vpc.CreateSubnetResponse
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		result, err := me.client.UseVpcClient().CreateSubnet(request)
		if err != nil {
			return retryError(err)
		}
		response = result
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s create subnet failed, reason: %v", logId, err)
		return "", err
	}

	subnetId = *response.Response.Subnet.SubnetId

	return
}

func (me *VpcService) ModifySubnetAttribute(ctx context.Context, subnetId, name string, isMulticast bool) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifySubnetAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	var enableMulticast = map[bool]string{true: "true", false: "false"}[isMulticast]

	request.SubnetId = &subnetId
	request.SubnetName = &name
	request.EnableBroadcast = &enableMulticast
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err := me.client.UseVpcClient().ModifySubnetAttribute(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s modify subnet failed, reason: %v", logId, err)
		return err
	}
	return
}

func (me *VpcService) DeleteSubnet(ctx context.Context, subnetId string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDeleteSubnetRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()
	request.SubnetId = &subnetId
	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err := me.client.UseVpcClient().DeleteSubnet(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s delete subnet failed, reason: %v", logId, err)
		return err
	}
	return

}

func (me *VpcService) ReplaceRouteTableAssociation(ctx context.Context, subnetId string, routeTableId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewReplaceRouteTableAssociationRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()
	request.SubnetId = &subnetId
	request.RouteTableId = &routeTableId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ReplaceRouteTableAssociation(request)

	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())
	}
	return
}

func (me *VpcService) IsRouteTableInVpc(ctx context.Context, routeTableId, vpcId string) (info VpcRouteTableBasicInfo, has int, errRet error) {

	infos, err := me.DescribeRouteTables(ctx, routeTableId, "", vpcId, nil, nil, "")
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

func (me *VpcService) DescribeRouteTable(ctx context.Context, routeTableId string) (info VpcRouteTableBasicInfo, has int, errRet error) {

	infos, err := me.DescribeRouteTables(ctx, routeTableId, "", "", nil, nil, "")
	if err != nil {
		errRet = err
		return
	}

	has = len(infos)

	if has == 0 {
		return
	}
	info = infos[0]
	return
}
func (me *VpcService) DescribeRouteTables(ctx context.Context,
	routeTableId,
	routeTableName,
	vpcId string,
	tags map[string]string,
	associationMain *bool,
	tagKey string) (infos []VpcRouteTableBasicInfo, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeRouteTablesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	infos = make([]VpcRouteTableBasicInfo, 0, 100)
	var offset = 0
	var limit = 100
	var total = -1
	var hasTableMap = map[string]bool{}

	var filters []*vpc.Filter
	if routeTableId != "" {
		filters = me.fillFilter(filters, "route-table-id", routeTableId)
	}
	if vpcId != "" {
		filters = me.fillFilter(filters, "vpc-id", vpcId)
	}
	if routeTableName != "" {
		filters = me.fillFilter(filters, "route-table-name", routeTableName)
	}
	if associationMain != nil {
		filters = me.fillFilter(filters, "association.main", map[bool]string{true: "true", false: "false"}[*associationMain])
	}
	if tagKey != "" {
		filters = me.fillFilter(filters, "tag-key", tagKey)
	}
	for k, v := range tags {
		filters = me.fillFilter(filters, "tag:"+k, v)
	}
	if len(filters) > 0 {
		request.Filters = filters
	}

getMoreData:
	if total >= 0 {
		if offset >= total {
			return
		}
	}
	var strLimit = fmt.Sprintf("%d", limit)
	request.Limit = &strLimit

	var strOffset = fmt.Sprintf("%d", offset)
	request.Offset = &strOffset
	ratelimit.Check(request.GetAction())

	response, err := me.client.UseVpcClient().DescribeRouteTables(request)
	if err != nil {
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if total < 0 {
		total = int(*response.Response.TotalCount)
	}

	if len(response.Response.RouteTableSet) > 0 {
		offset += limit
	} else {
		// get empty Vpcinfo, we're done
		return
	}
	for _, item := range response.Response.RouteTableSet {
		var basicInfo VpcRouteTableBasicInfo
		basicInfo.createTime = *item.CreatedTime
		basicInfo.isDefault = *item.Main
		basicInfo.name = *item.RouteTableName
		basicInfo.routeTableId = *item.RouteTableId
		basicInfo.vpcId = *item.VpcId

		basicInfo.subnetIds = make([]string, 0, len(item.AssociationSet))
		for _, v := range item.AssociationSet {
			basicInfo.subnetIds = append(basicInfo.subnetIds, *v.SubnetId)
		}

		basicInfo.entryInfos = make([]VpcRouteEntryBasicInfo, 0, len(item.RouteSet))

		for _, v := range item.RouteSet {
			var entry VpcRouteEntryBasicInfo
			entry.destinationCidr = *v.DestinationCidrBlock
			entry.nextBub = *v.GatewayId
			entry.nextType = *v.GatewayType
			entry.description = *v.RouteDescription
			entry.routeEntryId = int64(*v.RouteId)
			entry.entryType = *v.RouteType
			entry.enabled = *v.Enabled
			basicInfo.entryInfos = append(basicInfo.entryInfos, entry)
		}
		if hasTableMap[basicInfo.routeTableId] {
			errRet = fmt.Errorf("get repeated route_table_id[%s] when doing DescribeRouteTables", basicInfo.routeTableId)
			return
		}
		hasTableMap[basicInfo.routeTableId] = true
		infos = append(infos, basicInfo)
	}
	goto getMoreData

}

func (me *VpcService) CreateRouteTable(ctx context.Context, name, vpcId string) (routeTableId string, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewCreateRouteTableRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if vpcId == "" {
		errRet = fmt.Errorf("CreateRouteTable can not invoke by empty vpc_id.")
		return
	}
	request.VpcId = &vpcId
	request.RouteTableName = &name
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateRouteTable(request)
	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		routeTableId = *response.Response.RouteTable.RouteTableId
	}
	return
}

func (me *VpcService) DeleteRouteTable(ctx context.Context, routeTableId string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDeleteRouteTableRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if routeTableId == "" {
		errRet = fmt.Errorf("DeleteRouteTable can not invoke by empty routeTableId.")
		return
	}
	request.RouteTableId = &routeTableId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DeleteRouteTable(request)
	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())
	}

	return
}

func (me *VpcService) ModifyRouteTableAttribute(ctx context.Context, routeTableId string, name string) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewModifyRouteTableAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if routeTableId == "" {
		errRet = fmt.Errorf("ModifyRouteTableAttribute can not invoke by empty routeTableId.")
		return
	}
	request.RouteTableId = &routeTableId
	request.RouteTableName = &name
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ModifyRouteTableAttribute(request)
	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())
	}

	return
}

func (me *VpcService) GetRouteId(ctx context.Context,
	routeTableId, destinationCidrBlock, nextType, nextHub, description string) (entryId int64, errRet error) {

	logId := getLogId(ctx)

	info, has, err := me.DescribeRouteTable(ctx, routeTableId)
	if err != nil {
		errRet = err
		return
	}
	if has == 0 {
		errRet = fmt.Errorf("not fonud the  route table of this  route entry")
		return
	}

	if has != 1 {
		errRet = fmt.Errorf("one routeTableId id get %d routeTableId infos", has)
		return
	}

	for _, v := range info.entryInfos {

		if v.destinationCidr == destinationCidrBlock && v.nextType == nextType && v.nextBub == nextHub {
			entryId = v.routeEntryId
			return
		}
	}
	errRet = fmt.Errorf("not found  route entry id from route table [%s]", routeTableId)

	for _, v := range info.entryInfos {
		log.Printf("%s[WARN] GetRouteId [%+v] vs [%+v],[%+v] vs [%+v],[%+v] vs [%+v]   %+v\n",
			logId,
			v.destinationCidr,
			destinationCidrBlock,
			v.nextType,
			nextType,
			v.nextBub,
			nextHub,
			v.destinationCidr == destinationCidrBlock && v.nextType == nextType && v.nextBub == nextHub)
	}

	return

}

func (me *VpcService) DeleteRoutes(ctx context.Context, routeTableId string, entryId uint64) (errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewDeleteRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if routeTableId == "" {
		errRet = fmt.Errorf("DeleteRoutes can not invoke by empty routeTableId.")
		return
	}

	request.RouteTableId = &routeTableId
	var route vpc.Route
	route.RouteId = &entryId
	request.Routes = []*vpc.Route{&route}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DeleteRoutes(request)
	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())
	}
	return
}

func (me *VpcService) CreateRoutes(ctx context.Context,
	routeTableId, destinationCidrBlock, nextType, nextHub, description string, enabled bool) (entryId int64, errRet error) {

	logId := getLogId(ctx)
	request := vpc.NewCreateRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	if routeTableId == "" {
		errRet = fmt.Errorf("CreateRoutes can not invoke by empty routeTableId.")
		return
	}
	request.RouteTableId = &routeTableId
	var route vpc.Route
	route.DestinationCidrBlock = &destinationCidrBlock
	route.RouteDescription = &description
	route.GatewayType = &nextType
	route.GatewayId = &nextHub
	route.Enabled = &enabled
	request.Routes = []*vpc.Route{&route}
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateRoutes(request)
	errRet = err
	if err == nil {
		log.Printf("[DEBUG]%s api[%s] , request body [%s], response body[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())
	} else {
		return
	}

	entryId, errRet = me.GetRouteId(ctx, routeTableId, destinationCidrBlock, nextType, nextHub, description)

	if errRet != nil {
		time.Sleep(3 * time.Second)
		entryId, errRet = me.GetRouteId(ctx, routeTableId, destinationCidrBlock, nextType, nextHub, description)
	}

	if errRet != nil {
		time.Sleep(5 * time.Second)
		entryId, errRet = me.GetRouteId(ctx, routeTableId, destinationCidrBlock, nextType, nextHub, description)
	}

	/*
		if *(response.Response.TotalCount) != 1 {
			errRet = fmt.Errorf("CreateRoutes  return %d routeTable . but we only request 1.", *response.Response.TotalCount)
			return
		}

		if len(response.Response.RouteTableSet) != 1 {
			errRet = fmt.Errorf("CreateRoutes  return %d routeTable  info . but we only request 1.", len(response.Response.RouteTableSet))
			return
		}

		if len(response.Response.RouteTableSet[0].RouteSet) != 1 {
			errRet = fmt.Errorf("CreateRoutes  return %d routeTableSet  info . but we only create 1.", len(response.Response.RouteTableSet[0].RouteSet))
			return
		}

		entryId = int64(*response.Response.RouteTableSet[0].RouteSet[0].RouteId)
	*/

	return
}

func (me *VpcService) SwitchRouteEnabled(ctx context.Context, routeTableId string, routeId uint64, enabled bool) error {
	if enabled {
		request := vpc.NewEnableRoutesRequest()
		request.RouteTableId = &routeTableId
		request.RouteIds = []*uint64{&routeId}
		return me.EnableRoutes(ctx, request)
	} else {
		request := vpc.NewDisableRoutesRequest()
		request.RouteTableId = &routeTableId
		request.RouteIds = []*uint64{&routeId}
		return me.DisableRoutes(ctx, request)
	}
}

func (me *VpcService) EnableRoutes(ctx context.Context, request *vpc.EnableRoutesRequest) (errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().EnableRoutes(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}
func (me *VpcService) DisableRoutes(ctx context.Context, request *vpc.DisableRoutesRequest) (errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DisableRoutes(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}
func (me *VpcService) CreateSecurityGroup(ctx context.Context, name, desc string, projectId *int) (id string, err error) {
	logId := getLogId(ctx)

	request := vpc.NewCreateSecurityGroupRequest()

	request.GroupName = &name
	request.GroupDescription = &desc

	if projectId != nil {
		request.ProjectId = helper.String(strconv.Itoa(*projectId))
	}

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := me.client.UseVpcClient().CreateSecurityGroup(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		if response.Response.SecurityGroup == nil || response.Response.SecurityGroup.SecurityGroupId == nil {
			err := fmt.Errorf("api[%s] return security group id is nil", request.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		id = *response.Response.SecurityGroup.SecurityGroupId
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s create security group failed, reason: %v", logId, err)
		return "", err
	}

	return
}

func (me *VpcService) DescribeSecurityGroup(ctx context.Context, id string) (sg *vpc.SecurityGroup, err error) {
	logId := getLogId(ctx)

	request := vpc.NewDescribeSecurityGroupsRequest()
	request.SecurityGroupIds = []*string{&id}

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := me.client.UseVpcClient().DescribeSecurityGroups(request)
		if err != nil {
			if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
				if sdkError.Code == "ResourceNotFound" {
					return nil
				}
			}

			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err, InternalError)
		}

		if len(response.Response.SecurityGroupSet) == 0 {
			return nil
		}

		sg = response.Response.SecurityGroupSet[0]

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s read security group failed, reason: %v", logId, err)
		return nil, err
	}

	return
}

func (me *VpcService) ModifySecurityGroup(ctx context.Context, id string, newName, newDesc *string) error {
	logId := getLogId(ctx)

	request := vpc.NewModifySecurityGroupAttributeRequest()

	request.SecurityGroupId = &id
	request.GroupName = newName
	request.GroupDescription = newDesc

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err := me.client.UseVpcClient().ModifySecurityGroupAttribute(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s modify security group failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) DeleteSecurityGroup(ctx context.Context, id string) error {
	logId := getLogId(ctx)

	request := vpc.NewDeleteSecurityGroupRequest()
	request.SecurityGroupId = &id

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := me.client.UseVpcClient().DeleteSecurityGroup(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s delete security group failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) DescribeSecurityGroupsAssociate(ctx context.Context, ids []string) ([]*vpc.SecurityGroupAssociationStatistics, error) {
	logId := getLogId(ctx)

	request := vpc.NewDescribeSecurityGroupAssociationStatisticsRequest()
	request.SecurityGroupIds = common.StringPtrs(ids)
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeSecurityGroupAssociationStatistics(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		return nil, err
	}

	return response.Response.SecurityGroupAssociationStatisticsSet, nil
}

func (me *VpcService) CreateSecurityGroupPolicy(ctx context.Context, info securityGroupRuleBasicInfo) (ruleId string, err error) {
	logId := getLogId(ctx)

	createRequest := vpc.NewCreateSecurityGroupPoliciesRequest()
	createRequest.SecurityGroupId = &info.SgId

	createRequest.SecurityGroupPolicySet = new(vpc.SecurityGroupPolicySet)

	policy := new(vpc.SecurityGroupPolicy)

	policy.CidrBlock = info.CidrIp
	policy.SecurityGroupId = info.SourceSgId
	policy.AddressTemplate = &vpc.AddressTemplateSpecification{}
	if info.AddressTemplateId != nil && *info.AddressTemplateId != "" {
		policy.AddressTemplate.AddressId = info.AddressTemplateId
	}
	if info.AddressTemplateGroupId != nil && *info.AddressTemplateGroupId != "" {
		policy.AddressTemplate.AddressGroupId = info.AddressTemplateGroupId
	}

	policy.ServiceTemplate = &vpc.ServiceTemplateSpecification{}
	if info.ProtocolTemplateId != nil && *info.ProtocolTemplateId != "" {
		policy.ServiceTemplate.ServiceId = info.ProtocolTemplateId
	}
	if info.ProtocolTemplateGroupId != nil && *info.ProtocolTemplateGroupId != "" {
		policy.ServiceTemplate.ServiceGroupId = info.ProtocolTemplateGroupId
	}

	if info.Protocol != nil {
		policy.Protocol = common.StringPtr(strings.ToUpper(*info.Protocol))
	}

	policy.Port = info.PortRange
	policy.PolicyDescription = info.Description
	policy.Action = common.StringPtr(strings.ToUpper(info.Action))

	switch strings.ToLower(info.PolicyType) {
	case "ingress":
		createRequest.SecurityGroupPolicySet.Ingress = []*vpc.SecurityGroupPolicy{policy}

	case "egress":
		createRequest.SecurityGroupPolicySet.Egress = []*vpc.SecurityGroupPolicy{policy}
	}
	ratelimit.Check(createRequest.GetAction())
	if _, err := me.client.UseVpcClient().CreateSecurityGroupPolicies(createRequest); err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, createRequest.GetAction(), createRequest.ToJsonString(), err)
		return "", err
	}

	if info.CidrIp == nil {
		info.CidrIp = common.StringPtr("")
	}
	if info.Protocol == nil {
		info.Protocol = common.StringPtr("ALL")
	}
	if info.PortRange == nil {
		info.PortRange = common.StringPtr("ALL")
	}
	if info.SourceSgId == nil {
		info.SourceSgId = common.StringPtr("")
	}

	ruleId, err = buildSecurityGroupRuleId(info)
	if err != nil {
		return "", fmt.Errorf("build rule id error, reason: %v", err)
	}

	return ruleId, nil
}

func (me *VpcService) DescribeSecurityGroupPolicy(ctx context.Context, ruleId string) (sgId string, policyType string, policy *vpc.SecurityGroupPolicy, errRet error) {
	logId := getLogId(ctx)

	info, err := parseSecurityGroupRuleId(ruleId)
	if err != nil {
		errRet = err
		return
	}

	request := vpc.NewDescribeSecurityGroupPoliciesRequest()
	request.SecurityGroupId = &info.SgId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeSecurityGroupPolicies(request)
	if err != nil {
		if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
			// if security group does not exist, security group rule does not exist too
			if sdkError.Code == "ResourceNotFound" {
				return
			}
		}

		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}

	policySet := response.Response.SecurityGroupPolicySet

	if policySet == nil {
		log.Printf("[DEBUG]%s policy set is nil", logId)
		return
	}

	var policies []*vpc.SecurityGroupPolicy

	switch strings.ToLower(info.PolicyType) {
	case "ingress":
		policies = policySet.Ingress

	case "egress":
		policies = policySet.Egress
	}

	for _, pl := range policies {
		if comparePolicyAndSecurityGroupInfo(pl, info) {
			policy = pl
			break
		}
	}

	if policy == nil {
		log.Printf("[DEBUG]%s can't find security group rule, maybe user modify rules on web console", logId)
		return
	}

	return info.SgId, info.PolicyType, policy, nil
}

func (me *VpcService) DeleteSecurityGroupPolicy(ctx context.Context, ruleId string) error {
	logId := getLogId(ctx)

	info, err := parseSecurityGroupRuleId(ruleId)
	if err != nil {
		return err
	}

	request := vpc.NewDeleteSecurityGroupPoliciesRequest()
	request.SecurityGroupId = &info.SgId
	request.SecurityGroupPolicySet = new(vpc.SecurityGroupPolicySet)

	policy := new(vpc.SecurityGroupPolicy)
	policy.Action = common.StringPtr(strings.ToUpper(info.Action))

	if *info.CidrIp != "" {
		policy.CidrBlock = info.CidrIp
	}

	if *info.Protocol != "ALL" {
		policy.Protocol = common.StringPtr(strings.ToUpper(*info.Protocol))
	}

	if *info.PortRange != "ALL" {
		policy.Port = info.PortRange
	}

	if *info.SourceSgId != "" {
		policy.SecurityGroupId = info.SourceSgId
	}

	if info.AddressTemplateGroupId != nil && *info.AddressTemplateGroupId != "" {
		policy.AddressTemplate = &vpc.AddressTemplateSpecification{}
		policy.AddressTemplate.AddressGroupId = info.AddressTemplateGroupId
	}

	if info.AddressTemplateId != nil && *info.AddressTemplateId != "" {
		policy.AddressTemplate = &vpc.AddressTemplateSpecification{}
		policy.AddressTemplate.AddressId = info.AddressTemplateId
	}

	if info.ProtocolTemplateGroupId != nil && *info.ProtocolTemplateGroupId != "" {
		policy.ServiceTemplate = &vpc.ServiceTemplateSpecification{}
		policy.ServiceTemplate.ServiceGroupId = info.ProtocolTemplateGroupId
	}

	if info.ProtocolTemplateId != nil && *info.ProtocolTemplateId != "" {
		policy.ServiceTemplate = &vpc.ServiceTemplateSpecification{}
		policy.ServiceTemplate.ServiceId = info.ProtocolTemplateId
	}

	switch strings.ToLower(info.PolicyType) {
	case "ingress":
		request.SecurityGroupPolicySet.Ingress = []*vpc.SecurityGroupPolicy{policy}

	case "egress":
		request.SecurityGroupPolicySet.Egress = []*vpc.SecurityGroupPolicy{policy}
	}
	ratelimit.Check(request.GetAction())
	if _, err := me.client.UseVpcClient().DeleteSecurityGroupPolicies(request); err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		return err
	}

	return nil
}

func (me *VpcService) ModifySecurityGroupPolicy(ctx context.Context, ruleId string, desc *string) error {
	logId := getLogId(ctx)

	info, err := parseSecurityGroupRuleId(ruleId)
	if err != nil {
		return err
	}

	request := vpc.NewReplaceSecurityGroupPolicyRequest()
	request.SecurityGroupId = &info.SgId
	request.SecurityGroupPolicySet = new(vpc.SecurityGroupPolicySet)

	policy := &vpc.SecurityGroupPolicy{
		Action:            &info.Action,
		CidrBlock:         info.CidrIp,
		Protocol:          info.Protocol,
		Port:              info.PortRange,
		SecurityGroupId:   info.SourceSgId,
		PolicyDescription: desc,
	}

	switch info.PolicyType {
	case "ingress":
		request.SecurityGroupPolicySet.Ingress = []*vpc.SecurityGroupPolicy{policy}

	case "egress":
		request.SecurityGroupPolicySet.Egress = []*vpc.SecurityGroupPolicy{policy}
	}
	ratelimit.Check(request.GetAction())
	if _, err := me.client.UseVpcClient().ReplaceSecurityGroupPolicy(request); err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		return err
	}

	return nil
}

func (me *VpcService) DescribeSecurityGroups(ctx context.Context, sgId, sgName *string, projectId *int, tags map[string]string) (sgs []*vpc.SecurityGroup, err error) {
	logId := getLogId(ctx)

	request := vpc.NewDescribeSecurityGroupsRequest()

	if sgId != nil {
		request.SecurityGroupIds = []*string{sgId}
	} else {
		if sgName != nil {
			request.Filters = append(request.Filters, &vpc.Filter{
				Name:   helper.String("security-group-name"),
				Values: []*string{sgName},
			})
		}

		if projectId != nil {
			request.Filters = append(request.Filters, &vpc.Filter{
				Name:   helper.String("project-id"),
				Values: []*string{helper.String(strconv.Itoa(*projectId))},
			})
		}

		for k, v := range tags {
			request.Filters = append(request.Filters, &vpc.Filter{
				Name:   helper.String("tag:" + k),
				Values: []*string{helper.String(v)},
			})
		}
	}

	request.Limit = helper.String(strconv.Itoa(DESCRIBE_SECURITY_GROUP_LIMIT))

	offset := 0
	count := DESCRIBE_SECURITY_GROUP_LIMIT
	// run loop at least once
	for count == DESCRIBE_SECURITY_GROUP_LIMIT {
		request.Offset = helper.String(strconv.Itoa(offset))

		if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())

			response, err := me.client.UseVpcClient().DescribeSecurityGroups(request)
			if err != nil {
				count = 0

				if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
					if sdkError.Code == "ResourceNotFound" {
						return nil
					}
				}

				log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
					logId, request.GetAction(), request.ToJsonString(), err)
				return retryError(err, InternalError)
			}

			set := response.Response.SecurityGroupSet
			count = len(set)
			sgs = append(sgs, set...)

			return nil
		}); err != nil {
			log.Printf("[CRITAL]%s read security groups failed, reason: %v", logId, err)
			return nil, err
		}

		offset += count
	}

	return
}

func (me *VpcService) modifyLiteRulesInSecurityGroup(ctx context.Context, sgId string, ingress, egress []VpcSecurityGroupLiteRule) error {
	logId := getLogId(ctx)

	request := vpc.NewModifySecurityGroupPoliciesRequest()
	request.SecurityGroupId = &sgId
	request.SecurityGroupPolicySet = new(vpc.SecurityGroupPolicySet)
	request.SecurityGroupPolicySet.Egress = getSecurityGroupPolicies(egress)
	request.SecurityGroupPolicySet.Ingress = getSecurityGroupPolicies(ingress)

	return resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := me.client.UseVpcClient().ModifySecurityGroupPolicies(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	})
}

func (me *VpcService) DeleteLiteRules(ctx context.Context, sgId string, rules []VpcSecurityGroupLiteRule, isIngress bool) error {
	logId := getLogId(ctx)

	request := vpc.NewDeleteSecurityGroupPoliciesRequest()
	request.SecurityGroupId = &sgId
	request.SecurityGroupPolicySet = new(vpc.SecurityGroupPolicySet)

	if isIngress {
		request.SecurityGroupPolicySet.Ingress = getSecurityGroupPolicies(rules)
	} else {
		request.SecurityGroupPolicySet.Egress = getSecurityGroupPolicies(rules)
	}

	return resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := me.client.UseVpcClient().DeleteSecurityGroupPolicies(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)

			return retryError(err)
		}

		return nil
	})
}

func (me *VpcService) AttachLiteRulesToSecurityGroup(ctx context.Context, sgId string, ingress, egress []VpcSecurityGroupLiteRule) error {
	logId := getLogId(ctx)

	if err := me.modifyLiteRulesInSecurityGroup(ctx, sgId, ingress, egress); err != nil {
		log.Printf("[CRITAL]%s attach lite rules to security group failed, reason: %v", logId, err)

		return err
	}

	return nil
}

func (me *VpcService) DescribeSecurityGroupPolices(ctx context.Context, sgId string) (ingress, egress []VpcSecurityGroupLiteRule, exist bool, err error) {
	logId := getLogId(ctx)

	request := vpc.NewDescribeSecurityGroupPoliciesRequest()
	request.SecurityGroupId = &sgId

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := me.client.UseVpcClient().DescribeSecurityGroupPolicies(request)
		if err != nil {
			if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
				if sdkError.Code == "ResourceNotFound" {
					return nil
				}
			}

			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		policySet := response.Response.SecurityGroupPolicySet

		for _, in := range policySet.Ingress {
			if nilFields := CheckNil(in, map[string]string{
				"Protocol":        "protocol",
				"Port":            "port",
				"Action":          "action",
				"SecurityGroupId": "nested security group id",
			}); len(nilFields) > 0 {
				err := fmt.Errorf("api[%s] security group ingress %v are nil", request.GetAction(), nilFields)
				log.Printf("[CRITAL]%s %v", logId, err)
			}

			liteRule := VpcSecurityGroupLiteRule{
				//protocol:        strings.ToUpper(*in.Protocol),
				port:            *in.Port,
				cidrIp:          *in.CidrBlock,
				action:          *in.Action,
				securityGroupId: *in.SecurityGroupId,
			}

			if in.Protocol != nil {
				liteRule.protocol = strings.ToUpper(*in.Protocol)
			}

			if in.AddressTemplate != nil {
				liteRule.addressId = *in.AddressTemplate.AddressId
				liteRule.addressGroupId = *in.AddressTemplate.AddressGroupId
			}

			ingress = append(ingress, liteRule)
		}

		for _, eg := range policySet.Egress {
			if nilFields := CheckNil(eg, map[string]string{
				"Protocol":        "protocol",
				"Port":            "port",
				"Action":          "action",
				"SecurityGroupId": "nested security group id",
			}); len(nilFields) > 0 {
				err := fmt.Errorf("api[%s] security group egress %v are nil", request.GetAction(), nilFields)
				log.Printf("[CRITAL]%s %v", logId, err)
			}

			liteRule := VpcSecurityGroupLiteRule{
				protocol:        strings.ToUpper(*eg.Protocol),
				port:            *eg.Port,
				action:          *eg.Action,
				cidrIp:          *eg.CidrBlock,
				securityGroupId: *eg.SecurityGroupId,
			}

			if eg.AddressTemplate != nil {
				liteRule.addressId = *eg.AddressTemplate.AddressId
				liteRule.addressGroupId = *eg.AddressTemplate.AddressGroupId
			}

			egress = append(egress, liteRule)
		}

		exist = true

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s describe security group policies failed, rason: %v", logId, err)
		return nil, nil, false, err
	}

	return
}

func (me *VpcService) DetachAllLiteRulesFromSecurityGroup(ctx context.Context, sgId string) error {
	logId := getLogId(ctx)

	request := vpc.NewModifySecurityGroupPoliciesRequest()
	request.SecurityGroupId = &sgId
	request.SecurityGroupPolicySet = &vpc.SecurityGroupPolicySet{
		Version: helper.String("0"),
	}

	return resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := me.client.UseVpcClient().ModifySecurityGroupPolicies(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	})
}

type securityGroupRuleBasicInfo struct {
	SgId                    string  `json:"sg_id"`
	PolicyType              string  `json:"policy_type"`
	CidrIp                  *string `json:"cidr_ip,omitempty"`
	Protocol                *string `json:"protocol"`
	PortRange               *string `json:"port_range"`
	Action                  string  `json:"action"`
	SourceSgId              *string `json:"source_sg_id"`
	Description             *string `json:"description,omitempty"`
	AddressTemplateId       *string `json:"address_template_id,omitempty"`
	AddressTemplateGroupId  *string `json:"address_template_group_id,omitempty"`
	ProtocolTemplateId      *string `json:"protocol_template_id,omitempty"`
	ProtocolTemplateGroupId *string `json:"protocol_template_group_id,omitempty"`
}

// Build an ID for a Security Group Rule (new version)
func buildSecurityGroupRuleId(info securityGroupRuleBasicInfo) (ruleId string, err error) {
	b, err := json.Marshal(info)
	if err != nil {
		return "", err
	}

	log.Printf("[DEBUG] build rule is %s", string(b))

	return base64.StdEncoding.EncodeToString(b), nil
}

// Parse Security Group Rule ID
func parseSecurityGroupRuleId(ruleId string) (info securityGroupRuleBasicInfo, errRet error) {
	log.Printf("[DEBUG] parseSecurityGroupRuleId before: %v", ruleId)

	// new version ID
	if b, err := base64.StdEncoding.DecodeString(ruleId); err == nil {
		errRet = json.Unmarshal(b, &info)
		return
	}

	// old version ID
	m := make(map[string]string)
	ruleQueryStrings := strings.Split(ruleId, "&")
	if len(ruleQueryStrings) == 0 {
		errRet = errors.New("ruleId is invalid")
		return
	}
	for _, str := range ruleQueryStrings {
		arr := strings.Split(str, "=")
		if len(arr) != 2 {
			errRet = errors.New("ruleId is invalid")
			return
		}
		m[arr[0]] = arr[1]
	}

	info.SgId = m["sgId"]
	info.PolicyType = m["direction"]
	info.Action = m["action"]

	// the newest version include template
	addressTemplateId, addressTemplateOk := m["address_template_id"]
	addressGroupTemplateId, addressTemplateGroupOk := m["address_template_group_id"]
	if addressTemplateOk || addressTemplateGroupOk {
		if addressTemplateGroupOk {
			info.AddressTemplateGroupId = common.StringPtr(addressGroupTemplateId)
		} else {
			info.AddressTemplateId = common.StringPtr(addressTemplateId)
		}
		info.CidrIp = common.StringPtr("")
		info.SourceSgId = common.StringPtr("")
	} else {
		if m["sourceSgid"] == "" {
			info.CidrIp = common.StringPtr(m["cidrIp"])
		} else {
			info.CidrIp = common.StringPtr("")
		}
		info.SourceSgId = common.StringPtr(m["sourceSgid"])
	}

	protocolTemplateId, protocolTemplateOk := m["protocol_template_id"]
	protocolGroupTemplateId, protocolTemplateGroupOk := m["protocol_template_group_id"]
	if protocolTemplateOk || protocolTemplateGroupOk {
		if protocolTemplateGroupOk {
			info.ProtocolTemplateGroupId = common.StringPtr(protocolGroupTemplateId)
		} else {
			info.ProtocolTemplateId = common.StringPtr(protocolTemplateId)
		}
		info.Protocol = common.StringPtr("")
		info.PortRange = common.StringPtr("")
	} else {
		info.Protocol = common.StringPtr(m["ipProtocol"])
		info.PortRange = common.StringPtr(m["portRange"])
	}

	info.Description = common.StringPtr(m["description"])

	log.Printf("[DEBUG] parseSecurityGroupRuleId after: %v", info)
	return
}

func comparePolicyAndSecurityGroupInfo(policy *vpc.SecurityGroupPolicy, info securityGroupRuleBasicInfo) bool {
	// policy.CidrBlock will be nil if address template is set
	if policy.CidrBlock != nil && *policy.CidrBlock != "" {
		if *policy.CidrBlock != *info.CidrIp {
			return false
		}
	}

	// policy.Port will be nil if protocol template is set
	if policy.Port != nil && *policy.Port != "" {
		if *policy.Port != *info.PortRange {
			return false
		}
	}

	// policy.Protocol will be nil if protocol template is set
	if policy.Protocol != nil && *policy.Protocol != "" {
		if !strings.EqualFold(*policy.Protocol, *info.Protocol) {
			return false
		}
	}

	// policy.SecurityGroupId always not nil
	if *policy.SecurityGroupId != *info.SourceSgId {
		return false
	}

	if !strings.EqualFold(*policy.Action, info.Action) {
		return false
	}

	// if template is not null it must be compared
	if info.ProtocolTemplateId != nil && *info.ProtocolTemplateId != "" {
		if policy.ServiceTemplate == nil || policy.ServiceTemplate.ServiceId == nil || *info.ProtocolTemplateId != *policy.ServiceTemplate.ServiceId {
			log.Printf("%s %v test", *info.ProtocolTemplateId, policy.ServiceTemplate)
			return false
		}
	}
	if info.ProtocolTemplateGroupId != nil && *info.ProtocolTemplateGroupId != "" {
		if policy.ServiceTemplate == nil || policy.ServiceTemplate.ServiceGroupId == nil || *info.ProtocolTemplateGroupId != *policy.ServiceTemplate.ServiceGroupId {
			log.Printf("%s %v test", *info.ProtocolTemplateGroupId, policy.ServiceTemplate)
			return false
		}
	}
	if info.AddressTemplateGroupId != nil && *info.AddressTemplateGroupId != "" {
		if policy.AddressTemplate == nil || policy.AddressTemplate.AddressGroupId == nil || *info.AddressTemplateGroupId != *policy.AddressTemplate.AddressGroupId {
			return false
		}
	}
	if info.AddressTemplateId != nil && *info.AddressTemplateId != "" {
		if policy.AddressTemplate == nil || policy.AddressTemplate.AddressId == nil || *info.AddressTemplateId != *policy.AddressTemplate.AddressId {
			return false
		}
	}

	return true
}

func parseRule(str string) (liteRule VpcSecurityGroupLiteRule, err error) {
	split := strings.Split(str, "#")
	if len(split) != 4 {
		err = fmt.Errorf("invalid security group rule %s", str)
		return
	}

	var (
		source string
		// source is "sg-xxxxxx" / "ipm-xxxxxx" / "ipmg-xxxxxx" formatted
		isInstanceIdSource = true
	)

	liteRule.action, source, liteRule.port, liteRule.protocol = split[0], split[1], split[2], split[3]

	if securityGroupIdRE.MatchString(source) {
		liteRule.securityGroupId = source
	} else if ipAddressIdRE.MatchString(source) {
		liteRule.addressId = source
	} else if ipAddressGroupIdRE.MatchString(source) {
		liteRule.addressGroupId = source
	} else {
		isInstanceIdSource = false
		liteRule.cidrIp = source
	}

	switch liteRule.action {
	default:
		err = fmt.Errorf("invalid action %s, allow action is `ACCEPT` or `DROP`", liteRule.action)
		return
	case "ACCEPT", "DROP":
	}

	if net.ParseIP(liteRule.cidrIp) == nil && !isInstanceIdSource {
		if _, _, err = net.ParseCIDR(liteRule.cidrIp); err != nil {
			err = fmt.Errorf("invalid cidr_ip %s, allow cidr_ip format is `8.8.8.8` or `10.0.1.0/24`", liteRule.cidrIp)
			return
		}
	}

	if liteRule.port != "ALL" && !portRE.MatchString(liteRule.port) {
		err = fmt.Errorf("invalid port %s, allow port format is `ALL`, `53`, `80,443` or `80-90`", liteRule.port)
		return
	}

	switch liteRule.protocol {
	default:
		err = fmt.Errorf("invalid protocol %s, allow protocol is `ALL`, `TCP`, `UDP` or `ICMP`", liteRule.protocol)
		return

	case "ALL", "ICMP":
		if liteRule.port != "ALL" {
			err = fmt.Errorf("when protocol is %s, port must be ALL", liteRule.protocol)
			return
		}

		// when protocol is ALL or ICMP, port should be "" to avoid sdk error
		liteRule.port = ""

	case "TCP", "UDP":
	}

	return
}

/*
EIP
*/
func (me *VpcService) DescribeEipById(ctx context.Context, eipId string) (eip *vpc.Address, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeAddressesRequest()
	request.AddressIds = []*string{&eipId}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeAddresses(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.AddressSet) < 1 {
		return
	}
	eip = response.Response.AddressSet[0]
	return
}

func (me *VpcService) DescribeEipByFilter(ctx context.Context, filters map[string][]string) (eips []*vpc.Address, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeAddressesRequest()
	request.Filters = make([]*vpc.Filter, 0, len(filters))
	for k, v := range filters {
		filter := &vpc.Filter{
			Name:   helper.String(k),
			Values: []*string{},
		}
		for _, vv := range v {
			filter.Values = append(filter.Values, helper.String(vv))
		}
		request.Filters = append(request.Filters, filter)
	}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeAddresses(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		errRet = err
		return
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	eips = response.Response.AddressSet
	return
}

func (me *VpcService) ModifyEipName(ctx context.Context, eipId, eipName string) error {
	logId := getLogId(ctx)
	request := vpc.NewModifyAddressAttributeRequest()
	request.AddressId = &eipId
	request.AddressName = &eipName

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ModifyAddressAttribute(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return nil
}

func (me *VpcService) ModifyEipBandwidthOut(ctx context.Context, eipId string, bandwidthOut int) error {
	logId := getLogId(ctx)
	request := vpc.NewModifyAddressesBandwidthRequest()
	request.AddressIds = []*string{&eipId}
	request.InternetMaxBandwidthOut = helper.IntInt64(bandwidthOut)

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ModifyAddressesBandwidth(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return nil
}

func (me *VpcService) DeleteEip(ctx context.Context, eipId string) error {
	logId := getLogId(ctx)
	request := vpc.NewReleaseAddressesRequest()
	request.AddressIds = []*string{&eipId}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ReleaseAddresses(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return nil
}

func (me *VpcService) AttachEip(ctx context.Context, eipId, instanceId string) error {
	logId := getLogId(ctx)
	request := vpc.NewAssociateAddressRequest()
	request.AddressId = &eipId
	request.InstanceId = &instanceId

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().AssociateAddress(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return nil
}

func (me *VpcService) DescribeNatGatewayById(ctx context.Context, natGateWayId string) (natGateWay *vpc.NatGateway, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeNatGatewaysRequest()
	request.NatGatewayIds = []*string{&natGateWayId}
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeNatGateways(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	if len(response.Response.NatGatewaySet) > 0 {
		natGateWay = response.Response.NatGatewaySet[0]
	}

	return
}

func (me *VpcService) DescribeNatGatewayByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.NatGateway, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeNatGatewaysRequest()
	)
	request.Filters = make([]*vpc.Filter, 0, len(filters))
	for k, v := range filters {
		filter := vpc.Filter{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.NatGateway, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeNatGateways(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.NatGatewaySet) < 1 {
			break
		}
		instances = append(instances, response.Response.NatGatewaySet...)
		if len(response.Response.NatGatewaySet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DeleteNatGateway(ctx context.Context, natGatewayId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteNatGatewayRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.NatGatewayId = &natGatewayId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteNatGateway(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func (me *VpcService) DisassociateNatGatewayAddress(ctx context.Context, request *vpc.DisassociateNatGatewayAddressRequest) (errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	// Check if Nat Gateway Ip still associate
	gateway, err := me.DescribeNatGatewayById(ctx, *request.NatGatewayId)

	if err != nil {
		errRet = err
		return
	}

	if gateway == nil || len(gateway.PublicIpAddressSet) == 0 {
		return
	}

	var gatewayAddresses []string
	var candidates []*string

	for i := range gateway.PublicIpAddressSet {
		addr := gateway.PublicIpAddressSet[i].PublicIpAddress
		gatewayAddresses = append(gatewayAddresses, *addr)
	}

	for i := range request.PublicIpAddresses {
		addr := request.PublicIpAddresses[i]
		if helper.StringsContain(gatewayAddresses, *addr) {
			candidates = append(candidates, addr)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	request.PublicIpAddresses = candidates

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DisassociateNatGatewayAddress(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *VpcService) UnattachEip(ctx context.Context, eipId string) error {
	eipUnattachLocker.Lock()
	defer eipUnattachLocker.Unlock()

	logId := getLogId(ctx)
	eip, err := me.DescribeEipById(ctx, eipId)
	if err != nil {
		return err
	}
	if eip == nil || *eip.AddressStatus == EIP_STATUS_UNBIND {
		return nil
	}

	// DisassociateAddress Doesn't support Disassociate NAT Address
	if eip.InstanceId != nil && strings.HasPrefix(*eip.InstanceId, "nat-") {
		request := vpc.NewDisassociateNatGatewayAddressRequest()
		request.NatGatewayId = eip.InstanceId
		request.PublicIpAddresses = []*string{eip.AddressIp}
		err := me.DisassociateNatGatewayAddress(ctx, request)
		if err != nil {
			return err
		}

		outErr := resource.Retry(readRetryTimeout*3, func() *resource.RetryError {
			eip, err := me.DescribeEipById(ctx, eipId)
			if err != nil {
				return retryError(err)
			}
			if eip != nil && *eip.AddressStatus != EIP_STATUS_UNBIND {
				return resource.RetryableError(fmt.Errorf("eip is still %s", EIP_STATUS_UNBIND))
			}
			return nil
		})

		if outErr != nil {
			return outErr
		}
	}

	request := vpc.NewDisassociateAddressRequest()
	request.AddressId = &eipId
	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DisassociateAddress(request)
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
			logId, request.GetAction(), request.ToJsonString(), err.Error())
		return err
	}
	if response.Response.TaskId == nil {
		return nil
	}
	taskId, err := strconv.ParseUint(*response.Response.TaskId, 10, 64)
	if err != nil {
		return nil
	}

	taskRequest := vpc.NewDescribeTaskResultRequest()
	taskRequest.TaskId = &taskId
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(taskRequest.GetAction())
		taskResponse, err := me.client.UseVpcClient().DescribeTaskResult(taskRequest)
		if err != nil {
			return retryError(err)
		}
		if taskResponse.Response.Result != nil && *taskResponse.Response.Result == EIP_TASK_STATUS_RUNNING {
			return resource.RetryableError(errors.New("eip task is running"))
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (me *VpcService) CreateEni(
	ctx context.Context,
	name, vpcId, subnetId, desc string,
	securityGroups []string,
	ipv4Count *int,
	ipv4s []VpcEniIP,
) (id string, err error) {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	createRequest := vpc.NewCreateNetworkInterfaceRequest()
	createRequest.NetworkInterfaceName = &name
	createRequest.VpcId = &vpcId
	createRequest.SubnetId = &subnetId
	createRequest.NetworkInterfaceDescription = &desc

	if len(securityGroups) > 0 {
		createRequest.SecurityGroupIds = common.StringPtrs(securityGroups)
	}

	if ipv4Count != nil {
		// create will assign a primary ip, secondary ip count is *ipv4Count-1
		createRequest.SecondaryPrivateIpAddressCount = helper.IntUint64(*ipv4Count - 1)
	}

	var wantIpv4 []string

	for _, ipv4 := range ipv4s {
		wantIpv4 = append(wantIpv4, ipv4.ip.String())
		createRequest.PrivateIpAddresses = append(createRequest.PrivateIpAddresses, &vpc.PrivateIpAddressSpecification{
			PrivateIpAddress: helper.String(ipv4.ip.String()),
			Primary:          helper.Bool(ipv4.primary),
			Description:      ipv4.desc,
		})
	}

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(createRequest.GetAction())

		response, err := client.CreateNetworkInterface(createRequest)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, createRequest.GetAction(), createRequest.ToJsonString(), err)
			return retryError(err)
		}

		eni := response.Response.NetworkInterface

		if eni == nil {
			err := fmt.Errorf("api[%s] eni is nil", createRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if eni.NetworkInterfaceId == nil {
			err := fmt.Errorf("api[%s] eni id is nil", createRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		ipv4Set := eni.PrivateIpAddressSet

		if len(wantIpv4) > 0 {
			checkMap := make(map[string]bool, len(wantIpv4))
			for _, ipv4 := range wantIpv4 {
				checkMap[ipv4] = false
			}

			for _, ipv4 := range ipv4Set {
				if ipv4.PrivateIpAddress == nil {
					err := fmt.Errorf("api[%s] eni ipv4 ip is nil", createRequest.GetAction())
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}

				checkMap[*ipv4.PrivateIpAddress] = true
			}

			for ipv4, checked := range checkMap {
				if !checked {
					err := fmt.Errorf("api[%s] doesn't assign %s ip", createRequest.GetAction(), ipv4)
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}
			}
		} else {
			if len(ipv4Set) != *ipv4Count {
				err := fmt.Errorf("api[%s] doesn't assign enough ip", createRequest.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			wantIpv4 = make([]string, 0, *ipv4Count)
			for _, ipv4 := range ipv4Set {
				if ipv4.PrivateIpAddress == nil {
					err := fmt.Errorf("api[%s] eni ipv4 ip is nil", createRequest.GetAction())
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}

				wantIpv4 = append(wantIpv4, *ipv4.PrivateIpAddress)
			}
		}

		id = *eni.NetworkInterfaceId

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s create eni failed, reason: %v", logId, err)
		return "", err
	}

	if err := waitEniReady(ctx, id, client, wantIpv4, nil); err != nil {
		log.Printf("[CRITAL]%s create eni failed, reason: %v", logId, err)
		return "", err
	}

	return
}

func (me *VpcService) describeEnis(
	ctx context.Context,
	ids []string,
	vpcId, subnetId, id, cvmId, sgId, name, desc, ipv4 *string,
	tags map[string]string,
) (enis []*vpc.NetworkInterface, err error) {
	logId := getLogId(ctx)

	request := vpc.NewDescribeNetworkInterfacesRequest()

	if len(ids) > 0 {
		request.NetworkInterfaceIds = common.StringPtrs(ids)
	}

	if vpcId != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("vpc-id"),
			Values: []*string{vpcId},
		})
	}

	if subnetId != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("subnet-id"),
			Values: []*string{subnetId},
		})
	}

	if id != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("network-interface-id"),
			Values: []*string{id},
		})
	}

	if cvmId != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("attachment.instance-id"),
			Values: []*string{cvmId},
		})
	}

	if sgId != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("groups.security-group-id"),
			Values: []*string{sgId},
		})
	}

	if name != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("network-interface-name"),
			Values: []*string{name},
		})
	}

	if desc != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("network-interface-description"),
			Values: []*string{desc},
		})
	}

	if ipv4 != nil {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("address-ip"),
			Values: []*string{ipv4},
		})
	}

	for k, v := range tags {
		request.Filters = append(request.Filters, &vpc.Filter{
			Name:   helper.String("tag:" + k),
			Values: []*string{helper.String(v)},
		})
	}

	var offset uint64
	request.Offset = &offset
	request.Limit = helper.IntUint64(ENI_DESCRIBE_LIMIT)

	count := ENI_DESCRIBE_LIMIT
	for count == ENI_DESCRIBE_LIMIT {
		if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())

			response, err := me.client.UseVpcClient().DescribeNetworkInterfaces(request)
			if err != nil {
				count = 0

				if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
					if sdkError.Code == "ResourceNotFound" {
						return nil
					}
				}

				log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
					logId, request.GetAction(), request.ToJsonString(), err)
				return retryError(err)
			}

			eniSet := response.Response.NetworkInterfaceSet
			count = len(eniSet)
			enis = append(enis, eniSet...)

			return nil
		}); err != nil {
			log.Printf("[CRITAL]%s read eni list failed, reason: %v", logId, err)
			return nil, err
		}

		offset += uint64(count)
	}

	return
}

func (me *VpcService) DescribeEniById(ctx context.Context, ids []string) (enis []*vpc.NetworkInterface, err error) {
	return me.describeEnis(ctx, ids, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func (me *VpcService) ModifyEniAttribute(ctx context.Context, id string, name, desc *string, sgs []string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	request := vpc.NewModifyNetworkInterfaceAttributeRequest()
	request.NetworkInterfaceId = &id
	request.NetworkInterfaceName = name
	request.NetworkInterfaceDescription = desc
	request.SecurityGroupIds = common.StringPtrs(sgs)

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := client.ModifyNetworkInterfaceAttribute(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s modify eni attribute failed, reason: %v", logId, err)
		return err
	}

	if err := waitEniReady(ctx, id, client, nil, nil); err != nil {
		log.Printf("[CRITAL]%s modify eni attribute failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) UnAssignIpv4FromEni(ctx context.Context, id string, ipv4s []string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	request := vpc.NewUnassignPrivateIpAddressesRequest()
	request.NetworkInterfaceId = &id
	request.PrivateIpAddresses = make([]*vpc.PrivateIpAddressSpecification, 0, len(ipv4s))
	for _, ipv4 := range ipv4s {
		request.PrivateIpAddresses = append(request.PrivateIpAddresses, &vpc.PrivateIpAddressSpecification{
			PrivateIpAddress: helper.String(ipv4),
		})
	}

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := client.UnassignPrivateIpAddresses(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s unassign ipv4 from eni failed, reason: %v", logId, err)
		return err
	}

	if err := waitEniReady(ctx, id, client, nil, ipv4s); err != nil {
		log.Printf("[CRITAL]%s unassign ipv4 from eni failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) AssignIpv4ToEni(ctx context.Context, id string, ipv4s []VpcEniIP, ipv4Count *int) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	request := vpc.NewAssignPrivateIpAddressesRequest()
	request.NetworkInterfaceId = &id

	if ipv4Count != nil {
		request.SecondaryPrivateIpAddressCount = helper.IntUint64(*ipv4Count)
	}

	var wantIpv4 []string

	if len(ipv4s) > 0 {
		request.PrivateIpAddresses = make([]*vpc.PrivateIpAddressSpecification, 0, len(ipv4s))
		wantIpv4 = make([]string, 0, len(ipv4s))

		for _, ipv4 := range ipv4s {
			wantIpv4 = append(wantIpv4, ipv4.ip.String())
			request.PrivateIpAddresses = append(request.PrivateIpAddresses, &vpc.PrivateIpAddressSpecification{
				PrivateIpAddress: helper.String(ipv4.ip.String()),
				Primary:          helper.Bool(ipv4.primary),
				Description:      ipv4.desc,
			})
		}
	}

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := client.AssignPrivateIpAddresses(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		ipv4Set := response.Response.PrivateIpAddressSet

		if len(wantIpv4) > 0 {
			checkMap := make(map[string]bool, len(wantIpv4))
			for _, ipv4 := range wantIpv4 {
				checkMap[ipv4] = false
			}

			for _, ipv4 := range ipv4Set {
				if ipv4.PrivateIpAddress == nil {
					err := fmt.Errorf("api[%s] eni ipv4 ip is nil", request.GetAction())
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}

				checkMap[*ipv4.PrivateIpAddress] = true
			}

			for ipv4, checked := range checkMap {
				if !checked {
					err := fmt.Errorf("api[%s] doesn't assign %s ip", request.GetAction(), ipv4)
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}
			}
		} else {
			if len(ipv4Set) != *ipv4Count {
				err := fmt.Errorf("api[%s] doesn't assign enough ip", request.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			wantIpv4 = make([]string, 0, *ipv4Count)
			for _, ipv4 := range ipv4Set {
				if ipv4.PrivateIpAddress == nil {
					err := fmt.Errorf("api[%s] eni ipv4 ip is nil", request.GetAction())
					log.Printf("[CRITAL]%s %v", logId, err)
					return resource.NonRetryableError(err)
				}

				wantIpv4 = append(wantIpv4, *ipv4.PrivateIpAddress)
			}
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s assign ipv4 to eni failed, reason: %v", logId, err)
		return err
	}

	if err := waitEniReady(ctx, id, client, wantIpv4, nil); err != nil {
		log.Printf("[CRITAL]%s assign ipv4 to eni failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) DeleteEni(ctx context.Context, id string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	deleteRequest := vpc.NewDeleteNetworkInterfaceRequest()
	deleteRequest.NetworkInterfaceId = &id

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(deleteRequest.GetAction())

		if _, err := client.DeleteNetworkInterface(deleteRequest); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, deleteRequest.GetAction(), deleteRequest.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s delete eni failed, reason: %v", logId, err)
		return err
	}

	describeRequest := vpc.NewDescribeNetworkInterfacesRequest()
	describeRequest.NetworkInterfaceIds = []*string{&id}

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(describeRequest.GetAction())

		response, err := client.DescribeNetworkInterfaces(describeRequest)
		if err != nil {
			if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
				if sdkError.Code == "ResourceNotFound" {
					return nil
				}
			}

			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, describeRequest.GetAction(), describeRequest.ToJsonString(), err)
			return retryError(err)
		}

		for _, eni := range response.Response.NetworkInterfaceSet {
			if eni.NetworkInterfaceId == nil {
				err := fmt.Errorf("api[%s] eni id is nil", describeRequest.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			if *eni.NetworkInterfaceId == id {
				err := errors.New("eni still exists")
				log.Printf("[DEBUG]%s %v", logId, err)
				return resource.RetryableError(err)
			}
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s delete eni failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) AttachEniToCvm(ctx context.Context, eniId, cvmId string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	attachRequest := vpc.NewAttachNetworkInterfaceRequest()
	attachRequest.NetworkInterfaceId = &eniId
	attachRequest.InstanceId = &cvmId

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(attachRequest.GetAction())

		if _, err := client.AttachNetworkInterface(attachRequest); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, attachRequest.GetAction(), attachRequest.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s attach eni to instance failed, reason: %v", logId, err)
		return err
	}

	describeRequest := vpc.NewDescribeNetworkInterfacesRequest()
	describeRequest.NetworkInterfaceIds = []*string{&eniId}

	if err := resource.Retry(2*readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(describeRequest.GetAction())

		response, err := client.DescribeNetworkInterfaces(describeRequest)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, describeRequest.GetAction(), describeRequest.ToJsonString(), err)
			return retryError(err)
		}

		var eni *vpc.NetworkInterface
		for _, e := range response.Response.NetworkInterfaceSet {
			if e.NetworkInterfaceId == nil {
				err := fmt.Errorf("api[%s] eni id is nil", describeRequest.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			if *e.NetworkInterfaceId == eniId {
				eni = e
				break
			}
		}

		if eni == nil {
			err := fmt.Errorf("api[%s] eni not found", describeRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if eni.Attachment == nil {
			err := fmt.Errorf("api[%s] eni attachment is not ready", describeRequest.GetAction())
			log.Printf("[DEBUG]%s %v", logId, err)
			return resource.RetryableError(err)
		}

		if eni.Attachment.InstanceId == nil {
			err := fmt.Errorf("api[%s] eni attach instance id is nil", describeRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if *eni.Attachment.InstanceId != cvmId {
			err := fmt.Errorf("api[%s] eni attach instance id is not right", describeRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if eni.State == nil {
			err := fmt.Errorf("api[%s] eni state is nil", describeRequest.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if *eni.State != ENI_STATE_AVAILABLE {
			err := errors.New("eni is not ready")
			log.Printf("[DEBUG]%s %v", logId, err)
			return resource.RetryableError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s attach eni to instance failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) DetachEniFromCvm(ctx context.Context, eniId, cvmId string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	request := vpc.NewDetachNetworkInterfaceRequest()
	request.NetworkInterfaceId = &eniId
	request.InstanceId = &cvmId

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := client.DetachNetworkInterface(request); err != nil {
			if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
				switch sdkError.Code {
				case "UnsupportedOperation.InvalidState":
					return resource.RetryableError(errors.New("cvm may still bind eni"))

				case "ResourceNotFound":
					// eni or cvm doesn't exist
					return nil
				}
			}

			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s detach eni from instance failed, reason: %v", logId, err)
		return err
	}

	if err := waitEniDetach(ctx, eniId, client); err != nil {
		log.Printf("[CRITAL]%s detach eni from instance failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) ModifyEniPrimaryIpv4Desc(ctx context.Context, id, ip string, desc *string) error {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	request := vpc.NewModifyPrivateIpAddressesAttributeRequest()
	request.NetworkInterfaceId = &id
	request.PrivateIpAddresses = []*vpc.PrivateIpAddressSpecification{
		{
			PrivateIpAddress: &ip,
			Description:      desc,
		},
	}

	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if _, err := client.ModifyPrivateIpAddressesAttribute(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s modify eni primary ipv4 description failed, reason: %v", logId, err)
		return err
	}

	if err := waitEniReady(ctx, id, client, []string{ip}, nil); err != nil {
		log.Printf("[CRITAL]%s modify eni primary ipv4 description failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func (me *VpcService) DescribeEniByFilters(
	ctx context.Context,
	vpcId, subnetId, cvmId, sgId, name, desc, ipv4 *string,
	tags map[string]string,
) (enis []*vpc.NetworkInterface, err error) {
	return me.describeEnis(ctx, nil, vpcId, subnetId, nil, cvmId, sgId, name, desc, ipv4, tags)
}

func (me *VpcService) DescribeHaVipByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.HaVip, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeHaVipsRequest()
	)
	request.Filters = make([]*vpc.Filter, 0, len(filters))
	for k, v := range filters {
		filter := vpc.Filter{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.HaVip, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeHaVips(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.HaVipSet) < 1 {
			break
		}
		instances = append(instances, response.Response.HaVipSet...)
		if len(response.Response.HaVipSet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DescribeHaVipEipById(ctx context.Context, haVipEipAttachmentId string) (eip string, haVip string, has bool, errRet error) {
	logId := getLogId(ctx)
	client := me.client.UseVpcClient()

	items := strings.Split(haVipEipAttachmentId, "#")
	if len(items) != 2 {
		errRet = fmt.Errorf("decode HA VIP EIP attachment ID error %s", haVipEipAttachmentId)
		return
	}
	haVipId := items[0]
	addressIp := items[1]

	request := vpc.NewDescribeHaVipsRequest()
	request.HaVipIds = []*string{&haVipId}
	eip = ""
	haVip = ""
	if err := resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		if result, err := client.DescribeHaVips(request); err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		} else {
			length := len(result.Response.HaVipSet)
			if length != 1 {
				if length == 0 {
					return nil
				} else {
					err = fmt.Errorf("query havip %s eip %s failed, the SDK returns %d HaVips", haVipId, addressIp, length)
					return resource.NonRetryableError(err)
				}
			} else {
				eip = *result.Response.HaVipSet[0].AddressIp
				if addressIp != eip {
					return nil
				}
				has = true
				haVip = haVipId
			}
		}
		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s describe HA VIP attachment failed, reason: %v", logId, err)
		errRet = err
	}
	return eip, haVip, has, errRet
}

func (me *VpcService) DeleteHaVip(ctx context.Context, haVipId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteHaVipRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.HaVipId = &haVipId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteHaVip(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func waitEniReady(ctx context.Context, id string, client *vpc.Client, wantIpv4s []string, dropIpv4s []string) error {
	logId := getLogId(ctx)

	wantCheckMap := make(map[string]bool, len(wantIpv4s))
	for _, ipv4 := range wantIpv4s {
		wantCheckMap[ipv4] = false
	}

	dropCheckMap := make(map[string]struct{}, len(dropIpv4s))
	for _, ipv4 := range dropIpv4s {
		dropCheckMap[ipv4] = struct{}{}
	}

	request := vpc.NewDescribeNetworkInterfacesRequest()
	request.NetworkInterfaceIds = []*string{helper.String(id)}

	if err := resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := client.DescribeNetworkInterfaces(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		var eni *vpc.NetworkInterface
		for _, networkInterface := range response.Response.NetworkInterfaceSet {
			if networkInterface.NetworkInterfaceId == nil {
				err := fmt.Errorf("api[%s] eni id is nil", request.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			if *networkInterface.NetworkInterfaceId == id {
				eni = networkInterface
				break
			}
		}

		if eni == nil {
			err := fmt.Errorf("api[%s] eni not exist", request.GetAction())
			log.Printf("[DEBUG]%s %v", logId, err)
			return resource.RetryableError(err)
		}

		if eni.State == nil {
			err := fmt.Errorf("api[%s] eni state is nil", request.GetAction())
			log.Printf("[CRITAL]%s %v", logId, err)
			return resource.NonRetryableError(err)
		}

		if *eni.State != ENI_STATE_AVAILABLE {
			err := errors.New("eni is not available")
			log.Printf("[DEBUG]%s %v", logId, err)
			return resource.RetryableError(err)
		}

		for _, ipv4 := range eni.PrivateIpAddressSet {
			if ipv4.PrivateIpAddress == nil {
				err := fmt.Errorf("api[%s] eni ipv4 ip is nil", request.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			// check drop
			if _, ok := dropCheckMap[*ipv4.PrivateIpAddress]; ok {
				err := fmt.Errorf("api[%s] drop ip %s still exists", request.GetAction(), *ipv4.PrivateIpAddress)
				log.Printf("[DEBUG]%s %v", logId, err)
				return resource.RetryableError(err)
			}

			// check want
			if _, ok := wantCheckMap[*ipv4.PrivateIpAddress]; ok {
				wantCheckMap[*ipv4.PrivateIpAddress] = true
			}

			if ipv4.State == nil {
				err := fmt.Errorf("api[%s] eni ipv4 state is nil", request.GetAction())
				log.Printf("[CRITAL]%s %v", logId, err)
				return resource.NonRetryableError(err)
			}

			if *ipv4.State != ENI_IP_AVAILABLE {
				err := errors.New("eni ipv4 is not available")
				log.Printf("[DEBUG]%s %v", logId, err)
				return resource.RetryableError(err)
			}
		}

		for ipv4, checked := range wantCheckMap {
			if !checked {
				err := fmt.Errorf("api[%s] ipv4 %s is no ready", request.GetAction(), ipv4)
				log.Printf("[DEBUG]%s %v", logId, err)
				return resource.RetryableError(err)
			}
		}

		return nil
	}); err != nil {
		log.Printf("[CRITAL]%s eni is not available failed, reason: %v", logId, err)
		return err
	}

	return nil
}

func flattenVpnSPDList(spd []*vpc.SecurityPolicyDatabase) (mapping []*map[string]interface{}) {
	mapping = make([]*map[string]interface{}, 0, len(spd))
	for _, spg := range spd {
		item := make(map[string]interface{})
		item["local_cidr_block"] = spg.LocalCidrBlock
		item["remote_cidr_block"] = spg.RemoteCidrBlock
		mapping = append(mapping, &item)
	}
	return
}

func waitEniDetach(ctx context.Context, id string, client *vpc.Client) error {
	logId := getLogId(ctx)

	request := vpc.NewDescribeNetworkInterfacesRequest()
	request.NetworkInterfaceIds = []*string{helper.String(id)}

	return resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())

		response, err := client.DescribeNetworkInterfaces(request)
		if err != nil {
			if sdkError, ok := err.(*sdkErrors.TencentCloudSDKError); ok && sdkError.Code == "ResourceNotFound" {
				return nil
			}

			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			return retryError(err)
		}

		enis := response.Response.NetworkInterfaceSet

		if len(enis) == 0 {
			return nil
		}

		eni := enis[0]

		if eni.Attachment == nil {
			return nil
		}

		if eni.Attachment.InstanceId != nil && *eni.Attachment.InstanceId != "" {
			return resource.RetryableError(fmt.Errorf("eni %s still bind in cvm %s", id, *eni.Attachment.InstanceId))
		}

		if eni.State == nil {
			return resource.NonRetryableError(fmt.Errorf("eni %s state is nil", id))
		}

		if *eni.State != ENI_STATE_AVAILABLE {
			return resource.RetryableError(errors.New("eni is not available"))
		}

		return nil
	})
}

// deal acl
func parseACLRule(str string) (liteRule VpcACLRule, err error) {
	split := strings.Split(str, "#")
	if len(split) != 4 {
		err = fmt.Errorf("invalid acl rule %s", str)
		return
	}

	liteRule.action, liteRule.cidrIp, liteRule.port, liteRule.protocol = split[0], split[1], split[2], split[3]

	switch liteRule.action {
	default:
		err = fmt.Errorf("invalid action %s, allow action is `ACCEPT` or `DROP`", liteRule.action)
		return
	case "ACCEPT", "DROP":
	}

	if net.ParseIP(liteRule.cidrIp) == nil {
		if _, _, err = net.ParseCIDR(liteRule.cidrIp); err != nil {
			err = fmt.Errorf("invalid cidr_ip %s, allow cidr_ip format is `8.8.8.8` or `10.0.1.0/24`", liteRule.cidrIp)
			return
		}
	}

	if liteRule.port != "ALL" && !regexp.MustCompile(`^(\d{1,5},)*\d{1,5}$|^\d{1,5}-\d{1,5}$`).MatchString(liteRule.port) {
		err = fmt.Errorf("invalid port %s, allow port format is `ALL`, `53`, `80,443` or `80-90`", liteRule.port)
		return
	}

	switch liteRule.protocol {
	default:
		err = fmt.Errorf("invalid protocol %s, allow protocol is `ALL`, `TCP`, `UDP` or `ICMP`", liteRule.protocol)
		return

	case "ALL", "ICMP":
		if liteRule.port != "ALL" {
			err = fmt.Errorf("when protocol is %s, port must be ALL", liteRule.protocol)
			return
		}

		// when protocol is ALL or ICMP, port should be "" to avoid sdk error
		liteRule.port = ""

	case "TCP", "UDP":
	}

	return
}

func (me *VpcService) CreateVpcNetworkAcl(ctx context.Context, vpcID string, name string) (aclID string, errRet error) {
	var (
		logId    = getLogId(ctx)
		request  = vpc.NewCreateNetworkAclRequest()
		response *vpc.CreateNetworkAclResponse
		err      error
	)

	request.VpcId = &vpcID
	request.NetworkAclName = &name

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, err = me.client.UseVpcClient().CreateNetworkAcl(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	})

	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}

	aclID = *response.Response.NetworkAcl.NetworkAclId
	return
}

func (me *VpcService) AttachRulesToACL(ctx context.Context, aclID string, ingressParm, egressParm []VpcACLRule) (errRet error) {
	logId := getLogId(ctx)

	if len(ingressParm) == 0 && len(egressParm) == 0 {
		return
	}
	if errRet = me.ModifyNetWorkAclRules(ctx, aclID, ingressParm, egressParm); errRet != nil {
		log.Printf("[CRITAL]%s attach rules to acl failed, reason: %v", logId, errRet)
	}
	return
}

func (me *VpcService) ModifyNetWorkAclRules(ctx context.Context, aclID string, ingressParm, egressParm []VpcACLRule) (errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewModifyNetworkAclEntriesRequest()
		err     error
		ingress []*vpc.NetworkAclEntry
		egress  []*vpc.NetworkAclEntry
	)

	for i := range ingressParm {
		policy := &vpc.NetworkAclEntry{
			Protocol:  &ingressParm[i].protocol,
			CidrBlock: &ingressParm[i].cidrIp,
			Action:    &ingressParm[i].action,
		}

		if ingressParm[i].port != "" {
			policy.Port = &ingressParm[i].port
		}

		ingress = append(ingress, policy)
	}

	for i := range egressParm {
		policy := &vpc.NetworkAclEntry{
			Protocol:  &egressParm[i].protocol,
			CidrBlock: &egressParm[i].cidrIp,
			Action:    &egressParm[i].action,
		}

		if egressParm[i].port != "" {
			policy.Port = &egressParm[i].port
		}

		egress = append(egress, policy)
	}

	request.NetworkAclId = &aclID
	request.NetworkAclEntrySet = &vpc.NetworkAclEntrySet{
		Ingress: ingress,
		Egress:  egress,
	}

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err = me.client.UseVpcClient().ModifyNetworkAclEntries(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	})

	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}

	return
}

func (me *VpcService) DescribeNetWorkByACLID(ctx context.Context, aclID string) (info *vpc.NetworkAcl, has int, errRet error) {
	results, err := me.DescribeNetWorkAcls(ctx, aclID, "", "")
	if err != nil {
		errRet = err
		return
	}

	has = len(results)
	if has == 0 {
		return
	}

	info = results[0]
	return
}

func (me *VpcService) DeleteAcl(ctx context.Context, aclID string) (errRet error) {
	var (
		logId       = getLogId(ctx)
		err         error
		networkAcls []*vpc.NetworkAcl
		request     = vpc.NewDeleteNetworkAclRequest()
	)

	// Disassociate Network Acl Subnets
	networkAcls, err = me.DescribeNetWorkAcls(ctx, aclID, "", "")
	if err != nil {
		errRet = err
		return
	}

	if len(networkAcls) > 0 {
		subnets := networkAcls[0].SubnetSet
		if len(subnets) > 0 {
			requestSubnet := vpc.NewDisassociateNetworkAclSubnetsRequest()
			requestSubnet.NetworkAclId = &aclID

			for i := range subnets {
				requestSubnet.SubnetIds = append(requestSubnet.SubnetIds, subnets[i].SubnetId)
			}

			err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
				ratelimit.Check(request.GetAction())
				_, err = me.client.UseVpcClient().DisassociateNetworkAclSubnets(requestSubnet)
				if err != nil {
					if ee, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
						if ee.Code == VPCNotFound {
							log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
								logId, request.GetAction(), request.ToJsonString(), err)
							return nil
						}
					}
					return retryError(err, InternalError)
				}
				return nil
			})
			if err != nil {
				errRet = err
				return
			}
		}
	}

	// delete acl
	request.NetworkAclId = &aclID
	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err = me.client.UseVpcClient().DeleteNetworkAcl(request)

		if err != nil {
			ee, ok := err.(*sdkErrors.TencentCloudSDKError)
			if !ok {
				return retryError(err, InternalError)
			}
			if ee.Code == VPCNotFound {
				log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
					logId, request.GetAction(), request.ToJsonString(), err)
				return nil
			}
			return retryError(err, InternalError)
		}

		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}

	return
}

func (me *VpcService) ModifyVpcNetworkAcl(ctx context.Context, id *string, name *string) (errRet error) {
	var (
		logId   = getLogId(ctx)
		err     error
		request = vpc.NewModifyNetworkAclAttributeRequest()
	)

	request.NetworkAclId = id
	request.NetworkAclName = name

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err = me.client.UseVpcClient().ModifyNetworkAclAttribute(request)
		if err != nil {
			ee, ok := err.(*sdkErrors.TencentCloudSDKError)
			if !ok {
				return retryError(err, InternalError)
			}
			if ee.Code == VPCNotFound {
				log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
					logId, request.GetAction(), request.ToJsonString(), err)
				return resource.NonRetryableError(err)
			}
			return retryError(err, InternalError)
		}

		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}

	return
}

func (me *VpcService) AssociateAclSubnets(ctx context.Context, aclId string, subnetIds []string) (errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewAssociateNetworkAclSubnetsRequest()
		err     error
		subIds  []*string
	)

	for _, i := range subnetIds {
		subIds = append(subIds, &i)
	}

	request.NetworkAclId = &aclId
	request.SubnetIds = subIds

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err = me.client.UseVpcClient().AssociateNetworkAclSubnets(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}
	return
}

func (me *VpcService) DescribeNetWorkAcls(ctx context.Context, aclID, vpcID, name string) (info []*vpc.NetworkAcl, errRet error) {
	var (
		logId            = getLogId(ctx)
		request          = vpc.NewDescribeNetworkAclsRequest()
		response         *vpc.DescribeNetworkAclsResponse
		err              error
		filters          []*vpc.Filter
		offset, pageSize uint64 = 0, 100
	)

	if vpcID != "" {
		filters = me.fillFilter(filters, "vpc-id", vpcID)
	}
	if aclID != "" {
		filters = me.fillFilter(filters, "network-acl-id", aclID)
	}
	if name != "" {
		filters = me.fillFilter(filters, "network-acl-name", name)
	}

	if len(filters) > 0 {
		request.Filters = filters
	}

	request.Offset = &offset
	request.Limit = &pageSize
	for {
		err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())
			response, err = me.client.UseVpcClient().DescribeNetworkAcls(request)
			if err != nil {
				if ee, ok := err.(*sdkErrors.TencentCloudSDKError); ok {
					if ee.Code == VPCNotFound {
						return nil
					}
				}
				return retryError(err, InternalError)
			}
			return nil
		})

		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), err)
			errRet = err
			return
		}
		if response.Response == nil {
			return
		}

		info = append(info, response.Response.NetworkAclSet...)
		if len(response.Response.NetworkAclSet) < int(pageSize) {
			break
		}

		offset += pageSize
	}

	return
}

func (me *VpcService) DescribeByAclId(ctx context.Context, attachmentAcl string) (has bool, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDisassociateNetworkAclSubnetsRequest()
		aclId   string
	)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	if attachmentAcl == "" {
		errRet = fmt.Errorf("DisassociateNetworkAclSubnets  can not invoke by empty routeTableId.")
		return
	}

	aclId = strings.Split(attachmentAcl, "#")[0]

	results, err := me.DescribeNetWorkAcls(ctx, aclId, "", "")
	if err != nil {
		errRet = err
		return
	}
	if len(results) < 1 || len(results[0].SubnetSet) < 1 {
		return
	}

	has = true
	return
}

func (me *VpcService) DeleteAclAttachment(ctx context.Context, attachmentAcl string) (errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDisassociateNetworkAclSubnetsRequest()
		err     error
	)

	if attachmentAcl == "" {
		errRet = fmt.Errorf("DeleteRouteTable can not invoke by empty NetworkAclId.")
		return
	}

	items := strings.Split(attachmentAcl, "#")
	request.NetworkAclId = &items[0]
	request.SubnetIds = helper.Strings(items[1:])

	err = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, err = me.client.UseVpcClient().DisassociateNetworkAclSubnets(request)
		if err != nil {
			return retryError(err, InternalError)
		}
		return nil
	})

	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
			logId, request.GetAction(), request.ToJsonString(), err)
		errRet = err
		return
	}
	return
}

func (me *VpcService) DescribeVpngwById(ctx context.Context, vpngwId string) (has bool, gateway *vpc.VpnGateway, err error) {
	var (
		logId    = getLogId(ctx)
		request  = vpc.NewDescribeVpnGatewaysRequest()
		response *vpc.DescribeVpnGatewaysResponse
	)
	request.VpnGatewayIds = []*string{&vpngwId}
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		response, err = me.client.UseVpcClient().DescribeVpnGateways(request)
		if err != nil {
			ee, ok := err.(*sdkErrors.TencentCloudSDKError)
			if !ok {
				return retryError(err)
			}
			if ee.Code == VPCNotFound {
				return nil
			} else {
				return retryError(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]", logId, request.GetAction(), request.ToJsonString(), err)
		return
	}
	if response == nil || response.Response == nil || len(response.Response.VpnGatewaySet) < 1 {
		has = false
		return
	}

	gateway = response.Response.VpnGatewaySet[0]
	has = true
	return
}

func (me *VpcService) DescribeVpnGwByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.VpnGateway, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeVpnGatewaysRequest()
	)
	request.Filters = make([]*vpc.FilterObject, 0, len(filters))
	for k, v := range filters {
		filter := vpc.FilterObject{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.VpnGateway, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeVpnGateways(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.VpnGatewaySet) < 1 {
			break
		}
		instances = append(instances, response.Response.VpnGatewaySet...)
		if len(response.Response.VpnGatewaySet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DeleteVpnGateway(ctx context.Context, vpnGatewayId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteVpnGatewayRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.VpnGatewayId = &vpnGatewayId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteVpnGateway(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func (me *VpcService) DescribeCustomerGatewayByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.CustomerGateway, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeCustomerGatewaysRequest()
	)
	request.Filters = make([]*vpc.Filter, 0, len(filters))
	for k, v := range filters {
		filter := vpc.Filter{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.CustomerGateway, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeCustomerGateways(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.CustomerGatewaySet) < 1 {
			break
		}
		instances = append(instances, response.Response.CustomerGatewaySet...)
		if len(response.Response.CustomerGatewaySet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DeleteCustomerGateway(ctx context.Context, customerGatewayId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteCustomerGatewayRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.CustomerGatewayId = &customerGatewayId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteCustomerGateway(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func (me *VpcService) CreateAddressTemplate(ctx context.Context, name string, addresses []interface{}) (templateId string, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateAddressTemplateRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.AddressTemplateName = &name
	request.Addresses = make([]*string, len(addresses))
	for i, v := range addresses {
		request.Addresses[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateAddressTemplate(request)
	if err != nil {
		errRet = err
		return
	}

	if response == nil || response.Response == nil || response.Response.AddressTemplate == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	}

	templateId = *response.Response.AddressTemplate.AddressTemplateId
	return
}

func (me *VpcService) DescribeAddressTemplateById(ctx context.Context, templateId string) (template *vpc.AddressTemplate, has bool, errRet error) {
	filter := vpc.Filter{Name: helper.String("address-template-id"), Values: []*string{&templateId}}
	templates, err := me.DescribeAddressTemplates(ctx, []*vpc.Filter{&filter})
	if err != nil {
		errRet = err
		return
	}

	if len(templates) == 0 {
		return
	}
	if len(templates) > 1 {
		errRet = fmt.Errorf("TencentCloud SDK return more than one templates, instanceId %s", templateId)
	}

	has = true
	template = templates[0]
	return
}

func (me *VpcService) DescribeAddressTemplates(ctx context.Context, filter []*vpc.Filter) (templateList []*vpc.AddressTemplate, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeAddressTemplatesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()

	var offset, limit = 0, 100
	request.Filters = filter

	for {
		request.Offset = helper.String(strconv.Itoa(offset))
		request.Limit = helper.String(strconv.Itoa(limit))

		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeAddressTemplates(request)
		if err != nil {
			errRet = err
			return
		}
		if response == nil || response.Response == nil {
			errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
		}
		templateList = append(templateList, response.Response.AddressTemplateSet...)
		if len(response.Response.AddressTemplateSet) < limit {
			return
		}
		offset += limit
	}
}

func (me *VpcService) ModifyAddressTemplate(ctx context.Context, templateId string, name string, addresses []interface{}) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyAddressTemplateAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.AddressTemplateId = &templateId
	request.AddressTemplateName = &name
	request.Addresses = make([]*string, len(addresses))
	for i, v := range addresses {
		request.Addresses[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().ModifyAddressTemplateAttribute(request)
	return err
}

func (me *VpcService) DeleteAddressTemplate(ctx context.Context, templateId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteAddressTemplateRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.AddressTemplateId = &templateId

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().DeleteAddressTemplate(request)
	return err
}

func (me *VpcService) CreateAddressTemplateGroup(ctx context.Context, name string, addressTemplate []interface{}) (templateId string, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateAddressTemplateGroupRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.AddressTemplateGroupName = &name
	request.AddressTemplateIds = make([]*string, len(addressTemplate))
	for i, v := range addressTemplate {
		request.AddressTemplateIds[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateAddressTemplateGroup(request)
	if err != nil {
		errRet = err
		return
	}

	if response == nil || response.Response == nil || response.Response.AddressTemplateGroup == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	}

	templateId = *response.Response.AddressTemplateGroup.AddressTemplateGroupId
	return
}

func (me *VpcService) ModifyAddressTemplateGroup(ctx context.Context, templateGroupId string, name string, templateIds []interface{}) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyAddressTemplateGroupAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.AddressTemplateGroupId = &templateGroupId
	request.AddressTemplateGroupName = &name
	request.AddressTemplateIds = make([]*string, len(templateIds))
	for i, v := range templateIds {
		request.AddressTemplateIds[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().ModifyAddressTemplateGroupAttribute(request)
	return err
}

func (me *VpcService) DescribeAddressTemplateGroupById(ctx context.Context, templateGroupId string) (templateGroup *vpc.AddressTemplateGroup, has bool, errRet error) {
	filter := vpc.Filter{Name: helper.String("address-template-group-id"), Values: []*string{&templateGroupId}}
	templateGroups, err := me.DescribeAddressTemplateGroups(ctx, []*vpc.Filter{&filter})
	if err != nil {
		errRet = err
		return
	}

	if len(templateGroups) == 0 {
		return
	}
	if len(templateGroups) > 1 {
		errRet = fmt.Errorf("TencentCloud SDK return more than one template group, instanceId %s", templateGroupId)
	}

	has = true
	templateGroup = templateGroups[0]
	return
}

func (me *VpcService) DescribeAddressTemplateGroups(ctx context.Context, filter []*vpc.Filter) (templateList []*vpc.AddressTemplateGroup, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeAddressTemplateGroupsRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()

	var offset, limit = 0, 100
	request.Filters = filter

	for {
		request.Offset = helper.String(strconv.Itoa(offset))
		request.Limit = helper.String(strconv.Itoa(limit))

		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeAddressTemplateGroups(request)
		if err != nil {
			errRet = err
			return
		}
		if response == nil || response.Response == nil {
			errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
		}
		templateList = append(templateList, response.Response.AddressTemplateGroupSet...)
		if len(response.Response.AddressTemplateGroupSet) < limit {
			return
		}
		offset += limit
	}
}

func (me *VpcService) DeleteAddressTemplateGroup(ctx context.Context, templateGroupId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteAddressTemplateGroupRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.AddressTemplateGroupId = &templateGroupId

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().DeleteAddressTemplateGroup(request)
	return err
}

func (me *VpcService) CreateServiceTemplate(ctx context.Context, name string, services []interface{}) (templateId string, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateServiceTemplateRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.ServiceTemplateName = &name
	request.Services = make([]*string, len(services))
	for i, v := range services {
		request.Services[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateServiceTemplate(request)
	if err != nil {
		errRet = err
		return
	}

	if response == nil || response.Response == nil || response.Response.ServiceTemplate == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	}

	templateId = *response.Response.ServiceTemplate.ServiceTemplateId
	return
}

func (me *VpcService) ModifyServiceTemplate(ctx context.Context, templateId string, name string, services []interface{}) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyServiceTemplateAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.ServiceTemplateId = &templateId
	request.ServiceTemplateName = &name
	request.Services = make([]*string, len(services))
	for i, v := range services {
		request.Services[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().ModifyServiceTemplateAttribute(request)
	return err
}

func (me *VpcService) DescribeServiceTemplateById(ctx context.Context, templateId string) (template *vpc.ServiceTemplate, has bool, errRet error) {
	filter := vpc.Filter{Name: helper.String("service-template-id"), Values: []*string{&templateId}}
	templates, err := me.DescribeServiceTemplates(ctx, []*vpc.Filter{&filter})
	if err != nil {
		errRet = err
		return
	}

	if len(templates) == 0 {
		return
	}
	if len(templates) > 1 {
		errRet = fmt.Errorf("TencentCloud SDK return more than one templates, instanceId %s", templateId)
	}

	has = true
	template = templates[0]
	return
}

func (me *VpcService) DescribeServiceTemplates(ctx context.Context, filter []*vpc.Filter) (templateList []*vpc.ServiceTemplate, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeServiceTemplatesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()

	var offset, limit = 0, 100
	request.Filters = filter

	for {
		request.Offset = helper.String(strconv.Itoa(offset))
		request.Limit = helper.String(strconv.Itoa(limit))

		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeServiceTemplates(request)
		if err != nil {
			errRet = err
			return
		}
		if response == nil || response.Response == nil {
			errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
		}
		templateList = append(templateList, response.Response.ServiceTemplateSet...)
		if len(response.Response.ServiceTemplateSet) < limit {
			return
		}
		offset += limit
	}
}

func (me *VpcService) DeleteServiceTemplate(ctx context.Context, templateId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteServiceTemplateRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.ServiceTemplateId = &templateId

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().DeleteServiceTemplate(request)
	return err
}

func (me *VpcService) CreateServiceTemplateGroup(ctx context.Context, name string, serviceTemplate []interface{}) (templateId string, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateServiceTemplateGroupRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.ServiceTemplateGroupName = &name
	request.ServiceTemplateIds = make([]*string, len(serviceTemplate))
	for i, v := range serviceTemplate {
		request.ServiceTemplateIds[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateServiceTemplateGroup(request)
	if err != nil {
		errRet = err
		return
	}

	if response == nil || response.Response == nil || response.Response.ServiceTemplateGroup == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	}

	templateId = *response.Response.ServiceTemplateGroup.ServiceTemplateGroupId
	return
}

func (me *VpcService) DescribeServiceTemplateGroupById(ctx context.Context, templateGroupId string) (template *vpc.ServiceTemplateGroup, has bool, errRet error) {
	filter := vpc.Filter{Name: helper.String("service-template-group-id"), Values: []*string{&templateGroupId}}
	templates, err := me.DescribeServiceTemplateGroups(ctx, []*vpc.Filter{&filter})
	if err != nil {
		errRet = err
		return
	}

	if len(templates) == 0 {
		return
	}
	if len(templates) > 1 {
		errRet = fmt.Errorf("TencentCloud SDK return more than one templates, instanceId %s", templateGroupId)
	}

	has = true
	template = templates[0]
	return
}

func (me *VpcService) DescribeServiceTemplateGroups(ctx context.Context, filter []*vpc.Filter) (templateList []*vpc.ServiceTemplateGroup, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeServiceTemplateGroupsRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()

	var offset, limit = 0, 100
	request.Filters = filter

	for {
		request.Offset = helper.String(strconv.Itoa(offset))
		request.Limit = helper.String(strconv.Itoa(limit))

		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeServiceTemplateGroups(request)
		if err != nil {
			errRet = err
			return
		}
		if response == nil || response.Response == nil {
			errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
		}
		templateList = append(templateList, response.Response.ServiceTemplateGroupSet...)
		if len(response.Response.ServiceTemplateGroupSet) < limit {
			return
		}
		offset += limit
	}
}

func (me *VpcService) ModifyServiceTemplateGroup(ctx context.Context, serviceGroupId string, name string, templateIds []interface{}) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyServiceTemplateGroupAttributeRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]",
				logId, request.GetAction(), request.ToJsonString(), errRet)
		}
	}()

	request.ServiceTemplateGroupId = &serviceGroupId
	request.ServiceTemplateGroupName = &name
	request.ServiceTemplateIds = make([]*string, len(templateIds))
	for i, v := range templateIds {
		request.ServiceTemplateIds[i] = helper.String(v.(string))
	}

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().ModifyServiceTemplateGroupAttribute(request)
	return err
}

func (me *VpcService) DeleteServiceTemplateGroup(ctx context.Context, templateGroupId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteServiceTemplateGroupRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.ServiceTemplateGroupId = &templateGroupId

	ratelimit.Check(request.GetAction())
	_, err := me.client.UseVpcClient().DeleteServiceTemplateGroup(request)
	return err
}

func (me *VpcService) CreateVpnGatewayRoute(ctx context.Context, vpnGatewayId string, vpnGwRoutes []*vpc.VpnGatewayRoute) (errRet error, routes []*vpc.VpnGatewayRoute) {
	logId := getLogId(ctx)
	request := vpc.NewCreateVpnGatewayRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.VpnGatewayId = &vpnGatewayId
	request.Routes = vpnGwRoutes

	var response *vpc.CreateVpnGatewayRoutesResponse
	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet = me.client.UseVpcClient().CreateVpnGatewayRoutes(request)
		if errRet != nil {
			log.Printf("[CRITAL]%s create vpn gateway route failed, reason: %v", logId, errRet)
			return retryError(errRet, InternalError)
		}
		return nil
	})
	if errRet != nil {
		return errRet, nil
	}

	if response == nil || response.Response == nil || response.Response.Routes == nil || len(response.Response.Routes) == 0 {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %+v, %s", response, request.GetAction())
	} else {
		routes = response.Response.Routes
	}
	return
}

func (me *VpcService) ModifyVpnGatewayRoute(ctx context.Context, vpnGatewayId, routeId, status string) (errRet error, routes *vpc.VpnGatewayRoute) {
	logId := getLogId(ctx)
	request := vpc.NewModifyVpnGatewayRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.VpnGatewayId = &vpnGatewayId
	request.Routes = []*vpc.VpnGatewayRouteModify{{
		RouteId: &routeId,
		Status:  &status,
	}}

	var response *vpc.ModifyVpnGatewayRoutesResponse
	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet = me.client.UseVpcClient().ModifyVpnGatewayRoutes(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	if errRet != nil {
		return errRet, nil
	}

	if response == nil || response.Response == nil || response.Response.Routes == nil || len(response.Response.Routes) == 0 {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	} else {
		routes = response.Response.Routes[0]
	}
	return
}

func (me *VpcService) DeleteVpnGatewayRoutes(ctx context.Context, vpnGatewayId string, routeIds []*string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteVpnGatewayRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.VpnGatewayId = &vpnGatewayId
	request.RouteIds = routeIds

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteVpnGatewayRoutes(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func (me *VpcService) DescribeVpnGatewayRoutes(ctx context.Context, vpnGatewayId string, filters []*vpc.Filter) (errRet error, result []*vpc.VpnGatewayRoute) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeVpnGatewayRoutesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.VpnGatewayId = &vpnGatewayId
	if filters != nil && len(filters) > 0 {
		request.Filters = filters
	}

	offset := int64(0)
	limit := int64(VPN_DESCRIBE_LIMIT)
	for {
		request.Offset = &offset
		request.Limit = &limit
		var response *vpc.DescribeVpnGatewayRoutesResponse
		errRet = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())
			response, errRet = me.client.UseVpcClient().DescribeVpnGatewayRoutes(request)
			if errRet != nil {
				return retryError(errRet, InternalError)
			}
			return nil
		})
		if errRet != nil {
			return errRet, nil
		}

		if response == nil || response.Response == nil {
			return fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction()), nil
		} else if len(response.Response.Routes) > 0 {
			result = append(result, response.Response.Routes...)
		} else {
			return
		}
		offset = offset + limit
	}
}

func (me *VpcService) DescribeVpcTaskResult(ctx context.Context, taskId *string) (err error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeVpcTaskResultRequest()
	defer func() {
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), err.Error())
		}
	}()
	request.TaskId = taskId
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeVpcTaskResult(request)
		if err != nil {
			return retryError(err)
		}
		if response.Response.Status != nil && *response.Response.Status == VPN_TASK_STATUS_RUNNING {
			return resource.RetryableError(errors.New("VPN task is running"))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return
}

func (me *VpcService) DescribeTaskResult(ctx context.Context, taskId *uint64) (err error) {

	logId := getLogId(ctx)
	request := vpc.NewDescribeTaskResultRequest()
	defer func() {
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), err.Error())
		}
	}()
	request.TaskId = taskId
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeTaskResult(request)
		if err != nil {
			return retryError(err)
		}
		if response.Response.Result != nil && *response.Response.Result == VPN_TASK_STATUS_RUNNING {
			return resource.RetryableError(errors.New("VPN task is running"))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return
}

func (me *VpcService) DescribeVpnSslServerById(ctx context.Context, sslId string) (has bool, gateway *vpc.SslVpnSever, err error) {
	var (
		logId    = getLogId(ctx)
		request  = vpc.NewDescribeVpnGatewaySslServersRequest()
		response *vpc.DescribeVpnGatewaySslServersResponse
	)
	request.SslVpnServerIds = []*string{&sslId}
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		response, err = me.client.UseVpcClient().DescribeVpnGatewaySslServers(request)
		if err != nil {
			ee, ok := err.(*sdkErrors.TencentCloudSDKError)
			if !ok {
				return retryError(err)
			}
			if ee.Code == VPCNotFound {
				return nil
			} else {
				return retryError(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]", logId, request.GetAction(), request.ToJsonString(), err)
		return
	}
	if response == nil || response.Response == nil || len(response.Response.SslVpnSeverSet) < 1 {
		has = false
		return
	}

	gateway = response.Response.SslVpnSeverSet[0]
	has = true
	return
}

func (me *VpcService) DescribeVpnGwSslServerByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.SslVpnSever, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeVpnGatewaySslServersRequest()
	)
	request.Filters = make([]*vpc.FilterObject, 0, len(filters))
	for k, v := range filters {
		filter := vpc.FilterObject{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.SslVpnSever, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeVpnGatewaySslServers(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.SslVpnSeverSet) < 1 {
			break
		}
		instances = append(instances, response.Response.SslVpnSeverSet...)
		if len(response.Response.SslVpnSeverSet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DeleteVpnGatewaySslServer(ctx context.Context, SslServerId string) (taskId uint64, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteVpnGatewaySslServerRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.SslVpnServerId = &SslServerId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet := me.client.UseVpcClient().DeleteVpnGatewaySslServer(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		taskId = *response.Response.TaskId
		return nil
	})
	return
}

func (me *VpcService) DescribeVpnSslClientById(ctx context.Context, sslId string) (has bool, gateway *vpc.SslVpnClient, err error) {
	var (
		logId    = getLogId(ctx)
		request  = vpc.NewDescribeVpnGatewaySslClientsRequest()
		response *vpc.DescribeVpnGatewaySslClientsResponse
	)
	request.SslVpnClientIds = []*string{&sslId}
	err = resource.Retry(readRetryTimeout, func() *resource.RetryError {
		response, err = me.client.UseVpcClient().DescribeVpnGatewaySslClients(request)
		if err != nil {
			ee, ok := err.(*sdkErrors.TencentCloudSDKError)
			if !ok {
				return retryError(err)
			}
			if ee.Code == VPCNotFound {
				return nil
			} else {
				return retryError(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%v]", logId, request.GetAction(), request.ToJsonString(), err)
		return
	}
	if response == nil || response.Response == nil || len(response.Response.SslVpnClientSet) < 1 {
		has = false
		return
	}

	gateway = response.Response.SslVpnClientSet[0]
	has = true
	return
}

func (me *VpcService) DescribeVpnGwSslClientByFilter(ctx context.Context, filters map[string]string) (instances []*vpc.SslVpnClient, errRet error) {
	var (
		logId   = getLogId(ctx)
		request = vpc.NewDescribeVpnGatewaySslClientsRequest()
	)
	request.Filters = make([]*vpc.Filter, 0, len(filters))
	for k, v := range filters {
		filter := vpc.Filter{
			Name:   helper.String(k),
			Values: []*string{helper.String(v)},
		}
		request.Filters = append(request.Filters, &filter)
	}

	var offset uint64 = 0
	var pageSize uint64 = 100
	instances = make([]*vpc.SslVpnClient, 0)

	for {
		request.Offset = &offset
		request.Limit = &pageSize
		ratelimit.Check(request.GetAction())
		response, err := me.client.UseVpcClient().DescribeVpnGatewaySslClients(request)
		if err != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), err.Error())
			errRet = err
			return
		}
		log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
			logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

		if response == nil || len(response.Response.SslVpnClientSet) < 1 {
			break
		}
		instances = append(instances, response.Response.SslVpnClientSet...)
		if len(response.Response.SslVpnClientSet) < int(pageSize) {
			break
		}
		offset += pageSize
	}
	return
}

func (me *VpcService) DeleteVpnGatewaySslClient(ctx context.Context, SslClientId string) (taskId *uint64, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteVpnGatewaySslClientRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.SslVpnClientId = &SslClientId

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet := me.client.UseVpcClient().DeleteVpnGatewaySslClient(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		taskId = response.Response.TaskId
		return nil
	})
	return
}

func (me *VpcService) CreateNatGatewaySnat(ctx context.Context, natGatewayId string, snat *vpc.SourceIpTranslationNatRule) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewCreateNatGatewaySourceIpTranslationNatRuleRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.NatGatewayId = &natGatewayId
	request.SourceIpTranslationNatRules = []*vpc.SourceIpTranslationNatRule{snat}

	var response *vpc.CreateNatGatewaySourceIpTranslationNatRuleResponse
	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet = me.client.UseVpcClient().CreateNatGatewaySourceIpTranslationNatRule(request)
		if errRet != nil {
			log.Printf("[CRITAL]%s create nat gateway source ip translation nat rule failed, reason: %v", logId, errRet)
			return retryError(errRet, InternalError)
		}
		return nil
	})
	if errRet != nil {
		return errRet
	}

	if response == nil || response.Response == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %+v, %s", response, request.GetAction())
	}
	return
}

func (me *VpcService) ModifyNatGatewaySnat(ctx context.Context, natGatewayId string, snat *vpc.SourceIpTranslationNatRule) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewModifyNatGatewaySourceIpTranslationNatRuleRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.NatGatewayId = &natGatewayId
	request.SourceIpTranslationNatRule = snat

	var response *vpc.ModifyNatGatewaySourceIpTranslationNatRuleResponse
	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		response, errRet = me.client.UseVpcClient().ModifyNatGatewaySourceIpTranslationNatRule(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	if errRet != nil {
		return errRet
	}

	if response == nil || response.Response == nil {
		errRet = fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction())
	}
	return
}

func (me *VpcService) DeleteNatGatewaySnat(ctx context.Context, natGatewayId string, snatId string) (errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDeleteNatGatewaySourceIpTranslationNatRuleRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.NatGatewayId = &natGatewayId
	request.NatGatewaySnatIds = []*string{&snatId}

	errRet = resource.Retry(writeRetryTimeout, func() *resource.RetryError {
		ratelimit.Check(request.GetAction())
		_, errRet = me.client.UseVpcClient().DeleteNatGatewaySourceIpTranslationNatRule(request)
		if errRet != nil {
			return retryError(errRet, InternalError)
		}
		return nil
	})
	return
}

func (me *VpcService) DescribeNatGatewaySnats(ctx context.Context, natGatewayId string, filters []*vpc.Filter) (errRet error, result []*vpc.SourceIpTranslationNatRule) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeNatGatewaySourceIpTranslationNatRulesRequest()
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail,reason[%s]", logId, request.GetAction(), errRet.Error())
		}
	}()
	request.NatGatewayId = &natGatewayId
	if filters != nil && len(filters) > 0 {
		request.Filters = filters
	}

	offset := int64(0)
	limit := int64(VPN_DESCRIBE_LIMIT)
	for {
		request.Offset = &offset
		request.Limit = &limit
		var response *vpc.DescribeNatGatewaySourceIpTranslationNatRulesResponse
		errRet = resource.Retry(readRetryTimeout, func() *resource.RetryError {
			ratelimit.Check(request.GetAction())
			response, errRet = me.client.UseVpcClient().DescribeNatGatewaySourceIpTranslationNatRules(request)
			if errRet != nil {
				return retryError(errRet, InternalError)
			}
			return nil
		})
		if errRet != nil {
			return errRet, nil
		}

		if response == nil || response.Response == nil {
			return fmt.Errorf("TencentCloud SDK return nil response, %s", request.GetAction()), nil
		} else if len(response.Response.SourceIpTranslationNatRuleSet) > 0 {
			result = append(result, response.Response.SourceIpTranslationNatRuleSet...)
		} else {
			return
		}
		offset = offset + limit
	}
}

func (me *VpcService) DescribeAssistantCidr(ctx context.Context, vpcId string) (info []*vpc.AssistantCidr, errRet error) {
	logId := getLogId(ctx)
	request := vpc.NewDescribeAssistantCidrRequest()

	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	request.VpcIds = []*string{&vpcId}

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DescribeAssistantCidr(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	info = response.Response.AssistantCidrSet

	return
}

// CheckAssistantCidr used for check if cidr conflict
func (me *VpcService) CheckAssistantCidr(ctx context.Context, request *vpc.CheckAssistantCidrRequest) (info []*vpc.ConflictSource, errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CheckAssistantCidr(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	info = response.Response.ConflictSourceSet

	return
}

func (me *VpcService) CreateAssistantCidr(ctx context.Context, request *vpc.CreateAssistantCidrRequest) (info []*vpc.AssistantCidr, errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().CreateAssistantCidr(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	info = response.Response.AssistantCidrSet

	return
}

func (me *VpcService) ModifyAssistantCidr(ctx context.Context, request *vpc.ModifyAssistantCidrRequest) (errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().ModifyAssistantCidr(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}

func (me *VpcService) DeleteAssistantCidr(ctx context.Context, request *vpc.DeleteAssistantCidrRequest) (errRet error) {
	logId := getLogId(ctx)
	defer func() {
		if errRet != nil {
			log.Printf("[CRITAL]%s api[%s] fail, request body [%s], reason[%s]\n",
				logId, request.GetAction(), request.ToJsonString(), errRet.Error())
		}
	}()

	ratelimit.Check(request.GetAction())
	response, err := me.client.UseVpcClient().DeleteAssistantCidr(request)

	if err != nil {
		errRet = err
		return
	}

	log.Printf("[DEBUG]%s api[%s] success, request body [%s], response body [%s]\n",
		logId, request.GetAction(), request.ToJsonString(), response.ToJsonString())

	return
}
