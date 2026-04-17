package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/reviews"
	runpkg "github.com/compozy/compozy/internal/core/run"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	runModeTask   = "task"
	runModeReview = "review"
	runModeExec   = "exec"

	runStatusStarting  = "starting"
	runStatusRunning   = "running"
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
	runStatusCancelled = "cancelled"

	defaultRunListLimit     = 100
	defaultPresentationMode = "stream"
	maxRunEventPageLimit    = 1000
	runShutdownTimeout      = time.Second
	runStreamBufferSize     = 64
	defaultStreamPageLimit  = 256
	cancelRequestedByDaemon = "daemon"
	completedNoWorkSummary  = "no work"
)

// RunManagerConfig wires the daemon-owned run manager dependencies.
type RunManagerConfig struct {
	GlobalDB          *globaldb.GlobalDB
	LifecycleContext  context.Context
	Now               func() time.Time
	OpenRunScope      func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	Prepare           func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	Execute           func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	ExecuteExec       func(context.Context, *model.RuntimeConfig, model.RunScope) error
	LoadProjectConfig func(context.Context, string) (workspacecfg.ProjectConfig, error)
}

// RunManager owns daemon-backed task, review, and exec runs.
type RunManager struct {
	globalDB          *globaldb.GlobalDB
	lifecycleCtx      context.Context
	now               func() time.Time
	openRunScope      func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	prepare           func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	execute           func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	executeExec       func(context.Context, *model.RuntimeConfig, model.RunScope) error
	loadProjectConfig func(context.Context, string) (workspacecfg.ProjectConfig, error)

	mu     sync.RWMutex
	active map[string]*activeRun
}

type activeRun struct {
	runID        string
	workspaceID  string
	workflowSlug string
	mode         string
	scope        model.RunScope
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}

	stateMu         sync.RWMutex
	cancelRequested bool
}

type runtimeOverrideInput struct {
	RunID                      *string   `json:"run_id"`
	IDE                        *string   `json:"ide"`
	Model                      *string   `json:"model"`
	OutputFormat               *string   `json:"output_format"`
	ReasoningEffort            *string   `json:"reasoning_effort"`
	AccessMode                 *string   `json:"access_mode"`
	Timeout                    *string   `json:"timeout"`
	TailLines                  *int      `json:"tail_lines"`
	AddDirs                    *[]string `json:"add_dirs"`
	AutoCommit                 *bool     `json:"auto_commit"`
	MaxRetries                 *int      `json:"max_retries"`
	RetryBackoffMultiplier     *float64  `json:"retry_backoff_multiplier"`
	Concurrent                 *int      `json:"concurrent"`
	BatchSize                  *int      `json:"batch_size"`
	Verbose                    *bool     `json:"verbose"`
	Persist                    *bool     `json:"persist"`
	IncludeCompleted           *bool     `json:"include_completed"`
	IncludeResolved            *bool     `json:"include_resolved"`
	TUI                        *bool     `json:"tui"`
	EnableExecutableExtensions *bool     `json:"enable_executable_extensions"`
}

type reviewBatchingInput struct {
	Concurrent      *int  `json:"concurrent"`
	BatchSize       *int  `json:"batch_size"`
	IncludeResolved *bool `json:"include_resolved"`
}

type startRunSpec struct {
	workspace        globaldb.Workspace
	workflowID       *string
	workflowSlug     string
	mode             string
	presentationMode string
	runtimeCfg       *model.RuntimeConfig
}

type terminalState struct {
	status    string
	errorText string
	kind      eventspkg.EventKind
	payload   any
}

type runStream struct {
	events chan apicore.RunStreamItem
	errors chan error
	close  func() error
}

var _ apicore.RunService = (*RunManager)(nil)

// NewRunManager constructs a daemon-owned run manager.
func NewRunManager(cfg RunManagerConfig) (*RunManager, error) {
	if cfg.GlobalDB == nil {
		return nil, errors.New("daemon: run manager global db is required")
	}

	lifecycleCtx := cfg.LifecycleContext
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}

	now := cfg.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	openRunScope := cfg.OpenRunScope
	if openRunScope == nil {
		openRunScope = model.OpenRunScope
	}

	prepare := cfg.Prepare
	if prepare == nil {
		prepare = plan.Prepare
	}

	execute := cfg.Execute
	if execute == nil {
		execute = func(ctx context.Context, prep *model.SolvePreparation, runtimeCfg *model.RuntimeConfig) error {
			if prep == nil {
				return errors.New("daemon: workflow preparation is required")
			}
			return runpkg.Execute(
				ctx,
				prep.Jobs,
				prep.RunArtifacts,
				prep.Journal(),
				prep.EventBus(),
				runtimeCfg,
				prep.RuntimeManager(),
			)
		}
	}

	executeExec := cfg.ExecuteExec
	if executeExec == nil {
		executeExec = runpkg.ExecuteExec
	}

	loadProjectConfig := cfg.LoadProjectConfig
	if loadProjectConfig == nil {
		loadProjectConfig = func(ctx context.Context, root string) (workspacecfg.ProjectConfig, error) {
			projectCfg, _, err := workspacecfg.LoadConfig(ctx, root)
			return projectCfg, err
		}
	}

	return &RunManager{
		globalDB:          cfg.GlobalDB,
		lifecycleCtx:      lifecycleCtx,
		now:               now,
		openRunScope:      openRunScope,
		prepare:           prepare,
		execute:           execute,
		executeExec:       executeExec,
		loadProjectConfig: loadProjectConfig,
		active:            make(map[string]*activeRun),
	}, nil
}

// StartTaskRun starts one daemon-owned task workflow run.
func (m *RunManager) StartTaskRun(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.TaskRunRequest,
) (apicore.Run, error) {
	workspaceRow, workflowID, runtimeCfg, presentationMode, err := m.prepareTaskStart(
		detachContext(ctx),
		workspaceRef,
		workflowSlug,
		req,
	)
	if err != nil {
		return apicore.Run{}, err
	}

	return m.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     strings.TrimSpace(workflowSlug),
		mode:             runModeTask,
		presentationMode: presentationMode,
		runtimeCfg:       runtimeCfg,
	})
}

