package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/pkg/compozy/events"
)

const maxPageLimit = 500

// Handlers contains the shared daemon API handler logic used by both transports.
type Handlers struct {
	TransportName     string
	Logger            *slog.Logger
	Now               func() time.Time
	HeartbeatInterval time.Duration

	Daemon     DaemonService
	Workspaces WorkspaceService
	Tasks      TaskService
	Reviews    ReviewService
	Runs       RunService
	Sync       SyncService
	Exec       ExecService

	settingsMu sync.RWMutex
	streamDone <-chan struct{}
	httpPort   *atomic.Int64
}

// NewHandlers builds the shared handler set with transport-specific defaults applied.
func NewHandlers(cfg *HandlerConfig) *Handlers {
	if cfg == nil {
		cfg = &HandlerConfig{}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	interval := cfg.HeartbeatInterval
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}

	done := cfg.StreamDone
	if done == nil {
		done = make(chan struct{})
	}

	return &Handlers{
		TransportName:     strings.TrimSpace(cfg.TransportName),
		Logger:            logger,
		Now:               now,
		HeartbeatInterval: interval,
		Daemon:            cfg.Daemon,
		Workspaces:        cfg.Workspaces,
		Tasks:             cfg.Tasks,
		Reviews:           cfg.Reviews,
		Runs:              cfg.Runs,
		Sync:              cfg.Sync,
		Exec:              cfg.Exec,
		streamDone:        done,
		httpPort:          &atomic.Int64{},
	}
}

// Clone copies the handler set so each transport can own its runtime state independently.
func (h *Handlers) Clone() *Handlers {
	if h == nil {
		return nil
	}

	clone := NewHandlers(&HandlerConfig{
		TransportName:     h.TransportName,
		Logger:            h.Logger,
		Now:               h.Now,
		HeartbeatInterval: h.HeartbeatInterval,
		StreamDone:        h.streamDoneChannel(),
		Daemon:            h.Daemon,
		Workspaces:        h.Workspaces,
		Tasks:             h.Tasks,
		Reviews:           h.Reviews,
		Runs:              h.Runs,
		Sync:              h.Sync,
		Exec:              h.Exec,
	})
	clone.httpPort = h.httpPort
	return clone
}

// SetStreamDone updates the transport shutdown bridge used by streaming handlers.
func (h *Handlers) SetStreamDone(done <-chan struct{}) {
	if h == nil {
		return
	}
	if done == nil {
		done = make(chan struct{})
	}
	h.settingsMu.Lock()
	h.streamDone = done
	h.settingsMu.Unlock()
}

// SetHTTPPort overrides the reported HTTP port for daemon status responses.
func (h *Handlers) SetHTTPPort(port int) {
	if h == nil || port <= 0 {
		return
	}
	if h.httpPort == nil {
		h.httpPort = &atomic.Int64{}
	}
	h.httpPort.Store(int64(port))
}

func (h *Handlers) transportName() string {
	if h == nil || strings.TrimSpace(h.TransportName) == "" {
		return "api"
	}
	return h.TransportName
}

func (h *Handlers) now() time.Time {
	if h == nil || h.Now == nil {
		return time.Now().UTC()
	}
	return h.Now().UTC()
}

func (h *Handlers) streamDoneChannel() <-chan struct{} {
	if h == nil {
		return nil
	}
	h.settingsMu.RLock()
	defer h.settingsMu.RUnlock()
	return h.streamDone
}

func (h *Handlers) respondError(c *gin.Context, err error) {
	RespondError(c, err)
}

func (h *Handlers) bindJSON(c *gin.Context, action string, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		h.respondError(c, invalidJSONProblem(h.transportName(), action, err))
		return false
	}
	return true
}

func (h *Handlers) requireWorkspaceRef(c *gin.Context, value string) (string, bool) {
	workspace := strings.TrimSpace(value)
	if workspace == "" {
		h.respondError(c, validationProblem("workspace_required", "workspace is required", nil))
		return "", false
	}
	return workspace, true
}

func (h *Handlers) requireWorkspaceContext(c *gin.Context, value string) (string, bool) {
	workspace := strings.TrimSpace(value)
	if workspace == "" {
		h.respondError(c, workspaceContextProblem(
			"workspace_context_missing",
			"active workspace context is required",
			nil,
			nil,
		))
		return "", false
	}
	return workspace, true
}

