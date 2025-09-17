package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func setupDiagnosticEndpoints(router *gin.Engine, version, prefixURL string, server *Server) {
	router.GET("/", createRootHandler(version, prefixURL))
	router.GET("/health", CreateHealthHandler(server, version))
	router.GET("/mcp/health", func(c *gin.Context) {
		ready := false
		if server != nil {
			ready = server.isMCPReady()
		}
		code := determineHealthStatusCode(ready)
		status := "ok"
		if !ready {
			status = statusNotReady
		}
		c.JSON(code, gin.H{"status": status})
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
		host := c.Request.Host
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, host)
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"name":        "Compozy API",
				"version":     version,
				"description": "Next-level Agentic Orchestration Platform, tasks, and tools",
				"endpoints": gin.H{
					"health":  fmt.Sprintf("%s/health", baseURL),
					"api":     fmt.Sprintf("%s%s", baseURL, prefixURL),
					"swagger": fmt.Sprintf("%s/swagger/index.html", baseURL),
					"docs":    fmt.Sprintf("%s/docs/index.html", baseURL),
					"openapi": fmt.Sprintf("%s/swagger/doc.json", baseURL),
				},
			},
			"message": "Success",
		})
	}
}