// StartReviewRun starts one daemon-owned review-fix run.
func (m *RunManager) StartReviewRun(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	workspaceRow, workflowID, runtimeCfg, presentationMode, err := m.prepareReviewStart(
		detachContext(ctx),
		workspaceRef,
		workflowSlug,
		round,
		req,
	)
	if err != nil {
		return apicore.Run{}, err
	}

	return m.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     strings.TrimSpace(workflowSlug),
		mode:             runModeReview,
		presentationMode: presentationMode,
		runtimeCfg:       runtimeCfg,
	})
}

// StartExecRun starts one daemon-owned exec run.
func (m *RunManager) StartExecRun(
	ctx context.Context,
	req apicore.ExecRequest,
) (apicore.Run, error) {
	workspaceRow, runtimeCfg, presentationMode, err := m.prepareExecStart(detachContext(ctx), req)
	if err != nil {
		return apicore.Run{}, err
	}

	return m.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		mode:             runModeExec,
		presentationMode: presentationMode,
		runtimeCfg:       runtimeCfg,
	})
}

// List returns durable run summaries filtered by workspace, mode, or status.
func (m *RunManager) List(ctx context.Context, query apicore.RunListQuery) ([]apicore.Run, error) {
	listCtx := detachContext(ctx)
	opts := globaldb.ListRunsOptions{
		Status: strings.TrimSpace(query.Status),
		Mode:   strings.TrimSpace(query.Mode),
		Limit:  query.Limit,
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultRunListLimit
	}

	if workspaceRef := strings.TrimSpace(query.Workspace); workspaceRef != "" {
		workspaceRow, err := m.globalDB.Get(listCtx, workspaceRef)
		if err != nil {
			return nil, err
		}
		opts.WorkspaceID = workspaceRow.ID
	}

	rows, err := m.globalDB.ListRuns(listCtx, opts)
	if err != nil {
		return nil, err
	}

	result := make([]apicore.Run, 0, len(rows))
	for i := range rows {
		run, err := m.toCoreRun(listCtx, rows[i], "")
		if err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, nil
}

// Get returns one durable run summary.
func (m *RunManager) Get(ctx context.Context, runID string) (apicore.Run, error) {
	row, err := m.globalDB.GetRun(detachContext(ctx), strings.TrimSpace(runID))
	if err != nil {
		return apicore.Run{}, err
	}
	return m.toCoreRun(detachContext(ctx), row, "")
}

// Snapshot returns the dense attach snapshot for one run.
func (m *RunManager) Snapshot(ctx context.Context, runID string) (apicore.RunSnapshot, error) {
	listCtx := detachContext(ctx)
	row, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID))
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	runView, err := m.toCoreRun(listCtx, row, "")
	if err != nil {
		return apicore.RunSnapshot{}, err
	}

	runDB, err := openRunDB(listCtx, row.RunID)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	defer func() {
		_ = runDB.Close()
	}()

	jobRows, err := runDB.ListJobState(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	transcriptRows, err := runDB.ListTranscriptMessages(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	lastEvent, err := runDB.LastEvent(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}

	snapshot := apicore.RunSnapshot{
		Run:        runView,
		Jobs:       make([]apicore.RunJobState, 0, len(jobRows)),
		Transcript: make([]apicore.RunTranscriptMessage, 0, len(transcriptRows)),
	}
	for i := range jobRows {
		snapshot.Jobs = append(snapshot.Jobs, apicore.RunJobState{
			JobID:      jobRows[i].JobID,
			TaskID:     jobRows[i].TaskID,
			Status:     jobRows[i].Status,
			AgentName:  jobRows[i].AgentName,
			SummaryRaw: rawMessageOrNil(jobRows[i].SummaryJSON),
			UpdatedAt:  jobRows[i].UpdatedAt,
		})
	}
	for i := range transcriptRows {
		snapshot.Transcript = append(snapshot.Transcript, apicore.RunTranscriptMessage{
			Sequence:    transcriptRows[i].Sequence,
			Stream:      transcriptRows[i].Stream,
			Role:        transcriptRows[i].Role,
			Content:     transcriptRows[i].Content,
			MetadataRaw: rawMessageOrNil(transcriptRows[i].MetadataJSON),
			Timestamp:   transcriptRows[i].Timestamp,
		})
	}
	if lastEvent != nil {
		cursor := apicore.CursorFromEvent(*lastEvent)
		snapshot.NextCursor = &cursor
	}
	return snapshot, nil
}

// Events returns persisted run events after the supplied cursor.
func (m *RunManager) Events(
	ctx context.Context,
	runID string,
	query apicore.RunEventPageQuery,
) (apicore.RunEventPage, error) {
	listCtx := detachContext(ctx)
	if _, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID)); err != nil {
		return apicore.RunEventPage{}, err
	}

	runDB, err := openRunDB(listCtx, runID)
	if err != nil {
		return apicore.RunEventPage{}, err
	}
	defer func() {
		_ = runDB.Close()
	}()

	limit := query.Limit
	if limit <= 0 {
		limit = defaultRunListLimit
	}
	if limit > maxRunEventPageLimit {
		limit = maxRunEventPageLimit
	}

	events, err := runDB.ListEvents(listCtx, query.After.Sequence)
	if err != nil {
		return apicore.RunEventPage{}, err
	}

	filtered := make([]eventspkg.Event, 0, len(events))
	for _, item := range events {
		if apicore.EventAfterCursor(item, query.After) {
			filtered = append(filtered, item)
		}
	}

	page := apicore.RunEventPage{}
	if len(filtered) <= limit {
		page.Events = filtered
		if len(filtered) > 0 {
			cursor := apicore.CursorFromEvent(filtered[len(filtered)-1])
			page.NextCursor = &cursor
		}
		return page, nil
	}

	page.Events = append(page.Events, filtered[:limit]...)
	page.HasMore = true
	if len(page.Events) > 0 {
		cursor := apicore.CursorFromEvent(page.Events[len(page.Events)-1])
		page.NextCursor = &cursor
	}
	return page, nil
}

