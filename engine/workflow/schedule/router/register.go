package schedulerouter

import "github.com/gin-gonic/gin"

// Register registers all schedule-related routes
func Register(apiBase *gin.RouterGroup) {
	// Schedule routes
	schedulesGroup := apiBase.Group("/schedules")
	{
		// GET /schedules
		// List all scheduled workflows
		schedulesGroup.GET("", listSchedules)

		// GET /schedules/:workflow_id
		// Get schedule details for a specific workflow
		schedulesGroup.GET("/:workflow_id", getSchedule)

		// PATCH /schedules/:workflow_id
		// Update schedule (enable/disable)
		schedulesGroup.PATCH("/:workflow_id", updateSchedule)

		// DELETE /schedules/:workflow_id
		// Remove schedule from Temporal
		schedulesGroup.DELETE("/:workflow_id", deleteSchedule)
	}
}
