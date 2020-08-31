package controller

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"github.com/e421083458/golang_common/lib"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"gatekeeper/config"
	"gatekeeper/constant"
	"gatekeeper/core/resource"
	"gatekeeper/core/service"
	"gatekeeper/model/entity"
	"gatekeeper/model/running"
	"gatekeeper/util"
)

//AdminRegister admin路口注册
func AdminRegister(router *gin.RouterGroup) {
	admin := Admin{}
	router.GET("/index", admin.Index)
	router.GET("/login", admin.Login)
	router.POST("/login", admin.Login)
	router.GET("/loginout", admin.LoginOut)
	router.GET("/open", admin.Open)
	router.GET("/close", admin.Close)
	router.GET("/delete", admin.Delete)
	router.GET("/add_http", admin.AddHTTP)
	router.GET("/service_list", admin.ServiceList)
	router.GET("/app_list", admin.AppList)
	router.GET("/add_app", admin.AddAPP)
	router.GET("/edit_app", admin.EditAPP)
	router.POST("/save_app", admin.SaveAPP)
	router.GET("/del_app", admin.DelAPP)
	router.GET("/app_detail", admin.APPDetail)
	router.GET("/service_detail", admin.ServiceDetail)
	router.POST("/save_service", admin.SaveService)
	router.GET("/edit_service", admin.EditService)
}

//Index 首页action
func (admin *Admin) Index(c *gin.Context) {
	admin.ServiceList(c)
	return
}

//LoginOut 退出action
func (admin *Admin) LoginOut(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     constant.AdminCookiePrefix + "user_name",
		Value:    "",
		Expires:  time.Now().Add(-31500000 * time.Second),
		Path:     "/",
		Domain:   "",
		Secure:   false,
		HttpOnly: true,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     constant.AdminCookiePrefix + "token",
		Value:    "",
		Expires:  time.Now().Add(-31500000 * time.Second),
		Path:     "/",
		Domain:   "",
		Secure:   false,
		HttpOnly: true,
	})
	c.Redirect(302, "/admin/login")
	return
}

//LoginAuth 登陆验证action
func (admin *Admin) LoginAuth(c *gin.Context) error {
	userName, err := c.Cookie(constant.AdminCookiePrefix + "user_name")
	if err != nil {
		return err
	}
	token, err := c.Cookie(constant.AdminCookiePrefix + "token")
	if err != nil {
		return err
	}

	//base64解密
	bytesPass, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return err
	}

	//aes解密
	tokenStr, err := util.AesDecrypt(c.Request.Context(), bytesPass, []byte(constant.AdminCookieSecrit))
	if err != nil {
		return err
	}
	if string(tokenStr) != userName {
		return errors.New("login error")
	}
	return nil
}

//Login 登陆action
func (admin *Admin) Login(c *gin.Context) {
	userName := c.PostForm("user_name")
	passport := c.PostForm("passport")
	if config.AuthConf.AdminName == userName && config.AuthConf.AdminPassport == passport {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     constant.AdminCookiePrefix + "user_name",
			Value:    userName,
			Expires:  time.Now().Add(time.Second * constant.AdminExpired),
			Path:     "/",
			Domain:   "",
			Secure:   false,
			HttpOnly: true,
		})

		var aeskey = []byte(constant.AdminCookieSecrit)
		pass := []byte(userName)
		xpass, err := util.AesEncrypt(c.Request.Context(), pass, aeskey)
		if err != nil {
			util.ResponseError(c, 500, err)
			return
		}
		pass64 := base64.StdEncoding.EncodeToString(xpass) //base64
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     constant.AdminCookiePrefix + "token",
			Value:    pass64,
			Expires:  time.Now().Add(time.Second * constant.AdminExpired),
			Path:     "/",
			Domain:   "",
			Secure:   false,
			HttpOnly: true,
		})
		c.Redirect(302, "/admin/index")
		return
	}
	t := template.New("login.html")
	t, err := t.ParseFiles("./tmpl/green/login.html")
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	err2 := t.Execute(c.Writer, "")
	if err2 != nil {
		util.ResponseError(c, 500, err)
	}
	return
}

//Open 打开流量action
func (admin *Admin) Open(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleName := c.Query("name")
	addr := c.Query("addr")
	tx := config.DB.Begin()
	base := &entity.GatewayModuleBase{Name: moduleName}
	baseInfo, err := base.FindByName(tx, moduleName)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("base.FindByName:"+err.Error()))
		return
	}
	load := &entity.GatewayLoadBalance{}
	load, err = load.GetByModule(tx, baseInfo.ID)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("load.GetByModule:"+err.Error()))
		return
	}
	forbidList := []string{}
	for _, item := range strings.Split(load.ForbidList, ",") {
		if item != "" && item != addr {
			forbidList = append(forbidList, item)
		}
	}
	load.ForbidList = strings.Join(forbidList, ",")
	lerr := load.Save(tx)
	if lerr != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("load.Save:"+lerr.Error()))
		return
	}
	tx.Commit()
	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	c.Redirect(302, "/admin/service_detail?module_name="+moduleName)
}