// OpenStream returns a replay-plus-live run stream.
func (m *RunManager) OpenStream(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
) (apicore.RunStream, error) {
	listCtx := detachContext(ctx)
	row, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}

	active := m.getActive(row.RunID)
	stream := &runStream{
		events: make(chan apicore.RunStreamItem, runStreamBufferSize),
		errors: make(chan error, 1),
	}

	streamCtx, cancel := context.WithCancel(listCtx)
	stream.close = func() error {
		cancel()
		return nil
	}

	go m.streamRun(streamCtx, stream, row, active, after)
	return stream, nil
}

// Cancel requests cancellation for one active run.
func (m *RunManager) Cancel(ctx context.Context, runID string) error {
	listCtx := detachContext(ctx)
	row, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	if isTerminalRunStatus(row.Status) {
		return nil
	}

	active := m.getActive(row.RunID)
	if active == nil {
		return nil
	}
	if active.markCancelRequested() {
		active.cancel()
	}
	return nil
}

func (m *RunManager) prepareTaskStart(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.TaskRunRequest,
) (globaldb.Workspace, *string, *model.RuntimeConfig, string, error) {
	workspaceRow, workflowID, projectCfg, err := m.resolveWorkflowContext(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	presentationMode, err := normalizePresentationMode(req.PresentationMode)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	overrides, err := parseRuntimeOverrides(req.RuntimeOverrides)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	tasksDir := model.TaskDirectoryForWorkspace(workspaceRow.RootDir, workflowSlug)
	if err := requireDirectory(tasksDir); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot:              workspaceRow.RootDir,
		Name:                       strings.TrimSpace(workflowSlug),
		TasksDir:                   tasksDir,
		Mode:                       model.ExecutionModePRDTasks,
		EnableExecutableExtensions: true,
	}
	applySoundConfig(runtimeCfg, projectCfg.Sound)
	if err := applyRuntimeOverridesFromProject(
		runtimeCfg,
		workspacecfg.RuntimeOverrides(projectCfg.Defaults),
		"defaults",
	); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	if err := applyTaskProjectConfig(runtimeCfg, projectCfg.Start); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides, "runtime_overrides"); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	runtimeCfg.ApplyDefaults()
	runtimeCfg.TUI = false
	runtimeCfg.DaemonOwned = true
	runtimeCfg.EnableExecutableExtensions = true
	if err := validateDaemonRuntimeConfig(runtimeCfg); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	return workspaceRow, workflowID, runtimeCfg, presentationMode, nil
}

func (m *RunManager) prepareReviewStart(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
	req apicore.ReviewRunRequest,
) (globaldb.Workspace, *string, *model.RuntimeConfig, string, error) {
	if round <= 0 {
		return globaldb.Workspace{}, nil, nil, "", apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"round_invalid",
			"round must be a positive integer",
			map[string]any{"field": "round"},
			nil,
		)
	}

	workspaceRow, workflowID, projectCfg, err := m.resolveWorkflowContext(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	presentationMode, err := normalizePresentationMode(req.PresentationMode)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	overrides, err := parseRuntimeOverrides(req.RuntimeOverrides)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	batching, err := parseReviewBatching(req.Batching)
	if err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	reviewDir := filepath.Join(
		model.TaskDirectoryForWorkspace(workspaceRow.RootDir, workflowSlug),
		reviews.RoundDirName(round),
	)
	if err := requireDirectory(reviewDir); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}

	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot:              workspaceRow.RootDir,
		Name:                       strings.TrimSpace(workflowSlug),
		Round:                      round,
		ReviewsDir:                 reviewDir,
		Mode:                       model.ExecutionModePRReview,
		EnableExecutableExtensions: true,
	}
	applySoundConfig(runtimeCfg, projectCfg.Sound)
	if err := applyRuntimeOverridesFromProject(
		runtimeCfg,
		workspacecfg.RuntimeOverrides(projectCfg.Defaults),
		"defaults",
	); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	if err := applyReviewProjectConfig(runtimeCfg, projectCfg.FixReviews); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides, "runtime_overrides"); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	applyReviewBatching(runtimeCfg, batching)
	runtimeCfg.ApplyDefaults()
	runtimeCfg.TUI = false
	runtimeCfg.DaemonOwned = true
	runtimeCfg.EnableExecutableExtensions = true
	if err := validateDaemonRuntimeConfig(runtimeCfg); err != nil {
		return globaldb.Workspace{}, nil, nil, "", err
	}
	return workspaceRow, workflowID, runtimeCfg, presentationMode, nil
}

func (m *RunManager) prepareExecStart(
	ctx context.Context,
	req apicore.ExecRequest,
) (globaldb.Workspace, *model.RuntimeConfig, string, error) {
	workspacePath := strings.TrimSpace(req.WorkspacePath)
	if workspacePath == "" {
		return globaldb.Workspace{}, nil, "", apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"workspace_path_required",
			"workspace path is required",
			map[string]any{"field": "workspace_path"},
			nil,
		)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return globaldb.Workspace{}, nil, "", apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"prompt_required",
			"prompt is required",
			map[string]any{"field": "prompt"},
			nil,
		)
	}

	workspaceRow, err := m.globalDB.ResolveOrRegister(ctx, workspacePath)
	if err != nil {
		return globaldb.Workspace{}, nil, "", err
	}

	projectCfg, err := m.loadProjectConfig(ctx, workspaceRow.RootDir)
	if err != nil {
		return globaldb.Workspace{}, nil, "", err
	}

	presentationMode, err := normalizePresentationMode(req.PresentationMode)
	if err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	overrides, err := parseRuntimeOverrides(req.RuntimeOverrides)
	if err != nil {
		return globaldb.Workspace{}, nil, "", err
	}

	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Mode:          model.ExecutionModeExec,
		PromptText:    req.Prompt,
		Persist:       true,
	}
	applySoundConfig(runtimeCfg, projectCfg.Sound)
	if err := applyRuntimeOverridesFromProject(
		runtimeCfg,
		workspacecfg.RuntimeOverrides(projectCfg.Defaults),
		"defaults",
	); err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	if err := applyExecProjectConfig(runtimeCfg, projectCfg.Exec); err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides, "runtime_overrides"); err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	runtimeCfg.ApplyDefaults()
	runtimeCfg.Persist = true
	runtimeCfg.TUI = false
	runtimeCfg.DaemonOwned = true
	if err := validateDaemonRuntimeConfig(runtimeCfg); err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	return workspaceRow, runtimeCfg, presentationMode, nil
}

