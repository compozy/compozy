package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
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
	runModeTask        = "task"
	runModeReview      = "review"
	runModeReviewWatch = "review_watch"
	runModeExec        = "exec"

	runStatusStarting  = "starting"
	runStatusRunning   = "running"
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
	runStatusCancelled = "canceled"
	runStatusCrashed   = "crashed"

	defaultRunListLimit       = 100
	defaultPresentationMode   = "stream"
	maxRunEventPageLimit      = 1000
	defaultRunDBCacheIdleTTL  = 30 * time.Second
	runStreamBufferSize       = 64
	workspaceStreamBufferSize = 128
	defaultStreamPageLimit    = 256
	cancelRequestedByDaemon   = "daemon"
	completedNoWorkSummary    = "no work"
)

// RunManagerConfig wires the daemon-owned run manager dependencies.
type RunManagerConfig struct {
	GlobalDB               *globaldb.GlobalDB
	LifecycleContext       context.Context
	ShutdownDrainTimeout   time.Duration
	Now                    func() time.Time
	OpenRunScope           func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	Prepare                func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	Execute                func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	ExecuteExec            func(context.Context, *model.RuntimeConfig, model.RunScope) error
	OpenRunDB              func(context.Context, string) (*rundb.RunDB, error)
	LoadProjectConfig      func(context.Context, string) (workspacecfg.ProjectConfig, error)
	ReviewProviderRegistry reviewProviderRegistryFactory
	ReviewWatchGit         ReviewWatchGit
	WatcherDebounce        time.Duration
	RunDBCacheTTL          time.Duration
	LookupWorkflowSlugs    func(context.Context, []string) (map[string]string, error)
	GetWorkflow            func(context.Context, string) (globaldb.Workflow, error)
}

// RunManager owns daemon-backed task, review, and exec runs.
type RunManager struct {
	globalDB               *globaldb.GlobalDB
	lifecycleCtx           context.Context
	now                    func() time.Time
	openRunScope           func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error)
	prepare                func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error)
	execute                func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error
	executeExec            func(context.Context, *model.RuntimeConfig, model.RunScope) error
	openRunDB              func(context.Context, string) (*rundb.RunDB, error)
	loadProjectConfig      func(context.Context, string) (workspacecfg.ProjectConfig, error)
	reviewProviderRegistry reviewProviderRegistryFactory
	reviewWatchGit         ReviewWatchGit
	lookupWorkflowSlugs    func(context.Context, []string) (map[string]string, error)
	getWorkflow            func(context.Context, string) (globaldb.Workflow, error)
	shutdownDrainTimeout   time.Duration
	watcherDebounce        time.Duration
	runDBIdleTTL           time.Duration

	mu                  sync.RWMutex
	active              map[string]*activeRun
	activeReviewWatches map[reviewWatchKey]string

	runWG   sync.WaitGroup
	runDBMu sync.Mutex
	runDBs  map[string]*cachedRunDB

	metricsMu               sync.RWMutex
	terminalTotals          map[string]uint64
	acpStallTotals          map[string]uint64
	journalTerminalDrops    uint64
	journalNonTerminalDrops uint64
	journalDropsByRun       map[string]journalDropTotals
	incompleteRunIDs        map[string]struct{}
	workspaceEvents         *eventspkg.Bus[apicore.WorkspaceEvent]
	workspaceEventSeq       atomic.Uint64
}

type activeRun struct {
	runID          string
	workspaceID    string
	workflowSlug   string
	workflowID     *string
	mode           string
	scope          model.RunScope
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	workflowRoot   string
	watcher        *workflowWatcher
	reviewWatch    *preparedReviewWatch
	reviewWatchKey *reviewWatchKey

	stateMu         sync.RWMutex
	cancelRequested bool
	closeTimeout    time.Duration
}

type runtimeOverrideInput struct {
	DryRun                     *bool                       `json:"dry_run"`
	RunID                      *string                     `json:"run_id"`
	IDE                        *string                     `json:"ide"`
	Model                      *string                     `json:"model"`
	AgentName                  *string                     `json:"agent_name"`
	ExplicitRuntime            *model.ExplicitRuntimeFlags `json:"explicit_runtime"`
	OutputFormat               *string                     `json:"output_format"`
	ReasoningEffort            *string                     `json:"reasoning_effort"`
	AccessMode                 *string                     `json:"access_mode"`
	Timeout                    *string                     `json:"timeout"`
	TailLines                  *int                        `json:"tail_lines"`
	AddDirs                    *[]string                   `json:"add_dirs"`
	AutoCommit                 *bool                       `json:"auto_commit"`
	MaxRetries                 *int                        `json:"max_retries"`
	RetryBackoffMultiplier     *float64                    `json:"retry_backoff_multiplier"`
	Concurrent                 *int                        `json:"concurrent"`
	BatchSize                  *int                        `json:"batch_size"`
	Verbose                    *bool                       `json:"verbose"`
	Persist                    *bool                       `json:"persist"`
	IncludeCompleted           *bool                       `json:"include_completed"`
	IncludeResolved            *bool                       `json:"include_resolved"`
	TaskRuntimeRules           *[]model.TaskRuntimeRule    `json:"task_runtime_rules"`
	TUI                        *bool                       `json:"tui"`
	EnableExecutableExtensions *bool                       `json:"enable_executable_extensions"`
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
	workflowRoot     string
	mode             string
	presentationMode string
	runtimeCfg       *model.RuntimeConfig
	reviewWatch      *preparedReviewWatch
	reviewWatchKey   *reviewWatchKey
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

type liveRunSubscription struct {
	bus         *eventspkg.Bus[eventspkg.Event]
	ch          <-chan eventspkg.Event
	unsubscribe func()
	subID       eventspkg.SubID
}

type cachedRunDB struct {
	db             *rundb.RunDB
	references     int
	lastUsedAt     time.Time
	evictOnRelease bool
}

type runDBLease struct {
	manager *RunManager
	runID   string
	db      *rundb.RunDB
}

var _ apicore.RunService = (*RunManager)(nil)
var _ apicore.WorkspaceEventService = (*RunManager)(nil)

// NewRunManager constructs a daemon-owned run manager.
func NewRunManager(cfg RunManagerConfig) (*RunManager, error) {
	if cfg.GlobalDB == nil {
		return nil, errors.New("daemon: run manager global db is required")
	}

	return &RunManager{
		globalDB:               cfg.GlobalDB,
		lifecycleCtx:           resolveRunManagerLifecycleContext(cfg.LifecycleContext),
		now:                    resolveRunManagerNow(cfg.Now),
		openRunScope:           resolveRunManagerOpenRunScope(cfg.OpenRunScope),
		prepare:                resolveRunManagerPrepare(cfg.Prepare),
		execute:                resolveRunManagerExecute(cfg.Execute),
		executeExec:            resolveRunManagerExecuteExec(cfg.ExecuteExec),
		openRunDB:              resolveRunManagerOpenRunDB(cfg.OpenRunDB),
		loadProjectConfig:      resolveRunManagerLoadProjectConfig(cfg.LoadProjectConfig),
		reviewProviderRegistry: resolveReviewProviderRegistryFactory(cfg.ReviewProviderRegistry),
		reviewWatchGit:         resolveReviewWatchGit(cfg.ReviewWatchGit),
		lookupWorkflowSlugs:    resolveRunManagerWorkflowSlugLookup(cfg.GlobalDB, cfg.LookupWorkflowSlugs),
		getWorkflow:            resolveRunManagerGetWorkflow(cfg.GlobalDB, cfg.GetWorkflow),
		shutdownDrainTimeout:   resolveRunManagerShutdownDrainTimeout(cfg.ShutdownDrainTimeout),
		watcherDebounce:        resolveWatcherDebounce(cfg.WatcherDebounce),
		runDBIdleTTL:           resolveRunManagerRunDBCacheTTL(cfg.RunDBCacheTTL),
		active:                 make(map[string]*activeRun),
		activeReviewWatches:    make(map[reviewWatchKey]string),
		runDBs:                 make(map[string]*cachedRunDB),
		terminalTotals:         make(map[string]uint64),
		acpStallTotals:         make(map[string]uint64),
		journalDropsByRun:      make(map[string]journalDropTotals),
		incompleteRunIDs:       make(map[string]struct{}),
		workspaceEvents:        eventspkg.New[apicore.WorkspaceEvent](workspaceStreamBufferSize),
	}, nil
}

func resolveRunManagerLifecycleContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func resolveRunManagerNow(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}
	return func() time.Time {
		return time.Now().UTC()
	}
}