//Close 关闭流量action
func (admin *Admin) Close(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleName := c.Query("name")
	addr := c.Query("addr")

	tx := config.DB.Begin()
	base := &entity.GatewayModuleBase{Name: moduleName}
	baseInfo, err := base.FindByName(tx, moduleName)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("base.FindByName:"+err.Error()))
		return
	}
	load := &entity.GatewayLoadBalance{}
	load, err = load.GetByModule(tx, baseInfo.ID)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("load.GetByModule:"+err.Error()))
		return
	}
	forbidList := []string{}
	for _, item := range strings.Split(load.ForbidList, ",") {
		if item != "" {
			forbidList = append(forbidList, item)
		}
	}
	if !util.InStringList(addr, forbidList) {
		forbidList = append(forbidList, addr)
	}
	load.ForbidList = strings.Join(forbidList, ",")
	//fmt.Println(load.ForbidList)
	lerr := load.Save(tx)
	if lerr != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("load.Save:"+err.Error()))
		return
	}
	tx.Commit()

	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	c.Redirect(302, "/admin/service_detail?module_name="+moduleName)
	return
}

//ClusterReloadModule 集群配置更新action
func (admin *Admin) ClusterReloadModule() error {
	clusterList := config.BaseConf.Cluster.ClusterList
	clusterAddr := config.BaseConf.Cluster.ClusterAddr

	//集群配置更新
	for _, item := range strings.Split(clusterList, ",") {
		resp, bts, _ := util.HttpGET(fmt.Sprintf(
			"http://%s%s/reload",
			item, clusterAddr), nil, 5000, nil)
		if resp.StatusCode != 200 {
			return errors.New("clusterList.update:" + string(bts))
		}
	}
	return nil
}

//Delete 删除服务action
func (admin *Admin) Delete(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleName := c.Query("module_name")
	if moduleName == "" {
		util.ResponseError(c, 500, errors.New("module name 必传！"))
		return
	}
	tx := config.DB.Begin()
	base := &entity.GatewayModuleBase{}
	baseInfo, err := base.FindByName(tx, moduleName)
	if err != nil {
		util.ResponseError(c, 500, errors.New("base.FindByName:"+err.Error()))
		return
	}
	if baseInfo.Name != moduleName {
		util.ResponseError(c, 500, errors.New("module name 不存在"))
		return
	}
	baseInfo.Del(tx)
	access := &entity.GatewayAccessControl{ModuleID: baseInfo.ID}
	access.Del(tx)
	load := &entity.GatewayLoadBalance{ModuleID: baseInfo.ID}
	load.Del(tx)
	match := &entity.GatewayMatchRule{ModuleID: baseInfo.ID}
	match.Del(tx)
	tx.Commit()
	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	c.Redirect(302, "/admin/service_list")
}

func (admin *Admin) getDBAPPConf() (*running.Apps, error) {
	defer func() {
		if err := recover(); err != nil {
			config.SysLog.Error("GetDBAPPConf_recover:%v", err)
		}
	}()
	apps, err := (&entity.GatewayAPP{}).GetAll(config.DB, "id desc")
	if err != nil {
		return nil, err
	}
	appConf := &running.Apps{make(map[string]*entity.GatewayAPP)}
	for _, app := range apps {
		appConf.Apps[app.Name] = app
	}
	return appConf, nil
}

//AppList app列表action
func (admin *Admin) AppList(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	appConf, errm := admin.getDBAPPConf()
	if errm != nil {
		util.ResponseError(c, 500, errm)
		return
	}
	appListObj := &APPListObj{ActiveURL: "/admin/app_list"}
	for _, app := range appConf.Apps {
		counter := resource.FlowCounters.GetAPPCounter(app.AppID)
		DateFormat := "2006-01-02"
		qdp, _ := counter.GetDayCount(time.Now().Format(DateFormat))
		appListObj.List = append(appListObj.List, APPItemObj{
			GatewayAPP: app,
			QPS:        counter.QPS,
			QPD:        qdp,
		})
	}
	t, err := admin.getTemplateByURL("/admin/app_list")
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	err2 := admin.executeTemplate(t, c.Writer, appListObj, "/admin/app_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
	}
	return
}