func (h *Handlers) respondWorkspaceContextError(c *gin.Context, workspaceRef string, err error) {
	if errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		h.respondError(c, workspaceContextProblem(
			"workspace_context_stale",
			"active workspace context is stale",
			map[string]any{"workspace": strings.TrimSpace(workspaceRef)},
			err,
		))
		return
	}
	h.respondError(c, err)
}

func requireNonEmptyString(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return validationProblem(
			field+"_required",
			fmt.Sprintf("%s is required", strings.ReplaceAll(field, "_", " ")),
			map[string]any{"field": field},
		)
	}
	return nil
}

func parsePositiveInt(value string, field string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed <= 0 {
		return 0, validationProblem(
			field+"_invalid",
			fmt.Sprintf("%s must be a positive integer", strings.ReplaceAll(field, "_", " ")),
			map[string]any{"field": field},
		)
	}
	return parsed, nil
}

func parseOptionalBool(value string, field string) (bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, validationProblem(
			field+"_invalid",
			fmt.Sprintf("%s must be a boolean", strings.ReplaceAll(field, "_", " ")),
			map[string]any{"field": field},
		)
	}
	return parsed, nil
}

func parseCursorHeader(raw string) (StreamCursor, error) {
	cursor, err := ParseCursor(raw)
	if err != nil {
		return StreamCursor{}, validationProblem(
			"invalid_cursor",
			err.Error(),
			map[string]any{"header": "Last-Event-ID"},
		)
	}
	return cursor, nil
}

func parseCursorQuery(raw string) (StreamCursor, error) {
	cursor, err := ParseCursor(raw)
	if err != nil {
		return StreamCursor{}, validationProblem(
			"invalid_cursor",
			err.Error(),
			map[string]any{"field": "after"},
		)
	}
	return cursor, nil
}

type workspaceRegisterBody = contract.WorkspaceRegisterRequest
type workspaceUpdateBody = contract.WorkspaceUpdateRequest
type workspaceResolveBody = contract.WorkspaceResolveRequest
type workflowRefBody = contract.WorkflowRefRequest
type taskRunBody = contract.TaskRunRequest
type reviewFetchBody = contract.ReviewFetchRequest
type reviewRunBody = contract.ReviewRunRequest
type syncBody = contract.SyncRequest
type execBody = contract.ExecRequest

// DaemonStatus returns the primary daemon status view.
func (h *Handlers) DaemonStatus(c *gin.Context) {
	if h.Daemon == nil {
		h.respondError(c, serviceUnavailableProblem("daemon service"))
		return
	}

	status, err := h.Daemon.Status(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}

	if h.httpPort != nil {
		if httpPort := int(h.httpPort.Load()); httpPort > 0 {
			status.HTTPPort = httpPort
		}
	}

	c.JSON(http.StatusOK, contract.DaemonStatusResponse{Daemon: status})
}

// DaemonHealth returns daemon readiness and degraded-state details.
func (h *Handlers) DaemonHealth(c *gin.Context) {
	if h.Daemon == nil {
		h.respondError(c, serviceUnavailableProblem("daemon service"))
		return
	}

	health, err := h.Daemon.Health(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}

	status := http.StatusOK
	if !health.Ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, contract.DaemonHealthResponse{Health: health})
}

// DaemonMetrics returns the daemon metrics in Prometheus text format.
func (h *Handlers) DaemonMetrics(c *gin.Context) {
	if h.Daemon == nil {
		h.respondError(c, serviceUnavailableProblem("daemon service"))
		return
	}

	metrics, err := h.Daemon.Metrics(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}

	contentType := strings.TrimSpace(metrics.ContentType)
	if contentType == "" {
		contentType = "text/plain; version=0.0.4; charset=utf-8"
	}
	c.Data(http.StatusOK, contentType, []byte(metrics.Body))
}

// StopDaemon requests a graceful daemon shutdown.
func (h *Handlers) StopDaemon(c *gin.Context) {
	if h.Daemon == nil {
		h.respondError(c, serviceUnavailableProblem("daemon service"))
		return
	}

	force, err := parseOptionalBool(c.Query("force"), "force")
	if err != nil {
		h.respondError(c, err)
		return
	}

	if err := h.Daemon.Stop(c.Request.Context(), force); err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, contract.MutationAcceptedResponse{Accepted: true})
}