func (m *RunManager) resolveWorkflowContext(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (globaldb.Workspace, *string, workspacecfg.ProjectConfig, error) {
	workspaceRow, err := m.globalDB.ResolveOrRegister(ctx, workspaceRef)
	if err != nil {
		return globaldb.Workspace{}, nil, workspacecfg.ProjectConfig{}, err
	}
	projectCfg, err := m.loadProjectConfig(ctx, workspaceRow.RootDir)
	if err != nil {
		return globaldb.Workspace{}, nil, workspacecfg.ProjectConfig{}, err
	}
	workflowID, err := m.ensureWorkflowIdentity(ctx, workspaceRow.ID, workflowSlug)
	if err != nil {
		return globaldb.Workspace{}, nil, workspacecfg.ProjectConfig{}, err
	}
	return workspaceRow, workflowID, projectCfg, nil
}

func (m *RunManager) ensureWorkflowIdentity(
	ctx context.Context,
	workspaceID string,
	workflowSlug string,
) (*string, error) {
	slug := strings.TrimSpace(workflowSlug)
	if slug == "" {
		return nil, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"workflow_slug_required",
			"workflow slug is required",
			map[string]any{"field": "slug"},
			nil,
		)
	}

	workflow, err := m.globalDB.GetActiveWorkflowBySlug(ctx, workspaceID, slug)
	if err == nil {
		return &workflow.ID, nil
	}
	if !errors.Is(err, globaldb.ErrWorkflowNotFound) {
		return nil, err
	}

	workflow, err = m.globalDB.PutWorkflow(ctx, globaldb.Workflow{
		WorkspaceID: workspaceID,
		Slug:        slug,
	})
	if err == nil {
		return &workflow.ID, nil
	}
	if !errors.Is(err, globaldb.ErrWorkflowSlugConflict) {
		return nil, err
	}

	workflow, err = m.globalDB.GetActiveWorkflowBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, err
	}
	return &workflow.ID, nil
}

func (m *RunManager) startRun(ctx context.Context, spec startRunSpec) (apicore.Run, error) {
	if spec.runtimeCfg == nil {
		return apicore.Run{}, errors.New("daemon: runtime config is required")
	}
	if err := ensureHomeLayout(); err != nil {
		return apicore.Run{}, err
	}

	runtimeCfg := spec.runtimeCfg.Clone()
	runtimeCfg.ApplyDefaults()
	runtimeCfg.RunID = model.BuildRunID(runtimeCfg)
	runID := runtimeCfg.RunID

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		return apicore.Run{}, err
	}
	if err := reserveRunDirectory(runArtifacts.RunDir); err != nil {
		return apicore.Run{}, err
	}

	scope, err := m.openRunScope(detachContext(ctx), runtimeCfg, model.OpenRunScopeOptions{
		EnableExecutableExtensions: runtimeCfg.EnableExecutableExtensions,
	})
	if err != nil {
		cleanupRunDirectory(runArtifacts.RunDir)
		return apicore.Run{}, err
	}

	startedAt := m.now().UTC()
	row, err := m.globalDB.PutRun(detachContext(ctx), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      spec.workspace.ID,
		WorkflowID:       spec.workflowID,
		Mode:             spec.mode,
		Status:           runStatusStarting,
		PresentationMode: spec.presentationMode,
		StartedAt:        startedAt,
		RequestID:        apicore.RequestIDFromContext(ctx),
	})
	if err != nil {
		_ = closeRunScope(scope)
		cleanupRunDirectory(runArtifacts.RunDir)
		return apicore.Run{}, err
	}

	runCtx, cancel := context.WithCancel(withRequestID(m.lifecycleCtx, apicore.RequestIDFromContext(ctx)))
	active := &activeRun{
		runID:        row.RunID,
		workspaceID:  row.WorkspaceID,
		workflowSlug: strings.TrimSpace(spec.workflowSlug),
		mode:         spec.mode,
		scope:        scope,
		ctx:          runCtx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}
	m.setActive(active)

	go m.runAsync(active, row, runtimeCfg)

	return m.toCoreRun(detachContext(ctx), row, active.workflowSlug)
}

func (m *RunManager) runAsync(active *activeRun, row globaldb.Run, runtimeCfg *model.RuntimeConfig) {
	defer close(active.done)
	defer active.cancel()
	defer m.removeActive(active.runID)

	if runtimeCfg.Mode == model.ExecutionModeExec {
		m.executeExecRun(active, row, runtimeCfg)
		return
	}
	m.executeWorkflowRun(active, row, runtimeCfg)
}

func (m *RunManager) executeWorkflowRun(active *activeRun, row globaldb.Run, runtimeCfg *model.RuntimeConfig) {
	scope := active.scope
	var (
		executionErr error
		fallback     terminalState
	)

	if err := context.Cause(active.ctx); err != nil {
		fallback = cancelledTerminalState(scope.RunArtifacts(), err)
		m.finishRun(active, row, fallback)
		return
	}

	if err := startScopeRuntime(active.ctx, scope); err != nil {
		fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested(), false)
		m.finishRun(active, row, fallback)
		return
	}

	row.Status = runStatusRunning
	updated, err := m.globalDB.UpdateRun(detachContext(active.ctx), row)
	if err != nil {
		fallback = failedTerminalState(scope.RunArtifacts(), err)
		m.finishRun(active, row, fallback)
		return
	}
	row = updated

	prep, err := m.prepare(active.ctx, runtimeCfg, scope)
	if err != nil {
		switch {
		case errors.Is(err, plan.ErrNoWork):
			fallback = completedTerminalState(scope.RunArtifacts(), completedNoWorkSummary)
		default:
			fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested(), false)
		}
		m.finishRun(active, row, fallback)
		return
	}
	prep.SetRunScope(scope)

	executionErr = m.execute(active.ctx, prep, runtimeCfg)
	fallback = fallbackTerminalState(scope.RunArtifacts(), executionErr, active.cancelWasRequested(), false)
	m.finishRun(active, row, fallback)
}

