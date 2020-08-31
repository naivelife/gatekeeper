package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"gatekeeper/config"
	"gatekeeper/constant"
	"gatekeeper/core"
	"gatekeeper/model/entity"
	"gatekeeper/model/running"
	"gatekeeper/util"
)

// SysConfMgr 全局系统配置变量
var SysConfMgr *SysConfigManage

// SysConfigManage 系统配置管理器
type SysConfigManage struct {
	moduleConfig       *running.Modules
	moduleConfigLocker sync.RWMutex

	moduleIPListMap       map[string][]string //可用ip
	moduleIPListMapLocker sync.RWMutex

	moduleActiveIPListMap       map[string][]string //活动ip
	moduleActiveIPListMapLocker sync.RWMutex

	moduleForbidIPListMap       map[string][]string //禁用ip
	moduleForbidIPListMapLocker sync.RWMutex

	moduleProxyFuncMap       map[string]func(rr core.RR) *httputil.ReverseProxy
	moduleProxyFuncMapLocker sync.RWMutex

	moduleTransportMap       map[string]*http.Transport
	moduleTransportMapLocker sync.RWMutex

	moduleRRMap       map[string]core.RR
	moduleRRMapLocker sync.RWMutex

	appConfig       *running.Apps
	appConfigLocker sync.RWMutex

	loadConfigContext context.Context //重新载入配置时，需要执行close
	loadConfigCancel  func()          //停止配置自动检查
}

// 实例化全局系统配置
func NewSysConfigManage() *SysConfigManage {
	return &SysConfigManage{
		moduleIPListMap:       map[string][]string{},
		moduleActiveIPListMap: map[string][]string{},
		moduleForbidIPListMap: map[string][]string{},
		moduleProxyFuncMap:    map[string]func(rr core.RR) *httputil.ReverseProxy{},
		moduleTransportMap:    map[string]*http.Transport{},
		moduleRRMap:           map[string]core.RR{},
	}
}

// InitConfig 初始化配置
func (s *SysConfigManage) InitConfig() {
	s.loadConfigContext, s.loadConfigCancel = context.WithCancel(context.Background())
	if err := s.refreshAPPConfig(); err != nil {
		config.SysLog.Error("err:%s", err.Error())
	}
	if err := s.refreshModuleConfig(); err != nil {
		config.SysLog.Error("err:%s", err.Error())
	}
	s.checkIPList()
	s.configModuleRR()
	s.configModuleProxyMap()
}

// ReloadConfig 刷新配置
func (s *SysConfigManage) ReloadConfig() {
	// 刷新获取配置
	s.refreshAPPConfig()
	s.refreshModuleConfig()

	// 如果非首次加载，执行cancel
	if s.loadConfigCancel != nil {
		s.loadConfigCancel()
	}
	s.loadConfigContext, s.loadConfigCancel = context.WithCancel(context.Background())

	//检测及配置
	s.checkIPList()
	s.configModuleRR()
	s.configModuleProxyMap()
}

// MonitorConfig 自动刷新配置
func (s *SysConfigManage) MonitorConfig() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				config.SysLog.Error("monitor config recover error, err:%s", err)
			}
		}()
		for {
			time.Sleep(time.Duration(config.BaseConf.Interval) * time.Millisecond)
			s.ReloadConfig()
		}
	}()
}

// ConfigChangeNotice 配置变更通知
func (s *SysConfigManage) ConfigChangeNotice() <-chan struct{} {
	return s.loadConfigContext.Done()
}

// GetModuleConfig 获取全局系统配置
func (s *SysConfigManage) GetModuleConfig() *running.Modules {
	s.moduleConfigLocker.RLock()
	defer s.moduleConfigLocker.RUnlock()
	return s.moduleConfig
}

// GetModuleConfigByName 通过模块名获取模块配置
func (s *SysConfigManage) GetModuleConfigByName(name string) *running.GatewayModule {
	module, ok := s.moduleConfig.Module[name]
	if !ok {
		return nil
	}
	return module
}

// GetActiveIPList 获取活动IP
func (s *SysConfigManage) GetActiveIPList(moduleName string) []string {
	s.moduleActiveIPListMapLocker.RLock()
	activeIPList, _ := s.moduleActiveIPListMap[moduleName]
	s.moduleActiveIPListMapLocker.RUnlock()
	return activeIPList
}

