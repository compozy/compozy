package wfrouter

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/worker"
	appconfig "github.com/compozy/compozy/pkg/config"
)

const workflowStartTimeoutDefault = 5 * time.Second

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
// @Failure     400 {object} router.Response{error=router.ErrorInfo} "Invalid event"
// @Failure     404 {object} router.Response{error=router.ErrorInfo} "Not found"
// @Failure     409 {object} router.Response{error=router.ErrorInfo} "Conflict"
// @Failure     503 {object} router.Response{error=router.ErrorInfo} "Worker unavailable"
// @Failure     500 {object} router.Response{error=router.ErrorInfo} "Internal server error"
// @Router      /events [post]
func handleEvent(c *gin.Context) {
	req, ok := parseEventRequest(c)
	if !ok {
		return
	}
	state := router.GetAppStateWithWorker(c)
	if state == nil {
		return
	}
	workerMgr := state.Worker
	dispatcherID := workerMgr.GetDispatcherID()
	taskQueue := workerMgr.GetTaskQueue()
	eventID := core.MustNewID().String()
	ctx, cancel := startEventContext(c.Request.Context())
	defer cancel()
	if err := dispatchEventSignal(
		ctx,
		workerMgr,
		dispatcherID,
		taskQueue,
		req,
		eventID,
		state.ProjectConfig.Name,
	); err != nil {
		respondDispatchError(c, err)
		return
	}
	router.RespondAccepted(c, "event received", EventResponse{
		Message: "event received",
		EventID: eventID,
	})
}

// parseEventRequest binds and validates the event submission payload.
func parseEventRequest(c *gin.Context) (EventRequest, bool) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondRouterError(c, http.StatusBadRequest, "Invalid event format", err)
		return EventRequest{}, false
	}
	return req, true
}

// startEventContext applies dispatch timeouts derived from global configuration.
func startEventContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := workflowStartTimeoutDefault
	if cfg := appconfig.FromContext(parent); cfg != nil && cfg.Worker.StartWorkflowTimeout > 0 {
		timeout = cfg.Worker.StartWorkflowTimeout
	}
	return context.WithTimeout(parent, timeout)
}

// dispatchEventSignal forwards an event signal to the dispatcher workflow.
func dispatchEventSignal(
	ctx context.Context,
	workerMgr *worker.Worker,
	dispatcherID string,
	taskQueue string,
	req EventRequest,
	eventID string,
	projectName string,
) error {
	_, err := workerMgr.GetClient().SignalWithStartWorkflow(
		ctx,
		dispatcherID,
		worker.DispatcherEventChannel,
		worker.EventSignal{
			Name:          req.Name,
			Payload:       req.Payload,
			CorrelationID: eventID,
		},
		client.StartWorkflowOptions{ID: dispatcherID, TaskQueue: taskQueue},
		worker.DispatcherWorkflow,
		projectName,
	)
	return err
}

// respondDispatchError maps Temporal dispatch errors to HTTP responses.
func respondDispatchError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	reason := "Failed to send event"
	switch status.Code(err) {
	case codes.NotFound:
		statusCode = http.StatusNotFound
		reason = "Not found"
	case codes.InvalidArgument:
		statusCode = http.StatusBadRequest
		reason = "Invalid event"
	case codes.AlreadyExists:
		statusCode = http.StatusConflict
		reason = "Conflict"
	}
	respondRouterError(c, statusCode, reason, err)
}

// respondRouterError renders an error response with the router's envelope.
func respondRouterError(c *gin.Context, statusCode int, reason string, err error) {
	router.RespondWithError(c, statusCode, router.NewRequestError(statusCode, reason, err))
}
