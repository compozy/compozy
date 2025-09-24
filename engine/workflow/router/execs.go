package wfrouter

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// getExecution retrieves a workflow execution by ID
//
//	@Summary		Get workflow execution by ID
//	@Description	Retrieve a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			exec_id	path		string										true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200					{object}	router.Response{data=workflow.State}	"Workflow execution retrieved successfully"
//	@Failure		400					{object}	router.Response{error=router.ErrorInfo}		"Invalid execution ID"
//	@Failure		404					{object}	router.Response{error=router.ErrorInfo}		"Execution not found"
//	@Failure		503					{object}	router.Response{error=router.ErrorInfo}		"Worker unavailable"
//	@Failure		500					{object}	router.Response{error=router.ErrorInfo}		"Internal server error"
//	@Router			/executions/workflows/{exec_id} [get]
func getExecution(c *gin.Context) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewGetExecution(repo, execID)
	exec, err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid execution ID",
			conflict:   "execution conflict",
			defaultMsg: "failed to get execution",
		})
		return
	}
	router.RespondOK(c, "workflow execution retrieved", exec)
}

// listAllExecutions retrieves all workflow executions
//
//	@Summary		List all workflow executions
//	@Description	Retrieve a list of all workflow executions across all workflows
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	router.Response{data=object{executions=[]workflow.State}}	"Workflow executions retrieved successfully"
//	@Failure		503	{object}	router.Response{error=router.ErrorInfo}								"Worker unavailable"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/executions/workflows [get]
func listAllExecutions(c *gin.Context) {
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewListAllExecutions(repo)
	executions, err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "executions not found",
			badRequest: "invalid request",
			conflict:   "execution conflict",
			defaultMsg: "failed to list executions",
		})
		return
	}
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": executions,
	})
}

// listExecutionsByID retrieves executions for a specific workflow
//
//	@Summary		List executions by workflow ID
//	@Description	Retrieve all executions for a specific workflow
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string																true	"Workflow ID"	example("data-processing")
//	@Success		200			{object}	router.Response{data=object{executions=[]workflow.State}}	"Workflow executions retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}								"Invalid workflow ID"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}								"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}								"Internal server error"
//	@Router			/workflows/{workflow_id}/executions [get]
func listExecutionsByID(c *gin.Context) {
	wfID := router.GetWorkflowID(c)
	if wfID == "" {
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	repo := appState.Worker.WorkflowRepo()
	useCase := uc.NewListExecutionsByID(repo, wfID)
	execs, err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid workflow ID",
			conflict:   "execution conflict",
			defaultMsg: "failed to list executions by ID",
		})
		return
	}
	router.RespondOK(c, "workflow executions retrieved", gin.H{
		"executions": execs,
	})
}

// pauseExecution pauses a workflow execution
//
//	@Summary		Pause workflow execution
//	@Description	Pause a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			exec_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution paused successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}	"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{exec_id}/pause [post]
func pauseExecution(c *gin.Context) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	useCase := uc.NewPauseExecution(appState.Worker, execID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid execution ID",
			conflict:   "execution conflict",
			defaultMsg: "failed to pause execution",
		})
		return
	}
	router.RespondOK(c, "workflow execution paused", nil)
}

// resumeExecution resumes a workflow execution
//
//	@Summary		Resume workflow execution
//	@Description	Resume a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			exec_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution resumed successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}	"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{exec_id}/resume [post]
func resumeExecution(c *gin.Context) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	useCase := uc.NewResumeExecution(appState.Worker, execID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid execution ID",
			conflict:   "execution conflict",
			defaultMsg: "failed to resume execution",
		})
		return
	}
	router.RespondOK(c, "workflow execution resumed", nil)
}

// cancelExecution cancels a workflow execution
//
//	@Summary		Cancel workflow execution
//	@Description	Cancel a specific workflow execution by its execution ID
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			exec_id	path		string	true	"Workflow Execution ID"	example("workflowID_execID")
//	@Success		200			{object}	router.Response{data=string}	"Workflow execution canceled successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}	"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{exec_id}/cancel [post]
func cancelExecution(c *gin.Context) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}
	useCase := uc.NewCancelExecution(appState.Worker, execID)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid execution ID",
			conflict:   "Execution conflict",
			defaultMsg: "failed to cancel execution",
		})
		return
	}
	router.RespondOK(c, "workflow execution canceled", nil)
}