//ServiceList 服务列表action
func (admin *Admin) ServiceList(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleConf, err := admin.getDBModuleConf()
	if err != nil {
		util.ResponseError(c, 500, err)
		return
	}
	if moduleConf == nil {
		util.ResponseError(c, 500, errors.New("获取模块配置错误"))
		return
	}
	t, err := admin.getTemplateByURL("/admin/service_list")
	if err != nil {
		util.ResponseError(c, 500, err)
	}

	clusterIP := config.BaseConf.Cluster.ClusterIP
	httpAddr := config.BaseConf.Cluster.ClusterAddr

	detailList := []*ServiceDetailInfo{}
	for _, module := range moduleConf.Module {
		ipList := strings.Split(module.LoadBalance.IPList, ",")
		weightList := strings.Split(module.LoadBalance.WeightList, ",")
		detailInfo := &ServiceDetailInfo{}
		detailInfo.Module = module
		detailInfo.ModuleIPList = ipList
		detailInfo.WeightList = weightList
		detailInfo.ActiveIPList = service.SysConfMgr.GetActiveIPList(module.Base.Name)
		detailInfo.ForbidIPList = service.SysConfMgr.GetForbidIPList(module.Base.Name)
		detailInfo.AvaliableIPList = service.SysConfMgr.GetAvailableIPList(module.Base.Name)

		detailInfo.ModuleIPCount = len(detailInfo.ModuleIPList)
		detailInfo.ActiveIPCount = len(detailInfo.ActiveIPList)
		detailInfo.ForbidIPCount = len(detailInfo.ForbidIPList)
		detailInfo.AvaliableIPCount = len(detailInfo.AvaliableIPList)
		detailInfo.ClusterIP = clusterIP
		detailInfo.HTTPAddr = httpAddr
		detailInfo.QPS = resource.FlowCounters.GetRequestCounter(module.Base.Name).QPS
		today := time.Now().In(config.TimeLocation).Format(constant.DateFormat)

		counter := resource.FlowCounters.GetRequestCounter(module.Base.Name)
		counter.GetDayCount(today)
		dayCount, _ := counter.GetDayCount(today)
		detailInfo.DayRequest = fmt.Sprint(dayCount)
		detailList = append(detailList, detailInfo)
	}
	err2 := admin.executeTemplate(t, c.Writer, detailList, "/admin/service_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
	}
	return
}

func (admin *Admin) getDBModuleConf() (*running.Modules, error) {
	defer func() {
		if err := recover(); err != nil {
		}
	}()
	moduleConf := &running.Modules{Module: make(map[string]*running.GatewayModule)}
	bases, err := (&entity.GatewayModuleBase{}).GetAll(config.DB, "id desc")
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
		if base != nil && loadBalance != nil {
			moduleConf.Module[base.Name] = &running.GatewayModule{
				Base:          base,
				MatchRule:     matchRules,
				AccessControl: accessControl,
				LoadBalance:   loadBalance,
			}
		}
	}
	return moduleConf, nil
}

//ServiceDetail 服务详情action
func (admin *Admin) ServiceDetail(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleName := c.Query("module_name")
	moduleConf, err := admin.getDBModuleConf()
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	if moduleConf == nil {
		util.ResponseError(c, 500, errors.New("获取模块配置错误"))
		return
	}
	t, err := admin.getTemplateByURL("/admin/service_detail")
	if err != nil {
		util.ResponseError(c, 500, err)
		return
	}
	var module *running.GatewayModule
	for _, tmpModule := range moduleConf.Module {
		if tmpModule.Base.Name == moduleName {
			module = tmpModule
		}
	}
	if module == nil {
		util.ResponseError(c, 500, errors.New("module_name not found"))
		return
	}
	ipList := strings.Split(module.LoadBalance.IPList, ",")
	weightList := strings.Split(module.LoadBalance.WeightList, ",")

	detailInfo := &ServiceDetailInfo{}
	detailInfo.Module = module
	detailInfo.ModuleIPList = ipList
	detailInfo.WeightList = weightList
	detailInfo.ActiveIPList = service.SysConfMgr.GetActiveIPList(moduleName)
	detailInfo.ForbidIPList = service.SysConfMgr.GetForbidIPList(moduleName)
	detailInfo.AvaliableIPList = service.SysConfMgr.GetAvailableIPList(moduleName)

	counter := resource.FlowCounters.GetRequestCounter(module.Base.Name)
	for i := 0; i <= time.Now().In(config.TimeLocation).Hour(); i++ {
		todaydate := time.Now().In(config.TimeLocation).Format("20060102")
		todayhour := todaydate + fmt.Sprint(i)
		if i < 10 {
			todayhour = todaydate + "0" + fmt.Sprint(i)
		}
		if hourstat, err := counter.GetHourCount(todayhour); err == nil {
			second := int64(3600)
			if i == time.Now().In(config.TimeLocation).Hour() {
				hourTime, _ := time.ParseInLocation(constant.TimeFormat,
					time.Now().In(config.TimeLocation).Format("2006-01-02 15:00:00"),
					config.TimeLocation)
				second = time.Now().Unix() - hourTime.Unix()
			}
			detailInfo.DailyHourAvg += "[" + fmt.Sprint(i) + ", " + fmt.Sprintf("%.4f", float64(hourstat)/float64(second)) + "],"
			detailInfo.DailyHourStat += fmt.Sprint(hourstat) + ","
			if hourstat > detailInfo.DailyStatMax {
				detailInfo.DailyStatMax = hourstat
			}
		} else {
			detailInfo.DailyHourStat += "0,"
			detailInfo.DailyHourAvg += "[" + fmt.Sprint(i) + ", " + "0" + "],"
		}
	}

	if len(detailInfo.DailyHourStat) > 0 {
		detailInfo.DailyStatMax = int64(float64(detailInfo.DailyStatMax) * 1.2)
		detailInfo.DailyHourStat = detailInfo.DailyHourStat[0 : len(detailInfo.DailyHourStat)-1]
	}
	err = admin.executeTemplate(t, c.Writer, detailInfo, "/admin/service_list")
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	return
}

