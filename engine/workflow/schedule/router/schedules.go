package schrouter

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/gin-gonic/gin"
)

// convertToScheduleInfoResponse converts schedule.Info to ScheduleInfoResponse
func convertToScheduleInfoResponse(info *schedule.Info) ScheduleInfoResponse {
	resp := ScheduleInfoResponse{
		WorkflowID:    info.WorkflowID,
		ScheduleID:    info.ScheduleID,
		Cron:          info.Cron,
		Timezone:      info.Timezone,
		Enabled:       info.Enabled,
		IsOverride:    info.IsOverride,
		YAMLConfig:    info.YAMLConfig,
		LastRunStatus: info.LastRunStatus,
	}
	// Handle optional time fields
	if !info.NextRunTime.IsZero() {
		resp.NextRunTime = &info.NextRunTime
	}
	resp.LastRunTime = info.LastRunTime
	return resp
}

// getScheduleManager retrieves the schedule manager from app state
func getScheduleManager(c *gin.Context) (schedule.Manager, bool) {
	appState := router.GetAppState(c)
	if appState == nil {
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"application state not available",
			errors.New("app state not found in context"),
		))
		return nil, false
	}
	scheduleManager, ok := appState.Extensions[appstate.ScheduleManagerKey].(schedule.Manager)
	if !ok || scheduleManager == nil {
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"schedule manager not initialized",
			errors.New("schedule manager not found in app state"),
		))
		return nil, false
	}
	return scheduleManager, true
}

// listSchedules retrieves all scheduled workflows
//
//	@Summary		List all scheduled workflows
//	@Description	Retrieve a list of all scheduled workflows with their current status and override information
//	@Tags			schedules
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=ScheduleListResponse}		"Schedules retrieved successfully"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/schedules [get]
//	@Security		BearerAuth
func listSchedules(c *gin.Context) {
	scheduleManager, ok := getScheduleManager(c)
	if !ok {
		return // Error already handled by getScheduleManager
	}
	schedules, err := scheduleManager.ListSchedules(c.Request.Context())
	if err != nil {
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"failed to list schedules",
			err,
		))
		return
	}
	// Convert to response format
	response := ScheduleListResponse{
		Schedules: make([]ScheduleInfoResponse, 0, len(schedules)),
		Total:     len(schedules),
	}
	for _, info := range schedules {
		response.Schedules = append(response.Schedules, convertToScheduleInfoResponse(info))
	}
	router.RespondOK(c, "schedules retrieved", response)
}

// getSchedule retrieves a specific scheduled workflow
//
//	@Summary		Get schedule by workflow ID
//	@Description	Retrieve detailed information about a specific scheduled workflow including YAML configuration
//	@Tags			schedules
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"	example("daily-report")
//	@Success		200			{object}	router.Response{data=ScheduleInfoResponse}		"Schedule retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Schedule not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/schedules/{workflow_id} [get]
//	@Security		BearerAuth
func getSchedule(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	scheduleManager, ok := getScheduleManager(c)
	if !ok {
		return // Error already handled by getScheduleManager
	}
	info, err := scheduleManager.GetSchedule(c.Request.Context(), workflowID)
	if err != nil {
		if errors.Is(err, schedule.ErrScheduleNotFound) {
			router.RespondWithError(c, http.StatusNotFound, router.NewRequestError(
				http.StatusNotFound,
				"schedule not found",
				err,
			))
			return
		}
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get schedule",
			err,
		))
		return
	}
	// Convert to response format
	response := convertToScheduleInfoResponse(info)
	router.RespondOK(c, "schedule retrieved", response)
}

// updateSchedule updates a scheduled workflow
//
//	@Summary		Update schedule
//	@Description	Update a scheduled workflow's enabled state and/or cron expression. At least one field must be provided. This creates a temporary override that persists until the next YAML reload.
//	@Tags			schedules
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"	example("daily-report")
//	@Param			request		body		UpdateScheduleRequest					true	"Update request with at least one field (enabled or cron)"
//	@Success		200			{object}	router.Response{data=ScheduleInfoResponse}		"Schedule updated successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Schedule not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/schedules/{workflow_id} [patch]
//	@Security		BearerAuth
func updateSchedule(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		router.RespondWithError(c, http.StatusBadRequest, router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		))
		return
	}
	// Validate request - at least one field must be provided
	if req.Enabled == nil && req.Cron == nil {
		router.RespondWithError(c, http.StatusBadRequest, router.NewRequestError(
			http.StatusBadRequest,
			"at least one of 'enabled' or 'cron' is required",
			errors.New("request must contain at least one field to update"),
		))
		return
	}
	scheduleManager, ok := getScheduleManager(c)
	if !ok {
		return // Error already handled by getScheduleManager
	}
	// Update the schedule
	updateReq := schedule.UpdateRequest{
		Enabled: req.Enabled,
		Cron:    req.Cron,
	}
	err := scheduleManager.UpdateSchedule(c.Request.Context(), workflowID, updateReq)
	if err != nil {
		if errors.Is(err, schedule.ErrScheduleNotFound) {
			router.RespondWithError(c, http.StatusNotFound, router.NewRequestError(
				http.StatusNotFound,
				"schedule not found",
				err,
			))
			return
		}
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"failed to update schedule",
			err,
		))
		return
	}
	// Get updated schedule info
	info, err := scheduleManager.GetSchedule(c.Request.Context(), workflowID)
	if err != nil {
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get updated schedule",
			err,
		))
		return
	}
	// Convert to response format
	response := convertToScheduleInfoResponse(info)
	router.RespondOK(c, "schedule updated", response)
}

// deleteSchedule removes a scheduled workflow
//
//	@Summary		Delete schedule
//	@Description	Remove a scheduled workflow from Temporal. The schedule will be recreated on the next YAML reload if still defined.
//	@Tags			schedules
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string									true	"Workflow ID"	example("daily-report")
//	@Success		204			{object}	nil										"Schedule deleted successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid workflow ID"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Schedule not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/schedules/{workflow_id} [delete]
//	@Security		BearerAuth
func deleteSchedule(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	scheduleManager, ok := getScheduleManager(c)
	if !ok {
		return // Error already handled by getScheduleManager
	}
	err := scheduleManager.DeleteSchedule(c.Request.Context(), workflowID)
	if err != nil {
		if errors.Is(err, schedule.ErrScheduleNotFound) {
			router.RespondWithError(c, http.StatusNotFound, router.NewRequestError(
				http.StatusNotFound,
				"schedule not found",
				err,
			))
			return
		}
		router.RespondWithError(c, http.StatusInternalServerError, router.NewRequestError(
			http.StatusInternalServerError,
			"failed to delete schedule",
			err,
		))
		return
	}
	router.RespondNoContent(c)
}
