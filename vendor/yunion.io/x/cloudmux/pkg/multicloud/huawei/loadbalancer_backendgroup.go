// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package huawei

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/cloudmux/pkg/apis/compute"
	"yunion.io/x/cloudmux/pkg/cloudprovider"
	"yunion.io/x/cloudmux/pkg/multicloud"
)

type SElbBackendGroup struct {
	multicloud.SResourceBase
	HuaweiTags
	lb     *SLoadbalancer
	region *SRegion

	LBAlgorithm        string        `json:"lb_algorithm"`
	Protocol           string        `json:"protocol"`
	Description        string        `json:"description"`
	AdminStateUp       bool          `json:"admin_state_up"`
	Loadbalancers      []Listener    `json:"loadbalancers"`
	TenantID           string        `json:"tenant_id"`
	ProjectID          string        `json:"project_id"`
	Listeners          []Listener    `json:"listeners"`
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	HealthMonitorID    string        `json:"healthmonitor_id"`
	SessionPersistence StickySession `json:"session_persistence"`
}

func (self *SElbBackendGroup) GetLoadbalancerId() string {
	return self.lb.GetId()
}

func (self *SElbBackendGroup) GetILoadbalancer() cloudprovider.ICloudLoadbalancer {
	return self.lb
}

type StickySession struct {
	Type               string `json:"type"`
	CookieName         string `json:"cookie_name"`
	PersistenceTimeout int    `json:"persistence_timeout"`
}

func (self *SElbBackendGroup) GetProtocolType() string {
	switch self.Protocol {
	case "TCP":
		return api.LB_LISTENER_TYPE_TCP
	case "UDP":
		return api.LB_LISTENER_TYPE_UDP
	case "HTTP":
		return api.LB_LISTENER_TYPE_HTTP
	default:
		return ""
	}
}

func (self *SElbBackendGroup) GetScheduler() string {
	switch self.LBAlgorithm {
	case "ROUND_ROBIN":
		return api.LB_SCHEDULER_WRR
	case "LEAST_CONNECTIONS":
		return api.LB_SCHEDULER_WLC
	case "SOURCE_IP":
		return api.LB_SCHEDULER_SCH
	default:
		return ""
	}
}

func ToHuaweiHealthCheckHttpCode(c string) string {
	c = strings.TrimSpace(c)
	segs := strings.Split(c, ",")
	ret := []string{}
	for _, seg := range segs {
		seg = strings.TrimLeft(seg, "http_")
		seg = strings.TrimSpace(seg)
		seg = strings.Replace(seg, "xx", "00", -1)
		ret = append(ret, seg)
	}

	return strings.Join(ret, ",")
}

func ToOnecloudHealthCheckHttpCode(c string) string {
	c = strings.TrimSpace(c)
	segs := strings.Split(c, ",")
	ret := []string{}
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		seg = strings.Replace(seg, "00", "xx", -1)
		seg = "http_" + seg
		ret = append(ret, seg)
	}

	return strings.Join(ret, ",")
}

func (self *SElbBackendGroup) GetHealthCheck() (*cloudprovider.SLoadbalancerHealthCheck, error) {
	if len(self.HealthMonitorID) == 0 {
		return nil, nil
	}

	health, err := self.region.GetLoadBalancerHealthCheck(self.HealthMonitorID)
	if err != nil {
		return nil, err
	}

	var healthCheckType string
	switch health.Type {
	case "TCP":
		healthCheckType = api.LB_HEALTH_CHECK_TCP
	case "UDP_CONNECT":
		healthCheckType = api.LB_HEALTH_CHECK_UDP
	case "HTTP":
		healthCheckType = api.LB_HEALTH_CHECK_HTTP
	default:
		healthCheckType = ""
	}

	ret := cloudprovider.SLoadbalancerHealthCheck{
		HealthCheckType:     healthCheckType,
		HealthCheckTimeout:  health.Timeout,
		HealthCheckDomain:   health.DomainName,
		HealthCheckURI:      health.URLPath,
		HealthCheckInterval: health.Delay,
		HealthCheckRise:     health.MaxRetries,
		HealthCheckHttpCode: ToOnecloudHealthCheckHttpCode(health.ExpectedCodes),
	}

	return &ret, nil
}