//SaveService 保存服务action
func (admin *Admin) SaveService(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	baseID := c.PostForm("base.id")
	moduleName := c.PostForm("base.name")
	serviceName := c.PostForm("base.service_name")
	loadType := c.PostForm("base.load_type")
	frontendPort := c.PostForm("base.frontend_addr")
	matchRule := c.PostForm("match.rule")
	urlRewrite := c.PostForm("match.url_rewrite")
	ipWeight := c.PostForm("load.ip_weight_list")
	checkURL := c.PostForm("load.check_url")
	whiteList := c.PostForm("access.white_list")
	whiteHostName := c.PostForm("access.white_host_name")
	blackList := c.PostForm("access.black_list")
	clientFlowLimit := c.PostForm("access.client_flow_limit")
	isOpen := c.PostForm("access.open")
	authType := c.PostForm("access.auth_type")
	//filterRule := c.PostForm("filter.rule")
	checkInterval := c.PostForm("load.check_interval")
	proxyConnectTimeout := c.PostForm("load.proxy_connect_timeout")
	proxyHeaderTimeout := c.PostForm("load.proxy_header_timeout")
	proxyBodyTimeout := c.PostForm("load.proxy_body_timeout")
	idleConnTimeout := c.PostForm("load.idle_conn_timeout")
	maxIdleConn := c.PostForm("load.max_idle_conn")
	//fmt.Println("isOpen",isOpen)
	checkIntervalInt, err := strconv.ParseInt(checkInterval, 10, 64)
	if err != nil {
		util.ResponseError(c, 500, errors.New("探活频率 格式化错误:"+err.Error()))
		return
	}
	if checkIntervalInt < 1000 {
		util.ResponseError(c, 500, errors.New("探活频率 最小 1000 ms"))
		return
	}
	proxyConnectTimeoutInt, err := strconv.ParseInt(proxyConnectTimeout, 10, 64)
	if err != nil {
		util.ResponseError(c, 500, errors.New("连接目标服务器超时 格式化错误:"+err.Error()))
		return
	}
	if proxyConnectTimeoutInt < 500 {
		util.ResponseError(c, 500, errors.New("连接目标服务器超时 最小 500 ms"))
		return
	}
	proxyHeaderTimeoutInt, err := strconv.ParseInt(proxyHeaderTimeout, 10, 64)
	if err != nil && loadType == "http" {
		util.ResponseError(c, 500, errors.New("获取header头超时 格式化错误:"+err.Error()))
		return
	}
	proxyBodyTimeoutInt, err := strconv.ParseInt(proxyBodyTimeout, 10, 64)
	if err != nil && loadType == "http" {
		util.ResponseError(c, 500, errors.New("获取body内容超时 格式化错误:"+err.Error()))
		return
	}
	idleConnTimeoutInt, err := strconv.ParseInt(idleConnTimeout, 10, 64)
	if err != nil && loadType == "http" {
		util.ResponseError(c, 500, errors.New("链接最大空闲时间 格式化错误:"+err.Error()))
		return
	}
	//fmt.Println(maxIdleConn)
	maxIdleConnInt, err := strconv.ParseInt(maxIdleConn, 10, 64)
	if err != nil && loadType == "http" {
		util.ResponseError(c, 500, errors.New("链接最大空闲时间 格式化错误:"+err.Error()))
		return
	}
	if strings.HasSuffix(matchRule, "/") {
		matchRule = util.Substr(matchRule, 0, int64(len(matchRule)-1))
	}

	if loadType == "http" {
		if moduleName == "" || serviceName == "" || matchRule == "" || ipWeight == "" || checkURL == "" {
			util.ResponseError(c, 500, errors.New("*字段，必须填写！"))
			return
		}
	} else if loadType == "tcp" {
		if moduleName == "" || serviceName == "" || frontendPort == "" || ipWeight == "" {
			util.ResponseError(c, 500, errors.New("*字段，必须填写！"))
			return
		}
	}

	//开启事务
	tx := config.DB.Begin()

	//先删除关联表
	var moduleID int64
	if baseID != "0" {
		tID, err := strconv.ParseInt(baseID, 10, 64)
		if err != nil {
			util.ResponseError(c, 500, errors.New("base.id，必须为数字！"))
			return
		}
		base := &entity.GatewayModuleBase{ID: tID}
		base.Del(tx)
		model := &entity.GatewayMatchRule{ModuleID: tID}
		model.Del(tx)
		model2 := &entity.GatewayLoadBalance{ModuleID: tID}
		model2.Del(tx)
		model3 := &entity.GatewayAccessControl{ModuleID: tID}
		model3.Del(tx)
		//model4 := &entity.GatewayDataFilter{ModuleID: tID}
		//model4.Del(tx)
		moduleID = tID
	}

	//base信息保存
	base := &entity.GatewayModuleBase{}
	base.ID = moduleID
	base.Name = moduleName
	base.ServiceName = serviceName
	base.LoadType = loadType
	base.PassAuthType = 2
	//服务标识验证
	if ok, _ := regexp.Match("^[0-9a-zA-Z_-]+$", []byte(moduleName)); !ok {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("服务名称格式错误，请重新填写！"))
		return
	}
	baseInfo, err := base.FindByName(tx, moduleName)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("base.FindByName:"+err.Error()))
		return
	}
	if baseInfo.Name == moduleName {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("服务名称重复，请重新填写！"))
		return
	}

	//tcp端口占用验证
	if loadType == "tcp" {
		base.FrontendAddr = frontendPort
		portInfo, err := base.FindByPort(tx, frontendPort)
		if err != nil {
			tx.Rollback()
			util.ResponseError(c, 500, errors.New("base.FindByPort:"+err.Error()))
			return
		}
		if portInfo.Name != "" {
			tx.Rollback()
			util.ResponseError(c, 500, errors.New("监听端口重复，请重新填写！"))
			return
		}
		//首次创建时验证
		if baseID == "0" {
			if ok, _ := regexp.Match("^:[0-9]+$", []byte(frontendPort)); !ok {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("监听端口格式错误，请重新填写！"))
				return
			}
			if err := util.CheckConnPort(frontendPort); err != nil {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("监听端口被占用，请重新填写！"))
				return
			}
		}
	}
	if err := base.Save(tx); err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("GatewayModuleBase.Save:"+err.Error()))
		return
	}

	//构造 gateway_match_rule
	matchRules := strings.Split(matchRule, ",")
	urlRewrites := strings.Split(urlRewrite, "\n")
	if baseID == "0" && (urlRewrite == "\n" || urlRewrite == "") {
		//urlRewrites为空时，自动填充
		for _, rule := range matchRules {
			urlRewrites = append(urlRewrites, fmt.Sprintf("^%s(.*) $1", rule))
		}
	}
	urlRewrite = strings.Join(urlRewrites, ",")
	for _, rule := range matchRules {
		model := &entity.GatewayMatchRule{}
		model.ModuleID = base.ID
		model.Type = "url_prefix"
		model.Rule = rule
		model.URLRewrite = urlRewrite
		if loadType == "http" {
			if !strings.HasPrefix(rule, config.BaseConf.Http.RoutePrefix) {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("访问前缀，必须以"+config.BaseConf.Http.RoutePrefix+"开头"))
				return
			}
			baseInfo, err := model.FindByURLPrefix(tx, rule)
			if err != nil {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("match.FindByURLPrefix:"+err.Error()))
				return
			}
			if baseInfo.Rule == rule && baseInfo.ModuleID != base.ID {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("访问前缀重复，请重新填写！"))
				return
			}
		}
		if err := model.Save(tx); err != nil {
			tx.Rollback()
			util.ResponseError(c, 500, errors.New("GatewayMatchRule.Save:"+err.Error()))
			return
		}
	}

	//构造 gateway_load_balance
	ipWeights := strings.Split(ipWeight, "\n")
	ipList := []string{}
	weightList := []string{}
	for _, ipItem := range ipWeights {
		ipItems := strings.Split(ipItem, " ")
		if util.InStringList(ipItems[0], ipList) {
			tx.Rollback()
			util.ResponseError(c, 500, errors.New("服务器ip重复"))
			return
		}
		ipList = append(ipList, ipItems[0])
		var weight int64
		if len(ipItems) > 1 {
			tmpWeight, err := strconv.ParseInt(ipItems[1], 10, 64)
			if err != nil {
				tx.Rollback()
				util.ResponseError(c, 500, errors.New("服务器权重必须为数字:"+err.Error()))
				return
			}
			weight = tmpWeight
		}
		weightList = append(weightList, strconv.Itoa(int(weight)))
	}
	model := &entity.GatewayLoadBalance{}
	model.ModuleID = base.ID
	model.CheckMethod = "httpchk"
	if loadType == "tcp" {
		model.CheckMethod = "tcpchk"
	}
	model.CheckURL = checkURL
	model.CheckTimeout = 2000
	model.CheckInterval = int(checkIntervalInt)
	model.Type = "round-robin"
	model.IPList = strings.Join(ipList, ",")
	model.WeightList = strings.Join(weightList, ",")
	model.ProxyConnectTimeout = int(proxyConnectTimeoutInt)
	model.ProxyHeaderTimeout = int(proxyHeaderTimeoutInt)
	model.ProxyBodyTimeout = int(proxyBodyTimeoutInt)
	model.IdleConnTimeout = int(idleConnTimeoutInt)
	//fmt.Println("save value",maxIdleConnInt)
	model.MaxIdleConn = int(maxIdleConnInt)
	if err := model.Save(tx); err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("GatewayLoadBalance.Save:"+err.Error()))
		return
	}

	//构造 gateway_access_control
	access := &entity.GatewayAccessControl{}
	access.ModuleID = base.ID
	access.BlackList = blackList
	access.WhiteList = whiteList
	access.WhiteHostName = whiteHostName
	var clientFlowLimitInt int64
	climit, cerr := strconv.ParseInt(clientFlowLimit, 10, 64)
	if cerr != nil {
		climit = 0
	}
	clientFlowLimitInt = climit
	access.ClientFlowLimit = clientFlowLimitInt
	if isOpen == "1" {
		access.Open = 1
	} else {
		access.Open = 0
	}
	access.ModuleID = base.ID
	access.AuthType = authType
	if err := access.Save(tx); err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("access.Save:"+err.Error()))
		//c.Error(500, "access.Save:"+err.Error())
		return
	}
	tx.Commit()
	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	util.ResponseSuccess(c, "")
	return
}