func (m *RunManager) executeExecRun(active *activeRun, row globaldb.Run, runtimeCfg *model.RuntimeConfig) {
	scope := active.scope
	var fallback terminalState

	if err := context.Cause(active.ctx); err != nil {
		fallback = cancelledTerminalState(scope.RunArtifacts(), err)
		m.finishRun(active, row, fallback)
		return
	}

	if err := startScopeRuntime(active.ctx, scope); err != nil {
		fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested(), false)
		m.finishRun(active, row, fallback)
		return
	}

	row.Status = runStatusRunning
	updated, err := m.globalDB.UpdateRun(detachContext(active.ctx), row)
	if err != nil {
		fallback = failedTerminalState(scope.RunArtifacts(), err)
		m.finishRun(active, row, fallback)
		return
	}
	row = updated

	executionErr := m.executeExec(active.ctx, runtimeCfg, scope)
	fallback = fallbackTerminalState(scope.RunArtifacts(), executionErr, active.cancelWasRequested(), false)
	m.finishRun(active, row, fallback)
}

func (m *RunManager) finishRun(active *activeRun, row globaldb.Run, fallback terminalState) {
	scope := active.scope
	terminal, err := resolveTerminalState(detachContext(active.ctx), scope.RunArtifacts().RunID, fallback, scope)
	if err != nil {
		terminal = failedTerminalState(scope.RunArtifacts(), err)
	}

	row.Status = terminal.status
	row.ErrorText = terminal.errorText
	if isTerminalRunStatus(terminal.status) {
		endedAt := m.now().UTC()
		row.EndedAt = &endedAt
	}
	if _, err := m.globalDB.UpdateRun(detachContext(active.ctx), row); err != nil {
		_ = closeRunScope(scope)
		return
	}

	_ = closeRunScope(scope)
}

func (m *RunManager) streamRun(
	ctx context.Context,
	stream *runStream,
	row globaldb.Run,
	active *activeRun,
	after apicore.StreamCursor,
) {
	defer close(stream.events)
	defer close(stream.errors)

	var (
		liveBus     *eventspkg.Bus[eventspkg.Event]
		liveCh      <-chan eventspkg.Event
		unsubscribe func()
		subID       eventspkg.SubID
		lastCursor  = after
	)
	if active != nil && active.scope != nil {
		liveBus = active.scope.RunEventBus()
		if liveBus != nil {
			subID, liveCh, unsubscribe = liveBus.Subscribe()
			defer unsubscribe()
		}
	}

	page, err := m.Events(ctx, row.RunID, apicore.RunEventPageQuery{
		After: after,
		Limit: defaultStreamPageLimit,
	})
	if err != nil {
		stream.errors <- err
		return
	}
	for _, item := range page.Events {
		lastCursor = apicore.CursorFromEvent(item)
		if !sendRunStreamItem(ctx, stream.events, apicore.RunStreamItem{Event: &item}) {
			return
		}
		if isTerminalEventKind(item.Kind) {
			return
		}
	}

	if active == nil || liveCh == nil {
		return
	}

	for {
		if liveBus != nil && liveBus.DroppedFor(subID) > 0 {
			overflow := apicore.RunStreamItem{
				Overflow: &apicore.RunStreamOverflow{Reason: "subscriber_dropped_messages"},
			}
			_ = sendRunStreamItem(ctx, stream.events, overflow)
			return
		}

		select {
		case <-ctx.Done():
			return
		case item, ok := <-liveCh:
			if !ok {
				return
			}
			if !apicore.EventAfterCursor(item, lastCursor) {
				continue
			}
			lastCursor = apicore.CursorFromEvent(item)
			if !sendRunStreamItem(ctx, stream.events, apicore.RunStreamItem{Event: &item}) {
				return
			}
			if isTerminalEventKind(item.Kind) {
				return
			}
		}
	}
}

func (m *RunManager) toCoreRun(
	ctx context.Context,
	row globaldb.Run,
	fallbackWorkflowSlug string,
) (apicore.Run, error) {
	run := apicore.Run{
		RunID:            row.RunID,
		WorkspaceID:      row.WorkspaceID,
		Mode:             row.Mode,
		Status:           row.Status,
		PresentationMode: row.PresentationMode,
		StartedAt:        row.StartedAt,
		EndedAt:          row.EndedAt,
		ErrorText:        row.ErrorText,
		RequestID:        row.RequestID,
	}

	if row.WorkflowID != nil {
		workflowID := strings.TrimSpace(*row.WorkflowID)
		run.WorkflowID = &workflowID
		workflow, err := m.globalDB.GetWorkflow(ctx, workflowID)
		if err != nil && !errors.Is(err, globaldb.ErrWorkflowNotFound) {
			return apicore.Run{}, err
		}
		if err == nil {
			run.WorkflowSlug = workflow.Slug
		}
	}
	if run.WorkflowSlug == "" {
		run.WorkflowSlug = strings.TrimSpace(fallbackWorkflowSlug)
	}
	return run, nil
}

func (m *RunManager) getActive(runID string) *activeRun {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active[strings.TrimSpace(runID)]
}

func (m *RunManager) setActive(run *activeRun) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[run.runID] = run
}

func (m *RunManager) removeActive(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, strings.TrimSpace(runID))
}

func (r *runStream) Events() <-chan apicore.RunStreamItem {
	if r == nil {
		return nil
	}
	return r.events
}