func (self *SElbBackendGroup) GetStickySession() (*cloudprovider.SLoadbalancerStickySession, error) {
	if len(self.SessionPersistence.Type) == 0 {
		return nil, nil
	}

	var stickySessionType string
	switch self.SessionPersistence.Type {
	case "SOURCE_IP":
		stickySessionType = api.LB_STICKY_SESSION_TYPE_INSERT
	case "HTTP_COOKIE":
		stickySessionType = api.LB_STICKY_SESSION_TYPE_INSERT
	case "APP_COOKIE":
		stickySessionType = api.LB_STICKY_SESSION_TYPE_SERVER
	}

	ret := cloudprovider.SLoadbalancerStickySession{
		StickySession:              api.LB_BOOL_ON,
		StickySessionCookie:        self.SessionPersistence.CookieName,
		StickySessionType:          stickySessionType,
		StickySessionCookieTimeout: self.SessionPersistence.PersistenceTimeout * 60,
	}

	return &ret, nil
}

func (self *SElbBackendGroup) GetId() string {
	return self.ID
}

func (self *SElbBackendGroup) GetName() string {
	return self.Name
}

func (self *SElbBackendGroup) GetGlobalId() string {
	return self.GetId()
}

func (self *SElbBackendGroup) GetStatus() string {
	return api.LB_STATUS_ENABLED
}

func (self *SElbBackendGroup) Refresh() error {
	ret, err := self.lb.region.GetLoadBalancerBackendGroupId(self.GetId())
	if err != nil {
		return err
	}
	ret.lb = self.lb

	err = jsonutils.Update(self, ret)
	if err != nil {
		return err
	}

	return nil
}

func (self *SElbBackendGroup) IsEmulated() bool {
	return false
}

func (self *SElbBackendGroup) GetProjectId() string {
	return self.ProjectID
}

func (self *SElbBackendGroup) IsDefault() bool {
	return false
}

func (self *SElbBackendGroup) GetType() string {
	return api.LB_BACKENDGROUP_TYPE_NORMAL
}

func (self *SElbBackendGroup) GetILoadbalancerBackends() ([]cloudprovider.ICloudLoadbalancerBackend, error) {
	ret, err := self.region.GetLoadBalancerBackends(self.GetId())
	if err != nil {
		return nil, err
	}

	iret := []cloudprovider.ICloudLoadbalancerBackend{}
	for i := range ret {
		backend := ret[i]
		backend.lb = self.lb
		backend.backendGroup = self

		iret = append(iret, &backend)
	}

	return iret, nil
}

func (self *SElbBackendGroup) GetILoadbalancerBackendById(serverId string) (cloudprovider.ICloudLoadbalancerBackend, error) {
	backend, err := self.region.GetElbBackend(self.GetId(), serverId)
	if err != nil {
		return nil, err
	}
	backend.lb = self.lb
	backend.backendGroup = self
	return backend, nil
}

func (self *SElbBackendGroup) AddBackendServer(serverId string, weight int, port int) (cloudprovider.ICloudLoadbalancerBackend, error) {
	instance, err := self.lb.region.GetInstanceByID(serverId)
	if err != nil {
		return nil, err
	}

	nics, err := instance.GetINics()
	if err != nil {
		return nil, err
	} else if len(nics) == 0 {
		return nil, fmt.Errorf("AddBackendServer %s no network interface found", serverId)
	}

	subnets, err := self.lb.region.getSubnetIdsByInstanceId(instance.GetId())
	if err != nil {
		return nil, err
	} else if len(subnets) == 0 {
		return nil, fmt.Errorf("AddBackendServer %s no subnet found", serverId)
	}

	net, err := self.lb.region.getNetwork(subnets[0])
	if err != nil {
		return nil, err
	}

	backend, err := self.region.AddLoadBalancerBackend(self.GetId(), net.NeutronSubnetID, nics[0].GetIP(), port, weight)
	if err != nil {
		return nil, err
	}

	backend.lb = self.lb
	backend.backendGroup = self
	return backend, nil
}