//SaveAPP 保存租户action
func (admin *Admin) SaveAPP(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	id := c.PostForm("id")
	appID := c.PostForm("app_id")
	name := c.PostForm("name")
	secret := c.PostForm("secret")
	totalQueryDaily := c.PostForm("total_query_daily")
	qps := c.PostForm("qps")
	openAPI := c.PostForm("open_api")
	whiteIps := c.PostForm("white_ips")
	cityIds := c.PostForm("city_ids")
	groupID := c.PostForm("group_id")
	timeout := c.PostForm("timeout")
	method := c.PostForm("method")

	if appID == "" || name == "" || secret == "" || totalQueryDaily == "" || qps == "" || timeout == "" {
		util.ResponseError(c, 500, errors.New("*字段，必须填写！"))
		return
	}
	if len(secret) != 32 {
		util.ResponseError(c, 500, errors.New("密钥必须32位！"))
		return
	}

	if ok, _ := regexp.Match("^[0-9a-zA-Z_-]+$", []byte(appID)); !ok {
		util.ResponseError(c, 500, errors.New("租户id格式错误，请重新填写！"))
		return
	}

	if !lib.InArrayString(method, []string{"any", "get", "post"}) {
		util.ResponseError(c, 500, errors.New("请求方法，请重新填写！"))
		return
	}

	//构造 gateway_module_base
	tx := config.DB.Begin()

	//原记录存在
	app := &entity.GatewayAPP{}
	if id != "0" {
		idInt, _ := strconv.ParseInt(id, 10, 64)
		appInfo, err := app.FindByID(tx, idInt)
		if err != nil {
			util.ResponseError(c, 500, errors.New("FindByID:"+err.Error()))
			return
		}
		app = appInfo
	} else {
		appInfo, err := app.FindByAppID(tx, appID)
		if err != nil {
			tx.Rollback()
			util.ResponseError(c, 500, errors.New("FindByAppID:"+err.Error()))
			return
		}
		//fmt.Println("appInfo",appInfo)
		if appInfo.AppID != "" {
			util.ResponseError(c, 500, errors.New("app_id 已经存在！"))
			return
		}
	}

	totalQueryDailyInt, _ := strconv.ParseInt(totalQueryDaily, 10, 64)
	qpsInt, _ := strconv.ParseInt(qps, 10, 64)
	groupIDInt, _ := strconv.ParseInt(groupID, 10, 64)
	timeoutInt, _ := strconv.ParseInt(timeout, 10, 64)

	app.AppID = appID
	app.Name = name
	app.Secret = secret
	app.OpenAPI = strings.Join(strings.Split(openAPI, "\n"), ",")
	app.WhiteIps = whiteIps
	app.CityIDs = cityIds
	app.TotalQueryDaily = totalQueryDailyInt
	app.QPS = qpsInt
	app.GroupID = groupIDInt
	app.Timeout = timeoutInt
	app.Method = method
	if err := app.Save(tx); err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("GatewayModuleBase.Save:"+err.Error()))
		return
	}
	tx.Commit()
	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	util.ResponseSuccess(c, "")
	return
}

