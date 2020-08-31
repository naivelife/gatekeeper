package middleware

import (
	"bytes"
	"context"
	"gatekeeper/config"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"time"
)

func RequestTraceLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		RequestInLog(c)
		defer RequestOutLog(c)
		c.Next()
	}
}

func RequestInLog(c *gin.Context) {
	bodyBytes, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {

	}
	c.Set(MiddlewareRequestBodyKey, bodyBytes)
	c.Set("startExecTime", time.Now())
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "request_url", c.Request.URL.Path))
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
}

func RequestOutLog(c *gin.Context) {
	endExecTime := time.Now()
	st, ok := c.Get("startExecTime")
	if !ok {
		config.AccessLog.Info("[uri:%s] [method:%s] [args:%+v] [from:%s] [key:%s]",
			c.Request.RequestURI,
			c.Request.Method,
			c.Request.PostForm,
			c.ClientIP(),
			"st")
	}
	startExecTime, ok := st.(time.Time)
	if !ok {
		config.AccessLog.Info("[uri:%s] [method:%s] [args:%+v] [from:%s] [key:%s] [proc_time:%f]",
			c.Request.RequestURI,
			c.Request.Method,
			c.Request.PostForm,
			c.ClientIP(),
			"st",
			st)
	}
	config.AccessLog.Info("[uri:%s] [method:%s] [args:%+v] [from:%s] [proc_time:%f]",
		c.Request.RequestURI,
		c.Request.Method,
		c.Request.PostForm,
		c.ClientIP(),
		endExecTime.Sub(startExecTime).Seconds())
}