// RegisterWorkspace registers a workspace explicitly.
func (h *Handlers) RegisterWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	var body workspaceRegisterBody
	if !h.bindJSON(c, "decode register workspace request", &body) {
		return
	}
	if err := requireNonEmptyString("path", body.Path); err != nil {
		h.respondError(c, err)
		return
	}

	result, err := h.Workspaces.Register(c.Request.Context(), body.Path, body.Name)
	if err != nil {
		h.respondError(c, err)
		return
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	c.JSON(status, contract.WorkspaceResponse{Workspace: result.Workspace})
}

// ListWorkspaces lists registered workspaces.
func (h *Handlers) ListWorkspaces(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	workspaces, err := h.Workspaces.List(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceListResponse{Workspaces: workspaces})
}

// GetWorkspace returns one workspace by ID or normalized path key.
func (h *Handlers) GetWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	workspace, err := h.Workspaces.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceResponse{Workspace: workspace})
}

// UpdateWorkspace updates mutable workspace metadata.
func (h *Handlers) UpdateWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	var body workspaceUpdateBody
	if !h.bindJSON(c, "decode update workspace request", &body) {
		return
	}
	if err := requireNonEmptyString("name", body.Name); err != nil {
		h.respondError(c, err)
		return
	}

	workspace, err := h.Workspaces.Update(
		c.Request.Context(),
		c.Param("id"),
		WorkspaceUpdateInput(body),
	)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceResponse{Workspace: workspace})
}

// DeleteWorkspace unregisters one workspace.
func (h *Handlers) DeleteWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	if err := h.Workspaces.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ResolveWorkspace resolves or lazily registers a workspace path.
func (h *Handlers) ResolveWorkspace(c *gin.Context) {
	if h.Workspaces == nil {
		h.respondError(c, serviceUnavailableProblem("workspace service"))
		return
	}

	var body workspaceResolveBody
	if !h.bindJSON(c, "decode resolve workspace request", &body) {
		return
	}
	if err := requireNonEmptyString("path", body.Path); err != nil {
		h.respondError(c, err)
		return
	}

	workspace, err := h.Workspaces.Resolve(c.Request.Context(), body.Path)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceResponse{Workspace: workspace})
}

// GetDashboard returns the active-workspace dashboard aggregate.
func (h *Handlers) GetDashboard(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	dashboard, err := h.Tasks.Dashboard(c.Request.Context(), workspace)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"dashboard": dashboard})
}

// ListTaskWorkflows lists task workflows for one workspace.
func (h *Handlers) ListTaskWorkflows(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	workflows, err := h.Tasks.ListWorkflows(c.Request.Context(), workspace)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskWorkflowListResponse{Workflows: workflows})
}

// GetTaskWorkflow returns the richer workflow overview payload.
func (h *Handlers) GetTaskWorkflow(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	workflow, err := h.Tasks.WorkflowOverview(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"workflow": workflow})
}

// ListTaskItems lists parsed task items for one workflow.
func (h *Handlers) ListTaskItems(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	items, err := h.Tasks.ListItems(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskItemsResponse{Items: items})
}

// GetTaskBoard returns the workflow task-board read model.
func (h *Handlers) GetTaskBoard(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	board, err := h.Tasks.TaskBoard(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"board": board})
}

// GetWorkflowSpec returns the workflow PRD, TechSpec, and ADR documents.
func (h *Handlers) GetWorkflowSpec(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	spec, err := h.Tasks.WorkflowSpec(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"spec": spec})
}

// GetWorkflowMemory returns the memory index for one workflow.
func (h *Handlers) GetWorkflowMemory(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	memory, err := h.Tasks.WorkflowMemoryIndex(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"memory": memory})
}

// GetWorkflowMemoryFile returns one workflow memory document by opaque daemon-issued ID.
func (h *Handlers) GetWorkflowMemoryFile(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	document, err := h.Tasks.WorkflowMemoryFile(
		c.Request.Context(),
		workspace,
		c.Param("slug"),
		c.Param("file_id"),
	)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"document": document})
}

// GetTaskItemDetail returns the richer workflow task detail payload.
func (h *Handlers) GetTaskItemDetail(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	taskDetail, err := h.Tasks.TaskDetail(
		c.Request.Context(),
		workspace,
		c.Param("slug"),
		c.Param("task_id"),
	)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": taskDetail})
}

