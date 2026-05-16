package server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite-proxy/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// HandleProxy is the main reverse-proxy handler.
//
// URL pattern: /proxy/:cluster/*path
//
// The handler:
//  1. Extracts the cluster name from the URL.
//  2. Loads (or reuses) the in-memory rest.Config for that cluster.
//  3. Rewrites the request and forwards it to the real K8s API server.
//  4. Returns the K8s response verbatim.
func HandleProxy(c *gin.Context) {
	clusterName := c.Param("cluster")
	if clusterName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster name is required"})
		return
	}

	restCfg, err := globalCache.Get(clusterName)
	if err != nil {
		if handled := writeProxyUpstreamError(c, err); handled {
			return
		}
		klog.Warningf("proxy: failed to get kubeconfig for cluster %q: %v", clusterName, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("cannot connect to cluster %q: %v", clusterName, err)})
		return
	}

	// Build an authenticating round-tripper from the rest.Config.
	rt, err := rest.TransportFor(restCfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build transport: " + err.Error()})
		return
	}

	// Target is the K8s API server.
	target, err := url.Parse(restCfg.Host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid K8s API server URL: " + err.Error()})
		return
	}

	// Strip the /proxy/:cluster prefix from the path that we forward.
	proxyPath := c.Param("path")
	if proxyPath == "" {
		proxyPath = "/"
	}

	if namespace := extractNamespaceFromProxyPath(proxyPath); namespace != "" {
		allowed, err := globalCache.IsNamespaceAllowed(clusterName, namespace)
		if err != nil {
			if handled := writeProxyUpstreamError(c, err); handled {
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to validate namespace access: %v", err)})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("namespace %s is not allowed for proxy", namespace),
				"code":  "namespace_forbidden",
			})
			return
		}
	}

	proxy := &httputil.ReverseProxy{
		Transport: rt,
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = singleJoiningSlash(target.Path, proxyPath)

			// Preserve query string from the original request.
			req.URL.RawQuery = c.Request.URL.RawQuery

			// Remove the host header so the upstream can set it properly.
			req.Host = target.Host

			// Remove Authorization header from the client – the round-tripper
			// adds the correct credentials from the in-memory kubeconfig.
			req.Header.Del("Authorization")
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			klog.Warningf("proxy error for cluster %q: %v", clusterName, err)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, `{"error":%q}`, err.Error())
		},
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func singleJoiningSlash(a, b string) string {
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	}
	return a + b
}

func extractNamespaceFromProxyPath(proxyPath string) string {
	cleaned := path.Clean("/" + proxyPath)
	segments := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == "namespaces" && segments[i+1] != "" && segments[i+1] != "." {
			return segments[i+1]
		}
	}
	return ""
}

func writeProxyUpstreamError(c *gin.Context, err error) bool {
	switch {
	case api.ErrorStatus(err) == http.StatusUnauthorized:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key 无效或已过期，请重新配置", "code": "unauthorized"})
		return true
	case api.ErrorStatus(err) == http.StatusForbidden && api.ErrorCode(err) == "proxy_forbidden":
		c.JSON(http.StatusForbidden, gin.H{"error": "当前 API key 没有访问该集群/命名空间的代理权限", "code": "proxy_forbidden"})
		return true
	default:
		return false
	}
}