// GetForbidIPList 获取禁用IP
func (s *SysConfigManage) GetForbidIPList(moduleName string) []string {
	s.moduleForbidIPListMapLocker.RLock()
	ipList, _ := s.moduleForbidIPListMap[moduleName]
	s.moduleForbidIPListMapLocker.RUnlock()
	return ipList
}

// GetAvailableIPList 获取可用IP
func (s *SysConfigManage) GetAvailableIPList(moduleName string) []string {
	s.moduleIPListMapLocker.RLock()
	ipList, _ := s.moduleIPListMap[moduleName]
	s.moduleIPListMapLocker.RUnlock()
	return ipList
}

// GetModuleIPList 获取当前可用的ip列表
func (s *SysConfigManage) GetModuleIPList(moduleName string) ([]string, error) {
	ipList := s.GetAvailableIPList(moduleName)
	if len(ipList) != 0 {
		return ipList, nil
	}
	return []string{}, errors.New("module ip empty")
}

// GetModuleConfIPList 获取配置的ip列表
func (s *SysConfigManage) GetModuleConfIPList(moduleName string) ([]string, error) {
	if moduleConf := s.GetModuleConfigByName(moduleName); moduleConf != nil {
		ipList := strings.Split(moduleConf.LoadBalance.IPList, ",")
		return ipList, nil
	}
	return []string{}, errors.New("module ip empty")
}

// GetModuleRR 获取模块的负载信息
func (s *SysConfigManage) GetModuleRR(moduleName string) (core.RR, error) {
	s.moduleRRMapLocker.RLock()
	defer s.moduleRRMapLocker.RUnlock()
	rr, ok := s.moduleRRMap[moduleName]
	if ok {
		return rr, nil
	}
	return nil, errors.New("module rr empty")
}

// GetConfIPWeightMap 返回模块对应的ip及权重
func (s *SysConfigManage) GetConfIPWeightMap(module *running.GatewayModule, defaultWeight int64) map[string]int64 {
	confIPList := strings.Split(module.LoadBalance.IPList, ",")
	confWeightList := strings.Split(module.LoadBalance.WeightList, ",")
	confIPWeightMap := map[string]int64{}
	for index, ipAddr := range confIPList {
		if len(confWeightList) >= index+1 {
			weight, err := strconv.ParseInt(confWeightList[index], 10, 64)
			if err != nil {
				weight = defaultWeight
			}
			confIPWeightMap[ipAddr] = weight
		} else {
			confIPWeightMap[ipAddr] = defaultWeight
		}
	}
	return confIPWeightMap
}

// GetAppConfigByAPPID 获取租户数据
func (s *SysConfigManage) GetAppConfigByAPPID(appID string) (*entity.GatewayAPP, error) {
	app, ok := s.appConfig.Apps[appID]
	if !ok {
		return nil, errors.New("app config empty")
	}
	return app, nil
}

// GetModuleHTTPProxy 获取http代理方法
func (s *SysConfigManage) GetModuleHTTPProxy(moduleName string) (*httputil.ReverseProxy, error) {
	rr, err := s.GetModuleRR(moduleName)
	if err != nil {
		return nil, err
	}
	s.moduleProxyFuncMapLocker.RLock()
	defer s.moduleProxyFuncMapLocker.RUnlock()
	proxyFunc, ok := s.moduleProxyFuncMap[moduleName]
	if ok {
		return proxyFunc(rr), nil
	}
	return nil, errors.New("module proxy empty")
}

