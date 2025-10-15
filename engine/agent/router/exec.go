package agentrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	agentexec "github.com/compozy/compozy/engine/agent/exec"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// getAgentExecutionStatus handles GET /executions/agents/{exec_id}.
//
//	@Summary		Get agent execution status
//	@Description	Retrieve the latest status for a direct agent execution.
//	@Tags			executions
//	@Produce		json
//	@Param			exec_id	path	string	true	"Agent execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200	{object}	router.Response{data=agentrouter.ExecutionStatusDTO}	"Execution status retrieved"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Failed to load execution"
//	@Router			/executions/agents/{exec_id} [get]
func getAgentExecutionStatus(c *gin.Context) {
	execID := router.GetAgentExecID(c)
	if execID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	ctx := c.Request.Context()
	taskState, err := repo.GetState(ctx, execID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "execution not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to load execution", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usageRepo := router.ResolveUsageRepository(c, state)
	usageSummary := router.ResolveTaskUsageSummary(ctx, usageRepo, taskState.TaskExecID)
	resp := newExecutionStatusDTO(taskState)
	resp.Usage = usageSummary
	router.RespondOK(c, "execution status retrieved", resp)
}

func newExecutionStatusDTO(state *task.State) ExecutionStatusDTO {
	dto := ExecutionStatusDTO{
		ExecID:         state.TaskExecID.String(),
		Status:         state.Status,
		Component:      state.Component,
		TaskID:         state.TaskID,
		WorkflowID:     state.WorkflowID,
		WorkflowExecID: state.WorkflowExecID.String(),
		CreatedAt:      state.CreatedAt,
		UpdatedAt:      state.UpdatedAt,
	}
	if state.Output != nil {
		outputCopy := *state.Output
		dto.Output = &outputCopy
	}
	if state.Error != nil {
		errorCopy := *state.Error
		dto.Error = &errorCopy
	}
	return dto
}

func parseAgentExecRequest(c *gin.Context) (*AgentExecRequest, bool) {
	req := router.GetRequestBody[AgentExecRequest](c)
	if req == nil {
		return nil, false
	}
	if req.Action == "" && req.Prompt == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "either action or prompt is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	if req.Timeout < 0 {
		reqErr := router.NewRequestError(http.StatusBadRequest, "timeout must be non-negative", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	if req.Timeout == 0 {
		req.Timeout = int(agentexec.DefaultTimeout / time.Second)
	}
	if req.Timeout > int(agentexec.MaxTimeout/time.Second) {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("timeout cannot exceed %d seconds", int(agentexec.MaxTimeout/time.Second)),
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func respondAgentTimeout(
	c *gin.Context,
	repo task.Repository,
	usageRepo usage.Repository,
	agentID string,
	execID core.ID,
	metrics *monitoring.ExecutionMetrics,
) int {
	return router.RespondExecutionTimeout(
		c,
		repo,
		execID,
		func(state *task.State) any {
			if state == nil {
				return nil
			}
			ctx := c.Request.Context()
			dto := newExecutionStatusDTO(state)
			dto.Usage = router.ResolveTaskUsageSummary(ctx, usageRepo, state.TaskExecID)
			return dto
		},
		router.TimeoutResponseOptions{
			ResourceKind:  "Agent",
			ResourceID:    agentID,
			ExecutionKind: monitoring.ExecutionKindAgent,
			Metrics:       metrics,
		},
	)
}

func buildAgentSyncPayload(
	ctx context.Context,
	repo task.Repository,
	usageRepo usage.Repository,
	agentID string,
	execID core.ID,
	output *core.Output,
) gin.H {
	payload := gin.H{"exec_id": execID.String()}
	if output != nil {
		payload["output"] = output
	}
	snapshotCtx := context.WithoutCancel(ctx)
	if stateSnapshot, stateErr := repo.GetState(snapshotCtx, execID); stateErr == nil && stateSnapshot != nil {
		dto := newExecutionStatusDTO(stateSnapshot)
		dto.Usage = router.ResolveTaskUsageSummary(ctx, usageRepo, stateSnapshot.TaskExecID)
		payload["state"] = dto
		if stateSnapshot.Output != nil {
			payload["output"] = stateSnapshot.Output
		}
	} else if stateErr != nil {
		logger.FromContext(ctx).Warn(
			"Failed to load agent execution state after completion",
			"agent_id", agentID,
			"exec_id", execID.String(),
			"error", stateErr,
		)
	}
	if summary := router.ResolveTaskUsageSummary(ctx, usageRepo, execID); summary != nil {
		payload["usage"] = summary
	}
	return payload
}

type ExecutionStatusDTO struct {
	ExecID         string               `json:"exec_id"`
	Status         core.StatusType      `json:"status"`
	Component      core.ComponentType   `json:"component"`
	TaskID         string               `json:"task_id"`
	WorkflowID     string               `json:"workflow_id"`
	WorkflowExecID string               `json:"workflow_exec_id"`
	Output         *core.Output         `json:"output,omitempty"`
	Error          *core.Error          `json:"error,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	Usage          *router.UsageSummary `json:"usage,omitempty"`
}

// AgentExecRequest represents the payload accepted by agent execution endpoints.
// Provide either Action or Prompt. If both are set, Action takes precedence.
type AgentExecRequest struct {
	// Action selects a predefined agent action to execute.
	Action string `json:"action,omitempty"`
	// Prompt supplies an ad-hoc prompt for the agent when no action is provided.
	Prompt string `json:"prompt,omitempty"`
	// With passes structured input parameters to the agent execution.
	With core.Input `json:"with,omitempty"`
	// Timeout in seconds for synchronous execution.
	Timeout int `json:"timeout,omitempty"`
}

type agentSyncRequest struct {
	agentID       string
	req           *AgentExecRequest
	resourceStore resources.ResourceStore
}

func validateAgentSyncRequest(c *gin.Context) (*agentSyncRequest, int, bool) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return nil, http.StatusBadRequest, false
	}
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		return nil, http.StatusInternalServerError, false
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		return nil, http.StatusBadRequest, false
	}
	return &agentSyncRequest{
		agentID:       agentID,
		req:           req,
		resourceStore: resourceStore,
	}, http.StatusOK, true
}

func prepareAgentExecution(
	c *gin.Context,
	state *appstate.State,
	syncReq *agentSyncRequest,
) (*agentexec.Runner, agentexec.ExecuteRequest, task.Repository, int, bool) {
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, agentexec.ExecuteRequest{}, nil, http.StatusInternalServerError, false
	}
	runner := agentexec.NewRunner(state, repo, syncReq.resourceStore)
	execReq := agentexec.ExecuteRequest{
		AgentID: syncReq.agentID,
		Action:  syncReq.req.Action,
		Prompt:  syncReq.req.Prompt,
		With:    syncReq.req.With,
		Timeout: time.Duration(syncReq.req.Timeout) * time.Second,
	}
	return runner, execReq, repo, http.StatusOK, true
}

type executionResult struct {
	output  *core.Output
	execID  core.ID
	outcome string
	status  int
}

func runAgentExecution(
	c *gin.Context,
	runner *agentexec.Runner,
	repo task.Repository,
	usageRepo usage.Repository,
	prepared *agentexec.PreparedExecution,
	metrics *monitoring.ExecutionMetrics,
) *executionResult {
	result, err := runner.ExecutePrepared(c.Request.Context(), prepared)
	if err != nil {
		var execID core.ID
		if result != nil {
			execID = result.ExecID
		}
		if errors.Is(err, context.DeadlineExceeded) && result != nil {
			reqTimeout := respondAgentTimeout(c, repo, usageRepo, prepared.Metadata.AgentID, execID, metrics)
			return &executionResult{
				execID:  execID,
				outcome: monitoring.ExecutionOutcomeTimeout,
				status:  reqTimeout,
			}
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "agent execution failed", err)
		return &executionResult{
			execID:  execID,
			outcome: monitoring.ExecutionOutcomeError,
			status:  http.StatusInternalServerError,
		}
	}
	return &executionResult{
		output:  result.Output,
		execID:  result.ExecID,
		outcome: monitoring.ExecutionOutcomeSuccess,
		status:  http.StatusOK,
	}
}

func handlePreparationError(c *gin.Context, err error) int {
	if errors.Is(err, agentexec.ErrActionOrPromptRequired) || errors.Is(err, agentexec.ErrNegativeTimeout) ||
		errors.Is(err, agentexec.ErrTimeoutTooLarge) {
		reqErr := router.NewRequestError(http.StatusBadRequest, err.Error(), err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr.StatusCode
	}
	if errors.Is(err, agentexec.ErrUnknownAction) {
		reqErr := router.NewRequestError(http.StatusBadRequest, "unknown action", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr.StatusCode
	}
	if errors.Is(err, agentuc.ErrNotFound) {
		reqErr := router.NewRequestError(http.StatusNotFound, "agent not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr.StatusCode
	}
	if errors.Is(err, agentexec.ErrAgentIDRequired) {
		reqErr := router.NewRequestError(http.StatusBadRequest, "agent id is required", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr.StatusCode
	}
	router.RespondWithServerError(c, router.ErrInternalCode, "failed to prepare agent execution", err)
	return http.StatusInternalServerError
}

// executeAgentSync handles POST /agents/{agent_id}/executions/sync.
//
//	@Summary		Execute agent synchronously
//	@Description	Execute an agent and wait for the output in the same HTTP response.
//	@Tags			agents
//	@Accept			json
//	@Produce		json
//	@Param			agent_id	path	string	true	"Agent ID"	example("assistant")
//	@Param			X-Idempotency-Key	header	string	false	"Optional idempotency key to prevent duplicate execution"
//	@Param			payload	body	agentrouter.AgentExecRequest	true	"Execution request"
//	@Success		200	{object}	router.Response{data=agentrouter.AgentExecSyncResponse}	"Agent executed"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Agent not found"
//	@Failure		408	{object}	router.Response{error=router.ErrorInfo}	"Execution timeout"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/agents/{agent_id}/executions/sync [post]
func executeAgentSync(c *gin.Context) {
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	metrics, finalizeMetrics, recordError := router.SyncExecutionMetricsScope(c, state, monitoring.ExecutionKindAgent)
	outcome := monitoring.ExecutionOutcomeError
	defer func() { finalizeMetrics(outcome) }()
	syncReq, status, ok := validateAgentSyncRequest(c)
	if !ok {
		recordError(status)
		return
	}
	runner, execReq, repo, status, ok := prepareAgentExecution(c, state, syncReq)
	if !ok {
		recordError(status)
		return
	}
	usageRepo := router.ResolveUsageRepository(c, state)
	prepared, err := runner.Prepare(c.Request.Context(), execReq)
	if err != nil {
		recordError(handlePreparationError(c, err))
		return
	}
	result := runAgentExecution(c, runner, repo, usageRepo, prepared, metrics)
	outcome = result.outcome
	if result.outcome != monitoring.ExecutionOutcomeSuccess {
		recordError(result.status)
		return
	}
	router.RespondOK(
		c,
		"agent executed",
		buildAgentSyncPayload(c.Request.Context(), repo, usageRepo, syncReq.agentID, result.execID, result.output),
	)
}

type asyncRequestContext struct {
	agentID string
	state   *appstate.State
	metrics *monitoring.ExecutionMetrics
}

func validateAsyncRequest(c *gin.Context) (*asyncRequestContext, int, bool) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return nil, http.StatusBadRequest, false
	}
	state := router.GetAppState(c)
	if state == nil {
		return nil, http.StatusInternalServerError, false
	}
	metrics := router.ResolveExecutionMetrics(c, state)
	return &asyncRequestContext{
		agentID: agentID,
		state:   state,
		metrics: metrics,
	}, http.StatusOK, true
}

type asyncResources struct {
	resourceStore resources.ResourceStore
	req           *AgentExecRequest
}

func parseAsyncResources(c *gin.Context) (*asyncResources, int, bool) {
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		return nil, http.StatusInternalServerError, false
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		return nil, http.StatusBadRequest, false
	}
	return &asyncResources{
		resourceStore: resourceStore,
		req:           req,
	}, http.StatusOK, true
}

// executeAgentAsync handles POST /agents/{agent_id}/executions.
//
//	@Summary		Start agent execution asynchronously
//	@Description	Start an asynchronous agent execution and return a polling handle.
//	@Tags			agents
//	@Accept			json
//	@Produce		json
//	@Param			agent_id	path	string	true	"Agent ID"	example("assistant")
//	@Param			X-Correlation-ID	header	string	false	"Optional correlation ID for request tracing"
//	@Param			payload	body	agentrouter.AgentExecRequest	true	"Execution request"
//	@Success		202	{object}	router.Response{data=agentrouter.AgentExecAsyncResponse}	"Agent execution started"
//	@Header			202	{string}	Location	"Execution status URL"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Agent not found"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/agents/{agent_id}/executions [post]
func executeAgentAsync(c *gin.Context) {
	asyncCtx, _, ok := validateAsyncRequest(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	recordError := func(code int) {
		if asyncCtx.metrics != nil && code >= http.StatusBadRequest {
			asyncCtx.metrics.RecordError(ctx, monitoring.ExecutionKindAgent, code)
		}
	}
	resources, status, ok := parseAsyncResources(c)
	if !ok {
		recordError(status)
		return
	}
	syncReq := &agentSyncRequest{
		agentID:       asyncCtx.agentID,
		req:           resources.req,
		resourceStore: resources.resourceStore,
	}
	runner, execReq, _, status, ok := prepareAgentExecution(c, asyncCtx.state, syncReq)
	if !ok {
		recordError(status)
		return
	}
	prepared, err := runner.Prepare(c.Request.Context(), execReq)
	if err != nil {
		recordError(handlePreparationError(c, err))
		return
	}
	execID, err := prepared.Executor.ExecuteAsync(c.Request.Context(), prepared.Config, &prepared.Metadata)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "agent execution failed", err)
		recordError(http.StatusInternalServerError)
		return
	}
	execURL := fmt.Sprintf("%s/agents/%s", routes.Executions(), execID.String())
	c.Header("Location", execURL)
	router.RespondAccepted(c, "agent execution started", gin.H{"exec_id": execID.String(), "exec_url": execURL})
}
