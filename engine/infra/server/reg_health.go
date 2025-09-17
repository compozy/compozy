package server

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Health endpoint
//
//	@Summary      Get server health
//	@Description  Returns overall service health, readiness and components status
//	@Tags         health,diagnostics
//	@Accept       json
//	@Produce      json
//	@Success      200 {object} map[string]interface{} "Service is healthy"
//	@Failure      503 {object} map[string]interface{} "Service is not ready"
//	@Router       /api/v0/health [get]
func CreateHealthHandler(server *Server, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ready, healthStatus, scheduleStatus, memoryHealth, temporalReady, workerReady, mcpReady := gatherSystemStatus(
			ctx,
			server,
		)
		response := buildHealthResponse(healthStatus, version, ready, scheduleStatus, memoryHealth)
		response["temporal"] = gin.H{"ready": temporalReady}
		response["worker"] = gin.H{"ready": workerReady}
		response["mcp_proxy"] = gin.H{"ready": mcpReady}
		statusCode := determineHealthStatusCode(ready)
		c.JSON(statusCode, gin.H{
			"data":    response,
			"message": "Success",
		})
	}
}

func gatherSystemStatus(ctx context.Context, server *Server) (bool, string, gin.H, gin.H, bool, bool, bool) {
	ready, healthStatus, scheduleStatus := buildScheduleStatus(ctx, server)
	temporalReady := false
	workerReady := false
	mcpReady := false
	if server != nil {
		temporalReady = server.isTemporalReady()
		workerReady = server.isWorkerReady()
		mcpReady = server.isMCPReady()
		if !server.isFullyReady() {
			ready = false
			healthStatus = statusNotReady
		}
	}
	memoryHealth := buildMemoryHealth(ctx, &ready, &healthStatus)
	return ready, healthStatus, scheduleStatus, memoryHealth, temporalReady, workerReady, mcpReady
}

func buildScheduleStatus(ctx context.Context, server *Server) (bool, string, gin.H) {
	ready := true
	healthStatus := "healthy"
	scheduleStatus := gin.H{
		"reconciled": true,
		"status":     statusReady,
	}
	if server != nil {
		completed, lastAttempt, attemptCount, lastError := server.GetReconciliationStatus()
		scheduleStatus = gin.H{
			"reconciled":    completed,
			"last_attempt":  lastAttempt,
			"attempt_count": attemptCount,
		}
		switch {
		case completed:
			scheduleStatus["status"] = statusReady
		case lastError != nil:
			logger.FromContext(ctx).Warn("Readiness probe check failed due to reconciliation error", "error", lastError)
			scheduleStatus["status"] = "retrying"
			scheduleStatus["last_error"] = "reconciliation failed"
			scheduleStatus["last_error_message"] = lastError.Error()
			ready = false
			healthStatus = statusNotReady
		default:
			scheduleStatus["status"] = "initializing"
			ready = false
			healthStatus = statusNotReady
		}
	}
	return ready, healthStatus, scheduleStatus
}

func buildMemoryHealth(ctx context.Context, ready *bool, healthStatus *string) gin.H {
	var memoryHealth gin.H
	if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
		memoryHealth = memory.GetMemoryHealthForMainEndpoint(ctx, globalHealthService)
		if memoryHealthy, exists := memoryHealth["healthy"].(bool); exists && !memoryHealthy {
			*ready = false
			*healthStatus = "degraded"
		}
	}
	return memoryHealth
}

func buildHealthResponse(healthStatus, version string, ready bool, scheduleStatus, memoryHealth gin.H) gin.H {
	response := gin.H{
		"status":    healthStatus,
		"version":   version,
		"ready":     ready,
		"schedules": scheduleStatus,
	}
	if memoryHealth != nil {
		response["memory"] = memoryHealth
	}
	return response
}

func determineHealthStatusCode(ready bool) int {
	if !ready {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}
