package tkrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	// fallbackTaskExecTimeoutDefault defines the default timeout when configuration is unavailable.
	fallbackTaskExecTimeoutDefault = 60 * time.Second
	// fallbackTaskExecTimeoutMax caps direct task executions when configuration is unavailable.
	fallbackTaskExecTimeoutMax = 300 * time.Second
)

// getTaskExecutionStatus handles GET /executions/tasks/{exec_id}.
//
//	@Summary		Get task execution status
//	@Description	Retrieve the latest status for a direct task execution. The response includes a usage field containing aggregated LLM token counts grouped by provider and model.
//	@Tags			executions
//	@Produce		json
//	@Param			exec_id	path	string	true	"Task execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200	{object}	router.Response{data=tkrouter.TaskExecutionStatusDTO}	"Execution status retrieved. The data.usage field is an array of usage entries with prompt_tokens, completion_tokens, total_tokens, and optional reasoning_tokens, cached_prompt_tokens, input_audio_tokens, and output_audio_tokens per provider/model combination."
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Execution not found"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Failed to load execution"
//	@Router			/executions/tasks/{exec_id} [get]
func getTaskExecutionStatus(c *gin.Context) {
	execID := router.GetTaskExecID(c)
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
	resp := newTaskExecutionStatusDTO(taskState)
	resp.Usage = router.NewUsageSummary(taskState.Usage)
	router.RespondOK(c, "execution status retrieved", resp)
}

