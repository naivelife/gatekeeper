package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gatekeeper/core/service"
	"gatekeeper/util"
)

//中间件常量
const (
	MiddlewareServiceKey     = "gateway_service"
	MiddlewareRequestBodyKey = "request_body"
)

//MatchRule 匹配模块中间件
func MatchRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		gws := service.NewGateWayService(c.Writer, c.Request)
		if err := gws.MatchRule(); err != nil {
			util.ResponseError(c, http.StatusBadRequest, err)
			return
		}
		c.Set(MiddlewareServiceKey, gws)
		c.Next()
	}
}