func (r *runStream) Errors() <-chan error {
	if r == nil {
		return nil
	}
	return r.errors
}

func (r *runStream) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

func (r *activeRun) markCancelRequested() bool {
	if r == nil {
		return false
	}
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.cancelRequested {
		return false
	}
	r.cancelRequested = true
	return true
}

func (r *activeRun) cancelWasRequested() bool {
	if r == nil {
		return false
	}
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.cancelRequested
}

func ensureHomeLayout() error {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return fmt.Errorf("daemon: resolve home paths: %w", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return fmt.Errorf("daemon: ensure home layout: %w", err)
	}
	return nil
}

func reserveRunDirectory(runDir string) error {
	cleanRunDir := strings.TrimSpace(runDir)
	if cleanRunDir == "" {
		return errors.New("daemon: run directory is required")
	}
	if err := os.MkdirAll(filepath.Dir(cleanRunDir), 0o755); err != nil {
		return fmt.Errorf("daemon: create run parent directory: %w", err)
	}
	if err := os.Mkdir(cleanRunDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return globaldb.ErrRunAlreadyExists
		}
		return fmt.Errorf("daemon: reserve run directory %q: %w", cleanRunDir, err)
	}
	return nil
}

func cleanupRunDirectory(runDir string) {
	cleanRunDir := strings.TrimSpace(runDir)
	if cleanRunDir == "" {
		return
	}
	_ = os.RemoveAll(cleanRunDir)
}

func closeRunScope(scope model.RunScope) error {
	if scope == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), runShutdownTimeout)
	defer cancel()
	return scope.Close(closeCtx)
}

func startScopeRuntime(ctx context.Context, scope model.RunScope) error {
	if scope == nil {
		return nil
	}
	if runtimeManager := scope.RunManager(); runtimeManager != nil {
		return runtimeManager.Start(ctx)
	}
	return nil
}

func openRunDB(ctx context.Context, runID string) (*rundb.RunDB, error) {
	runArtifacts, err := model.ResolveHomeRunArtifacts(strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	return rundb.Open(ctx, runArtifacts.RunDBPath)
}

func resolveTerminalState(
	ctx context.Context,
	runID string,
	fallback terminalState,
	scope model.RunScope,
) (terminalState, error) {
	runDB, err := openRunDB(ctx, runID)
	if err != nil {
		return terminalState{}, err
	}
	defer func() {
		_ = runDB.Close()
	}()

	lastEvent, err := runDB.LastEvent(ctx)
	if err != nil {
		return terminalState{}, err
	}
	if terminal, ok, err := terminalStateFromEvent(lastEvent); err != nil {
		return terminalState{}, err
	} else if ok {
		return terminal, nil
	}

	if fallback.kind == "" {
		return terminalState{}, fmt.Errorf("daemon: run %q has no terminal event", runID)
	}
	if scope == nil || scope.RunJournal() == nil {
		return terminalState{}, fmt.Errorf("daemon: run %q cannot append fallback terminal event", runID)
	}
	if err := submitSyntheticEvent(ctx, scope.RunJournal(), runID, fallback.kind, fallback.payload); err != nil {
		return terminalState{}, err
	}

	lastEvent, err = runDB.LastEvent(ctx)
	if err != nil {
		return terminalState{}, err
	}
	terminal, ok, err := terminalStateFromEvent(lastEvent)
	if err != nil {
		return terminalState{}, err
	}
	if !ok {
		return terminalState{}, fmt.Errorf("daemon: run %q terminal event missing after fallback append", runID)
	}
	return terminal, nil
}

func terminalStateFromEvent(event *eventspkg.Event) (terminalState, bool, error) {
	if event == nil {
		return terminalState{}, false, nil
	}

	switch event.Kind {
	case eventspkg.EventKindRunCompleted:
		var payload kinds.RunCompletedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return terminalState{}, false, fmt.Errorf("daemon: decode run.completed payload: %w", err)
		}
		return terminalState{
			status:    runStatusCompleted,
			errorText: "",
			kind:      event.Kind,
			payload:   payload,
		}, true, nil
	case eventspkg.EventKindRunFailed:
		var payload kinds.RunFailedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return terminalState{}, false, fmt.Errorf("daemon: decode run.failed payload: %w", err)
		}
		return terminalState{
			status:    runStatusFailed,
			errorText: strings.TrimSpace(payload.Error),
			kind:      event.Kind,
			payload:   payload,
		}, true, nil
	case eventspkg.EventKindRunCancelled:
		var payload kinds.RunCancelledPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return terminalState{}, false, fmt.Errorf("daemon: decode run.cancelled payload: %w", err)
		}
		return terminalState{
			status:    runStatusCancelled,
			errorText: strings.TrimSpace(payload.Reason),
			kind:      event.Kind,
			payload:   payload,
		}, true, nil
	default:
		return terminalState{}, false, nil
	}
}

func submitSyntheticEvent(
	ctx context.Context,
	runJournal submitter,
	runID string,
	kind eventspkg.EventKind,
	payload any,
) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("daemon: marshal %s payload: %w", kind, err)
	}
	return runJournal.Submit(ctx, eventspkg.Event{
		RunID:   strings.TrimSpace(runID),
		Kind:    kind,
		Payload: rawPayload,
	})
}

type submitter interface {
	Submit(context.Context, eventspkg.Event) error
}

func fallbackTerminalState(
	runArtifacts model.RunArtifacts,
	err error,
	cancelRequested bool,
	noWork bool,
) terminalState {
	switch {
	case noWork:
		return completedTerminalState(runArtifacts, completedNoWorkSummary)
	case cancelRequested || errors.Is(err, context.Canceled):
		return cancelledTerminalState(runArtifacts, err)
	case err == nil:
		return completedTerminalState(runArtifacts, "")
	default:
		return failedTerminalState(runArtifacts, err)
	}
}