// ValidateTaskWorkflow validates task files for one workflow.
func (h *Handlers) ValidateTaskWorkflow(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	var body workflowRefBody
	if !h.bindJSON(c, "decode validate task request", &body) {
		return
	}
	workspace, ok := h.requireWorkspaceRef(c, body.Workspace)
	if !ok {
		return
	}

	result, err := h.Tasks.Validate(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// StartTaskRun starts one task workflow run.
func (h *Handlers) StartTaskRun(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	var body taskRunBody
	if !h.bindJSON(c, "decode task run request", &body) {
		return
	}
	workspace, ok := h.requireWorkspaceContext(c, body.Workspace)
	if !ok {
		return
	}

	run, err := h.Tasks.StartRun(c.Request.Context(), workspace, c.Param("slug"), TaskRunRequest{
		Workspace:        workspace,
		PresentationMode: strings.TrimSpace(body.PresentationMode),
		RuntimeOverrides: body.RuntimeOverrides,
	})
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusCreated, contract.RunResponse{Run: run})
}

// ArchiveTaskWorkflow archives a completed workflow.
func (h *Handlers) ArchiveTaskWorkflow(c *gin.Context) {
	if h.Tasks == nil {
		h.respondError(c, serviceUnavailableProblem("task service"))
		return
	}

	var body workflowRefBody
	if !h.bindJSON(c, "decode archive workflow request", &body) {
		return
	}
	workspace, ok := h.requireWorkspaceContext(c, body.Workspace)
	if !ok {
		return
	}

	result, err := h.Tasks.Archive(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// FetchReview imports provider feedback into a review round.
func (h *Handlers) FetchReview(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	var body reviewFetchBody
	if !h.bindJSON(c, "decode review fetch request", &body) {
		return
	}
	workspace, ok := h.requireWorkspaceRef(c, body.Workspace)
	if !ok {
		return
	}
	if body.Round != nil && *body.Round <= 0 {
		h.respondError(c, validationProblem(
			"round_invalid",
			"round must be a positive integer",
			map[string]any{"field": "round"},
		))
		return
	}

	result, err := h.Reviews.Fetch(c.Request.Context(), workspace, c.Param("slug"), ReviewFetchRequest{
		Workspace: workspace,
		Provider:  strings.TrimSpace(body.Provider),
		PRRef:     strings.TrimSpace(body.PRRef),
		Round:     body.Round,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	c.JSON(status, contract.ReviewFetchResponse{Review: result.Summary})
}

// GetLatestReview returns the latest review summary for one workflow.
func (h *Handlers) GetLatestReview(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}

	review, err := h.Reviews.GetLatest(c.Request.Context(), workspace, c.Param("slug"))
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, contract.ReviewSummaryResponse{Review: review})
}

// GetReviewRound returns one review round summary.
func (h *Handlers) GetReviewRound(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}
	round, err := parsePositiveInt(c.Param("round"), "round")
	if err != nil {
		h.respondError(c, err)
		return
	}

	reviewRound, err := h.Reviews.GetRound(c.Request.Context(), workspace, c.Param("slug"), round)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, contract.ReviewRoundResponse{Round: reviewRound})
}

// ListReviewIssues returns one review round issue list.
func (h *Handlers) ListReviewIssues(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}
	round, err := parsePositiveInt(c.Param("round"), "round")
	if err != nil {
		h.respondError(c, err)
		return
	}

	issues, err := h.Reviews.ListIssues(c.Request.Context(), workspace, c.Param("slug"), round)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, contract.ReviewIssuesResponse{Issues: issues})
}

// GetReviewIssue returns the richer review issue detail payload.
func (h *Handlers) GetReviewIssue(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	workspace, ok := h.requireWorkspaceContext(c, c.Query("workspace"))
	if !ok {
		return
	}
	round, err := parsePositiveInt(c.Param("round"), "round")
	if err != nil {
		h.respondError(c, err)
		return
	}

	review, err := h.Reviews.ReviewDetail(
		c.Request.Context(),
		workspace,
		c.Param("slug"),
		round,
		c.Param("issue_id"),
	)
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"review": review})
}

// StartReviewRun starts one review-fix run.
func (h *Handlers) StartReviewRun(c *gin.Context) {
	if h.Reviews == nil {
		h.respondError(c, serviceUnavailableProblem("review service"))
		return
	}

	var body reviewRunBody
	if !h.bindJSON(c, "decode review run request", &body) {
		return
	}
	workspace, ok := h.requireWorkspaceContext(c, body.Workspace)
	if !ok {
		return
	}
	round, err := parsePositiveInt(c.Param("round"), "round")
	if err != nil {
		h.respondError(c, err)
		return
	}

	run, err := h.Reviews.StartRun(c.Request.Context(), workspace, c.Param("slug"), round, ReviewRunRequest{
		Workspace:        workspace,
		PresentationMode: strings.TrimSpace(body.PresentationMode),
		RuntimeOverrides: body.RuntimeOverrides,
		Batching:         body.Batching,
	})
	if err != nil {
		h.respondWorkspaceContextError(c, workspace, err)
		return
	}
	c.JSON(http.StatusCreated, contract.RunResponse{Run: run})
}

