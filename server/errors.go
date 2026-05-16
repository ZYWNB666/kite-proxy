package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite-proxy/pkg/api"
)

func respondAPIError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, api.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "API key 无效或已过期，请重新配置",
			"code":  "unauthorized",
		})
	case errors.Is(err, api.ErrProxyForbidden):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "当前 API key 没有访问该集群/命名空间的代理权限",
			"code":  "proxy_forbidden",
		})
	default:
		c.JSON(http.StatusBadGateway, gin.H{
			"error": fmt.Sprintf("无法连接到 Kite 服务端：%v", err),
			"code":  "kite_unreachable",
		})
	}
}
