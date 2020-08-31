package controller

import (
	"github.com/gin-gonic/gin"

	"gatekeeper/core/service"
	"gatekeeper/util"
)

//Gateway struct
type Gateway struct {
}

//Index /index
func (g *Gateway) Index(c *gin.Context) {
	util.ResponseSuccess(c, string("gateway index"))
	return
}

//Ping /ping
func (g *Gateway) Ping(c *gin.Context) {
	util.ResponseSuccess(c, string("gateway pong"))
	return
}

//Reload /reload
func (g *Gateway) Reload(c *gin.Context) {
	service.SysConfMgr.ReloadConfig()
	util.ResponseSuccess(c, string("gateway config loaded"))
	return
}