//EditAPP 修改app action
func (admin *Admin) EditAPP(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}

	appID := c.Query("app_id")
	tx := config.DB
	app := &entity.GatewayAPP{}

	appInfo, err := app.FindByAppID(tx, appID)
	if err != nil {
		util.ResponseError(c, 500, errors.New("FindByAppID:"+err.Error()))
		return
	}
	if appInfo.AppID == "" {
		util.ResponseError(c, 500, errors.New("app_id 不存在！"))
		return
	}
	app = appInfo
	app.OpenAPI = strings.Join(strings.Split(app.OpenAPI, ","), "\r\n")
	t, err := admin.getTemplateByURL("/admin/edit_app")
	if err != nil {
		util.ResponseError(c, 500, err)
		return
	}
	err2 := admin.executeTemplate(t, c.Writer, app, "/admin/app_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
		return
	}
	return
}

//APPDetail 租户详情action
func (admin *Admin) APPDetail(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}

	appID := c.Query("app_id")
	tx := config.DB
	app := &entity.GatewayAPP{}
	appInfo, err := app.FindByAppID(tx, appID)
	if err != nil {
		util.ResponseError(c, 500, errors.New("FindByAppID:"+err.Error()))
		return
	}
	if appInfo.AppID == "" {
		util.ResponseError(c, 500, errors.New("app_id 不存在"))
		return
	}
	app = appInfo

	detailInfo := &APPDetailInfo{}
	detailInfo.APPInfo = app

	counter := resource.FlowCounters.GetAPPCounter(appID)
	for i := 0; i <= time.Now().In(config.TimeLocation).Hour(); i++ {
		todaydate := time.Now().In(config.TimeLocation).Format("20060102")
		todayhour := todaydate + fmt.Sprint(i)
		if i < 10 {
			todayhour = todaydate + "0" + fmt.Sprint(i)
		}
		hourstat, err := counter.GetHourCount(todayhour)
		if err == nil {
			detailInfo.DailyHourStat += fmt.Sprint(hourstat) + ","
			second := int64(3600)
			if i == time.Now().In(config.TimeLocation).Hour() {
				hourTime, _ := time.ParseInLocation(constant.TimeFormat,
					time.Now().In(config.TimeLocation).Format("2006-01-02 15:00:00"),
					config.TimeLocation)
				second = time.Now().Unix() - hourTime.Unix()
			}
			detailInfo.DailyHourAvg += "[" + fmt.Sprint(i) + ", " + fmt.Sprintf("%.4f", float64(hourstat)/float64(second)) + "],"
			if hourstat > detailInfo.DailyStatMax {
				detailInfo.DailyStatMax = hourstat
			}
		}
		detailInfo.DailyHourStat += "0,"
		detailInfo.DailyHourAvg += "[" + fmt.Sprint(i) + ", " + "0" + "],"
	}

	if len(detailInfo.DailyHourStat) > 0 {
		detailInfo.DailyStatMax = int64(float64(detailInfo.DailyStatMax) * 1.2)
		detailInfo.DailyHourStat = detailInfo.DailyHourStat[0 : len(detailInfo.DailyHourStat)-1]
	}
	//fmt.Println(detailInfo.DailyHourStat)
	//templateName := "app_detail.html"
	//t := template.New(templateName)                       //创建一个模板
	//t, err := t.ParseFiles("./tmpl/green/" + templateName) //解析模板文件
	t, err := admin.getTemplateByURL("/admin/app_detail")
	if err != nil {
		util.ResponseError(c, 500, err)
		return
	}
	err2 := admin.executeTemplate(t, c.Writer, detailInfo, "/admin/app_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
	}
	return
}

