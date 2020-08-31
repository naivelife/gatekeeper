package middleware

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"

	"gatekeeper/core/service"
	"gatekeeper/util"
)

func LoadBalance() gin.HandlerFunc {
	return func(c *gin.Context) {
		gws, ok := c.MustGet(MiddlewareServiceKey).(*service.GateWayService)
		if !ok {
			util.ResponseError(c, http.StatusBadRequest, errors.New("gateway_service not valid"))
			return
		}
		proxy, err := gws.LoadBalance()
		if err != nil {
			util.ResponseError(c, http.StatusProxyAuthRequired, err)
			return
		}
		requestBody, ok := c.MustGet(MiddlewareRequestBodyKey).([]byte)
		if !ok {
			util.ResponseError(c, http.StatusBadRequest, errors.New("request_body not valid"))
			return
		}
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
