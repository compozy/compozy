package wfrouter

import (
	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
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
		reqErr := router.NewRequestError(400, "Invalid event format", err)
		router.RespondWithError(c, 400, reqErr)
		return
	}

	// Get authenticated claims for project scoping
	claimsValue := c.Request.Context().Value(authClaimsKey)
	claims, ok := claimsValue.(*AuthClaims)
	if !ok || claims == nil {
		reqErr := router.NewRequestError(500, "Authentication context missing", nil)
		router.RespondWithError(c, 500, reqErr)
		return
	}

	state := router.GetAppState(c)
	workerMgr := state.Worker
	// Use project ID from auth claims for deterministic dispatcher workflow ID
	projectName := claims.ProjectID
	if projectName == "" {
		reqErr := router.NewRequestError(400, "Project ID missing from authentication token", nil)
		router.RespondWithError(c, 400, reqErr)
		return
	}
	dispatcherID := "dispatcher-" + slug.Make(projectName)
	taskQueue := slug.Make(projectName)

	// Generate correlation ID for tracking
	eventID := core.MustNewID().String()

	// Send signal with start
	_, err := workerMgr.GetClient().SignalWithStartWorkflow(
		c.Request.Context(),
		dispatcherID,
		"event_channel",
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
		reqErr := router.NewRequestError(500, "Failed to send event", err)
		router.RespondWithError(c, 500, reqErr)
		return
	}
	router.RespondAccepted(c, "event received", EventResponse{
		Message: "event received",
		EventID: eventID,
	})
}