// SignalRequest represents the request body for sending a signal to a workflow execution
type SignalRequest struct {
	// SignalName is the name of the signal to send to the workflow execution
	SignalName string `json:"signal_name" binding:"required" example:"ready_signal"`
	// Payload contains the data to send with the signal
	Payload core.Input `json:"payload"`
}

// SignalResponse represents the response for sending a signal
type SignalResponse struct {
	Message string `json:"message" example:"Signal sent successfully"`
}

// sendSignalToExecution sends a signal to a workflow execution
//
//	@Summary		Send signal to workflow execution
//	@Description	Send a signal with payload to a specific workflow execution
//	@Tags			executions
//	@Accept			json
//	@Produce		json
//	@Param			exec_id	path		string										true	"Workflow Execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Param			signal	body		SignalRequest							true	"Signal data"
//	@Success		200		{object}	router.Response{data=SignalResponse}	"Signal sent successfully"
//	@Failure		400		{object}	router.Response{error=router.ErrorInfo}	"Invalid execution ID or signal data"
//	@Failure		404		{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		503		{object}	router.Response{error=router.ErrorInfo}	"Worker unavailable"
//	@Failure		500		{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/executions/workflows/{exec_id}/signals [post]
func sendSignalToExecution(c *gin.Context) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	appState, ok := ensureWorkerReady(c)
	if !ok {
		return
	}

	body := router.GetRequestBody[SignalRequest](c)
	if body == nil {
		return
	}
	if body.SignalName == "" {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"signal_name is required",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	useCase := uc.NewSendSignalToExecution(appState.Worker, execID, body.SignalName, body.Payload)
	err := useCase.Execute(c.Request.Context())
	if err != nil {
		respondWithExecutionError(c, err, executionErrorMessages{
			notFound:   "execution not found",
			badRequest: "Invalid execution ID or signal data",
			conflict:   "execution conflict",
			defaultMsg: "failed to send signal to execution",
		})
		return
	}

	router.RespondOK(c, "signal sent successfully", SignalResponse{
		Message: "Signal sent successfully",
	})
}

type executionErrorMessages struct {
	notFound   string
	badRequest string
	conflict   string
	defaultMsg string
}

func respondWithExecutionError(c *gin.Context, err error, msgs executionErrorMessages) {
	statusCode := http.StatusInternalServerError
	reason := msgs.defaultMsg
	if reason == "" {
		reason = "internal server error"
	}
	switch {
	case isWorkflowNotFound(err):
		statusCode = http.StatusNotFound
		if msgs.notFound != "" {
			reason = msgs.notFound
		} else {
			reason = "execution not found"
		}
	default:
		if grpcStatus, ok := status.FromError(err); ok {
			switch grpcStatus.Code() {
			case codes.InvalidArgument:
				statusCode = http.StatusBadRequest
				if msgs.badRequest != "" {
					reason = msgs.badRequest
				} else {
					reason = "invalid request"
				}
			case codes.AlreadyExists, codes.FailedPrecondition:
				statusCode = http.StatusConflict
				switch {
				case msgs.conflict != "":
					reason = msgs.conflict
				case msgs.defaultMsg != "":
					reason = msgs.defaultMsg
				default:
					reason = "execution conflict"
				}
			case codes.NotFound:
				statusCode = http.StatusNotFound
				if msgs.notFound != "" {
					reason = msgs.notFound
				} else {
					reason = "execution not found"
				}
			case codes.Unavailable:
				statusCode = http.StatusServiceUnavailable
				reason = "worker unavailable"
			}
		}
	}
	log := logger.FromContext(c.Request.Context())
	log.Error(
		"workflow request failed",
		"error", err,
		"status_code", statusCode,
		"reason", reason,
	)
	reqErr := router.NewRequestError(statusCode, reason, err) // ensure router sanitizes details
	router.RespondWithError(c, statusCode, reqErr)
}

func isWorkflowNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, store.ErrWorkflowNotFound) {
		return true
	}
	return strings.Contains(err.Error(), "workflow state not found")
}
