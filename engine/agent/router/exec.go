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
	// defaultAgentExecTimeout defines the fallback timeout applied when callers omit a value.
	defaultAgentExecTimeout = 60 * time.Second
	// maxAgentExecTimeout caps how long agent executions are allowed to run.
	maxAgentExecTimeout = 300 * time.Second
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
		req.Timeout = int(defaultAgentExecTimeout / time.Second)
	}
	if req.Timeout > int(maxAgentExecTimeout/time.Second) {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("timeout cannot exceed %d seconds", int(maxAgentExecTimeout/time.Second)),
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func loadAgentConfig(
	ctx context.Context,
	resourceStore resources.ResourceStore,
	projectName string,
	agentID string,
) (*agent.Config, error) {
	getUC := agentuc.NewGet(resourceStore)
	out, err := getUC.Execute(ctx, &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		return nil, err
	}
	agentConfig := &agent.Config{}
	if err := agentConfig.FromMap(out.Agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent config: %w", err)
	}
	return agentConfig, nil
}

func validateAgentAction(agentConfig *agent.Config, action string) error {
	if action == "" {
		return nil
	}
	if _, err := agent.FindActionConfig(agentConfig.Actions, action); err != nil {
		return err
	}
	return nil
}

func buildTaskConfig(agentID string, agentConfig *agent.Config, req *AgentExecRequest) *task.Config {
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
	return cfg
}

func loadAgentTaskConfig(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	state *appstate.State,
	agentID string,
	req *AgentExecRequest,
) (*task.Config, bool) {
	agentConfig, err := loadAgentConfig(c.Request.Context(), resourceStore, state.ProjectConfig.Name, agentID)
	if err != nil {
		if errors.Is(err, agentuc.ErrNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "agent not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to load agent", err)
		return nil, false
	}
	if err := validateAgentAction(agentConfig, req.Action); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "unknown action", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	cfg := buildTaskConfig(agentID, agentConfig, req)
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
	snapshotCtx := context.WithoutCancel(ctx)
	if stateSnapshot, stateErr := repo.GetState(snapshotCtx, execID); stateErr == nil && stateSnapshot != nil {
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

type agentExecution struct {
	taskCfg  *task.Config
	repo     task.Repository
	executor tkrouter.DirectExecutor
	meta     tkrouter.ExecMetadata
}

func prepareAgentExecution(
	c *gin.Context,
	state *appstate.State,
	syncReq *agentSyncRequest,
) (*agentExecution, int, bool) {
	taskCfg, ok := loadAgentTaskConfig(c, syncReq.resourceStore, state, syncReq.agentID, syncReq.req)
	if !ok {
		return nil, router.StatusOrFallback(c, http.StatusInternalServerError), false
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, http.StatusInternalServerError, false
	}
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return nil, http.StatusInternalServerError, false
	}
	meta := tkrouter.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   syncReq.agentID,
		ActionID:  resolveAgentActionID(syncReq.req, taskCfg),
		TaskID:    taskCfg.ID,
	}
	return &agentExecution{
		taskCfg:  taskCfg,
		repo:     repo,
		executor: executor,
		meta:     meta,
	}, http.StatusOK, true
}

type executionResult struct {
	output  *core.Output
	execID  core.ID
	outcome string
	status  int
}

func runAgentExecution(
	c *gin.Context,
	exec *agentExecution,
	agentID string,
	timeout time.Duration,
	metrics *monitoring.ExecutionMetrics,
) *executionResult {
	output, execID, err := exec.executor.ExecuteSync(c.Request.Context(), exec.taskCfg, &exec.meta, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			status := respondAgentTimeout(c, exec.repo, agentID, execID, metrics)
			return &executionResult{
				execID:  execID,
				outcome: monitoring.ExecutionOutcomeTimeout,
				status:  status,
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
		output:  output,
		execID:  execID,
		outcome: monitoring.ExecutionOutcomeSuccess,
		status:  http.StatusOK,
	}
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
	exec, status, ok := prepareAgentExecution(c, state, syncReq)
	if !ok {
		recordError(status)
		return
	}
	result := runAgentExecution(c, exec, syncReq.agentID, time.Duration(syncReq.req.Timeout)*time.Second, metrics)
	outcome = result.outcome
	if result.outcome != monitoring.ExecutionOutcomeSuccess {
		recordError(result.status)
		return
	}
	router.RespondOK(
		c,
		"agent executed",
		buildAgentSyncPayload(c.Request.Context(), exec.repo, syncReq.agentID, result.execID, result.output),
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

func loadAsyncTaskConfig(
	c *gin.Context,
	resources *asyncResources,
	asyncCtx *asyncRequestContext,
) (*task.Config, int, bool) {
	taskCfg, ok := loadAgentTaskConfig(c, resources.resourceStore, asyncCtx.state, asyncCtx.agentID, resources.req)
	if !ok {
		return nil, router.StatusOrFallback(c, http.StatusInternalServerError), false
	}
	return taskCfg, http.StatusOK, true
}

type asyncExecutor struct {
	repo     task.Repository
	executor tkrouter.DirectExecutor
}

func resolveAsyncExecutor(c *gin.Context, state *appstate.State) (*asyncExecutor, int, bool) {
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, http.StatusInternalServerError, false
	}
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return nil, http.StatusInternalServerError, false
	}
	return &asyncExecutor{
		repo:     repo,
		executor: executor,
	}, http.StatusOK, true
}

func startAsyncExecution(
	c *gin.Context,
	executor *asyncExecutor,
	taskCfg *task.Config,
	asyncCtx *asyncRequestContext,
	resources *asyncResources,
) (int, bool) {
	meta := tkrouter.ExecMetadata{
		Component: core.ComponentAgent,
		AgentID:   asyncCtx.agentID,
		ActionID:  resolveAgentActionID(resources.req, taskCfg),
		TaskID:    taskCfg.ID,
	}
	execID, err := executor.executor.ExecuteAsync(c.Request.Context(), taskCfg, &meta)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "agent execution failed", err)
		return http.StatusInternalServerError, false
	}
	execURL := fmt.Sprintf("%s/agents/%s", routes.Executions(), execID.String())
	c.Header("Location", execURL)
	router.RespondAccepted(c, "agent execution started", gin.H{"exec_id": execID.String(), "exec_url": execURL})
	return http.StatusAccepted, true
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
	taskCfg, status, ok := loadAsyncTaskConfig(c, resources, asyncCtx)
	if !ok {
		recordError(status)
		return
	}
	executor, status, ok := resolveAsyncExecutor(c, asyncCtx.state)
	if !ok {
		recordError(status)
		return
	}
	status, ok = startAsyncExecution(c, executor, taskCfg, asyncCtx, resources)
	if !ok {
		recordError(status)
	}
}