func resolveRunManagerOpenRunScope(
	openRunScope func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error),
) func(context.Context, *model.RuntimeConfig, model.OpenRunScopeOptions) (model.RunScope, error) {
	if openRunScope != nil {
		return openRunScope
	}
	return model.OpenRunScope
}

func resolveRunManagerPrepare(
	prepare func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error),
) func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
	if prepare != nil {
		return prepare
	}
	return plan.Prepare
}

func resolveRunManagerExecute(
	execute func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error,
) func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
	if execute != nil {
		return execute
	}
	return func(ctx context.Context, prep *model.SolvePreparation, runtimeCfg *model.RuntimeConfig) error {
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

func resolveRunManagerExecuteExec(
	executeExec func(context.Context, *model.RuntimeConfig, model.RunScope) error,
) func(context.Context, *model.RuntimeConfig, model.RunScope) error {
	if executeExec != nil {
		return executeExec
	}
	return runpkg.ExecuteExec
}

func resolveRunManagerOpenRunDB(
	openRunDB func(context.Context, string) (*rundb.RunDB, error),
) func(context.Context, string) (*rundb.RunDB, error) {
	if openRunDB != nil {
		return openRunDB
	}
	return openRunDBForRunID
}

func resolveRunManagerLoadProjectConfig(
	loadProjectConfig func(context.Context, string) (workspacecfg.ProjectConfig, error),
) func(context.Context, string) (workspacecfg.ProjectConfig, error) {
	if loadProjectConfig != nil {
		return loadProjectConfig
	}
	return func(ctx context.Context, root string) (workspacecfg.ProjectConfig, error) {
		projectCfg, _, err := workspacecfg.LoadConfig(ctx, root)
		return projectCfg, err
	}
}

func resolveRunManagerShutdownDrainTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if settings, _, err := LoadRunLifecycleSettings(context.Background()); err == nil &&
		settings.ShutdownDrainTimeout > 0 {
		return settings.ShutdownDrainTimeout
	}
	return defaultShutdownDrainTimeout
}

func resolveRunManagerWorkflowSlugLookup(
	db *globaldb.GlobalDB,
	lookup func(context.Context, []string) (map[string]string, error),
) func(context.Context, []string) (map[string]string, error) {
	if lookup != nil {
		return lookup
	}
	return db.WorkflowSlugsByIDs
}

func resolveRunManagerGetWorkflow(
	db *globaldb.GlobalDB,
	getWorkflow func(context.Context, string) (globaldb.Workflow, error),
) func(context.Context, string) (globaldb.Workflow, error) {
	if getWorkflow != nil {
		return getWorkflow
	}
	return db.GetWorkflow
}

func resolveRunManagerRunDBCacheTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return defaultRunDBCacheIdleTTL
}