func newTaskExecutionStatusDTO(state *task.State) TaskExecutionStatusDTO {
	dto := TaskExecutionStatusDTO{
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

type taskExecutionTimeouts struct {
	Default time.Duration
	Max     time.Duration
}

func resolveTaskExecutionTimeouts(ctx context.Context, state *appstate.State) taskExecutionTimeouts {
	timeouts := taskExecutionTimeouts{Default: fallbackTaskExecTimeoutDefault, Max: fallbackTaskExecTimeoutMax}
	if cfg := config.FromContext(ctx); cfg != nil {
		if cfg.Runtime.TaskExecutionTimeoutDefault > 0 {
			timeouts.Default = cfg.Runtime.TaskExecutionTimeoutDefault
		}
		if cfg.Runtime.TaskExecutionTimeoutMax > 0 {
			timeouts.Max = cfg.Runtime.TaskExecutionTimeoutMax
		}
	}
	if state != nil && state.ProjectConfig != nil {
		runtimeCfg := state.ProjectConfig.Runtime
		if runtimeCfg.TaskExecutionTimeoutDefault > 0 {
			timeouts.Default = runtimeCfg.TaskExecutionTimeoutDefault
		}
		if runtimeCfg.TaskExecutionTimeoutMax > 0 {
			timeouts.Max = runtimeCfg.TaskExecutionTimeoutMax
		}
	}
	if timeouts.Max > 0 && timeouts.Default > timeouts.Max {
		timeouts.Default = timeouts.Max
	}
	return timeouts
}

func resolveDirectTaskTimeout(
	ctx context.Context,
	req *TaskExecRequest,
	taskCfg *task.Config,
	limits taskExecutionTimeouts,
) (time.Duration, error) {
	if req != nil && req.Timeout != nil {
		if *req.Timeout > 0 {
			requested := time.Duration(*req.Timeout) * time.Second
			if requested > limits.Max {
				return 0, fmt.Errorf("timeout cannot exceed %d seconds", int(limits.Max/time.Second))
			}
			return requested, nil
		}
	}
	var taskDuration time.Duration
	if taskCfg != nil && taskCfg.Timeout != "" {
		parsed, err := core.ParseHumanDuration(strings.TrimSpace(taskCfg.Timeout))
		if err == nil && parsed > 0 {
			taskDuration = parsed
		} else if err != nil {
			logger.FromContext(ctx).Warn(
				"Invalid task timeout; falling back to defaults",
				"timeout", taskCfg.Timeout,
				"error", err,
			)
		}
	}
	if taskDuration > limits.Max {
		return 0, fmt.Errorf("task timeout %s cannot exceed %s", taskDuration.String(), limits.Max.String())
	}
	if taskDuration > 0 {
		return taskDuration, nil
	}
	if limits.Default > limits.Max {
		return limits.Max, nil
	}
	return limits.Default, nil
}

func parseTaskExecRequest(c *gin.Context) (*TaskExecRequest, bool) {
	req := router.GetRequestBody[TaskExecRequest](c)
	if req == nil {
		return nil, false
	}
	if req.Timeout != nil && *req.Timeout < 0 {
		reqErr := router.NewRequestError(http.StatusBadRequest, "timeout must be non-negative", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func loadDirectTaskConfig(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	state *appstate.State,
	taskID string,
	req *TaskExecRequest,
	timeouts taskExecutionTimeouts,
) (*task.Config, time.Duration, bool) {
	projectName := state.ProjectConfig.Name
	taskCfg, ok := fetchTaskDefinition(c, resourceStore, projectName, taskID)
	if !ok {
		return nil, 0, false
	}
	if !hydrateTaskAgentConfig(c, resourceStore, projectName, taskID, taskCfg) {
		return nil, 0, false
	}
	applyTaskRequestOverrides(taskCfg, taskID, req)
	execTimeout, ok := computeDirectExecutionTimeout(c, req, taskCfg, timeouts)
	if !ok {
		return nil, 0, false
	}
	taskCfg.Timeout = execTimeout.String()
	return taskCfg, execTimeout, true
}

func fetchTaskDefinition(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	projectName string,
	taskID string,
) (*task.Config, bool) {
	getUC := taskuc.NewGet(resourceStore)
	out, err := getUC.Execute(c.Request.Context(), &taskuc.GetInput{Project: projectName, ID: taskID})
	if err != nil {
		if errors.Is(err, taskuc.ErrNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "task not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to load task", err)
		return nil, false
	}
	taskCfg := &task.Config{}
	if err := taskCfg.FromMap(out.Task); err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to decode task", err)
		return nil, false
	}
	return taskCfg, true
}

func hydrateTaskAgentConfig(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	projectName string,
	taskID string,
	taskCfg *task.Config,
) bool {
	if taskCfg.Agent == nil {
		return true
	}
	agentID := strings.TrimSpace(taskCfg.Agent.ID)
	if agentID == "" {
		reqErr := fmt.Errorf("task %s missing agent reference", taskID)
		router.RespondWithServerError(c, router.ErrInternalCode, "task missing agent identifier", reqErr)
		return false
	}
	if len(taskCfg.Agent.Actions) > 0 {
		return true
	}
	getAgent := agentuc.NewGet(resourceStore)
	agentOut, err := getAgent.Execute(c.Request.Context(), &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		if errors.Is(err, agentuc.ErrNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "agent not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return false
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to load agent", err)
		return false
	}
	agentCfg := &agent.Config{}
	if err := agentCfg.FromMap(agentOut.Agent); err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to decode agent", err)
		return false
	}
	taskCfg.Agent = agentCfg
	return true
}

func applyTaskRequestOverrides(taskCfg *task.Config, taskID string, req *TaskExecRequest) {
	if taskCfg.ID == "" {
		taskCfg.ID = taskID
	}
	if req == nil {
		return
	}
	if len(req.With) > 0 {
		withCopy := req.With
		taskCfg.With = &withCopy
	}
}

func computeDirectExecutionTimeout(
	c *gin.Context,
	req *TaskExecRequest,
	taskCfg *task.Config,
	limits taskExecutionTimeouts,
) (time.Duration, bool) {
	execTimeout, timeoutErr := resolveDirectTaskTimeout(c.Request.Context(), req, taskCfg, limits)
	if timeoutErr != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, timeoutErr.Error(), timeoutErr)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return execTimeout, true
}

func respondTaskTimeout(
	c *gin.Context,
	repo task.Repository,
	taskID string,
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
			dto := newTaskExecutionStatusDTO(state)
			dto.Usage = router.NewUsageSummary(state.Usage)
			return dto
		},
		router.TimeoutResponseOptions{
			ResourceKind:  "Task",
			ResourceID:    taskID,
			ExecutionKind: monitoring.ExecutionKindTask,
			Metrics:       metrics,
		},
	)
}

func buildTaskSyncPayload(
	c *gin.Context,
	repo task.Repository,
	taskID string,
	execID core.ID,
	output *core.Output,
) gin.H {
	ctx := c.Request.Context()
	payload := gin.H{"exec_id": execID.String()}
	if output != nil {
		payload["output"] = output
	}
	includeState := includesStateDetails(c)
	snapshotCtx := context.WithoutCancel(ctx)
	embeddedUsage := false
	if includeState {
		stateSnapshot, stateErr := repo.GetState(snapshotCtx, execID)
		if stateErr == nil && stateSnapshot != nil {
			dto := newTaskExecutionStatusDTO(stateSnapshot)
			dto.Usage = router.NewUsageSummary(stateSnapshot.Usage)
			if dto.Usage != nil {
				embeddedUsage = true
			}
			payload["state"] = dto
			if stateSnapshot.Output != nil {
				payload["output"] = stateSnapshot.Output
			}
		} else if stateErr != nil {
			logger.FromContext(ctx).Warn(
				"Failed to load task execution state after completion",
				"task_id", taskID,
				"exec_id", execID.String(),
				"error", stateErr,
			)
		}
	}
	if !embeddedUsage {
		if summary := router.ResolveTaskUsageSummary(snapshotCtx, repo, execID); summary != nil {
			payload["usage"] = summary
		}
	}
	return payload
}

func includesStateDetails(c *gin.Context) bool {
	raw := c.Query("include")
	if raw == "" {
		return false
	}
	for _, token := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(token), "state") {
			return true
		}
	}
	return false
}

