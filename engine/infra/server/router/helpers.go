package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/importer"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	taskRepoContextKey         = "router.task_repo"
	apiIdempotencyContextKey   = "router.api_idempotency"
	workflowRepoContextKey     = "router.workflow_repo"
	workflowRunnerContextKey   = "router.workflow_runner"
	executionMetricsContextKey = "router.execution_metrics"
)

type WorkflowRunner interface {
	TriggerWorkflow(
		ctx context.Context,
		workflowID string,
		input *core.Input,
		initTaskID string,
	) (*worker.WorkflowInput, error)
}

func GetServerAddress(c *gin.Context) string {
	return c.Request.Host
}

func GetAppState(c *gin.Context) *appstate.State {
	appState, err := appstate.GetState(c.Request.Context())
	if err != nil {
		reqErr := NewRequestError(
			http.StatusInternalServerError,
			"failed to get application state",
			err,
		)
		log := logger.FromContext(c.Request.Context())
		log.Error("Failed to get app state", "error", err)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return appState
}

func GetAppStateWithWorker(c *gin.Context) *appstate.State {
	state := GetAppState(c)
	if state == nil {
		if !c.Writer.Written() {
			reqErr := NewRequestError(
				http.StatusServiceUnavailable,
				ErrMsgAppStateNotInitialized,
				nil,
			)
			RespondWithError(c, reqErr.StatusCode, reqErr)
		}
		return nil
	}
	if state.Worker == nil {
		reqErr := NewRequestError(
			http.StatusServiceUnavailable,
			ErrMsgWorkerNotRunning,
			nil,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return state
}

func GetWorker(c *gin.Context) *worker.Worker {
	state := GetAppStateWithWorker(c)
	if state == nil {
		return nil
	}
	return state.Worker
}

func GetResourceStore(c *gin.Context) (resources.ResourceStore, bool) {
	state := GetAppState(c)
	if state == nil {
		if !c.Writer.Written() {
			reqErr := NewRequestError(http.StatusInternalServerError, "application state not initialized", nil)
			RespondWithError(c, reqErr.StatusCode, reqErr)
		}
		return nil, false
	}
	v, ok := state.ResourceStore()
	if !ok || v == nil {
		reqErr := NewRequestError(http.StatusServiceUnavailable, "resource store not available", nil)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	rs, ok := v.(resources.ResourceStore)
	if !ok || rs == nil {
		reqErr := NewRequestError(http.StatusInternalServerError, "invalid resource store instance", nil)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return rs, true
}

func GetRequestBody[T any](c *gin.Context) *T {
	var input T
	if err := c.ShouldBindJSON(&input); err != nil {
		core.RespondProblem(c, &core.Problem{
			Status: http.StatusBadRequest,
			Detail: "invalid input",
			Extras: map[string]any{
				"code":   ErrBadRequestCode,
				"detail": err.Error(),
			},
		})
		return nil
	}

	return &input
}

func GetURLParam(c *gin.Context, key string) string {
	param := c.Param(key)
	if param == "" {
		reqErr := NewRequestError(
			http.StatusBadRequest,
			fmt.Sprintf("%s is required", key),
			nil,
		)
		RespondWithError(c, reqErr.StatusCode, reqErr)
		return ""
	}
	return param
}

func getExecIDParam(c *gin.Context, primary string, fallbacks ...string) string {
	if value := strings.TrimSpace(c.Param(primary)); value != "" {
		return value
	}
	for _, key := range fallbacks {
		if value := strings.TrimSpace(c.Param(key)); value != "" {
			return value
		}
	}
	return GetURLParam(c, primary)
}

func GetWorkflowID(c *gin.Context) string {
	return GetURLParam(c, "workflow_id")
}

func GetWorkflowExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "exec_id"))
}

func GetWorkflowStateID(c *gin.Context) string {
	return GetURLParam(c, "state_id")
}

func GetTaskID(c *gin.Context) string {
	return GetURLParam(c, "task_id")
}

// StatusOrFallback returns the response status if it already reflects an error;
// otherwise it falls back to the provided status code so metrics can record failures reliably.
func StatusOrFallback(c *gin.Context, fallback int) int {
	if status := c.Writer.Status(); status >= http.StatusBadRequest {
		return status
	}
	return fallback
}

func GetTaskExecID(c *gin.Context) core.ID {
	return core.ID(getExecIDParam(c, "exec_id", "task_exec_id"))
}

func GetAgentID(c *gin.Context) string {
	return GetURLParam(c, "agent_id")
}

func GetAgentExecID(c *gin.Context) core.ID {
	return core.ID(getExecIDParam(c, "exec_id", "agent_exec_id"))
}

func GetToolID(c *gin.Context) string {
	return GetURLParam(c, "tool_id")
}

func GetToolExecID(c *gin.Context) core.ID {
	return core.ID(GetURLParam(c, "tool_exec_id"))
}

func ProjectRootPath(st *appstate.State) (string, bool) {
	if st == nil || st.CWD == nil {
		return "", false
	}
	path := st.CWD.PathStr()
	if path == "" {
		return "", false
	}
	return path, true
}

func ParseImportStrategyParam(value string) (importer.Strategy, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", string(importer.SeedOnly):
		return importer.SeedOnly, nil
	case string(importer.OverwriteConflicts):
		return importer.OverwriteConflicts, nil
	default:
		return "", fmt.Errorf("invalid strategy (allowed: %q|%q)", importer.SeedOnly, importer.OverwriteConflicts)
	}
}

func UpdatedBy(c *gin.Context) string {
	usr, ok := userctx.UserFromContext(c.Request.Context())
	if !ok || usr == nil {
		return ""
	}
	if usr.Email != "" {
		return usr.Email
	}
	if usr.ID != "" {
		return usr.ID.String()
	}
	return ""
}

func SetTaskRepository(c *gin.Context, repo task.Repository) {
	if c == nil {
		return
	}
	c.Set(taskRepoContextKey, repo)
}

func SetWorkflowRepository(c *gin.Context, repo workflow.Repository) {
	if c == nil {
		return
	}
	c.Set(workflowRepoContextKey, repo)
}

func TaskRepositoryFromContext(c *gin.Context) (task.Repository, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(taskRepoContextKey)
	if !ok {
		return nil, false
	}
	repo, ok := v.(task.Repository)
	if !ok || repo == nil {
		return nil, false
	}
	return repo, true
}

func SetAPIIdempotency(c *gin.Context, idem APIIdempotency) {
	if c == nil || idem == nil {
		return
	}
	c.Set(apiIdempotencyContextKey, idem)
}

func setExecutionMetrics(c *gin.Context, metrics *monitoring.ExecutionMetrics) {
	if c == nil || metrics == nil {
		return
	}
	c.Set(executionMetricsContextKey, metrics)
}

func SetWorkflowRunner(c *gin.Context, runner WorkflowRunner) {
	if c == nil || runner == nil {
		return
	}
	c.Set(workflowRunnerContextKey, runner)
}

func APIIdempotencyFromContext(c *gin.Context) (APIIdempotency, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(apiIdempotencyContextKey)
	if !ok {
		return nil, false
	}
	idem, ok := v.(APIIdempotency)
	if !ok || idem == nil {
		return nil, false
	}
	return idem, true
}

func executionMetricsFromContext(c *gin.Context) (*monitoring.ExecutionMetrics, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(executionMetricsContextKey)
	if !ok {
		return nil, false
	}
	metrics, ok := v.(*monitoring.ExecutionMetrics)
	if !ok || metrics == nil {
		return nil, false
	}
	return metrics, true
}

func ResolveAPIIdempotency(c *gin.Context, state *appstate.State) APIIdempotency {
	if idem, ok := APIIdempotencyFromContext(c); ok {
		return idem
	}
	if state == nil {
		return nil
	}
	service, ok := state.APIIdempotencyService()
	if !ok || service == nil {
		return nil
	}
	idem := NewAPIIdempotency(service)
	SetAPIIdempotency(c, idem)
	return idem
}

func ResolveExecutionMetrics(c *gin.Context, state *appstate.State) *monitoring.ExecutionMetrics {
	if metrics, ok := executionMetricsFromContext(c); ok {
		return metrics
	}
	if state == nil {
		return nil
	}
	service, ok := state.MonitoringService()
	if !ok || service == nil || !service.IsInitialized() {
		return nil
	}
	metrics := service.ExecutionMetrics()
	if metrics != nil {
		setExecutionMetrics(c, metrics)
	}
	return metrics
}

func SyncExecutionMetricsScope(
	c *gin.Context,
	state *appstate.State,
	kind string,
) (*monitoring.ExecutionMetrics, func(string), func(int)) {
	metrics := ResolveExecutionMetrics(c, state)
	if metrics == nil {
		return nil, func(string) {}, func(int) {}
	}
	start := time.Now()
	finalize := func(outcome string) {
		if outcome == "" {
			outcome = monitoring.ExecutionOutcomeError
		}
		metrics.RecordSyncLatency(c.Request.Context(), kind, outcome, time.Since(start))
	}
	recordError := func(code int) {
		if code >= http.StatusBadRequest {
			metrics.RecordError(c.Request.Context(), kind, code)
		}
	}
	return metrics, finalize, recordError
}

func workflowRepositoryFromContext(c *gin.Context) (workflow.Repository, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(workflowRepoContextKey)
	if !ok {
		return nil, false
	}
	repo, ok := v.(workflow.Repository)
	if !ok || repo == nil {
		return nil, false
	}
	return repo, true
}

func workflowRunnerFromContext(c *gin.Context) (WorkflowRunner, bool) {
	if c == nil {
		return nil, false
	}
	v, ok := c.Get(workflowRunnerContextKey)
	if !ok {
		return nil, false
	}
	runner, ok := v.(WorkflowRunner)
	if !ok || runner == nil {
		return nil, false
	}
	return runner, true
}

func ResolveTaskRepository(c *gin.Context, state *appstate.State) task.Repository {
	if repo, ok := TaskRepositoryFromContext(c); ok {
		return repo
	}
	if state != nil && state.Store != nil {
		return state.Store.NewTaskRepo()
	}
	return nil
}

func ResolveWorkflowRepository(c *gin.Context, state *appstate.State) workflow.Repository {
	if repo, ok := workflowRepositoryFromContext(c); ok {
		return repo
	}
	if state != nil && state.Store != nil {
		repo := state.Store.NewWorkflowRepo()
		if repo != nil {
			SetWorkflowRepository(c, repo)
		}
		return repo
	}
	return nil
}

func ResolveWorkflowRunner(c *gin.Context, state *appstate.State) WorkflowRunner {
	if runner, ok := workflowRunnerFromContext(c); ok {
		return runner
	}
	if state != nil && state.Worker != nil {
		SetWorkflowRunner(c, state.Worker)
		return state.Worker
	}
	return nil
}
