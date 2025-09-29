package agentrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	// defaultAgentExecTimeoutSeconds defines the fallback timeout applied when callers omit a value.
	defaultAgentExecTimeoutSeconds = 60
	// maxAgentExecTimeoutSeconds caps how long agent executions are allowed to run.
	maxAgentExecTimeoutSeconds = 300
	// directPromptActionID labels prompt-only executions so task state constraints remain satisfied.
	directPromptActionID = "__prompt__"
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
	resp := newExecutionStatusDTO(taskState)
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
		req.Timeout = defaultAgentExecTimeoutSeconds
	}
	if req.Timeout > maxAgentExecTimeoutSeconds {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("timeout cannot exceed %d seconds", maxAgentExecTimeoutSeconds),
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func loadAgentTaskConfig(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	state *appstate.State,
	agentID string,
	req *AgentExecRequest,
) (*task.Config, bool) {
	projectName := state.ProjectConfig.Name
	getUC := agentuc.NewGet(resourceStore)
	out, err := getUC.Execute(c.Request.Context(), &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		if errors.Is(err, agentuc.ErrNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "agent not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to load agent", err)
		return nil, false
	}
	agentConfig := &agent.Config{}
	if err := agentConfig.FromMap(out.Agent); err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to decode agent", err)
		return nil, false
	}
	if req.Action != "" {
		if _, err := agent.FindActionConfig(agentConfig.Actions, req.Action); err != nil {
			reqErr := router.NewRequestError(http.StatusBadRequest, "unknown action", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
	}
	cfg := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:    fmt.Sprintf("agent:%s", agentID),
			Type:  task.TaskTypeBasic,
			Agent: agentConfig,
		},
	}
	if req.Action != "" {
		cfg.Action = req.Action
	}
	if req.Prompt != "" {
		cfg.Prompt = req.Prompt
	}
	if len(req.With) > 0 {
		withCopy := req.With
		cfg.With = &withCopy
	}
	cfg.Timeout = (time.Duration(req.Timeout) * time.Second).String()
	return cfg, true
}

func resolveAgentActionID(req *AgentExecRequest, cfg *task.Config) string {
	if req != nil && req.Action != "" {
		return req.Action
	}
	if cfg != nil && cfg.Action != "" {
		return cfg.Action
	}
	if req != nil && req.Prompt != "" {
		return directPromptActionID
	}
	return ""
}

func respondAgentTimeout(
	c *gin.Context,
	repo task.Repository,
	agentID string,
	execID core.ID,
	metrics *monitoring.ExecutionMetrics,
) int {
	return router.RespondExecutionTimeout(
		c,
		repo,
		execID,
		func(state *task.State) any {
			return newExecutionStatusDTO(state)
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
	agentID string,
	execID core.ID,
	output *core.Output,
) gin.H {
	payload := gin.H{"exec_id": execID.String()}
	if output != nil {
		payload["output"] = output
	}
	if stateSnapshot, stateErr := repo.GetState(ctx, execID); stateErr == nil && stateSnapshot != nil {
		payload["state"] = newExecutionStatusDTO(stateSnapshot)
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
	return payload
}

type ExecutionStatusDTO struct {
	ExecID         string             `json:"exec_id"`
	Status         core.StatusType    `json:"status"`
	Component      core.ComponentType `json:"component"`
	TaskID         string             `json:"task_id"`
	WorkflowID     string             `json:"workflow_id"`
	WorkflowExecID string             `json:"workflow_exec_id"`
	Output         *core.Output       `json:"output,omitempty"`
	Error          *core.Error        `json:"error,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
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

// executeAgentSync handles POST /agents/{agent_id}/executions.
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
//	@Router			/agents/{agent_id}/executions [post]
func executeAgentSync(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	metrics, finalizeMetrics, recordError := router.SyncExecutionMetricsScope(c, state, monitoring.ExecutionKindAgent)
	outcome := monitoring.ExecutionOutcomeError
	defer func() { finalizeMetrics(outcome) }()
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	taskCfg, ok := loadAgentTaskConfig(c, resourceStore, state, agentID, req)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		recordError(http.StatusInternalServerError)
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return
	}
	meta := tkrouter.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   agentID,
		ActionID:  resolveAgentActionID(req, taskCfg),
		TaskID:    taskCfg.ID,
	}
	output, execID, execErr := executor.ExecuteSync(
		c.Request.Context(),
		taskCfg,
		&meta,
		time.Duration(req.Timeout)*time.Second,
	)
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) {
			outcome = monitoring.ExecutionOutcomeTimeout
			status := respondAgentTimeout(c, repo, agentID, execID, metrics)
			recordError(status)
			return
		}
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "agent execution failed", execErr)
		return
	}
	outcome = monitoring.ExecutionOutcomeSuccess
	router.RespondOK(
		c,
		"agent executed",
		buildAgentSyncPayload(c.Request.Context(), repo, agentID, execID, output),
	)
}

// executeAgentAsync handles POST /agents/{agent_id}/executions/async.
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
//	@Router			/agents/{agent_id}/executions/async [post]
func executeAgentAsync(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	metrics := router.ResolveExecutionMetrics(c, state)
	ctx := c.Request.Context()
	recordError := func(code int) {
		if metrics != nil && code >= http.StatusBadRequest {
			metrics.RecordError(ctx, monitoring.ExecutionKindAgent, code)
		}
	}
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	taskCfg, ok := loadAgentTaskConfig(c, resourceStore, state, agentID, req)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		recordError(http.StatusInternalServerError)
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return
	}
	meta := tkrouter.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   agentID,
		ActionID:  resolveAgentActionID(req, taskCfg),
		TaskID:    taskCfg.ID,
	}
	execID, execErr := executor.ExecuteAsync(c.Request.Context(), taskCfg, &meta)
	if execErr != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "agent execution failed", execErr)
		return
	}
	execURL := fmt.Sprintf("%s/agents/%s", routes.Executions(), execID.String())
	c.Header("Location", execURL)
	router.RespondAccepted(c, "agent execution started", gin.H{"exec_id": execID.String(), "exec_url": execURL})
}