func completedTerminalState(runArtifacts model.RunArtifacts, summary string) terminalState {
	return terminalState{
		status: runStatusCompleted,
		kind:   eventspkg.EventKindRunCompleted,
		payload: kinds.RunCompletedPayload{
			ArtifactsDir:   runArtifacts.RunDir,
			SummaryMessage: strings.TrimSpace(summary),
		},
	}
}

func failedTerminalState(runArtifacts model.RunArtifacts, err error) terminalState {
	return terminalState{
		status:    runStatusFailed,
		errorText: errorString(err),
		kind:      eventspkg.EventKindRunFailed,
		payload: kinds.RunFailedPayload{
			ArtifactsDir: runArtifacts.RunDir,
			Error:        errorString(err),
			ResultPath:   runArtifacts.ResultPath,
		},
	}
}

func cancelledTerminalState(runArtifacts model.RunArtifacts, err error) terminalState {
	reason := errorString(err)
	if reason == "" {
		reason = "cancelled"
	}
	return terminalState{
		status:    runStatusCancelled,
		errorText: reason,
		kind:      eventspkg.EventKindRunCancelled,
		payload: kinds.RunCancelledPayload{
			Reason:      reason,
			RequestedBy: cancelRequestedByDaemon,
		},
	}
}

func isTerminalRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case runStatusCompleted, runStatusFailed, runStatusCancelled, "canceled", "crashed":
		return true
	default:
		return false
	}
}

func isTerminalEventKind(kind eventspkg.EventKind) bool {
	switch kind {
	case eventspkg.EventKindRunCompleted, eventspkg.EventKindRunFailed, eventspkg.EventKindRunCancelled:
		return true
	default:
		return false
	}
}

func rawMessageOrNil(value string) json.RawMessage {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func parseRuntimeOverrides(raw json.RawMessage) (runtimeOverrideInput, error) {
	var input runtimeOverrideInput
	if len(bytes.TrimSpace(raw)) == 0 {
		return input, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return runtimeOverrideInput{}, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_runtime_overrides",
			fmt.Sprintf("runtime_overrides: %v", err),
			nil,
			err,
		)
	}
	return input, nil
}

func parseReviewBatching(raw json.RawMessage) (reviewBatchingInput, error) {
	var input reviewBatchingInput
	if len(bytes.TrimSpace(raw)) == 0 {
		return input, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return reviewBatchingInput{}, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_batching",
			fmt.Sprintf("batching: %v", err),
			nil,
			err,
		)
	}
	return input, nil
}

func applyRuntimeOverridesFromProject(
	cfg *model.RuntimeConfig,
	overrides workspacecfg.RuntimeOverrides,
	scope string,
) error {
	if cfg == nil {
		return nil
	}
	if err := applyOptionalString(&cfg.IDE, overrides.IDE); err != nil {
		return overrideValueError(scope, "ide", err)
	}
	if err := applyOptionalString(&cfg.Model, overrides.Model); err != nil {
		return overrideValueError(scope, "model", err)
	}
	if err := applyOptionalOutputFormat(cfg, overrides.OutputFormat); err != nil {
		return overrideValueError(scope, "output_format", err)
	}
	if err := applyOptionalString(&cfg.ReasoningEffort, overrides.ReasoningEffort); err != nil {
		return overrideValueError(scope, "reasoning_effort", err)
	}
	if err := applyOptionalString(&cfg.AccessMode, overrides.AccessMode); err != nil {
		return overrideValueError(scope, "access_mode", err)
	}
	if err := applyOptionalDuration(cfg, overrides.Timeout); err != nil {
		return overrideValueError(scope, "timeout", err)
	}
	if overrides.TailLines != nil {
		cfg.TailLines = *overrides.TailLines
	}
	if overrides.AddDirs != nil {
		cfg.AddDirs = corepkg.NormalizeAddDirs(*overrides.AddDirs)
	}
	if overrides.AutoCommit != nil {
		cfg.AutoCommit = *overrides.AutoCommit
	}
	if overrides.MaxRetries != nil {
		cfg.MaxRetries = *overrides.MaxRetries
	}
	if overrides.RetryBackoffMultiplier != nil {
		cfg.RetryBackoffMultiplier = *overrides.RetryBackoffMultiplier
	}
	return nil
}

func applyTaskProjectConfig(cfg *model.RuntimeConfig, projectCfg workspacecfg.StartConfig) error {
	if cfg == nil {
		return nil
	}
	if err := applyOptionalOutputFormat(cfg, projectCfg.OutputFormat); err != nil {
		return overrideValueError("start", "output_format", err)
	}
	if projectCfg.IncludeCompleted != nil {
		cfg.IncludeCompleted = *projectCfg.IncludeCompleted
	}
	cfg.TaskRuntimeRules = model.CloneTaskRuntimeRules(derefTaskRuntimeRules(projectCfg.TaskRuntimeRules))
	return nil
}

func applyReviewProjectConfig(cfg *model.RuntimeConfig, projectCfg workspacecfg.FixReviewsConfig) error {
	if cfg == nil {
		return nil
	}
	if err := applyOptionalOutputFormat(cfg, projectCfg.OutputFormat); err != nil {
		return overrideValueError("fix_reviews", "output_format", err)
	}
	if projectCfg.Concurrent != nil {
		cfg.Concurrent = *projectCfg.Concurrent
	}
	if projectCfg.BatchSize != nil {
		cfg.BatchSize = *projectCfg.BatchSize
	}
	if projectCfg.IncludeResolved != nil {
		cfg.IncludeResolved = *projectCfg.IncludeResolved
	}
	return nil
}

func applyExecProjectConfig(cfg *model.RuntimeConfig, projectCfg workspacecfg.ExecConfig) error {
	if cfg == nil {
		return nil
	}
	if err := applyRuntimeOverridesFromProject(cfg, projectCfg.RuntimeOverrides, "exec"); err != nil {
		return err
	}
	if projectCfg.Verbose != nil {
		cfg.Verbose = *projectCfg.Verbose
	}
	if projectCfg.Persist != nil {
		cfg.Persist = *projectCfg.Persist
	}
	return nil
}