//DelAPP 删除租户action
func (admin *Admin) DelAPP(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}

	tx := config.DB
	tx = tx.Begin() //这里一定要赋值
	appID := c.Query("app_id")
	app := &entity.GatewayAPP{}
	appInfo, err := app.FindByAppID(tx, appID)
	if err != nil {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("FindByAppID:"+err.Error()))
		return
	}
	if appInfo.AppID == "" {
		tx.Rollback()
		util.ResponseError(c, 500, errors.New("app_id 不存在！"))
		return
	}
	app = appInfo
	app.Del(tx)
	tx.Commit()
	if err := admin.ClusterReloadModule(); err != nil {
		util.ResponseError(c, 500, errors.New("ClusterReloadModule:"+err.Error()))
		return
	}
	c.Redirect(302, "/admin/app_list")
}

//EditService 编辑服务action
func (admin *Admin) EditService(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	moduleName := c.Query("module_name")
	moduleConf, errm := admin.getDBModuleConf()
	if errm != nil {
		util.ResponseError(c, 500, errm)
	}
	if moduleConf == nil {
		util.ResponseError(c, 500, errors.New("获取模块配置错误"))
		return
	}
	var module *running.GatewayModule
	for _, tmpModule := range moduleConf.Module {
		if tmpModule.Base.Name == moduleName {
			module = tmpModule
		}
	}
	if module == nil {
		util.ResponseError(c, 500, errors.New("module_name not found"))
		return
	}
	ipList := strings.Split(module.LoadBalance.IPList, ",")
	weightList := strings.Split(module.LoadBalance.WeightList, ",")

	detailInfo := &ServiceDetailInfo{}
	detailInfo.RoutePrefix = config.BaseConf.Http.RoutePrefix
	detailInfo.Module = module
	detailInfo.ModuleIPList = ipList
	detailInfo.WeightList = weightList
	detailInfo.ActiveIPList = service.SysConfMgr.GetActiveIPList(moduleName)
	detailInfo.ForbidIPList = service.SysConfMgr.GetForbidIPList(moduleName)
	detailInfo.AvaliableIPList = service.SysConfMgr.GetAvailableIPList(moduleName)

	matchRules := []string{}
	matchURLRules := []string{}
	matchRules = append(matchRules, module.MatchRule.Rule)
	matchURLRules = strings.Split(module.MatchRule.URLRewrite, ",")
	detailInfo.URLRewrite = strings.Join(matchURLRules, "\r")
	detailInfo.MatchRule = strings.Join(matchRules, ",")
	ipWeigths := []string{}
	for index, item := range ipList {
		if len(weightList)-1 >= index {
			item = item + " " + weightList[index]
		} else {
			item = item + " " + "100"
		}
		ipWeigths = append(ipWeigths, item)
	}
	detailInfo.IPWeightList = strings.Join(ipWeigths, "\r")
	if strings.Contains(module.AccessControl.AuthType, "passport") {
		detailInfo.Passport = "1"
	}
	moduleFilters := []string{}
	//for _, item := range module.DataFilter {
	//	if item.Type != "" {
	//		moduleFilters = append(moduleFilters, "type="+item.Type+" url="+item.Url+" rule="+item.Rule+" rule_ext="+item.RuleExt)
	//	}
	//}
	detailInfo.FilterRule = strings.Join(moduleFilters, "\r")

	t := template.New("")
	if module.Base.LoadType == "tcp" {
		tg, err := admin.getTemplateByURL("/admin/edit_tcp")
		if err != nil {
			util.ResponseError(c, 500, err)
			return
		}
		t = tg
	} else {
		tg, err := admin.getTemplateByURL("/admin/edit_http")
		if err != nil {
			util.ResponseError(c, 500, err)
			return
		}
		t = tg
	}
	err2 := admin.executeTemplate(t, c.Writer, detailInfo, "/admin/service_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
		return
	}
	return
}

