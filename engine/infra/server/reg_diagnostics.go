package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

func setupDiagnosticEndpoints(router *gin.Engine, version, prefixURL string, server *Server) {
	router.GET("/", createRootHandler(version, prefixURL))
	router.GET(prefixURL, createRootHandler(version, prefixURL))
	router.GET(prefixURL+"/health", CreateHealthHandler(server, version))
	router.GET("/mcp-proxy/health", func(c *gin.Context) {
		ready := false
		if server != nil {
			ready = server.isMCPReady()
		}
		code := determineHealthStatusCode(ready)
		status := "ok"
		if !ready {
			status = statusNotReady
		}
		c.JSON(code, gin.H{
			"data":    gin.H{"status": status},
			"message": "Success",
		})
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"data":    gin.H{"status": "ok"},
			"message": "Success",
		})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx := c.Request.Context()
		ready, healthStatus, scheduleStatus, _, temporalReady, workerReady, mcpReady := gatherSystemStatus(ctx, server)
		statusCode := determineHealthStatusCode(ready)
		c.JSON(statusCode, gin.H{
			"data": gin.H{
				"status":    healthStatus,
				"version":   version,
				"ready":     ready,
				"temporal":  gin.H{"ready": temporalReady},
				"worker":    gin.H{"running": workerReady},
				"mcp_proxy": gin.H{"ready": mcpReady},
				"schedules": scheduleStatus,
			},
			"message": "Success",
		})
	})
}

func createRootHandler(version, prefixURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := sanitizeHost(c.Request.Host)
		scheme := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Proto")))
		if scheme == "" {
			if c.Request.TLS != nil {
				scheme = schemeHTTPS
			} else {
				scheme = schemeHTTP
			}
		} else {
			if comma := strings.IndexByte(scheme, ','); comma >= 0 {
				scheme = scheme[:comma]
			}
		}
		scheme = normalizeScheme(scheme)
		if host == "" {
			host = sanitizeHost(c.Request.Header.Get("X-Forwarded-Host"))
		}
		if host == "" {
			host = sanitizeHost(c.Request.URL.Host)
		}
		if host == "" {
			host = "localhost"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, host)
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"name":        "Compozy API",
				"version":     version,
				"description": "Next-level Agentic Orchestration Platform, tasks, and tools",
				"endpoints": gin.H{
					"health":       fmt.Sprintf("%s%s/health", baseURL, prefixURL),
					"api":          fmt.Sprintf("%s%s", baseURL, prefixURL),
					"docs":         fmt.Sprintf("%s/docs/index.html", baseURL),
					"swagger":      fmt.Sprintf("%s/swagger/index.html", baseURL),
					"openapi_json": fmt.Sprintf("%s/openapi.json", baseURL),
				},
			},
			"message": "Success",
		})
	}
}

func normalizeScheme(raw string) string {
	switch raw {
	case schemeHTTPS:
		return schemeHTTPS
	default:
		return schemeHTTP
	}
}

func sanitizeHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if comma := strings.IndexByte(raw, ','); comma >= 0 {
		raw = raw[:comma]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}
