package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"gatekeeper/config"
	"gatekeeper/util"
)

//ClusterAuth 集群验证中间件
func ClusterAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterList := config.BaseConf.Cluster.ClusterList
		matchFlag := false
		ipList := strings.Split(clusterList, ",")
		ipList = append(ipList, "127.0.0.1")
		for _, host := range ipList {
			if c.ClientIP() == host {
				matchFlag = true
			}
		}
		if !matchFlag {
			util.ResponseError(c, http.StatusBadRequest, errors.New("ClusterAuth error"))
			return
		}
		c.Next()
	}
}