//AddHTTP 添加http服务action
func (admin *Admin) AddHTTP(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}

	t, err := admin.getTemplateByURL("/admin/add_http")
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	detailInfo := &ServiceDetailInfo{
		RoutePrefix: config.BaseConf.Http.RoutePrefix,
		Module: &running.GatewayModule{
			Base:          &entity.GatewayModuleBase{},
			MatchRule:     &entity.GatewayMatchRule{},
			LoadBalance:   &entity.GatewayLoadBalance{},
			AccessControl: &entity.GatewayAccessControl{},
		}}
	err = admin.executeTemplate(t, c.Writer, detailInfo, "/admin/service_list")
	if err != nil {
		util.ResponseError(c, 500, err)
	}
	return
}

//AddAPP 添加app的action
func (admin *Admin) AddAPP(c *gin.Context) {
	if err := admin.LoginAuth(c); err != nil {
		c.Redirect(302, "/admin/login")
		return
	}
	t, err := admin.getTemplateByURL("/admin/add_app")
	if err != nil {
		util.ResponseError(c, 500, err)
	}

	app := &entity.GatewayAPP{}
	app.Secret = fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
	err2 := admin.executeTemplate(t, c.Writer, app, "/admin/app_list")
	if err2 != nil {
		util.ResponseError(c, 500, err2)
	}
	return
}

func (admin *Admin) parseTemplate(tempFile string) (*template.Template, error) {
	return template.ParseFiles(
		tempFile,
		"./tmpl/green/layout/layout.html",
		"./tmpl/green/layout/footer.html",
		"./tmpl/green/layout/head.html",
		"./tmpl/green/layout/header.html",
		"./tmpl/green/layout/sidebar.html")
}

func (admin *Admin) executeTemplate(t *template.Template, wr io.Writer, data interface{}, activeURL string) error {
	m := make(map[string]interface{})
	m["data"] = data
	m["active_uri"] = activeURL
	return t.Execute(wr, m)
}

func (admin *Admin) getTemplateByURL(action string) (*template.Template, error) {
	switch action {
	case "/admin/add_http":
		return admin.parseTemplate("./tmpl/green/add_http.html")
	case "/admin/edit_http":
		return admin.parseTemplate("./tmpl/green/add_http.html")
	case "/admin/add_tcp":
		return admin.parseTemplate("./tmpl/green/add_tcp.html")
	case "/admin/edit_tcp":
		return admin.parseTemplate("./tmpl/green/add_tcp.html")
	case "/admin/service_detail":
		return admin.parseTemplate("./tmpl/green/service_detail.html")
	case "/admin/service_list":
		return admin.parseTemplate("./tmpl/green/service_list.html")
	case "/admin/app_list":
		return admin.parseTemplate("./tmpl/green/app_list.html")
	case "/admin/add_app":
		return admin.parseTemplate("./tmpl/green/add_app.html")
	case "/admin/edit_app":
		return admin.parseTemplate("./tmpl/green/add_app.html")
	case "/admin/app_detail":
		return admin.parseTemplate("./tmpl/green/app_detail.html")
	}
	return nil, errors.New("not found match action")
}
