package schedulerouter

import "github.com/gin-gonic/gin"

// Register registers all schedule-related routes
func Register(apiBase *gin.RouterGroup) {
	schedulesGroup := apiBase.Group("/schedules")
	{
		schedulesGroup.GET("", listSchedules)

		schedulesGroup.GET("/:workflow_id", getSchedule)

		schedulesGroup.PATCH("/:workflow_id", updateSchedule)

		// NOTE: DELETE removes the schedule from Temporal for the given workflow.
		schedulesGroup.DELETE("/:workflow_id", deleteSchedule)
	}
}
