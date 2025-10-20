package memory

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler provides HTTP endpoints for memory system health
type HealthHandler struct {
	healthService *HealthService
}

// NewHealthHandler creates a new memory health handler
func NewHealthHandler(healthService *HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

// GetMemorySystemHealth returns the overall memory system health
// @Summary Get memory system health
// @Description Returns comprehensive health information for the memory system
// @Tags memory,health
// @Accept json
// @Produce json
// @Success 200 {object} SystemHealth "Memory system is healthy"
// @Success 503 {object} SystemHealth "Memory system is unhealthy"
// @Router /memory/health [get]
func (mhh *HealthHandler) GetMemorySystemHealth(c *gin.Context) {
	ctx := c.Request.Context()
	health := mhh.healthService.GetOverallHealth(ctx)
	statusCode := http.StatusOK
	if !health.Healthy {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, health)
}

// GetMemoryInstanceHealth returns the health of a specific memory instance
// @Summary Get memory instance health
// @Description Returns health information for a specific memory instance
// @Tags memory,health
// @Accept json
// @Produce json
// @Param memory_id path string true "Memory Instance ID"
// @Success 200 {object} InstanceHealth "Memory instance health retrieved"
// @Success 404 {object} gin.H "Memory instance not found"
// @Router /memory/health/{memory_id} [get]
func (mhh *HealthHandler) GetMemoryInstanceHealth(c *gin.Context) {
	memoryID := c.Param("memory_id")
	if memoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "memory_id parameter is required",
		})
		return
	}
	health, exists := mhh.healthService.GetInstanceHealth(memoryID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "memory instance not found",
			"memory_id": memoryID,
		})
		return
	}
	statusCode := http.StatusOK
	if !health.Healthy {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, health)
}

// RegisterMemoryHealthRoutes registers memory health routes with the given router
func RegisterMemoryHealthRoutes(router gin.IRouter, healthService *HealthService) {
	if healthService == nil {
		return // Don't register routes if no health service
	}
	handler := NewHealthHandler(healthService)
	memoryGroup := router.Group("/memory")
	{
		memoryGroup.GET("/health", handler.GetMemorySystemHealth)
		memoryGroup.GET("/health/:memory_id", handler.GetMemoryInstanceHealth)
	}
}

// GetMemoryHealthForMainEndpoint returns memory health data for inclusion in the main /health endpoint
func GetMemoryHealthForMainEndpoint(ctx context.Context, healthService *HealthService) gin.H {
	if healthService == nil {
		return gin.H{
			"healthy": false,
			"error":   "memory health service not available",
		}
	}
	health := healthService.GetOverallHealth(ctx)
	// Return a simplified version for the main health endpoint
	return gin.H{
		"healthy":             health.Healthy,
		"total_instances":     health.TotalInstances,
		"healthy_instances":   health.HealthyInstances,
		"unhealthy_instances": health.UnhealthyInstances,
		"last_checked":        health.LastChecked,
		"system_errors":       health.SystemErrors,
	}
}