func (self *SElbBackendGroup) RemoveBackendServer(backendId string, weight int, port int) error {
	ibackend, err := self.GetILoadbalancerBackendById(backendId)
	if err != nil {
		if errors.Cause(err) == cloudprovider.ErrNotFound {
			return nil
		}

		return errors.Wrap(err, "ElbBackendGroup.GetILoadbalancerBackendById")
	}

	err = self.region.RemoveLoadBalancerBackend(self.GetId(), backendId)
	if err != nil {
		return errors.Wrap(err, "ElbBackendGroup.RemoveBackendServer")
	}

	return cloudprovider.WaitDeleted(ibackend, 2*time.Second, 30*time.Second)
}

func (self *SElbBackendGroup) Delete(ctx context.Context) error {
	if len(self.HealthMonitorID) > 0 {
		err := self.region.DeleteLoadbalancerHealthCheck(self.HealthMonitorID)
		if err != nil {
			return errors.Wrap(err, "ElbBackendGroup.Delete.DeleteLoadbalancerHealthCheck")
		}
	}

	// 删除后端服务器组的同时，删除掉无效的后端服务器数据
	{
		backends, err := self.region.getLoadBalancerAdminStateDownBackends(self.GetId())
		if err != nil {
			return errors.Wrap(err, "SElbBackendGroup.Delete.getLoadBalancerAdminStateDownBackends")
		}

		for i := range backends {
			backend := backends[i]
			err := self.RemoveBackendServer(backend.GetId(), backend.GetPort(), backend.GetWeight())
			if err != nil {
				return errors.Wrap(err, "SElbBackendGroup.Delete.RemoveBackendServer")
			}
		}
	}

	err := self.region.DeleteLoadBalancerBackendGroup(self.GetId())
	if err != nil {
		return errors.Wrap(err, "ElbBackendGroup.Delete.DeleteLoadBalancerBackendGroup")
	}

	return cloudprovider.WaitDeleted(self, 2*time.Second, 30*time.Second)
}

func (self *SElbBackendGroup) Sync(ctx context.Context, group *cloudprovider.SLoadbalancerBackendGroup) error {
	if group == nil {
		return nil
	}

	_, err := self.region.UpdateLoadBalancerBackendGroup(self.GetId(), group)
	return err
}

func (self *SRegion) GetLoadBalancerBackendGroupId(backendGroupId string) (*SElbBackendGroup, error) {
	ret := &SElbBackendGroup{region: self}
	res := fmt.Sprintf("elb/pools/" + backendGroupId)
	resp, err := self.lbGet(res)
	if err != nil {
		return nil, err
	}
	return ret, resp.Unmarshal(ret, "pool")
}

// https://support.huaweicloud.com/api-elb/zh-cn_topic_0096561550.html
func (self *SRegion) UpdateLoadBalancerBackendGroup(backendGroupID string, group *cloudprovider.SLoadbalancerBackendGroup) (*SElbBackendGroup, error) {
	params := map[string]interface{}{
		"name": group.Name,
	}
	var scheduler string
	if s, ok := LB_ALGORITHM_MAP[group.Scheduler]; !ok {
		return nil, fmt.Errorf("UpdateLoadBalancerBackendGroup unsupported scheduler %s", group.Scheduler)
	} else {
		scheduler = s
	}
	params["lb_algorithm"] = scheduler

	if group.StickySession == nil || group.StickySession.StickySession == api.LB_BOOL_OFF {
		params["session_persistence"] = jsonutils.JSONNull
	} else {
		s := map[string]interface{}{}
		timeout := int64(group.StickySession.StickySessionCookieTimeout / 60)
		if group.ListenType == api.LB_LISTENER_TYPE_UDP || group.ListenType == api.LB_LISTENER_TYPE_TCP {
			s["type"] = "SOURCE_IP"
			if timeout > 0 {
				s["persistence_timeout"] = timeout
			}
		} else {
			s["type"] = LB_STICKY_SESSION_MAP[group.StickySession.StickySessionType]
			if len(group.StickySession.StickySessionCookie) > 0 {
				s["cookie_name"] = group.StickySession.StickySessionCookie
			} else {
				if timeout > 0 {
					s["persistence_timeout"] = timeout
				}
			}
		}
		params["session_persistence"] = s
	}
	resp, err := self.lbUpdate("elb/pools/"+backendGroupID, map[string]interface{}{"pool": params})
	if err != nil {
		return nil, err
	}

	ret := &SElbBackendGroup{}
	err = resp.Unmarshal(ret, "pool")
	if err != nil {
		return nil, errors.Wrapf(err, "resp.Unmarshal")
	}

	if group.HealthCheck == nil && len(ret.HealthMonitorID) > 0 {
		err := self.DeleteLoadbalancerHealthCheck(ret.HealthMonitorID)
		if err != nil {
			return ret, errors.Wrap(err, "DeleteLoadbalancerHealthCheck")
		}
	}

	if group.HealthCheck != nil {
		if len(ret.HealthMonitorID) == 0 {
			_, err := self.CreateLoadBalancerHealthCheck(ret.GetId(), group.HealthCheck)
			if err != nil {
				return ret, errors.Wrap(err, "CreateLoadBalancerHealthCheck")
			}
		} else {
			_, err := self.UpdateLoadBalancerHealthCheck(ret.HealthMonitorID, group.HealthCheck)
			if err != nil {
				return ret, errors.Wrap(err, "UpdateLoadBalancerHealthCheck")
			}
		}
	}

	ret.region = self
	return ret, nil
}