func resolveWatcherDebounce(debounce time.Duration) time.Duration {
	if debounce > 0 {
		return debounce
	}
	return defaultWatcherDebounce
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
		workflowRoot:     strings.TrimSpace(runtimeCfg.TasksDir),
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
		workflowRoot:     filepath.Dir(strings.TrimSpace(runtimeCfg.ReviewsDir)),
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
	workflowSlugs, err := m.workflowSlugsForRuns(listCtx, rows)
	if err != nil {
		return nil, err
	}

	result := make([]apicore.Run, 0, len(rows))
	for i := range rows {
		run, err := m.toCoreRun(listCtx, rows[i], workflowSlugForRun(rows[i], workflowSlugs))
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

	lease, err := m.acquireRunDB(listCtx, row.RunID)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	defer func() {
		_ = lease.Close()
	}()
	runDB := lease.DB()

	lastEvent, err := runDB.LastEvent(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	if isTerminalRunStatus(runView.Status) {
		return m.compactSnapshot(listCtx, row.RunID, runView, runDB, lastEvent)
	}

	eventRows, err := runDB.ListEvents(listCtx, 0, 0)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	transcriptRows, err := runDB.ListTranscriptMessages(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	tokenUsageRows, err := runDB.ListTokenUsage(listCtx)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	if err := m.persistRuntimeIntegrity(listCtx, row.RunID, m.scopeForRun(row.RunID)); err != nil {
		slog.Default().Warn("daemon snapshot runtime integrity persistence failed", "run_id", row.RunID, "error", err)
	}
	integrity, err := m.loadRunIntegrity(
		listCtx,
		row.RunID,
		runView,
		runDB,
		eventRows.Events,
		transcriptRows,
		lastEvent,
	)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}

	builder := newRunSnapshotBuilder()
	for _, item := range eventRows.Events {
		if err := builder.applyEvent(item); err != nil {
			return apicore.RunSnapshot{}, err
		}
	}
	builder.applyTokenUsageRows(tokenUsageRows)

	snapshot := apicore.RunSnapshot{
		Run:               runView,
		Jobs:              builder.jobStates(),
		Transcript:        assembleSnapshotTranscript(transcriptRows),
		Usage:             builder.usage,
		Shutdown:          builder.shutdown,
		Incomplete:        integrity.Incomplete,
		IncompleteReasons: append([]string(nil), integrity.Reasons...),
	}
	if lastEvent != nil {
		cursor := apicore.CursorFromEvent(*lastEvent)
		snapshot.NextCursor = &cursor
	}
	return snapshot, nil
}

// RunDetail returns the richer browser-facing run detail payload.
func (m *RunManager) RunDetail(ctx context.Context, runID string) (apicore.RunDetailPayload, error) {
	if m == nil {
		return apicore.RunDetailPayload{}, errors.New("daemon: run manager is required")
	}

	query := resolveTransportQueryService(nil, m, nil, nil)
	if query == nil {
		return apicore.RunDetailPayload{}, errors.New("daemon: run detail query service is unavailable")
	}

	payload, err := query.RunDetail(detachContext(ctx), strings.TrimSpace(runID))
	if err != nil {
		return apicore.RunDetailPayload{}, mapQueryTransportError(err)
	}
	return transportRunDetail(payload), nil
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

	lease, err := m.acquireRunDB(listCtx, runID)
	if err != nil {
		return apicore.RunEventPage{}, err
	}
	defer func() {
		_ = lease.Close()
	}()
	runDB := lease.DB()

	limit := query.Limit
	if limit <= 0 {
		limit = defaultRunListLimit
	}
	if limit > maxRunEventPageLimit {
		limit = maxRunEventPageLimit
	}

	fromSeq := query.After.Sequence
	if fromSeq > 0 {
		fromSeq++
	}
	events, err := runDB.ListEvents(listCtx, fromSeq, limit)
	if err != nil {
		return apicore.RunEventPage{}, err
	}

	page := apicore.RunEventPage{
		Events:  events.Events,
		HasMore: events.HasMore,
	}
	if events.HasMore && len(page.Events) > limit {
		page.Events = page.Events[:limit]
	}
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
	applyTaskProjectConfig(runtimeCfg, projectCfg.Tasks.Run)
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides); err != nil {
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
	applyReviewProjectConfig(runtimeCfg, projectCfg.FixReviews)
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides); err != nil {
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

func (m *RunManager) syncWorkflowBeforeRun(ctx context.Context, active *activeRun) error {
	if active == nil || strings.TrimSpace(active.workflowRoot) == "" {
		return nil
	}
	result, err := corepkg.SyncDirect(ctx, model.SyncConfig{TasksDir: active.workflowRoot})
	if err != nil {
		return fmt.Errorf("daemon: sync workflow %s before run: %w", active.workflowRoot, err)
	}
	m.publishWorkflowSyncWorkspaceEvent(
		ctx,
		active.workspaceID,
		active.workflowID,
		active.workflowSlug,
		result.SyncedPaths,
	)
	return nil
}

func (m *RunManager) resolveExecWorkspace(ctx context.Context, workspacePath string) (globaldb.Workspace, error) {
	workspaceRow, err := m.globalDB.Get(ctx, workspacePath)
	if err == nil {
		if err := requireWorkspacePathAvailable(workspaceRow); err != nil {
			return globaldb.Workspace{}, err
		}
		return workspaceRow, nil
	}
	if !errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		return globaldb.Workspace{}, err
	}
	return m.globalDB.ResolveOrRegister(ctx, workspacePath)
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

	workspaceRow, err := m.resolveExecWorkspace(ctx, workspacePath)
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
	if err := applyPersistedExecRuntimeDefaults(runtimeCfg, workspaceRow.RootDir, overrides); err != nil {
		return globaldb.Workspace{}, nil, "", err
	}
	if err := applyRuntimeOverrideInput(runtimeCfg, overrides); err != nil {
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
	workspaceRow, err := resolveWorkspaceReference(ctx, m.globalDB, workspaceRef)
	if err != nil {
		return globaldb.Workspace{}, nil, workspacecfg.ProjectConfig{}, err
	}
	if err := requireWorkspacePathAvailable(workspaceRow); err != nil {
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

	startedAt := m.now().UTC()
	row, createdRun, err := m.prepareRunRow(
		detachContext(ctx),
		spec,
		runID,
		startedAt,
		apicore.RequestIDFromContext(ctx),
	)
	if err != nil {
		return apicore.Run{}, err
	}
	m.publishRunWorkspaceEvent(ctx, row, spec.workflowSlug, apicore.WorkspaceEventKindRunCreated)

	scope, err := m.openRunScopeForStart(ctx, runtimeCfg, spec.workspace.RootDir)
	if err != nil {
		if createdRun {
			if runArtifacts, artifactsErr := model.ResolveHomeRunArtifacts(runID); artifactsErr == nil {
				cleanupRunDirectory(runArtifacts.RunDir)
			}
			if deleteErr := m.globalDB.DeleteRun(detachContext(ctx), runID); deleteErr != nil {
				err = errors.Join(err, deleteErr)
			}
		}
		return apicore.Run{}, err
	}

	runCtx, cancel := context.WithCancel(withRequestID(m.lifecycleCtx, apicore.RequestIDFromContext(ctx)))
	started := false
	defer func() {
		if !started {
			cancel()
		}
	}()
	active := newActiveRun(runCtx, cancel, row, spec, scope)
	if err := m.syncWorkflowBeforeRun(runCtx, active); err != nil {
		active.cancel()
		return apicore.Run{}, m.failStartRun(ctx, row, active.currentCloseTimeout(), scope, createdRun, err)
	}
	if err := m.startWatcher(active); err != nil {
		active.cancel()
		return apicore.Run{}, m.failStartRun(ctx, row, active.currentCloseTimeout(), scope, createdRun, err)
	}
	m.setActive(active)

	m.runWG.Add(1)
	go m.runAsync(active, row, runtimeCfg)
	started = true

	return m.toCoreRun(detachContext(ctx), row, active.workflowSlug)
}

func (m *RunManager) prepareRunRow(
	ctx context.Context,
	spec startRunSpec,
	runID string,
	startedAt time.Time,
	requestID string,
) (globaldb.Run, bool, error) {
	if spec.mode == runModeExec && strings.TrimSpace(spec.runtimeCfg.RunID) != "" {
		row, ok, err := m.resumeExistingExecRun(ctx, spec, runID, startedAt, requestID)
		if err != nil {
			return globaldb.Run{}, false, err
		}
		if ok {
			return row, false, nil
		}
	}

	runArtifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		return globaldb.Run{}, false, err
	}
	if err := reserveRunDirectory(runArtifacts.RunDir); err != nil {
		return globaldb.Run{}, false, err
	}

	row, err := m.globalDB.PutRun(ctx, globaldb.Run{
		RunID:            runID,
		WorkspaceID:      spec.workspace.ID,
		WorkflowID:       spec.workflowID,
		Mode:             spec.mode,
		Status:           runStatusStarting,
		PresentationMode: spec.presentationMode,
		StartedAt:        startedAt,
		RequestID:        requestID,
	})
	if err != nil {
		cleanupRunDirectory(runArtifacts.RunDir)
		return globaldb.Run{}, false, err
	}
	return row, true, nil
}

func (m *RunManager) resumeExistingExecRun(
	ctx context.Context,
	spec startRunSpec,
	runID string,
	startedAt time.Time,
	requestID string,
) (globaldb.Run, bool, error) {
	if active := m.getActive(runID); active != nil {
		return globaldb.Run{}, false, globaldb.ErrRunAlreadyExists
	}

	runArtifacts, err := model.ResolvePersistedRunArtifacts(spec.workspace.RootDir, runID)
	if err != nil {
		return globaldb.Run{}, false, err
	}
	if _, err := os.Stat(runArtifacts.RunMetaPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return globaldb.Run{}, false, nil
		}
		return globaldb.Run{}, false, fmt.Errorf("stat persisted exec run %q: %w", runID, err)
	}

	record, err := runpkg.LoadPersistedExecRun(spec.workspace.RootDir, runID)
	if err != nil {
		return globaldb.Run{}, false, err
	}
	if record.Mode != model.ModeExec {
		return globaldb.Run{}, false, fmt.Errorf("run %q is not an exec run", runID)
	}

	row, err := m.globalDB.GetRun(ctx, runID)
	if err != nil {
		if !errors.Is(err, globaldb.ErrRunNotFound) {
			return globaldb.Run{}, false, err
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = startedAt
		}
		row, err = m.globalDB.PutRun(ctx, globaldb.Run{
			RunID:            runID,
			WorkspaceID:      spec.workspace.ID,
			Mode:             runModeExec,
			Status:           runStatusStarting,
			PresentationMode: spec.presentationMode,
			StartedAt:        record.CreatedAt.UTC(),
			RequestID:        requestID,
		})
		if err != nil {
			return globaldb.Run{}, false, err
		}
		return row, true, nil
	}

	if row.Mode != runModeExec {
		return globaldb.Run{}, false, fmt.Errorf("run %q is not an exec run", runID)
	}

	row.WorkspaceID = spec.workspace.ID
	row.WorkflowID = nil
	row.Status = runStatusStarting
	row.PresentationMode = spec.presentationMode
	if row.StartedAt.IsZero() {
		if record.CreatedAt.IsZero() {
			row.StartedAt = startedAt
		} else {
			row.StartedAt = record.CreatedAt.UTC()
		}
	}
	row.EndedAt = nil
	row.ErrorText = ""
	row.RequestID = requestID

	updatedRow, err := m.globalDB.UpdateRun(ctx, row)
	if err != nil {
		return globaldb.Run{}, false, err
	}
	return updatedRow, true, nil
}

func (m *RunManager) openRunScopeForStart(
	ctx context.Context,
	runtimeCfg *model.RuntimeConfig,
	workspaceRoot string,
) (model.RunScope, error) {
	scopeCtx := detachContext(ctx)
	if runtimeCfg != nil && runtimeCfg.EnableExecutableExtensions {
		resolvedRoot := strings.TrimSpace(runtimeCfg.WorkspaceRoot)
		if resolvedRoot == "" {
			resolvedRoot = strings.TrimSpace(workspaceRoot)
		}
		bridge, err := newExtensionBridge(m, resolvedRoot)
		if err != nil {
			return nil, err
		}
		scopeCtx = extensions.WithDaemonHostBridge(scopeCtx, bridge)
	}

	return m.openRunScope(scopeCtx, runtimeCfg, model.OpenRunScopeOptions{
		EnableExecutableExtensions: runtimeCfg.EnableExecutableExtensions,
	})
}

func newActiveRun(
	runCtx context.Context,
	cancel context.CancelFunc,
	row globaldb.Run,
	spec startRunSpec,
	scope model.RunScope,
) *activeRun {
	return &activeRun{
		runID:        row.RunID,
		workspaceID:  row.WorkspaceID,
		workflowSlug: strings.TrimSpace(spec.workflowSlug),
		workflowID:   cloneStringPtr(spec.workflowID),
		mode:         spec.mode,
		scope:        scope,
		ctx:          runCtx,
		cancel:       cancel,
		done:         make(chan struct{}),
		closeTimeout: defaultRunCloseTimeout,
		workflowRoot: strings.TrimSpace(spec.workflowRoot),
	}
}

func (m *RunManager) failStartRun(
	ctx context.Context,
	row globaldb.Run,
	closeTimeout time.Duration,
	scope model.RunScope,
	createdRun bool,
	err error,
) error {
	failedAt := m.now().UTC()
	row.Status = runStatusFailed
	row.ErrorText = err.Error()
	row.EndedAt = &failedAt
	m.recordTerminalOutcome(row.Mode, row.Status)
	updatedRow, updateErr := m.globalDB.UpdateRun(detachContext(ctx), row)
	if updateErr == nil {
		m.publishRunWorkspaceEvent(ctx, updatedRow, "", apicore.WorkspaceEventKindRunTerminal)
	}
	closeErr := closeRunScope(ctx, scope, closeTimeout)
	if createdRun {
		if runArtifacts, artifactsErr := model.ResolveHomeRunArtifacts(row.RunID); artifactsErr == nil {
			cleanupRunDirectory(runArtifacts.RunDir)
		}
	}
	return errors.Join(err, updateErr, closeErr)
}

func (m *RunManager) startWatcher(active *activeRun) error {
	if active == nil || strings.TrimSpace(active.workflowRoot) == "" {
		return nil
	}
	watcher, err := startWorkflowWatcher(active.ctx, workflowWatcherConfig{
		WorkflowRoot: active.workflowRoot,
		Debounce:     m.watcherDebounce,
		Sync: func(ctx context.Context, workflowRoot string) error {
			_, err := corepkg.SyncDirect(ctx, model.SyncConfig{TasksDir: workflowRoot})
			return err
		},
		Emit: func(ctx context.Context, item artifactSyncEvent) error {
			if err := emitArtifactUpdatedEvent(ctx, active.scope, active.runID, item); err != nil {
				return err
			}
			m.publishArtifactWorkspaceEvent(ctx, active, item)
			return nil
		},
		Logger: slog.Default(),
	})
	if err != nil {
		return err
	}
	active.setWatcher(watcher)
	return nil
}

func (m *RunManager) runAsync(active *activeRun, row globaldb.Run, runtimeCfg *model.RuntimeConfig) {
	defer m.runWG.Done()
	defer close(active.done)
	defer active.cancel()
	defer m.removeActive(active.runID)

	if active.mode == runModeReviewWatch {
		m.executeReviewWatchRun(active, row)
		return
	}
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
		fallback = cancelledTerminalState(err)
		m.finishRun(active, row, fallback)
		return
	}

	if err := startScopeRuntime(active.ctx, scope); err != nil {
		fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
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
	m.publishRunWorkspaceEvent(active.ctx, row, active.workflowSlug, apicore.WorkspaceEventKindRunStatusChanged)

	prep, err := m.prepare(active.ctx, runtimeCfg, scope)
	if err != nil {
		switch {
		case errors.Is(err, plan.ErrNoWork):
			fallback = completedTerminalState(scope.RunArtifacts(), completedNoWorkSummary)
		default:
			fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
		}
		m.finishRun(active, row, fallback)
		return
	}
	prep.SetRunScope(scope)
	if err := emitPreparedJobQueuedEvents(
		active.ctx,
		prep.Journal(),
		runtimeCfg.RunID,
		prep.Jobs,
		runtimeCfg.AccessMode,
	); err != nil {
		fallback = fallbackTerminalState(
			scope.RunArtifacts(),
			fmt.Errorf("publish prepared queued jobs: %w", err),
			active.cancelWasRequested(),
		)
		m.finishRun(active, row, fallback)
		return
	}

	executionErr = m.execute(active.ctx, prep, runtimeCfg)
	fallback = fallbackTerminalState(scope.RunArtifacts(), executionErr, active.cancelWasRequested())
	m.finishRun(active, row, fallback)
}

func (m *RunManager) executeExecRun(active *activeRun, row globaldb.Run, runtimeCfg *model.RuntimeConfig) {
	scope := active.scope
	var fallback terminalState

	if err := context.Cause(active.ctx); err != nil {
		fallback = cancelledTerminalState(err)
		m.finishRun(active, row, fallback)
		return
	}

	if err := startScopeRuntime(active.ctx, scope); err != nil {
		fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
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
	m.publishRunWorkspaceEvent(active.ctx, row, active.workflowSlug, apicore.WorkspaceEventKindRunStatusChanged)

	executionErr := m.executeExec(active.ctx, runtimeCfg, scope)
	fallback = fallbackTerminalState(scope.RunArtifacts(), executionErr, active.cancelWasRequested())
	m.finishRun(active, row, fallback)
}

func (m *RunManager) finishRun(active *activeRun, row globaldb.Run, fallback terminalState) {
	scope := active.scope
	if err := active.stopWatcher(); err != nil {
		slog.Default().Warn("daemon: stop workflow watcher", "run_id", active.runID, "error", err)
	}
	terminal, err := m.resolveTerminalState(detachContext(active.ctx), scope.RunArtifacts().RunID, fallback, scope)
	if err != nil {
		terminal = failedTerminalState(scope.RunArtifacts(), err)
	}

	row.Status = terminal.status
	row.ErrorText = terminal.errorText
	if isTerminalRunStatus(terminal.status) {
		endedAt := m.now().UTC()
		row.EndedAt = &endedAt
	}

	if err := m.persistRuntimeIntegrity(detachContext(active.ctx), row.RunID, scope); err != nil {
		slog.Default().Warn("daemon run integrity persistence failed", "run_id", row.RunID, "error", err)
	}
	if closeErr := closeRunScope(active.ctx, scope, active.currentCloseTimeout()); closeErr != nil {
		// Best-effort teardown should not block the terminal row mirror.
		_ = closeErr
	}
	m.recordTerminalOutcome(row.Mode, row.Status)
	if _, err := m.globalDB.UpdateRun(detachContext(active.ctx), row); err != nil {
		return
	}
	m.publishRunWorkspaceEvent(active.ctx, row, active.workflowSlug, apicore.WorkspaceEventKindRunTerminal)
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

	subscription := openLiveRunSubscription(active)
	defer subscription.close()

	lastCursor, terminal, err := m.replayRunStream(ctx, stream, row.RunID, after)
	if err != nil {
		stream.errors <- err
		return
	}
	if terminal || subscription == nil {
		return
	}

	streamLiveRunEvents(ctx, stream, subscription, lastCursor)
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
		run.WorkflowSlug = strings.TrimSpace(fallbackWorkflowSlug)
		if run.WorkflowSlug == "" {
			workflow, err := m.getWorkflow(ctx, workflowID)
			if err != nil && !errors.Is(err, globaldb.ErrWorkflowNotFound) {
				return apicore.Run{}, err
			}
			if err == nil {
				run.WorkflowSlug = workflow.Slug
			}
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
	if run.reviewWatchKey != nil {
		m.activeReviewWatches[*run.reviewWatchKey] = run.runID
	}
}

func (m *RunManager) removeActive(runID string) {
	trimmed := strings.TrimSpace(runID)
	m.mu.Lock()
	active := m.active[trimmed]
	delete(m.active, trimmed)
	if active != nil && active.reviewWatchKey != nil {
		delete(m.activeReviewWatches, *active.reviewWatchKey)
	}
	m.mu.Unlock()
	if err := m.evictRunDB(trimmed); err != nil {
		slog.Default().Warn("daemon: evict cached run db", "run_id", trimmed, "error", err)
	}
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

func (s *liveRunSubscription) close() {
	if s == nil || s.unsubscribe == nil {
		return
	}
	s.unsubscribe()
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

func (r *activeRun) setCloseTimeout(timeout time.Duration) {
	if r == nil || timeout <= 0 {
		return
	}
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if timeout > r.closeTimeout {
		r.closeTimeout = timeout
	}
}

func (r *activeRun) currentCloseTimeout() time.Duration {
	if r == nil {
		return defaultRunCloseTimeout
	}
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	if r.closeTimeout <= 0 {
		return defaultRunCloseTimeout
	}
	return r.closeTimeout
}

func (r *activeRun) setWatcher(watcher *workflowWatcher) {
	if r == nil {
		return
	}
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.watcher = watcher
}

func (r *activeRun) stopWatcher() error {
	if r == nil {
		return nil
	}

	r.stateMu.Lock()
	watcher := r.watcher
	r.watcher = nil
	r.stateMu.Unlock()

	if watcher == nil {
		return nil
	}
	return watcher.Stop()
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

func closeRunScope(ctx context.Context, scope model.RunScope, timeout time.Duration) error {
	if scope == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = defaultRunCloseTimeout
	}
	closeCtx, cancel := boundedLifecycleContext(ctx, timeout)
	defer cancel()
	return scope.Close(closeCtx)
}

func openLiveRunSubscription(active *activeRun) *liveRunSubscription {
	if active == nil || active.scope == nil {
		return nil
	}
	liveBus := active.scope.RunEventBus()
	if liveBus == nil {
		return nil
	}
	subID, liveCh, unsubscribe := liveBus.Subscribe()
	return &liveRunSubscription{
		bus:         liveBus,
		ch:          liveCh,
		unsubscribe: unsubscribe,
		subID:       subID,
	}
}

func (m *RunManager) replayRunStream(
	ctx context.Context,
	stream *runStream,
	runID string,
	after apicore.StreamCursor,
) (apicore.StreamCursor, bool, error) {
	lastCursor := after
	for {
		page, err := m.Events(ctx, runID, apicore.RunEventPageQuery{
			After: lastCursor,
			Limit: defaultStreamPageLimit,
		})
		if err != nil {
			return lastCursor, false, err
		}
		if len(page.Events) == 0 {
			return lastCursor, false, nil
		}
		for _, item := range page.Events {
			lastCursor = apicore.CursorFromEvent(item)
			if !sendRunStreamItem(ctx, stream.events, apicore.RunStreamItem{Event: &item}) {
				return lastCursor, true, nil
			}
			if isTerminalEventKind(item.Kind) {
				return lastCursor, true, nil
			}
		}
		if !page.HasMore {
			return lastCursor, false, nil
		}
	}
}

func streamLiveRunEvents(
	ctx context.Context,
	stream *runStream,
	subscription *liveRunSubscription,
	lastCursor apicore.StreamCursor,
) {
	for {
		if subscription.bus != nil && subscription.bus.DroppedFor(subscription.subID) > 0 {
			overflow := apicore.RunStreamItem{
				Overflow: &apicore.RunStreamOverflow{Reason: "subscriber_dropped_messages"},
			}
			_ = sendRunStreamItem(ctx, stream.events, overflow)
			return
		}

		select {
		case <-ctx.Done():
			return
		case item, ok := <-subscription.ch:
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

func startScopeRuntime(ctx context.Context, scope model.RunScope) error {
	if scope == nil {
		return nil
	}
	if runtimeManager := scope.RunManager(); runtimeManager != nil {
		return runtimeManager.Start(ctx)
	}
	return nil
}

func openRunDBForRunID(ctx context.Context, runID string) (*rundb.RunDB, error) {
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
	runDB, err := openRunDBForRunID(ctx, runID)
	if err != nil {
		return terminalState{}, err
	}
	defer func() {
		_ = runDB.Close()
	}()

	return resolveTerminalStateWithRunDB(ctx, runID, fallback, scope, runDB)
}

func (m *RunManager) resolveTerminalState(
	ctx context.Context,
	runID string,
	fallback terminalState,
	scope model.RunScope,
) (terminalState, error) {
	lease, err := m.acquireRunDB(ctx, runID)
	if err != nil {
		return terminalState{}, err
	}
	defer func() {
		_ = lease.Close()
	}()
	return resolveTerminalStateWithRunDB(ctx, runID, fallback, scope, lease.DB())
}

func resolveTerminalStateWithRunDB(
	ctx context.Context,
	runID string,
	fallback terminalState,
	scope model.RunScope,
	runDB *rundb.RunDB,
) (terminalState, error) {
	if runDB == nil {
		return terminalState{}, errors.New("daemon: run db is required")
	}

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

func (m *RunManager) workflowSlugsForRuns(
	ctx context.Context,
	rows []globaldb.Run,
) (map[string]string, error) {
	if len(rows) == 0 {
		return map[string]string{}, nil
	}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].WorkflowID == nil {
			continue
		}
		ids = append(ids, strings.TrimSpace(*rows[i].WorkflowID))
	}
	return m.lookupWorkflowSlugs(ctx, ids)
}

func workflowSlugForRun(row globaldb.Run, slugs map[string]string) string {
	if row.WorkflowID == nil {
		return ""
	}
	return strings.TrimSpace(slugs[strings.TrimSpace(*row.WorkflowID)])
}

func (m *RunManager) acquireRunDB(ctx context.Context, runID string) (*runDBLease, error) {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return nil, errors.New("daemon: run id is required")
	}
	if err := m.evictIdleRunDBs(m.now()); err != nil {
		return nil, err
	}

	m.runDBMu.Lock()
	if entry, ok := m.runDBs[trimmed]; ok {
		entry.references++
		entry.lastUsedAt = m.now().UTC()
		db := entry.db
		m.runDBMu.Unlock()
		return &runDBLease{manager: m, runID: trimmed, db: db}, nil
	}
	db, err := m.openRunDB(ctx, trimmed)
	if err != nil {
		m.runDBMu.Unlock()
		return nil, err
	}
	m.runDBs[trimmed] = &cachedRunDB{
		db:         db,
		references: 1,
		lastUsedAt: m.now().UTC(),
	}
	m.runDBMu.Unlock()
	return &runDBLease{manager: m, runID: trimmed, db: db}, nil
}

func (m *RunManager) releaseRunDB(runID string) error {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return nil
	}

	var closeDB *rundb.RunDB
	m.runDBMu.Lock()
	entry, ok := m.runDBs[trimmed]
	if ok {
		if entry.references > 0 {
			entry.references--
		}
		if entry.references == 0 {
			entry.lastUsedAt = m.now().UTC()
			if entry.evictOnRelease {
				delete(m.runDBs, trimmed)
				closeDB = entry.db
			}
		}
	}
	m.runDBMu.Unlock()
	if closeDB != nil {
		return closeDB.Close()
	}
	return nil
}

func (m *RunManager) evictRunDB(runID string) error {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return nil
	}

	var closeDB *rundb.RunDB
	m.runDBMu.Lock()
	entry, ok := m.runDBs[trimmed]
	if ok {
		if entry.references == 0 {
			delete(m.runDBs, trimmed)
			closeDB = entry.db
		} else {
			entry.evictOnRelease = true
		}
	}
	m.runDBMu.Unlock()
	if closeDB != nil {
		return closeDB.Close()
	}
	return nil
}

func (m *RunManager) evictIdleRunDBs(now time.Time) error {
	if m == nil || m.runDBIdleTTL <= 0 {
		return nil
	}
	cutoff := now.UTC().Add(-m.runDBIdleTTL)
	closeList := make([]*rundb.RunDB, 0)

	m.runDBMu.Lock()
	for runID, entry := range m.runDBs {
		if entry.references != 0 {
			continue
		}
		if entry.lastUsedAt.IsZero() || entry.lastUsedAt.After(cutoff) {
			continue
		}
		delete(m.runDBs, runID)
		closeList = append(closeList, entry.db)
	}
	m.runDBMu.Unlock()

	var err error
	for _, db := range closeList {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}

func (m *RunManager) closeRunDBCache(ctx context.Context) error {
	if m == nil {
		return nil
	}

	closeList := make([]*rundb.RunDB, 0)
	m.runDBMu.Lock()
	for runID, entry := range m.runDBs {
		if entry.references != 0 {
			entry.evictOnRelease = true
			continue
		}
		delete(m.runDBs, runID)
		closeList = append(closeList, entry.db)
	}
	m.runDBMu.Unlock()

	var err error
	for _, db := range closeList {
		closeCtx, cancel := boundedLifecycleContext(ctx, m.shutdownDrainTimeout)
		closeErr := db.CloseContext(closeCtx)
		cancel()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}

func (l *runDBLease) Close() error {
	if l == nil || l.manager == nil {
		return nil
	}
	return l.manager.releaseRunDB(l.runID)
}

func (l *runDBLease) DB() *rundb.RunDB {
	if l == nil {
		return nil
	}
	return l.db
}

func terminalStateFromEvent(event *eventspkg.Event) (terminalState, bool, error) {
	if event == nil {
		return terminalState{}, false, nil
	}

	switch event.Kind {
	case eventspkg.EventKindRunCrashed:
		var payload kinds.RunCrashedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return terminalState{}, false, fmt.Errorf("daemon: decode run.crashed payload: %w", err)
		}
		return terminalState{
			status:    runStatusCrashed,
			errorText: strings.TrimSpace(payload.Error),
			kind:      event.Kind,
			payload:   payload,
		}, true, nil
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
	_, err = runJournal.SubmitWithSeq(ctx, eventspkg.Event{
		RunID:   strings.TrimSpace(runID),
		Kind:    kind,
		Payload: rawPayload,
	})
	return err
}

func emitPreparedJobQueuedEvents(
	ctx context.Context,
	runJournal submitter,
	runID string,
	jobs []model.Job,
	accessMode string,
) error {
	if runJournal == nil || len(jobs) == 0 {
		return nil
	}

	for index := range jobs {
		job := jobs[index]
		payload := kinds.JobQueuedPayload{
			Index:           index,
			CodeFile:        queuedJobCodeFileLabel(job),
			CodeFiles:       append([]string(nil), job.CodeFiles...),
			Issues:          job.IssueCount(),
			TaskTitle:       strings.TrimSpace(job.TaskTitle),
			TaskType:        strings.TrimSpace(job.TaskType),
			SafeName:        strings.TrimSpace(job.SafeName),
			IDE:             strings.TrimSpace(job.IDE),
			Model:           strings.TrimSpace(job.Model),
			ReasoningEffort: strings.TrimSpace(job.ReasoningEffort),
			AccessMode:      strings.TrimSpace(accessMode),
			OutLog:          strings.TrimSpace(job.OutLog),
			ErrLog:          strings.TrimSpace(job.ErrLog),
		}
		if err := submitSyntheticEvent(ctx, runJournal, runID, eventspkg.EventKindJobQueued, payload); err != nil {
			return err
		}
	}
	return nil
}

func queuedJobCodeFileLabel(job model.Job) string {
	if len(job.CodeFiles) == 0 {
		return ""
	}
	if len(job.CodeFiles) > 3 {
		return fmt.Sprintf(
			"%s and %d more",
			strings.Join(job.CodeFiles[:3], ", "),
			len(job.CodeFiles)-3,
		)
	}
	return strings.Join(job.CodeFiles, ", ")
}

type submitter interface {
	SubmitWithSeq(context.Context, eventspkg.Event) (uint64, error)
}

func emitArtifactUpdatedEvent(
	ctx context.Context,
	scope model.RunScope,
	runID string,
	item artifactSyncEvent,
) error {
	if scope == nil || scope.RunJournal() == nil {
		return nil
	}
	payload := kinds.ArtifactUpdatedPayload{
		Path:       strings.TrimSpace(item.RelativePath),
		ChangeKind: strings.TrimSpace(item.ChangeKind),
		Checksum:   strings.TrimSpace(item.Checksum),
	}
	return submitSyntheticEvent(ctx, scope.RunJournal(), runID, eventspkg.EventKindArtifactUpdated, payload)
}

func fallbackTerminalState(
	runArtifacts model.RunArtifacts,
	err error,
	cancelRequested bool,
) terminalState {
	switch {
	case cancelRequested || errors.Is(err, context.Canceled):
		return cancelledTerminalState(err)
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

func cancelledTerminalState(err error) terminalState {
	reason := errorString(err)
	if reason == "" {
		reason = runStatusCancelled
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
	case runStatusCompleted, runStatusFailed, runStatusCancelled, runStatusCrashed:
		return true
	default:
		return false
	}
}

func isTerminalEventKind(kind eventspkg.EventKind) bool {
	switch kind {
	case eventspkg.EventKindRunCrashed,
		eventspkg.EventKindRunCompleted,
		eventspkg.EventKindRunFailed,
		eventspkg.EventKindRunCancelled:
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
	applyOptionalString(&cfg.IDE, overrides.IDE)
	applyOptionalString(&cfg.Model, overrides.Model)
	applyOptionalOutputFormat(cfg, overrides.OutputFormat)
	applyOptionalString(&cfg.ReasoningEffort, overrides.ReasoningEffort)
	applyOptionalString(&cfg.AccessMode, overrides.AccessMode)
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

func applyTaskProjectConfig(cfg *model.RuntimeConfig, projectCfg workspacecfg.TaskRunConfig) {
	if cfg == nil {
		return
	}
	applyOptionalOutputFormat(cfg, projectCfg.OutputFormat)
	if projectCfg.IncludeCompleted != nil {
		cfg.IncludeCompleted = *projectCfg.IncludeCompleted
	}
	cfg.TaskRuntimeRules = model.CloneTaskRuntimeRules(derefTaskRuntimeRules(projectCfg.TaskRuntimeRules))
}

func applyReviewProjectConfig(cfg *model.RuntimeConfig, projectCfg workspacecfg.FixReviewsConfig) {
	if cfg == nil {
		return
	}
	applyOptionalOutputFormat(cfg, projectCfg.OutputFormat)
	if projectCfg.Concurrent != nil {
		cfg.Concurrent = *projectCfg.Concurrent
	}
	if projectCfg.BatchSize != nil {
		cfg.BatchSize = *projectCfg.BatchSize
	}
	if projectCfg.IncludeResolved != nil {
		cfg.IncludeResolved = *projectCfg.IncludeResolved
	}
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

func applyPersistedExecRuntimeDefaults(
	cfg *model.RuntimeConfig,
	workspaceRoot string,
	overrides runtimeOverrideInput,
) error {
	if cfg == nil || overrides.RunID == nil || strings.TrimSpace(*overrides.RunID) == "" {
		return nil
	}

	runID := strings.TrimSpace(*overrides.RunID)
	runArtifacts, err := model.ResolvePersistedRunArtifacts(workspaceRoot, runID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(runArtifacts.RunMetaPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat persisted exec run: %w", err)
	}

	record, err := runpkg.LoadPersistedExecRun(workspaceRoot, runID)
	if err != nil {
		return err
	}

	cfg.Persist = true
	cfg.RunID = record.RunID
	cfg.WorkspaceRoot = record.WorkspaceRoot
	cfg.IDE = record.IDE
	cfg.Model = record.Model
	cfg.ReasoningEffort = record.ReasoningEffort
	cfg.AccessMode = record.AccessMode
	cfg.AddDirs = corepkg.NormalizeAddDirs(record.AddDirs)
	return nil
}

func applyRuntimeOverrideInput(cfg *model.RuntimeConfig, overrides runtimeOverrideInput) error {
	if cfg == nil {
		return nil
	}
	applyRuntimeOverrideStrings(cfg, overrides)
	if err := applyOptionalDuration(cfg, overrides.Timeout); err != nil {
		return overrideValueError("runtime_overrides", "timeout", err)
	}
	applyRuntimeOverrideScalars(cfg, overrides)
	return nil
}

func applyRuntimeOverrideStrings(cfg *model.RuntimeConfig, overrides runtimeOverrideInput) {
	applyOptionalString(&cfg.RunID, overrides.RunID)
	applyOptionalString(&cfg.IDE, overrides.IDE)
	applyOptionalString(&cfg.Model, overrides.Model)
	applyOptionalString(&cfg.AgentName, overrides.AgentName)
	applyOptionalOutputFormat(cfg, overrides.OutputFormat)
	applyOptionalString(&cfg.ReasoningEffort, overrides.ReasoningEffort)
	applyOptionalString(&cfg.AccessMode, overrides.AccessMode)
}

func applyRuntimeOverrideScalars(cfg *model.RuntimeConfig, overrides runtimeOverrideInput) {
	applyRuntimeOverrideExecutionScalars(cfg, overrides)
	applyRuntimeOverrideWorkflowScalars(cfg, overrides)
}

func applyRuntimeOverrideExecutionScalars(cfg *model.RuntimeConfig, overrides runtimeOverrideInput) {
	if overrides.DryRun != nil {
		cfg.DryRun = *overrides.DryRun
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
	if overrides.Verbose != nil {
		cfg.Verbose = *overrides.Verbose
	}
	if overrides.Persist != nil {
		cfg.Persist = *overrides.Persist
	}
	if overrides.EnableExecutableExtensions != nil {
		cfg.EnableExecutableExtensions = *overrides.EnableExecutableExtensions
	}
	if overrides.ExplicitRuntime != nil {
		cfg.ExplicitRuntime = *overrides.ExplicitRuntime
	}
}

func applyRuntimeOverrideWorkflowScalars(cfg *model.RuntimeConfig, overrides runtimeOverrideInput) {
	if overrides.Concurrent != nil {
		cfg.Concurrent = *overrides.Concurrent
	}
	if overrides.BatchSize != nil {
		cfg.BatchSize = *overrides.BatchSize
	}
	if overrides.IncludeCompleted != nil {
		cfg.IncludeCompleted = *overrides.IncludeCompleted
	}
	if overrides.IncludeResolved != nil {
		cfg.IncludeResolved = *overrides.IncludeResolved
	}
	if overrides.TaskRuntimeRules != nil {
		cfg.TaskRuntimeRules = model.CloneTaskRuntimeRules(*overrides.TaskRuntimeRules)
	}
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

func applyOptionalString(dst *string, value *string) {
	if value == nil {
		return
	}
	*dst = strings.TrimSpace(*value)
}

func applyOptionalOutputFormat(cfg *model.RuntimeConfig, value *string) {
	if cfg == nil || value == nil {
		return
	}
	cfg.OutputFormat = model.OutputFormat(strings.TrimSpace(*value))
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
