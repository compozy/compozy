package tkrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	// defaultTaskExecTimeoutSeconds defines the fallback timeout used when clients omit a value.
	defaultTaskExecTimeoutSeconds = 60
	// maxTaskExecTimeoutSeconds caps how long direct task executions may run.
	maxTaskExecTimeoutSeconds = 300
)

// getTaskExecutionStatus handles GET /executions/tasks/{exec_id}.
//
//	@Summary		Get task execution status
//	@Description	Retrieve the latest status for a direct task execution.
//	@Tags			executions
//	@Produce		json
//	@Param			exec_id	path	string	true	"Task execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Success		200	{object}	router.Response{data=tkrouter.TaskExecutionStatusDTO}	"Execution status retrieved"
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

func parseTaskExecRequest(c *gin.Context) (*TaskExecRequest, bool) {
	req := router.GetRequestBody[TaskExecRequest](c)
	if req == nil {
		return nil, false
	}
	if req.Timeout < 0 {
		reqErr := router.NewRequestError(http.StatusBadRequest, "timeout must be non-negative", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	if req.Timeout == 0 {
		req.Timeout = defaultTaskExecTimeoutSeconds
	}
	if req.Timeout > maxTaskExecTimeoutSeconds {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("timeout cannot exceed %d seconds", maxTaskExecTimeoutSeconds),
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func ensureTaskIdempotency(c *gin.Context, state *appstate.State, req *TaskExecRequest) bool {
	if state == nil {
		return false
	}
	idem := router.ResolveAPIIdempotency(c, state)
	if idem == nil {
		return true
	}
	body, err := json.Marshal(req)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to normalize request", err)
		return false
	}
	unique, reason, idemErr := idem.CheckAndSet(c.Request.Context(), c, "tasks", body, 0)
	if idemErr != nil {
		var reqErr *router.RequestError
		if errors.As(idemErr, &reqErr) {
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return false
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "idempotency check failed", idemErr)
		return false
	}
	if !unique {
		reqErr := router.NewRequestError(http.StatusConflict, "duplicate request", fmt.Errorf("%s", reason))
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return false
	}
	return true
}

func loadDirectTaskConfig(
	c *gin.Context,
	resourceStore resources.ResourceStore,
	state *appstate.State,
	taskID string,
	req *TaskExecRequest,
) (*task.Config, bool) {
	projectName := state.ProjectConfig.Name
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
	if taskCfg.ID == "" {
		taskCfg.ID = taskID
	}
	if len(req.With) > 0 {
		withCopy := req.With
		taskCfg.With = &withCopy
	}
	return taskCfg, true
}

func respondTaskTimeout(
	c *gin.Context,
	repo task.Repository,
	taskID string,
	execID core.ID,
	metrics *monitoring.ExecutionMetrics,
) int {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	payload := gin.H{"exec_id": execID.String()}
	if repo != nil {
		if state, err := repo.GetState(ctx, execID); err == nil {
			payload["state"] = newTaskExecutionStatusDTO(state)
		} else if err != nil && !errors.Is(err, store.ErrTaskNotFound) {
			log.Warn(
				"Failed to load task execution state after timeout",
				"task_id", taskID,
				"exec_id", execID.String(),
				"error", err,
			)
		}
	}
	log.Warn("Task execution timed out", "task_id", taskID, "exec_id", execID.String())
	resp := router.Response{
		Status:  http.StatusRequestTimeout,
		Message: "execution timeout",
		Data:    payload,
		Error: &router.ErrorInfo{
			Code:    router.ErrRequestTimeoutCode,
			Message: "execution timeout",
			Details: context.DeadlineExceeded.Error(),
		},
	}
	c.JSON(http.StatusRequestTimeout, resp)
	if metrics != nil {
		metrics.RecordTimeout(ctx, monitoring.ExecutionKindTask)
	}
	return http.StatusRequestTimeout
}

type TaskExecutionStatusDTO struct {
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

type TaskExecRequest struct {
	With    core.Input `json:"with,omitempty"`
	Timeout int        `json:"timeout,omitempty"`
}

// executeTaskSync handles POST /tasks/{task_id}/executions.
//
//	@Summary		Execute task synchronously
//	@Description	Execute a task and wait for the output in the same HTTP response.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path	string	true	"Task ID"	example("task-build-artifact")
//	@Param			payload	body	tkrouter.TaskExecRequest	true	"Execution request"
//	@Success		200	{object}	router.Response{data=tkrouter.TaskExecSyncResponse}	"Task executed"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Task not found"
//	@Failure		408	{object}	router.Response{error=router.ErrorInfo}	"Execution timeout"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/tasks/{task_id}/executions [post]
func executeTaskSync(c *gin.Context) {
	taskID := router.GetTaskID(c)
	if taskID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	metrics, finalizeMetrics, recordError := router.SyncExecutionMetricsScope(c, state, monitoring.ExecutionKindTask)
	outcome := monitoring.ExecutionOutcomeError
	defer func() { finalizeMetrics(outcome) }()
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		recordError(c.Writer.Status())
		return
	}
	req, ok := parseTaskExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	if !ensureTaskIdempotency(c, state, req) {
		recordError(c.Writer.Status())
		return
	}
	taskCfg, ok := loadDirectTaskConfig(c, resourceStore, state, taskID, req)
	if !ok {
		recordError(c.Writer.Status())
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
	output, execID, execErr := executor.ExecuteSync(
		c.Request.Context(),
		taskCfg,
		&meta,
		time.Duration(req.Timeout)*time.Second,
	)
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) {
			outcome = monitoring.ExecutionOutcomeTimeout
			status := respondTaskTimeout(c, repo, taskID, execID, metrics)
			recordError(status)
			return
		}
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "task execution failed", execErr)
		return
	}
	outcome = monitoring.ExecutionOutcomeSuccess
	router.RespondOK(c, "task executed", gin.H{"output": output, "exec_id": execID.String()})
}

// executeTaskAsync handles POST /tasks/{task_id}/executions/async.
//
//	@Summary		Start task execution asynchronously
//	@Description	Start an asynchronous task execution and return a polling handle.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path	string	true	"Task ID"	example("task-build-artifact")
//	@Param			payload	body	tkrouter.TaskExecRequest	true	"Execution request"
//	@Success		202	{object}	router.Response{data=tkrouter.TaskExecAsyncResponse}	"Task execution started"
//	@Header			202	{string}	Location	"Execution status URL"
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Task not found"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/tasks/{task_id}/executions/async [post]
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
		recordError(c.Writer.Status())
		return
	}
	req, ok := parseTaskExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	if !ensureTaskIdempotency(c, state, req) {
		recordError(c.Writer.Status())
		return
	}
	taskCfg, ok := loadDirectTaskConfig(c, resourceStore, state, taskID, req)
	if !ok {
		recordError(c.Writer.Status())
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