// 刷新Module信息从DB到内存
// 如果DB挂了，从本地配置文件恢复
func (s *SysConfigManage) refreshModuleConfig() error {
	defer func() {
		if err := recover(); err != nil {
			config.SysLog.Error("[refresh module config recover] [err:%s]", err)
		}
	}()
	configFile := *config.Conf + "module.json"
	config.SysLog.Info("[start refresh module config from file:%s]", configFile)
	fileConf, err := s.getFileModuleConf(configFile)
	if err != nil {
		config.SysLog.Error("[get module from file:%s] [err:%s]", configFile, err.Error())
		return err
	}

	dbConf, err := s.getDBModuleConf(true)
	if err != nil {
		config.SysLog.Error("[get module from db] [err:%s]", err.Error())
	}

	//如果db挂了默认降级走file
	if dbConf != nil {
		s.moduleConfigLocker.Lock()
		s.moduleConfig = dbConf
		s.moduleConfigLocker.Unlock()
		config.SysLog.Info("module_configured_by_db.")
		err := s.writeFileModuleConf(configFile, dbConf)
		if err != nil {
			config.SysLog.Error("WriteFileModuleConf_error:%v", err)
		} else {
			config.SysLog.Info("module_file_was_override.")
		}
	} else if fileConf != nil {
		s.moduleConfigLocker.Lock()
		s.moduleConfig = fileConf
		s.moduleConfigLocker.Unlock()
		config.SysLog.Info("module_configured_by_file.")
	} else {
		config.SysLog.Info("get_dbConf_and_fileConf_both_error")
		return err
	}
	config.SysLog.Info("ModuleConf:%v", s.GetModuleConfig())
	return nil
}

// 刷新租户信息从DB到内存
// 如果DB挂了，从本地配置文件恢复
func (s *SysConfigManage) refreshAPPConfig() error {
	defer func() {
		if err := recover(); err != nil {
			config.SysLog.Error("RefreshAPP_recover:%v", err)
		}
	}()
	configFile := *config.Conf + "app.json"
	config.SysLog.Info("module_file:%s", configFile)
	fileConf, err := s.getFileAPPConf(configFile)
	if err != nil {
		config.SysLog.Error("GetFileAPPConf_error:%v", err)
		return err
	}
	config.SysLog.Info("GetFileAPPConf:%v", fileConf)
	dbConf, err := s.getDBAPPConf(false)
	if err != nil {
		config.SysLog.Error("GetDBAPPConf_error:%v", err)
	}
	config.SysLog.Info("GetDBAPPConf:%v", dbConf)

	//如果db挂了默认降级走file
	if dbConf != nil {
		s.appConfigLocker.Lock()
		s.appConfig = dbConf
		s.appConfigLocker.Unlock()
		config.SysLog.Info("app_configured_by_db.")
		err := s.writeFileAppConf(configFile, dbConf)
		if err != nil {
			config.SysLog.Error("WriteFileModuleConf_error:%v", err)
		} else {
			config.SysLog.Info("app_file_was_override.")
		}
	} else if fileConf != nil {
		s.appConfigLocker.Lock()
		s.appConfig = fileConf
		s.appConfigLocker.Unlock()
		config.SysLog.Info("app_configured_by_file.")
	} else {
		config.SysLog.Info("get_dbConf_and_fileConf_both_error")
		return err
	}
	config.SysLog.Info("APPConf:%v", s.appConfig)
	return nil
}

// 配置模块服务发现检测
// 已运行模块周期刷新，直到IpContext停止
func (s *SysConfigManage) checkIPList() {
	moduleConfiger := s.GetModuleConfig()
	s.moduleIPListMapLocker.Lock()
	s.moduleForbidIPListMapLocker.Lock()
	for _, module := range moduleConfiger.Module {
		s.moduleIPListMap[module.Base.Name] = strings.Split(module.LoadBalance.IPList, ",")
		s.moduleForbidIPListMap[module.Base.Name] = strings.Split(module.LoadBalance.ForbidList, ",")
	}
	s.moduleForbidIPListMapLocker.Unlock()
	s.moduleIPListMapLocker.Unlock()
	for _, modulePt := range moduleConfiger.Module {
		module := modulePt
		go func() {
			defer func() {
				if err := recover(); err != nil {
					config.SysLog.Warn("checkModuleIpList_recover:%v", err)
				}
			}()
			t1 := time.NewTimer(time.Second * 10)
		Loop:
			for {
				select {
				case <-t1.C:
					activeIPList := s.checkModuleIPList(module.LoadBalance)
					s.moduleActiveIPListMapLocker.Lock()
					s.moduleActiveIPListMap[module.Base.Name] = activeIPList
					s.moduleActiveIPListMapLocker.Unlock()

					s.moduleForbidIPListMapLocker.Lock()
					forbidIPList, ok := s.moduleForbidIPListMap[module.Base.Name]
					s.moduleForbidIPListMapLocker.Unlock()
					if !ok {
						forbidIPList = []string{}
					}

					// 剔除禁用节点
					newIPList := []string{}
					for _, newIP := range activeIPList {

						if !util.InStringList(newIP, forbidIPList) {
							newIPList = append(newIPList, newIP)
						}
					}

					configIPList := strings.Split(module.LoadBalance.IPList, ",")
					s.moduleIPListMapLocker.Lock()
					s.moduleIPListMap[module.Base.Name] = newIPList
					s.moduleIPListMapLocker.Unlock()
					config.SysLog.Info("%s CheckModuleIpList newIPList=%+v configIPList=%+v", module.Base.Name, newIPList, configIPList)
					t1.Reset(time.Millisecond * time.Duration(module.LoadBalance.CheckInterval))
				case <-s.ConfigChangeNotice():
					config.SysLog.Info(module.Base.Name + "_CheckModuleIpList done")
					break Loop
				}
			}
		}()
	}
}