type TaskExecutionStatusDTO struct {
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

// TaskExecRequest represents the payload accepted by task execution endpoints.
type TaskExecRequest struct {
	// With passes structured input parameters to the task execution.
	With core.Input `json:"with,omitempty"`
	// Timeout in seconds for synchronous execution.
	Timeout *int `json:"timeout,omitempty"`
}

// executeTaskSync handles POST /tasks/{task_id}/executions/sync.
//
//	@Summary		Execute task synchronously
//	@Description	Execute a task and wait for the output in the same HTTP response.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path	string	true	"Task ID"	example("task-build-artifact")
//	@Param			X-Idempotency-Key	header	string	false	"Optional idempotency key to prevent duplicate execution"
//	@Param			payload	body	tkrouter.TaskExecRequest	true	"Execution request"
//	@Success		200	{object}	router.Response{data=tkrouter.TaskExecSyncResponse}	"Task executed"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Task not found"
//	@Failure		408	{object}	router.Response{error=router.ErrorInfo}	"Execution timeout"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/tasks/{task_id}/executions/sync [post]
func executeTaskSync(c *gin.Context) {
	setup, ok := validateTaskExecution(c)
	if !ok {
		return
	}
	outcome := monitoring.ExecutionOutcomeError
	defer func() { setup.finalize(outcome) }()
	prep, ok := prepareTaskExecution(c, setup)
	if !ok {
		return
	}
	outcome = executeTaskSyncAndRespond(c, setup, prep)
}

type syncTaskExecutionSetup struct {
	taskID        string
	state         *appstate.State
	resourceStore resources.ResourceStore
	req           *TaskExecRequest
	timeouts      taskExecutionTimeouts
	metrics       *monitoring.ExecutionMetrics
	finalize      func(string)
	recordError   func(int)
}

type syncTaskPreparation struct {
	taskCfg     *task.Config
	execTimeout time.Duration
	repo        task.Repository
	executor    DirectExecutor
	meta        ExecMetadata
}

func validateTaskExecution(c *gin.Context) (*syncTaskExecutionSetup, bool) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return nil, false
	}
	state := router.GetAppState(c)
	if state == nil {
		return nil, false
	}
	metrics, finalizeMetrics, recordError := router.SyncExecutionMetricsScope(c, state, monitoring.ExecutionKindTask)
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return nil, false
	}
	timeouts := resolveTaskExecutionTimeouts(c.Request.Context(), state)
	req, ok := parseTaskExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return nil, false
	}
	return &syncTaskExecutionSetup{
		taskID:        taskID,
		state:         state,
		resourceStore: resourceStore,
		req:           req,
		timeouts:      timeouts,
		metrics:       metrics,
		finalize:      finalizeMetrics,
		recordError:   recordError,
	}, true
}

