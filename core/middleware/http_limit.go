package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"gatekeeper/core/resource"
	"gatekeeper/core/service"
	"gatekeeper/util"
)

// HTTPLimit http限流中间件
func HTTPLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取上游服务
		gws, ok := c.MustGet(MiddlewareServiceKey).(*service.GateWayService)
		if !ok {
			util.ResponseError(c, http.StatusBadRequest, errors.New("gateway_service not valid"))
			return
		}

		// 入口流量统计
		currentModule := gws.CurrentModule()
		counter := resource.FlowCounters.GetRequestCounter(currentModule.Base.Name)
		counter.Increase(c.Request.Context(), c.Request.RemoteAddr)

		// 客户端ip限流
		remoteIP := util.Substr(c.Request.RemoteAddr, 0, int64(strings.Index(c.Request.RemoteAddr, ":")))
		if currentModule.AccessControl.ClientFlowLimit > 0 {
			limiter := resource.Limiters.GetLimiter(currentModule.Base.Name+"_"+remoteIP, currentModule.AccessControl.ClientFlowLimit)
			if limiter.Allow() == false {
				errMsg := fmt.Sprintf("moduleName:%s remoteIP：%s, QPS limit : %d, %d", currentModule.Base.Name, remoteIP, int64(limiter.Limit()), limiter.Burst())
				util.ResponseError(c, http.StatusBadRequest, errors.New(errMsg))
			}
		}
		c.Next()
	}
}
