package wfrouter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/worker"
)

type EventRequest struct {
	Name    string     `json:"name"    binding:"required"`
	Payload core.Input `json:"payload"`
}

type EventResponse struct {
	Message string `json:"message"`
	EventID string `json:"event_id"`
}

// handleEvent processes incoming events
// @Summary     Send event
// @Description Trigger workflows by sending events
// @Tags        events
// @Accept      json
// @Produce     json
// @Param       event body EventRequest true "Event data"
// @Success     202 {object} router.Response{data=EventResponse}
// @Failure     400 {object} router.Response{error=router.ErrorInfo}
// @Router      /events [post]
func handleEvent(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid event format", err)
		router.RespondWithError(c, http.StatusBadRequest, reqErr)
		return
	}

	state := router.GetAppState(c)
	workerMgr := state.Worker
	projectName := state.ProjectConfig.Name
	dispatcherID := workerMgr.GetDispatcherID() // Use this instance's dispatcher
	taskQueue := workerMgr.GetTaskQueue()       // Use this instance's task queue

	// Generate correlation ID for tracking
	eventID := core.MustNewID().String()

	// Send signal with start
	_, err := workerMgr.GetClient().SignalWithStartWorkflow(
		c.Request.Context(),
		dispatcherID,
		worker.DispatcherEventChannel,
		worker.EventSignal{
			Name:          req.Name,
			Payload:       req.Payload,
			CorrelationID: eventID,
		},
		client.StartWorkflowOptions{
			ID:        dispatcherID,
			TaskQueue: taskQueue,
		},
		worker.DispatcherWorkflow,
		projectName,
	)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "Failed to send event", err)
		router.RespondWithError(c, http.StatusInternalServerError, reqErr)
		return
	}
	router.RespondAccepted(c, "event received", EventResponse{
		Message: "event received",
		EventID: eventID,
	})
}