// https://support.huaweicloud.com/api-elb/zh-cn_topic_0096561551.html
func (self *SRegion) DeleteLoadBalancerBackendGroup(id string) error {
	_, err := self.lbDelete("elb/pools/" + id)
	return err
}

// https://support.huaweicloud.com/api-elb/zh-cn_topic_0096561556.html
func (self *SRegion) AddLoadBalancerBackend(backendGroupId, subnetId, ipaddr string, port, weight int) (*SElbBackend, error) {
	params := map[string]interface{}{
		"address":       ipaddr,
		"protocol_port": port,
		"subnet_id":     subnetId,
		"weight":        weight,
	}
	ret := &SElbBackend{}
	resp, err := self.lbCreate(fmt.Sprintf("elb/pools/%s/members", backendGroupId), map[string]interface{}{"member": params})
	if err != nil {
		return nil, err
	}
	return ret, resp.Unmarshal(ret, "member")
}

func (self *SRegion) RemoveLoadBalancerBackend(lbbgId string, backendId string) error {
	_, err := self.lbDelete(fmt.Sprintf("elb/pools/%s/members/%s", lbbgId, backendId))
	return err
}

func (self *SRegion) getLoadBalancerBackends(backendGroupId string) ([]SElbBackend, error) {
	res := fmt.Sprintf("elb/pools/%s/members", backendGroupId)
	resp, err := self.lbList(res, url.Values{})
	if err != nil {
		return nil, err
	}
	ret := []SElbBackend{}
	return ret, resp.Unmarshal(&ret, "members")
}

func (self *SRegion) GetLoadBalancerBackends(backendGroupId string) ([]SElbBackend, error) {
	ret, err := self.getLoadBalancerBackends(backendGroupId)
	if err != nil {
		return nil, errors.Wrap(err, "SRegion.GetLoadBalancerBackends.getLoadBalancerBackends")
	}

	// 过滤掉服务器已经被删除的backend。原因是运管平台查询不到已删除的服务器记录，导致同步出错。产生肮数据。
	filtedRet := []SElbBackend{}
	for i := range ret {
		if ret[i].AdminStateUp {
			backend := ret[i]
			filtedRet = append(filtedRet, backend)
		}
	}

	return filtedRet, nil
}

func (self *SRegion) getLoadBalancerAdminStateDownBackends(backendGroupId string) ([]SElbBackend, error) {
	ret, err := self.getLoadBalancerBackends(backendGroupId)
	if err != nil {
		return nil, errors.Wrap(err, "SRegion.getLoadBalancerAdminStateDownBackends.getLoadBalancerBackends")
	}

	filtedRet := []SElbBackend{}
	for i := range ret {
		if !ret[i].AdminStateUp {
			backend := ret[i]
			filtedRet = append(filtedRet, backend)
		}
	}

	return filtedRet, nil
}

func (self *SRegion) GetLoadBalancerHealthCheck(healthCheckId string) (*SElbHealthCheck, error) {
	resp, err := self.lbGet("elb/healthmonitors/" + healthCheckId)
	if err != nil {
		return nil, err
	}
	ret := &SElbHealthCheck{region: self}
	return ret, resp.Unmarshal(ret, "healthmonitor")
}