// ListRuns lists runs across workspaces or for one workspace.
func (h *Handlers) ListRuns(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	limit, err := parsePositiveInt(c.Query("limit"), "limit")
	if err != nil {
		h.respondError(c, err)
		return
	}
	if limit == 0 {
		limit = 100
	}
	if limit > maxPageLimit {
		h.respondError(c, validationProblem(
			"limit_invalid",
			fmt.Sprintf("limit must be less than or equal to %d", maxPageLimit),
			map[string]any{"field": "limit"},
		))
		return
	}

	runs, err := h.Runs.List(c.Request.Context(), RunListQuery{
		Workspace: strings.TrimSpace(c.Query("workspace")),
		Status:    strings.TrimSpace(c.Query("status")),
		Mode:      strings.TrimSpace(c.Query("mode")),
		Limit:     limit,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.RunListResponse{Runs: runs})
}

// GetRun returns one run summary.
func (h *Handlers) GetRun(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	run, err := h.Runs.Get(c.Request.Context(), c.Param("run_id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.RunResponse{Run: run})
}

// GetRunSnapshot returns the attach snapshot for one run.
func (h *Handlers) GetRunSnapshot(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	snapshot, err := h.Runs.Snapshot(c.Request.Context(), c.Param("run_id"))
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, contract.RunSnapshotResponseFromSnapshot(snapshot))
}

// ListRunEvents pages through persisted run events.
func (h *Handlers) ListRunEvents(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	after, err := parseCursorQuery(c.Query("after"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	limit, err := parsePositiveInt(c.Query("limit"), "limit")
	if err != nil {
		h.respondError(c, err)
		return
	}
	if limit == 0 {
		limit = 100
	}
	if limit > maxPageLimit {
		h.respondError(c, validationProblem(
			"limit_invalid",
			fmt.Sprintf("limit must be less than or equal to %d", maxPageLimit),
			map[string]any{"field": "limit"},
		))
		return
	}

	page, err := h.Runs.Events(c.Request.Context(), c.Param("run_id"), RunEventPageQuery{
		After: after,
		Limit: limit,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, contract.RunEventPageResponseFromPage(page))
}

// StreamRun streams live run events with cursor resume, heartbeat, and overflow semantics.
func (h *Handlers) StreamRun(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	after, err := parseCursorHeader(c.GetHeader("Last-Event-ID"))
	if err != nil {
		h.respondError(c, err)
		return
	}

	stream, err := h.Runs.OpenStream(c.Request.Context(), c.Param("run_id"), after)
	if err != nil {
		h.respondError(c, err)
		return
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil && h != nil && h.Logger != nil {
			h.Logger.Warn("close run stream", "run_id", c.Param("run_id"), "error", closeErr)
		}
	}()

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, NewProblem(http.StatusInternalServerError, "stream_unavailable", err.Error(), nil, err))
		return
	}
	h.streamRunLoop(c, writer, stream, after)
}

func (h *Handlers) streamRunLoop(c *gin.Context, writer FlushWriter, stream RunStream, after StreamCursor) {
	timer := time.NewTimer(h.HeartbeatInterval)
	defer timer.Stop()

	lastCursor := after
	runID := c.Param("run_id")
	requestCtx := c.Request.Context()
	streamDone := h.streamDoneChannel()
	eventCh := stream.Events()
	errCh := stream.Errors()
	for {
		select {
		case <-requestCtx.Done():
			return
		case <-streamDone:
			return
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err == nil {
				continue
			}
			if err := h.writeStreamError(requestCtx, writer, err); err != nil {
				return
			}
			return
		case item, ok := <-eventCh:
			if !ok {
				return
			}
			outcome, err := h.writeStreamItem(writer, runID, lastCursor, item)
			if err != nil {
				return
			}
			lastCursor = outcome.Cursor
			if outcome.ResetHeartbeat {
				resetTimer(timer, h.HeartbeatInterval)
			}
			if outcome.Stop {
				return
			}
		case <-timer.C:
			if err := h.writeStreamHeartbeat(writer, runID, lastCursor); err != nil {
				return
			}
			resetTimer(timer, h.HeartbeatInterval)
		}
	}
}

// CancelRun requests cancellation for one run.
func (h *Handlers) CancelRun(c *gin.Context) {
	if h.Runs == nil {
		h.respondError(c, serviceUnavailableProblem("run service"))
		return
	}

	if err := h.Runs.Cancel(c.Request.Context(), c.Param("run_id")); err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, contract.MutationAcceptedResponse{Accepted: true})
}

// SyncWorkflow runs explicit daemon reconciliation for a workspace or workflow.
func (h *Handlers) SyncWorkflow(c *gin.Context) {
	if h.Sync == nil {
		h.respondError(c, serviceUnavailableProblem("sync service"))
		return
	}

	var body syncBody
	if !h.bindJSON(c, "decode sync request", &body) {
		return
	}

	if strings.TrimSpace(body.Workspace) == "" && strings.TrimSpace(body.Path) == "" {
		h.respondError(c, validationProblem(
			"sync_target_required",
			"workspace or path is required",
			nil,
		))
		return
	}

	result, err := h.Sync.Sync(c.Request.Context(), SyncRequest{
		Workspace:    strings.TrimSpace(body.Workspace),
		Path:         strings.TrimSpace(body.Path),
		WorkflowSlug: strings.TrimSpace(body.WorkflowSlug),
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// StartExecRun starts one ad-hoc daemon-backed exec run.
func (h *Handlers) StartExecRun(c *gin.Context) {
	if h.Exec == nil {
		h.respondError(c, serviceUnavailableProblem("exec service"))
		return
	}

	var body execBody
	if !h.bindJSON(c, "decode exec request", &body) {
		return
	}
	if err := requireNonEmptyString("workspace_path", body.WorkspacePath); err != nil {
		h.respondError(c, err)
		return
	}
	if err := requireNonEmptyString("prompt", body.Prompt); err != nil {
		h.respondError(c, err)
		return
	}

	run, err := h.Exec.Start(c.Request.Context(), ExecRequest{
		WorkspacePath:    strings.TrimSpace(body.WorkspacePath),
		Prompt:           strings.TrimSpace(body.Prompt),
		PresentationMode: strings.TrimSpace(body.PresentationMode),
		RuntimeOverrides: body.RuntimeOverrides,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, contract.RunResponse{Run: run})
}

func (h *Handlers) writeStreamError(ctx context.Context, writer FlushWriter, err error) error {
	status := statusForError(err)
	return WriteSSE(writer, SSEMessage{
		Event: "error",
		Data: contract.TransportErrorEnvelope(
			RequestIDFromContext(ctx),
			status,
			err,
			detailsForError(err),
			true,
		),
	})
}

type streamItemOutcome struct {
	Cursor         StreamCursor
	ResetHeartbeat bool
	Stop           bool
}

func (h *Handlers) writeStreamItem(
	writer FlushWriter,
	runID string,
	lastCursor StreamCursor,
	item RunStreamItem,
) (streamItemOutcome, error) {
	switch {
	case item.Overflow != nil:
		return streamItemOutcome{Cursor: lastCursor, Stop: true}, WriteSSE(
			writer,
			OverflowMessage(runID, lastCursor, h.now(), item.Overflow.Reason),
		)
	case item.Event == nil:
		return streamItemOutcome{Cursor: lastCursor}, nil
	default:
		cursor := CursorFromEvent(*item.Event)
		message := SSEMessage{
			ID:    FormatCursor(item.Event.Timestamp, item.Event.Seq),
			Event: string(item.Event.Kind),
			Data:  item.Event,
		}
		if err := WriteSSE(writer, message); err != nil {
			return streamItemOutcome{}, err
		}
		return streamItemOutcome{
			Cursor:         cursor,
			ResetHeartbeat: true,
			Stop:           isTerminalRunEvent(item.Event.Kind),
		}, nil
	}
}

func (h *Handlers) writeStreamHeartbeat(writer FlushWriter, runID string, lastCursor StreamCursor) error {
	return WriteSSE(writer, HeartbeatMessage(runID, lastCursor, h.now()))
}

func isTerminalRunEvent(kind events.EventKind) bool {
	switch kind {
	case events.EventKindRunCompleted,
		events.EventKindRunFailed,
		events.EventKindRunCancelled,
		events.EventKindShutdownRequested,
		events.EventKindShutdownDraining,
		events.EventKindShutdownTerminated:
		return true
	default:
		return false
	}
}
