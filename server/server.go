package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"gatekeeper/config"
	"gatekeeper/controller"
	"gatekeeper/core/middleware"
)

var (
	HTTPSrvHandler *http.Server
)

func HTTPServerRun() {
	gin.SetMode(config.BaseConf.DebugModel)
	r := initRouter()
	HTTPSrvHandler = &http.Server{
		Addr:           config.BaseConf.Http.Addr,
		Handler:        r,
		ReadTimeout:    time.Duration(config.BaseConf.Http.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.BaseConf.Http.WriteTimeout) * time.Second,
		MaxHeaderBytes: 1 << uint(config.BaseConf.Http.MaxHeaderBytes),
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
			}
		}()
		if err := HTTPSrvHandler.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		}
	}()
}

func HTTPServerStop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := HTTPSrvHandler.Shutdown(ctx); err != nil {
		log.Fatalf(" [ERROR] HttpServer err:%v\n", err)
	}
	log.Printf(" [INFO] HttpServer stopped\n")
}

func initRouter() *gin.Engine {
	router := gin.New()
	router.Use(middleware.Recovery())

	admin := router.Group("/admin")
	admin.Use(middleware.RequestTraceLog())
	{
		controller.AdminRegister(admin)
	}

	router.Static("/assets", "./tmpl/green/assets")

	gateway := controller.Gateway{}
	router.GET("/ping", gateway.Ping)

	csr := router.Group("/")
	csr.Use(middleware.ClusterAuth())
	csr.GET("/reload", gateway.Reload)

	gw := router.Group(config.BaseConf.Http.RoutePrefix)
	gw.Use(
		middleware.RequestTraceLog(),
		middleware.MatchRule(),
		middleware.AccessControl(),
		middleware.HTTPLimit(),
		middleware.LoadBalance())
	{
		gw.GET("/*action", gateway.Index)
		gw.POST("/*action", gateway.Index)
		gw.DELETE("/*action", gateway.Index)
		gw.OPTIONS("/*action", gateway.Index)
	}
	return router
}