func applyRuntimeOverrideInput(cfg *model.RuntimeConfig, overrides runtimeOverrideInput, scope string) error {
	if cfg == nil {
		return nil
	}

	if err := applyOptionalString(&cfg.RunID, overrides.RunID); err != nil {
		return overrideValueError(scope, "run_id", err)
	}
	if err := applyOptionalString(&cfg.IDE, overrides.IDE); err != nil {
		return overrideValueError(scope, "ide", err)
	}
	if err := applyOptionalString(&cfg.Model, overrides.Model); err != nil {
		return overrideValueError(scope, "model", err)
	}
	if err := applyOptionalOutputFormat(cfg, overrides.OutputFormat); err != nil {
		return overrideValueError(scope, "output_format", err)
	}
	if err := applyOptionalString(&cfg.ReasoningEffort, overrides.ReasoningEffort); err != nil {
		return overrideValueError(scope, "reasoning_effort", err)
	}
	if err := applyOptionalString(&cfg.AccessMode, overrides.AccessMode); err != nil {
		return overrideValueError(scope, "access_mode", err)
	}
	if err := applyOptionalDuration(cfg, overrides.Timeout); err != nil {
		return overrideValueError(scope, "timeout", err)
	}
	if overrides.TailLines != nil {
		cfg.TailLines = *overrides.TailLines
	}
	if overrides.AddDirs != nil {
		cfg.AddDirs = corepkg.NormalizeAddDirs(*overrides.AddDirs)
	}
	if overrides.AutoCommit != nil {
		cfg.AutoCommit = *overrides.AutoCommit
	}
	if overrides.MaxRetries != nil {
		cfg.MaxRetries = *overrides.MaxRetries
	}
	if overrides.RetryBackoffMultiplier != nil {
		cfg.RetryBackoffMultiplier = *overrides.RetryBackoffMultiplier
	}
	if overrides.Concurrent != nil {
		cfg.Concurrent = *overrides.Concurrent
	}
	if overrides.BatchSize != nil {
		cfg.BatchSize = *overrides.BatchSize
	}
	if overrides.Verbose != nil {
		cfg.Verbose = *overrides.Verbose
	}
	if overrides.Persist != nil {
		cfg.Persist = *overrides.Persist
	}
	if overrides.IncludeCompleted != nil {
		cfg.IncludeCompleted = *overrides.IncludeCompleted
	}
	if overrides.IncludeResolved != nil {
		cfg.IncludeResolved = *overrides.IncludeResolved
	}
	if overrides.EnableExecutableExtensions != nil {
		cfg.EnableExecutableExtensions = *overrides.EnableExecutableExtensions
	}
	return nil
}

func applyReviewBatching(cfg *model.RuntimeConfig, batching reviewBatchingInput) {
	if cfg == nil {
		return
	}
	if batching.Concurrent != nil {
		cfg.Concurrent = *batching.Concurrent
	}
	if batching.BatchSize != nil {
		cfg.BatchSize = *batching.BatchSize
	}
	if batching.IncludeResolved != nil {
		cfg.IncludeResolved = *batching.IncludeResolved
	}
}

func applySoundConfig(cfg *model.RuntimeConfig, soundCfg workspacecfg.SoundConfig) {
	if cfg == nil {
		return
	}
	if soundCfg.Enabled != nil {
		cfg.SoundEnabled = *soundCfg.Enabled
	}
	if soundCfg.OnCompleted != nil {
		cfg.SoundOnCompleted = strings.TrimSpace(*soundCfg.OnCompleted)
	}
	if soundCfg.OnFailed != nil {
		cfg.SoundOnFailed = strings.TrimSpace(*soundCfg.OnFailed)
	}
}

func applyOptionalString(dst *string, value *string) error {
	if value == nil {
		return nil
	}
	*dst = strings.TrimSpace(*value)
	return nil
}

func applyOptionalOutputFormat(cfg *model.RuntimeConfig, value *string) error {
	if cfg == nil || value == nil {
		return nil
	}
	cfg.OutputFormat = model.OutputFormat(strings.TrimSpace(*value))
	return nil
}

func applyOptionalDuration(cfg *model.RuntimeConfig, value *string) error {
	if cfg == nil || value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		cfg.Timeout = 0
		return nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return err
	}
	cfg.Timeout = parsed
	return nil
}

func validateDaemonRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return agent.ErrRuntimeConfigNil
	}
	check := cfg.Clone()
	if check.Mode != model.ExecutionModeExec {
		check.RunID = ""
	}
	if err := agent.ValidateRuntimeConfig(check); err != nil {
		return apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_runtime",
			err.Error(),
			nil,
			err,
		)
	}
	return nil
}

func normalizePresentationMode(value string) (string, error) {
	mode := strings.TrimSpace(value)
	if mode == "" {
		mode = defaultPresentationMode
	}
	switch mode {
	case "ui", "stream", "detach":
		return mode, nil
	default:
		return "", apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_presentation_mode",
			"presentation_mode must be one of ui, stream, or detach",
			map[string]any{"field": "presentation_mode"},
			nil,
		)
	}
}

func requireDirectory(path string) error {
	info, err := os.Stat(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", strings.TrimSpace(path))
	}
	return nil
}

func derefTaskRuntimeRules(value *[]model.TaskRuntimeRule) []model.TaskRuntimeRule {
	if value == nil {
		return nil
	}
	return *value
}

func overrideValueError(scope string, field string, err error) error {
	return apicore.NewProblem(
		http.StatusUnprocessableEntity,
		"invalid_runtime_overrides",
		fmt.Sprintf("%s.%s: %v", strings.TrimSpace(scope), strings.TrimSpace(field), err),
		map[string]any{
			"scope": scope,
			"field": field,
		},
		err,
	)
}

func detachContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(requestID) == "" {
		return ctx
	}
	return apicore.WithRequestID(ctx, requestID)
}

func sendRunStreamItem(
	ctx context.Context,
	dst chan<- apicore.RunStreamItem,
	item apicore.RunStreamItem,
) bool {
	select {
	case dst <- item:
		return true
	case <-ctx.Done():
		return false
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
