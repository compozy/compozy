package schrouter

import "github.com/gin-gonic/gin"

// Register registers all schedule-related routes
func Register(apiBase *gin.RouterGroup) {
	// Schedule routes
	schedulesGroup := apiBase.Group("/schedules")
	{
		// GET /api/v0/schedules
		// List all scheduled workflows
		schedulesGroup.GET("", listSchedules)

		// GET /api/v0/schedules/:workflow_id
		// Get schedule details for a specific workflow
		schedulesGroup.GET("/:workflow_id", getSchedule)

		// PATCH /api/v0/schedules/:workflow_id
		// Update schedule (enable/disable)
		schedulesGroup.PATCH("/:workflow_id", updateSchedule)

		// DELETE /api/v0/schedules/:workflow_id
		// Remove schedule from Temporal
		schedulesGroup.DELETE("/:workflow_id", deleteSchedule)
	}
}
