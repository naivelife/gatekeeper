package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"gatekeeper/core/service"
	"gatekeeper/util"
)

func AccessControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		gws, ok := c.MustGet(MiddlewareServiceKey).(*service.GateWayService)
		if !ok {
			util.ResponseError(c, http.StatusBadRequest, errors.New("gateway_service not valid"))
			return
		}
		if err := gws.AccessControl(); err != nil {
			util.ResponseError(c, http.StatusUnauthorized, err)
			return
		}
		c.Set(MiddlewareServiceKey, gws)
		c.Next()
	}
}