func prepareTaskExecution(c *gin.Context, setup *syncTaskExecutionSetup) (*syncTaskPreparation, bool) {
	taskCfg, execTimeout, ok := loadDirectTaskConfig(
		c,
		setup.resourceStore,
		setup.state,
		setup.taskID,
		setup.req,
		setup.timeouts,
	)
	if !ok {
		setup.recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return nil, false
	}
	repo := router.ResolveTaskRepository(c, setup.state)
	if repo == nil {
		setup.recordError(http.StatusInternalServerError)
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	executor, err := ResolveDirectExecutor(setup.state, repo)
	if err != nil {
		setup.recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return nil, false
	}
	component := deriveTaskComponent(taskCfg)
	return &syncTaskPreparation{
		taskCfg:     taskCfg,
		execTimeout: execTimeout,
		repo:        repo,
		executor:    executor,
		meta:        ExecMetadata{Component: component, TaskID: taskCfg.ID},
	}, true
}

func executeTaskSyncAndRespond(
	c *gin.Context,
	setup *syncTaskExecutionSetup,
	prep *syncTaskPreparation,
) string {
	output, execID, execErr := prep.executor.ExecuteSync(
		c.Request.Context(),
		prep.taskCfg,
		&prep.meta,
		prep.execTimeout,
	)
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) {
			status := respondTaskTimeout(c, prep.repo, setup.taskID, execID, setup.metrics)
			setup.recordError(status)
			return monitoring.ExecutionOutcomeTimeout
		}
		setup.recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "task execution failed", execErr)
		return monitoring.ExecutionOutcomeError
	}
	router.RespondOK(
		c,
		"task executed",
		buildTaskSyncPayload(c, prep.repo, setup.taskID, execID, output),
	)
	return monitoring.ExecutionOutcomeSuccess
}

// executeTaskAsync handles POST /tasks/{task_id}/executions.
//
//	@Summary		Start task execution asynchronously
//	@Description	Start an asynchronous task execution and return a polling handle.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path	string	true	"Task ID"	example("task-build-artifact")
//	@Param			X-Correlation-ID	header	string	false	"Optional correlation ID for request tracing"
//	@Param			payload	body	tkrouter.TaskExecRequest	true	"Execution request"
//	@Success		202	{object}	router.Response{data=tkrouter.TaskExecAsyncResponse}	"Task execution started"
//	@Header			202	{string}	Location	"Execution status URL"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Task not found"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/tasks/{task_id}/executions [post]
func executeTaskAsync(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
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
			metrics.RecordError(ctx, monitoring.ExecutionKindTask, code)
		}
	}
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return
	}
	timeouts := resolveTaskExecutionTimeouts(c.Request.Context(), state)
	req, ok := parseTaskExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	taskCfg, _, ok := loadDirectTaskConfig(c, resourceStore, state, taskID, req, timeouts)
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
	executor, err := ResolveDirectExecutor(state, repo)
	if err != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return
	}
	component := deriveTaskComponent(taskCfg)
	meta := ExecMetadata{Component: component, TaskID: taskCfg.ID}
	execID, execErr := executor.ExecuteAsync(c.Request.Context(), taskCfg, &meta)
	if execErr != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "task execution failed", execErr)
		return
	}
	if metrics != nil {
		metrics.RecordAsyncStarted(ctx, monitoring.ExecutionKindTask)
	}
	execURL := fmt.Sprintf("%s/tasks/%s", routes.Executions(), execID.String())
	c.Header("Location", execURL)
	router.RespondAccepted(c, "task execution started", gin.H{"exec_id": execID.String(), "exec_url": execURL})
}

func deriveTaskComponent(cfg *task.Config) core.ComponentType {
	if cfg == nil {
		return core.ComponentTask
	}
	if cfg.Agent != nil {
		return core.ComponentAgent
	}
	if cfg.Tool != nil {
		return core.ComponentTool
	}
	return core.ComponentTask
}
