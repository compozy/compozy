package agentrouter

import (
	"context"
	"encoding/json"
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
		req.Timeout = 60
	}
	if req.Timeout > 300 {
		reqErr := router.NewRequestError(http.StatusBadRequest, "timeout cannot exceed 300 seconds", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return req, true
}

func ensureAgentIdempotency(c *gin.Context, state *appstate.State, req *AgentExecRequest) bool {
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
	unique, reason, idemErr := idem.CheckAndSet(c.Request.Context(), c, "agents", body, 0)
	if idemErr != nil {
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
	return cfg, true
}

func respondAgentTimeout(
	c *gin.Context,
	repo task.Repository,
	agentID string,
	execID core.ID,
	metrics *monitoring.ExecutionMetrics,
) int {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	payload := gin.H{"exec_id": execID.String()}
	if repo != nil {
		if state, err := repo.GetState(ctx, execID); err == nil {
			payload["state"] = newExecutionStatusDTO(state)
		} else if err != nil && !errors.Is(err, store.ErrTaskNotFound) {
			log.Warn(
				"Failed to load agent execution state after timeout",
				"agent_id", agentID,
				"exec_id", execID.String(),
				"error", err,
			)
		}
	}
	log.Warn("Agent execution timed out", "agent_id", agentID, "exec_id", execID.String())
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
		metrics.RecordTimeout(ctx, monitoring.ExecutionKindAgent)
	}
	return http.StatusRequestTimeout
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

type AgentExecRequest struct {
	Action  string     `json:"action,omitempty"`
	Prompt  string     `json:"prompt,omitempty"`
	With    core.Input `json:"with,omitempty"`
	Timeout int        `json:"timeout,omitempty"`
}

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
		recordError(c.Writer.Status())
		return
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	if !ensureAgentIdempotency(c, state, req) {
		recordError(c.Writer.Status())
		return
	}
	taskCfg, ok := loadAgentTaskConfig(c, resourceStore, state, agentID, req)
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
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return
	}
	meta := tkrouter.ExecMetadata{
		Component:  core.ComponentAgent,
		AgentID:    agentID,
		ActionID:   req.Action,
		TaskID:     taskCfg.ID,
		WorkflowID: agentID,
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
	router.RespondOK(c, "agent executed", gin.H{"output": output, "exec_id": execID.String()})
}

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
		recordError(c.Writer.Status())
		return
	}
	req, ok := parseAgentExecRequest(c)
	if !ok {
		recordError(http.StatusBadRequest)
		return
	}
	if !ensureAgentIdempotency(c, state, req) {
		recordError(c.Writer.Status())
		return
	}
	taskCfg, ok := loadAgentTaskConfig(c, resourceStore, state, agentID, req)
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
	executor, err := tkrouter.ResolveDirectExecutor(state, repo)
	if err != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to initialize executor", err)
		return
	}
	meta := tkrouter.ExecMetadata{
		Component:  core.ComponentAgent,
		AgentID:    agentID,
		ActionID:   req.Action,
		TaskID:     taskCfg.ID,
		WorkflowID: agentID,
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