// 后端服务器探活
// 返回存活状态的ip列表
func (s *SysConfigManage) checkModuleIPList(balance *entity.GatewayLoadBalance) []string {
	newIPList := []string{}
	ipList := strings.Split(balance.IPList, ",")
	for _, ip := range ipList {
		checkURL := fmt.Sprintf("http://%s%s", ip, balance.CheckURL)
		response, _, err := util.HttpGET(checkURL, nil, balance.CheckInterval, nil)
		if err != nil || response.StatusCode != 200 {
			config.SysLog.Warn("[host down] [host:%s] [url] [%s]", ip, checkURL)
		} else {
			newIPList = append(newIPList, ip)
		}
	}
	return newIPList
}

// 检查Module配置合法性
// Base配置不能为空，LoadBalance配置不能为空
func (s *SysConfigManage) checkModuleConf(conf *running.Modules) error {
	if conf == nil || len(conf.Module) == 0 {
		return errors.New("conf is empty")
	}
	for _, confItem := range conf.Module {
		if confItem.Base == nil {
			return errors.New("module.base is empty")
		}
		if confItem.LoadBalance == nil {
			return errors.New("module.load_balance is empty")
		}
	}
	return nil
}

// 读取配置文件中的ModuleConfig
func (s *SysConfigManage) getFileModuleConf(confPath string) (*running.Modules, error) {
	moduleConf := &running.Modules{}
	file, err := os.Open(confPath)
	if err != nil {
		return moduleConf, err
	}
	defer file.Close()
	bts, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bts, moduleConf); err != nil {
		return nil, err
	}
	if err := s.checkModuleConf(moduleConf); err != nil {
		return nil, err
	}
	return moduleConf, nil
}

