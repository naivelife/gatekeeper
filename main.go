package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"gatekeeper/config"
	"gatekeeper/core/resource"
	"gatekeeper/core/service"
	"gatekeeper/server"
)

func main() {
	config.Conf = flag.String("config", "./etc/", "input config file like ./conf/dev/")
	flag.Parse()

	// 配置文件不以/结尾，则手动拼接
	configPath := *config.Conf
	if configPath[len(configPath)-1] != '/' {
		configPath = configPath + "/"
	}
	*config.Conf = configPath

	// 初始化配置文件
	err := config.Init(*config.Conf)
	if err != nil {
		panic(err)
	}

	// 初始化resource
	resource.FlowCounters = resource.NewFlowCounterManager()
	resource.Limiters = resource.NewLimiterManager()

	// 运行时配置初始化
	service.SysConfMgr = service.NewSysConfigManage()
	service.SysConfMgr.InitConfig()
	service.SysConfMgr.MonitorConfig()

	// 启动http服务器
	server.HTTPServerRun()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 资源销毁
	config.Destroy()
	server.HTTPServerStop()
	signal.Stop(quit)
}
