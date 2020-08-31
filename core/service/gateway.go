package service

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"gatekeeper/constant"
	"gatekeeper/core/resource"
	"gatekeeper/model/running"
	"gatekeeper/util"
)

type GateWayService struct {
	currentModule *running.GatewayModule
	w             http.ResponseWriter
	req           *http.Request
}

func NewGateWayService(w http.ResponseWriter, req *http.Request) *GateWayService {
	return &GateWayService{
		w:   w,
		req: req,
	}
}

func (s *GateWayService) CurrentModule() *running.GatewayModule {
	return s.currentModule
}

func (s *GateWayService) SetCurrentModule(currentModule *running.GatewayModule) {
	s.currentModule = currentModule
}

// 访问控制校验
func (s *GateWayService) AccessControl() error {
	if s.currentModule.AccessControl == nil {
		return nil
	}

	// 若控制开关未打开，则直接return
	if !s.authModuleOpened() {
		return nil
	}

	// 首先校验app_id是否存在
	appID := s.req.Header.Get("app_id")
	if appID == "" {
		return errors.New("app_id empty")
	}
	switch {
	case s.authInBlackIPList():
		return errors.New("msg:AuthInBlackIPList")
	case s.authInWhiteIPList():
		return nil
	case s.authInWhiteHostList():
		return nil
	}

	if err := s.authAppSign(appID); err != nil {
		return err
	}

	if err := s.authLimit(appID); err != nil {
		return err
	}
	return nil
}

func (s *GateWayService) LoadBalance() (*httputil.ReverseProxy, error) {
	ipList, err := SysConfMgr.GetModuleIPList(s.currentModule.Base.Name)
	if err != nil {
		return nil, errors.New("get_iplist_error")
	}
	if len(ipList) == 0 {
		return nil, errors.New("empty_iplist_error")
	}
	proxy, err := s.GetModuleHTTPProxy()
	if err != nil {
		return nil, err
	}
	return proxy, nil
}

func (s *GateWayService) GetModuleHTTPProxy() (*httputil.ReverseProxy, error) {
	proxy, err := SysConfMgr.GetModuleHTTPProxy(s.currentModule.Base.Name)
	if err != nil {
		return &httputil.ReverseProxy{}, err
	}
	return proxy, nil
}

func (s *GateWayService) MatchRule() error {
	moduleName := s.req.Header.Get("module")
	if moduleName == "" {
		return errors.New("module name empty")
	}
	module := SysConfMgr.GetModuleConfigByName(moduleName)
	if module == nil {
		return errors.New("module not found")
	}
	if module.MatchRule.URLRewrite != "" {
		for _, uw := range strings.Split(module.MatchRule.URLRewrite, ",") {
			uws := strings.Split(uw, " ")
			if len(uws) == 2 {
				re, err := regexp.Compile(uws[0])
				if err != nil {
					return err
				}
				rep := re.ReplaceAllString(s.req.URL.Path, uws[1])
				s.req.URL.Path = rep
				if s.req.URL.Path != s.req.URL.Path {
					break
				}
			}
		}
	}
	s.SetCurrentModule(module)
	return nil
}

func (s *GateWayService) authModuleOpened() bool {
	if s.currentModule.AccessControl.Open == 1 {
		return true
	}
	return false
}

func (s *GateWayService) authInBlackIPList() bool {
	clientIP := util.RemoteIP(s.req)
	blackList := strings.Split(s.currentModule.AccessControl.BlackList, ",")
	if util.AuthIPList(clientIP, blackList) {
		return true
	}
	return false
}

func (s *GateWayService) authInWhiteIPList() bool {
	clientIP := util.RemoteIP(s.req)
	whiteList := strings.Split(s.currentModule.AccessControl.WhiteList, ",")
	if util.AuthIPList(clientIP, whiteList) {
		return true
	}
	return false
}

func (s *GateWayService) authInWhiteHostList() bool {
	hostname, err := os.Hostname()
	if err != nil {
		return false
	}
	whiteHostname := strings.Split(s.currentModule.AccessControl.WhiteHostName, ",")
	if util.AuthIPList(hostname, whiteHostname) {
		return true
	}
	return false
}

func (s *GateWayService) authAppSign(appID string) error {
	clientSign := s.req.Header.Get("sign")
	if appID == "" {
		return errors.New(fmt.Sprintf("AuthAppSign -error:%v",
			"app_id empty"))
	}
	appConfig, err := SysConfMgr.GetAppConfigByAPPID(appID)
	if err != nil {
		return errors.New(fmt.Sprintf(
			"AuthAppSign -error:%v -app_id:%v -sign:%v",
			"GetAppConfigByAPPID error", appID, clientSign))
	}
	if appConfig.Secret == "" {
		return errors.New(fmt.Sprintf(
			"AuthAppSign -error:%v -app_id:%v -sign:%v",
			"Secret empty", appID, clientSign))
	}
	if appConfig.WhiteIps != "" &&
		util.AuthIPList(util.RemoteIP(s.req), strings.Split(appConfig.WhiteIps, ",")) {
		return nil
	}
	signKey := appConfig.Secret
	if signKey != clientSign {
		return errors.New(fmt.Sprintf(
			"AuthAppSign -error:%v -app_id:%v -sign:%v",
			"sign error", appID, clientSign))
	}
	return nil
}

func (s *GateWayService) authLimit(appID string) error {
	appConfig, err := SysConfMgr.GetAppConfigByAPPID(appID)
	if err != nil {
		return err
	}
	v := s.req.Context().Value("request_url")
	reqPath, ok := v.(string)
	if !ok {
		reqPath = ""
	}
	if !util.InOrPrefixStringList(reqPath, strings.Split(appConfig.OpenAPI, ",")) {
		errmsg := "You don't have rights for this path:" + reqPath + " - " + appConfig.OpenAPI
		return errors.New(errmsg)
	}

	//限速器
	limiter := resource.Limiters.GetLimiter(appID, appConfig.QPS)
	if appConfig.QPS > 0 && limiter.Allow() == false {
		errmsg := fmt.Sprintf("QPS limit : %d, %d", int64(limiter.Limit()), limiter.Burst())
		return errors.New(errmsg)
	}

	if appConfig.GroupID > 0 {
		s.req.Header.Add(constant.HeaderKeyUserGroup, strconv.Itoa(int(appConfig.GroupID)))
		s.req.Header.Add(constant.HeaderKeyUserGroupKey, constant.UserGroupPerfix+strconv.Itoa(int(appConfig.GroupID)))
	}

	counter := resource.FlowCounters.GetAPPCounter(appID)
	if appConfig.TotalQueryDaily > 0 && counter.TotalCount > appConfig.TotalQueryDaily {
		errmsg := fmt.Sprintf("total query daily limit: %d", appConfig.TotalQueryDaily)
		return errors.New(errmsg)
	}
	counter.Increase()
	return nil
}