// 将ModuleConf写入到配置文件
func (s *SysConfigManage) writeFileModuleConf(confPath string, moduleConf *running.Modules) error {
	var out bytes.Buffer
	data, err := json.Marshal(moduleConf)
	if err != nil {
		return err
	}
	err = json.Indent(&out, data, "", "\t")
	bakPath := strings.Replace(confPath, ".json", "_bak.json", -1)
	os.Remove(bakPath)
	if err := os.Rename(confPath, bakPath); err != nil {
		return err
	}
	if err := ioutil.WriteFile(confPath, out.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

// 读取DB中的ModuleConfig
func (s *SysConfigManage) getDBModuleConf(isCheck bool) (*running.Modules, error) {
	defer func() {
		if err := recover(); err != nil {
			config.SysLog.Error("GetDBModuleConf_recover:%v", err)
		}
	}()
	moduleConf := &running.Modules{Module: make(map[string]*running.GatewayModule)}
	bases, err := (&entity.GatewayModuleBase{}).GetAll(config.DB)
	if err != nil {
		return nil, err
	}
	matchRuleArr, err := (&entity.GatewayMatchRule{}).GetAll(config.DB)
	if err != nil {
		return nil, err
	}
	accessControlArr, err := (&entity.GatewayAccessControl{}).GetAll(config.DB)
	if err != nil {
		return nil, err
	}
	loadBalanceArr, err := (&entity.GatewayLoadBalance{}).GetAll(config.DB)
	if err != nil {
		return nil, err
	}
	for _, base := range bases {
		matchRules := &entity.GatewayMatchRule{}
		for _, x := range matchRuleArr {
			if x.ModuleID == base.ID {
				matchRules = x
			}
		}
		accessControl := &entity.GatewayAccessControl{}
		for _, x := range accessControlArr {
			if x.ModuleID == base.ID {
				accessControl = x
			}
		}
		loadBalance := &entity.GatewayLoadBalance{}
		for _, x := range loadBalanceArr {
			if x.ModuleID == base.ID {
				loadBalance = x
			}
		}
		moduleConf.Module[base.Name] = &running.GatewayModule{
			Base:          base,
			MatchRule:     matchRules,
			AccessControl: accessControl,
			LoadBalance:   loadBalance,
		}
	}
	if isCheck {
		if err := s.checkModuleConf(moduleConf); err != nil {
			return nil, err
		}
	}
	return moduleConf, nil
}

// 检查App的配置合法性
func (s *SysConfigManage) checkAppConf(conf *running.Apps) error {
	if conf == nil {
		return errors.New("conf is empty")
	}
	for _, confItem := range conf.Apps {
		if confItem.Name == "" {
			return errors.New("app.name is empty")
		}
		if confItem.Secret == "" {
			return errors.New("app.secret is empty")
		}
		if confItem.AppID == "" {
			return errors.New("app.app_id is empty")
		}
	}
	return nil
}

// 获取配置文件中的App
func (s *SysConfigManage) getFileAPPConf(confPath string) (*running.Apps, error) {
	appConf := &running.Apps{}
	file, err := os.Open(confPath)
	if err != nil {
		return appConf, err
	}
	defer file.Close()
	bts, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bts, appConf); err != nil {
		return nil, err
	}
	if err := s.checkAppConf(appConf); err != nil {
		return nil, err
	}
	return appConf, nil
}

// 读取DB中的AppConfig
func (s *SysConfigManage) getDBAPPConf(isCheck bool) (*running.Apps, error) {
	defer func() {
		if err := recover(); err != nil {
			config.SysLog.Error("GetDBAPPConf_recover:%v", err)
		}
	}()
	db := config.DB
	apps, err := (&entity.GatewayAPP{}).GetAll(db)
	if err != nil {
		return nil, err
	}
	appConf := &running.Apps{Apps: make(map[string]*entity.GatewayAPP)}
	for _, app := range apps {
		appConf.Apps[app.Name] = app
	}
	if isCheck {
		if err := s.checkAppConf(appConf); err != nil {
			return nil, err
		}
	}
	return appConf, nil
}

// 将AppConf写入到配置文件当中
func (s *SysConfigManage) writeFileAppConf(confPath string, appConf *running.Apps) error {
	var out bytes.Buffer
	data, err := json.Marshal(appConf)
	if err != nil {
		return err
	}
	err = json.Indent(&out, data, "", "\t")
	bakPath := strings.Replace(confPath, ".json", "_bak.json", -1)
	os.Remove(bakPath)
	if err := os.Rename(confPath, bakPath); err != nil {
		return err
	}
	if err := ioutil.WriteFile(confPath, out.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

// 配置模块负载信息到ModuleRRMap+已运行模块周期刷新，直到IpContext停止
func (s *SysConfigManage) configModuleRR() error {
	modules := s.GetModuleConfig()
	for _, modulePointer := range modules.Module {
		currentModule := modulePointer
		go func(currentModule *running.GatewayModule) {
			defer func() {
				if err := recover(); err != nil {
					config.SysLog.Error("ConfigModuleRR_recover:%v", err)
				}
			}()
			if currentModule.Base.LoadType != "http" {
				return
			}
			t1 := time.NewTimer(0)
			ipList := []string{}
			ipWeightMap := map[string]int64{}
		Loop:
			for {
				select {
				case <-t1.C:
					newIPList := s.GetAvailableIPList(currentModule.Base.Name)
					newIPWeightMap := s.GetConfIPWeightMap(currentModule, constant.IPDefaultWeight)
					if !reflect.DeepEqual(ipList, newIPList) || !reflect.DeepEqual(ipWeightMap, newIPWeightMap) {
						Rw := core.NewWeightedRR(core.RRNginx)
						for _, ipAddr := range newIPList {
							w, ok := newIPWeightMap[ipAddr]
							if ok {
								Rw.Add(ipAddr, int(w))
							} else {
								Rw.Add(ipAddr, constant.IPDefaultWeight)
							}
						}
						s.moduleRRMapLocker.Lock()
						s.moduleRRMap[currentModule.Base.Name] = Rw
						s.moduleRRMapLocker.Unlock()
					}
					ipList = newIPList
					ipWeightMap = newIPWeightMap
					t1.Reset(time.Millisecond * time.Duration(currentModule.LoadBalance.CheckInterval))
				case <-s.ConfigChangeNotice():
					t1.Stop()
					break Loop
				}
			}
		}(currentModule)
	}
	return nil
}

// 配置Transport和ProxyFunc
func (s *SysConfigManage) configModuleProxyMap() error {
	modules := s.GetModuleConfig()
	for _, modulePointer := range modules.Module {
		currentModule := modulePointer
		proxyFunc := func(rr core.RR) *httputil.ReverseProxy {
			mtp, _ := s.getModuleTransport(currentModule.Base.Name)
			proxy := &httputil.ReverseProxy{
				Director: func(req *http.Request) {
					if rHost, ok := rr.Next().(string); ok {
						req.URL.Scheme = "http"
						if req.TLS != nil {
							req.URL.Scheme = "https"
						}
						req.URL.Host = rHost
						req.Host = config.BaseConf.Http.ReqHost
					}
				},
				ModifyResponse: func(response *http.Response) error {
					if strings.Contains(response.Header.Get("Connection"), "Upgrade") {
						return nil
					}
					var payload []byte
					var readErr error
					if strings.Contains(response.Header.Get("Content-Encoding"), "gzip") {
						gr, err := gzip.NewReader(response.Body)
						if err != nil {
							config.SysLog.Error("err:%s", err.Error())
						}
						payload, readErr = ioutil.ReadAll(gr)
						response.Header.Del(constant.ContentEncoding)
					} else {
						payload, readErr = ioutil.ReadAll(response.Body)
					}
					if readErr != nil {
						return readErr
					}

					newPayload := payload

					//过滤请求数据
					response.Body = ioutil.NopCloser(bytes.NewBuffer(newPayload))
					response.ContentLength = int64(len(newPayload))
					response.Header.Set("Content-Length", strconv.FormatInt(int64(len(newPayload)), 10))
					//if err := ModifyResponse(currentModule, response.Request, response); err != nil {
					//	return err
					//}
					return nil
				},
				Transport: mtp,
				ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
					util.HTTPError(http.StatusGatewayTimeout, fmt.Sprint(err), w, req)
					return
				},
			}
			return proxy
		}
		mtp := &http.Transport{
			//请求下游的时间
			DialContext: (&net.Dialer{
				//限制建立TCP连接的时间
				Timeout: time.Duration(currentModule.LoadBalance.ProxyConnectTimeout) * time.Millisecond,
			}).DialContext,
			//单机最大连接数
			MaxConnsPerHost: 0,
			//最大空闲链接数
			MaxIdleConns: currentModule.LoadBalance.MaxIdleConn,
			//链接最大空闲时间
			IdleConnTimeout: time.Duration(currentModule.LoadBalance.IdleConnTimeout) * time.Millisecond,
			//限制读取response header的时间
			ResponseHeaderTimeout: time.Duration(currentModule.LoadBalance.ProxyHeaderTimeout) * time.Millisecond,
		}
		s.moduleTransportMapLocker.Lock()
		s.moduleTransportMap[currentModule.Base.Name] = mtp
		s.moduleTransportMapLocker.Unlock()
		s.moduleProxyFuncMapLocker.Lock()
		s.moduleProxyFuncMap[currentModule.Base.Name] = proxyFunc
		s.moduleProxyFuncMapLocker.Unlock()
	}
	return nil
}

// 获取对应模块的Transport
func (s *SysConfigManage) getModuleTransport(name string) (*http.Transport, error) {
	s.moduleTransportMapLocker.RLock()
	if mtp, ok := s.moduleTransportMap[name]; ok {
		s.moduleTransportMapLocker.RUnlock()
		return mtp, nil
	}
	s.moduleTransportMapLocker.RUnlock()
	return nil, errors.New("transport not found")
}
