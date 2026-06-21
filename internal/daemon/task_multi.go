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
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	runparallel "github.com/compozy/compozy/internal/core/run/parallel"
	"github.com/compozy/compozy/internal/core/run/recovery"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/core/worktree"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	taskMultiItemStatusQueued    = "queued"
	taskMultiItemStatusRunning   = "running"
	taskMultiItemStatusCompleted = "completed"
	taskMultiItemStatusFailed    = "failed"
	taskMultiItemStatusCanceled  = "canceled"

	taskMultiChildPollInterval = 100 * time.Millisecond
)

type preparedTaskMulti struct {
	workspace        globaldb.Workspace
	mode             string
	presentationMode string
	parallelLimit    int
	items            []preparedTaskMultiItem
	parallelTasks    *preparedParallelTasks
}

type preparedTaskMultiItem struct {
	slug         string
	workflowID   *string
	workflowRoot string
	runtimeCfg   *model.RuntimeConfig
	recovery     workspacecfg.AgentRecoveryConfig
}

type preparedParallelTasks struct {
	config workspacecfg.ParallelTasksConfig
	waves  runparallel.Waves
	tasks  []runparallel.TaskSpec
}

type taskMultiSnapshotBuilder struct {
	items []apicore.TaskRunMultipleItem
	index map[string]int
}

type taskWorktreeChildRun struct {
	Run           apicore.Run
	Allocation    taskMultiWorktreeAllocation
	RuntimeConfig *model.RuntimeConfig
}

// StartTaskRunMultiple starts one daemon-owned parent for an ordered task queue.
func (m *RunManager) StartTaskRunMultiple(
	ctx context.Context,
	workspaceRef string,
	req apicore.TaskRunMultipleRequest,
) (apicore.Run, error) {
	slugs, err := normalizeTaskMultiSlugs(req.Slugs)
	if err != nil {
		return apicore.Run{}, err
	}
	mode, err := resolveTaskMultiMode(req.Mode)
	if err != nil {
		return apicore.Run{}, err
	}
	childOverrides, err := taskMultiChildRuntimeOverrides(req.RuntimeOverrides)
	if err != nil {
		return apicore.Run{}, err
	}
	prepared, err := m.prepareTaskMultiStart(detachContext(ctx), workspaceRef, slugs, mode, req, childOverrides)
	if err != nil {
		return apicore.Run{}, err
	}
	runtimeCfg, err := taskMultiParentRuntimeConfig(req.RuntimeOverrides, prepared.workspace.RootDir)
	if err != nil {
		return apicore.Run{}, err
	}
	return m.startRun(ctx, startRunSpec{
		workspace:        prepared.workspace,
		mode:             runModeTaskMulti,
		presentationMode: prepared.presentationMode,
		runtimeCfg:       runtimeCfg,
		taskMulti:        prepared,
	})
}

func (m *RunManager) startParallelTaskRunIfEnabled(
	ctx context.Context,
	workspaceRow globaldb.Workspace,
	workflowID *string,
	workflowSlug string,
	runtimeCfg *model.RuntimeConfig,
	recoveryCfg workspacecfg.AgentRecoveryConfig,
	parallelCfg workspacecfg.ParallelTasksConfig,
	presentationMode string,
) (apicore.Run, bool, error) {
	if runtimeCfg == nil {
		return apicore.Run{}, false, errors.New("daemon: runtime config is required")
	}
	parallelCfg = parallelCfg.ApplyDefaults()
	if parallelCfg.Enabled == nil || !*parallelCfg.Enabled {
		return apicore.Run{}, false, nil
	}
	waves, taskSpecs, err := buildDaemonParallelTaskPlan(
		ctx,
		runtimeCfg.TasksDir,
		strings.TrimSpace(workflowSlug),
		runtimeCfg.IncludeCompleted,
	)
	if err != nil {
		return apicore.Run{}, true, err
	}
	if len(taskSpecs) == 0 {
		return apicore.Run{}, true, taskMultiValidationProblem(
			"parallel_tasks_empty",
			"parallel task execution requires at least one task",
			"tasks",
		)
	}
	parentCfg := runtimeCfg.Clone()
	if parentCfg == nil {
		return apicore.Run{}, true, errors.New("daemon: runtime config is required")
	}
	parentCfg.Name = taskMultiRunName
	parentCfg.TargetTaskNumber = nil
	prepared := &preparedTaskMulti{
		workspace:        workspaceRow,
		mode:             workspacecfg.TaskRunMultipleModeParallel,
		presentationMode: presentationMode,
		parallelLimit:    parallelTaskMaxConcurrency(parallelCfg),
		items: []preparedTaskMultiItem{
			{
				slug:         strings.TrimSpace(workflowSlug),
				workflowID:   cloneStringPtr(workflowID),
				workflowRoot: strings.TrimSpace(runtimeCfg.TasksDir),
				runtimeCfg:   runtimeCfg,
				recovery:     recoveryCfg,
			},
		},
		parallelTasks: &preparedParallelTasks{
			config: parallelCfg,
			waves:  waves,
			tasks:  taskSpecs,
		},
	}
	run, err := m.startRun(ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     strings.TrimSpace(workflowSlug),
		workflowRoot:     strings.TrimSpace(runtimeCfg.TasksDir),
		mode:             runModeTaskMulti,
		presentationMode: presentationMode,
		runtimeCfg:       parentCfg,
		taskMulti:        prepared,
		recovery:         recoveryCfg,
	})
	return run, true, err
}

// RunMultipleSnapshot reconstructs the ordered child state for a parent multi-run.
func (m *RunManager) RunMultipleSnapshot(ctx context.Context, runID string) (apicore.TaskRunMultipleSnapshot, error) {
	listCtx := detachContext(ctx)
	row, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID))
	if err != nil {
		return apicore.TaskRunMultipleSnapshot{}, err
	}
	if row.Mode != runModeTaskMulti {
		return apicore.TaskRunMultipleSnapshot{}, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"run_not_task_multi",
			"run is not a multi-task parent",
			map[string]any{"run_id": row.RunID, "mode": row.Mode},
			nil,
		)
	}
	runView, err := m.toCoreRun(listCtx, row, "")
	if err != nil {
		return apicore.TaskRunMultipleSnapshot{}, err
	}

	lease, err := m.acquireRunDB(listCtx, row.RunID)
	if err != nil {
		return apicore.TaskRunMultipleSnapshot{}, err
	}
	defer func() {
		_ = lease.Close()
	}()
	eventRows, err := lease.DB().ListEvents(listCtx, 0, 0)
	if err != nil {
		return apicore.TaskRunMultipleSnapshot{}, err
	}
	builder := newTaskMultiSnapshotBuilder()
	for _, event := range eventRows.Events {
		if err := builder.applyEvent(event); err != nil {
			return apicore.TaskRunMultipleSnapshot{}, err
		}
	}
	return apicore.TaskRunMultipleSnapshot{
		Run:   runView,
		Items: builder.snapshotItems(),
	}, nil
}

func (m *RunManager) prepareTaskMultiStart(
	ctx context.Context,
	workspaceRef string,
	slugs []string,
	mode string,
	req apicore.TaskRunMultipleRequest,
	childOverrides json.RawMessage,
) (*preparedTaskMulti, error) {
	items := make([]preparedTaskMultiItem, 0, len(slugs))
	var workspaceRow globaldb.Workspace
	var presentationMode string
	for idx, slug := range slugs {
		row, workflowID, runtimeCfg, recoveryCfg, _, childPresentationMode, err := m.prepareTaskStart(
			ctx,
			workspaceRef,
			slug,
			apicore.TaskRunRequest{
				Workspace:        req.Workspace,
				PresentationMode: req.PresentationMode,
				RuntimeOverrides: childOverrides,
			},
		)
		if err != nil {
			return nil, err
		}
		if idx == 0 {
			workspaceRow = row
			presentationMode = childPresentationMode
		}
		items = append(items, preparedTaskMultiItem{
			slug:         strings.TrimSpace(slug),
			workflowID:   cloneStringPtr(workflowID),
			workflowRoot: strings.TrimSpace(runtimeCfg.TasksDir),
			runtimeCfg:   runtimeCfg,
			recovery:     recoveryCfg,
		})
	}
	if len(items) == 0 {
		return nil, taskMultiValidationProblem("slugs_required", "slugs is required", "slugs")
	}
	if strings.TrimSpace(presentationMode) == "" {
		var err error
		presentationMode, err = normalizePresentationMode(req.PresentationMode)
		if err != nil {
			return nil, err
		}
	}
	parallelLimit := 0
	if mode == workspacecfg.TaskRunMultipleModeParallel {
		limit, err := m.parallelLimitForRequest(ctx, workspaceRow.RootDir, req.ParallelLimit)
		if err != nil {
			return nil, err
		}
		parallelLimit = limit
	}
	return &preparedTaskMulti{
		workspace:        workspaceRow,
		mode:             mode,
		presentationMode: presentationMode,
		parallelLimit:    parallelLimit,
		items:            items,
	}, nil
}

// parallelLimitForRequest resolves the bounded-fanout limit for a parallel parent
// run. An explicit positive request limit wins; otherwise the workspace
// [tasks.run] configuration supplies the effective limit. The CLI only populates
// the request limit for parallel runs, so a missing or zero value must fall back
// to the configured/default limit rather than fail.
func (m *RunManager) parallelLimitForRequest(
	ctx context.Context,
	workspaceRoot string,
	requestLimit int,
) (int, error) {
	if requestLimit > 0 {
		return requestLimit, nil
	}
	projectCfg, err := m.loadProjectConfig(ctx, workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("load workspace config for parallel limit: %w", err)
	}
	return resolveTaskMultiParallelLimit(requestLimit, projectCfg.Tasks.Run.EffectiveRunMultipleParallelLimit()), nil
}

// resolveTaskMultiParallelLimit applies the parallel-limit precedence: a positive
// request limit wins, otherwise the configured effective limit is used, clamped to
// a minimum of one so the fanout limiter always has at least one slot.
func resolveTaskMultiParallelLimit(requestLimit, configuredLimit int) int {
	if requestLimit > 0 {
		return requestLimit
	}
	if configuredLimit < 1 {
		return workspacecfg.DefaultRunMultipleParallelLimit
	}
	return configuredLimit
}

func taskMultiParentRuntimeConfig(raw json.RawMessage, workspaceRoot string) (*model.RuntimeConfig, error) {
	overrides, err := parseRuntimeOverrides(raw)
	if err != nil {
		return nil, err
	}
	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot: strings.TrimSpace(workspaceRoot),
		Name:          taskMultiRunName,
		Mode:          model.ExecutionModePRDTasks,
		DaemonOwned:   true,
	}
	if overrides.RunID != nil {
		runtimeCfg.RunID = strings.TrimSpace(*overrides.RunID)
	}
	runtimeCfg.ApplyDefaults()
	runtimeCfg.TUI = false
	runtimeCfg.EnableExecutableExtensions = false
	return runtimeCfg, nil
}

func taskMultiChildRuntimeOverrides(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	if _, err := parseRuntimeOverrides(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_runtime_overrides",
			fmt.Sprintf("runtime_overrides: %v", err),
			nil,
			err,
		)
	}
	delete(fields, "run_id")
	if len(fields) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal child runtime overrides: %w", err)
	}
	return encoded, nil
}

func buildDaemonParallelTaskPlan(
	ctx context.Context,
	tasksDir string,
	workflowSlug string,
	includeCompleted bool,
) (runparallel.Waves, []runparallel.TaskSpec, error) {
	manifest, taskFiles, err := taskscore.LoadValidatedTaskGraphManifest(ctx, tasksDir, strings.TrimSpace(workflowSlug))
	if err != nil {
		return runparallel.Waves{}, nil, parallelTaskGraphManifestProblem(tasksDir, err)
	}
	executableIDs := make(map[string]struct{}, len(taskFiles))
	taskSpecs := make([]runparallel.TaskSpec, 0, len(taskFiles))
	for idx := range taskFiles {
		taskFile := taskFiles[idx]
		if !includeCompleted && taskscore.IsTaskCompleted(taskFile.Entry) {
			continue
		}
		executableIDs[taskFile.ID] = struct{}{}
		taskSpecs = append(taskSpecs, runparallel.TaskSpec{
			ID:     runparallel.TaskID(taskFile.ID),
			Number: taskFile.Number,
			Title:  taskFile.Entry.Title,
			Slug:   strings.TrimSpace(workflowSlug),
		})
	}
	nodes := make([]runparallel.TaskID, 0, len(taskSpecs))
	for _, taskSpec := range taskSpecs {
		nodes = append(nodes, taskSpec.ID)
	}
	edges := make([]runparallel.DependencyEdge, 0, len(manifest.Graph.Edges))
	for _, edge := range manifest.Graph.Edges {
		if _, ok := executableIDs[edge.From]; !ok {
			continue
		}
		if _, ok := executableIDs[edge.To]; !ok {
			continue
		}
		edges = append(edges, runparallel.DependencyEdge{
			From: runparallel.TaskID(edge.From),
			To:   runparallel.TaskID(edge.To),
		})
	}
	waves, err := runparallel.BuildWavesFromEdges(nodes, edges)
	if err != nil {
		return runparallel.Waves{}, nil, err
	}
	return waves, taskSpecs, nil
}

func parallelTaskGraphManifestProblem(tasksDir string, err error) error {
	manifestPath := filepath.Join(strings.TrimSpace(tasksDir), taskscore.TaskGraphManifestFileName)
	switch {
	case errors.Is(err, taskscore.ErrTaskGraphManifestMissing):
		return apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"parallel_tasks_manifest_required",
			"parallel task execution requires _tasks.md with schema compozy.tasks/v2",
			map[string]any{"field": taskscore.TaskGraphManifestFileName, "path": manifestPath},
			err,
		)
	case errors.Is(err, taskscore.ErrTaskGraphManifestInvalid):
		return apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"parallel_tasks_manifest_invalid",
			err.Error(),
			map[string]any{"field": taskscore.TaskGraphManifestFileName, "path": manifestPath},
			err,
		)
	default:
		return fmt.Errorf("read parallel task graph manifest: %w", err)
	}
}

func parallelTaskMaxConcurrency(cfg workspacecfg.ParallelTasksConfig) int {
	effective := cfg.ApplyDefaults()
	if effective.MaxConcurrency == nil || *effective.MaxConcurrency < 1 {
		return workspacecfg.DefaultParallelTasksMaxConcurrency
	}
	return *effective.MaxConcurrency
}

func parallelIntegrationBranch(runID string) string {
	segment := sanitizeTaskMultiWorktreeSegment(strings.TrimSpace(runID), taskMultiWorktreeSlugMaxLen)
	if segment == "" {
		segment = "run"
	}
	return "compozy/parallel-" + segment
}

func planParallelIntegrationPath(worktreesRoot string, workspaceRoot string, runID string) (string, error) {
	root := strings.TrimSpace(worktreesRoot)
	if root == "" {
		return "", errors.New("daemon: worktree allocator root is required")
	}
	workspace := strings.TrimSpace(workspaceRoot)
	if workspace == "" {
		return "", errors.New("daemon: worktree workspace root is required")
	}
	parent := sanitizeTaskMultiWorktreeSegment(runID, taskMultiWorktreeParentShortLen)
	if parent == "" {
		return "", errors.New("daemon: parallel parent run id is required")
	}
	parent += "-" + taskMultiShortHash(strings.TrimSpace(runID), taskMultiWorktreeParentHashLen)
	return filepath.Join(root, taskMultiWorkspaceHash(workspace), parent, "integration"), nil
}

func normalizeTaskMultiSlugs(values []string) ([]string, error) {
	slugs, err := taskscore.ParseCommaSeparatedSlugs(strings.Join(values, ","))
	if err != nil {
		return nil, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_task_slugs",
			err.Error(),
			map[string]any{"field": "slugs"},
			err,
		)
	}
	return slugs, nil
}

func resolveTaskMultiMode(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "", workspacecfg.TaskRunMultipleModeEnqueued:
		return workspacecfg.TaskRunMultipleModeEnqueued, nil
	case workspacecfg.TaskRunMultipleModeParallel:
		return workspacecfg.TaskRunMultipleModeParallel, nil
	default:
		return "", taskMultiValidationProblem(
			"invalid_run_multiple_mode",
			"run_multiple mode must be enqueued or parallel",
			"mode",
		)
	}
}

func taskMultiValidationProblem(code string, message string, field string) error {
	return apicore.NewProblem(
		http.StatusUnprocessableEntity,
		code,
		message,
		map[string]any{"field": field},
		nil,
	)
}

func (m *RunManager) executeTaskMultiRun(active *activeRun, row globaldb.Run) {
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

	err = m.runTaskMultiCoordinator(active)
	fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
	if err == nil {
		fallback = completedTerminalState(scope.RunArtifacts(), "multi-task queue completed")
	}
	m.finishRun(active, row, fallback)
}

// runTaskMultiCoordinator is the mode-aware scheduler entrypoint. It emits the
// shared queue-start and item-queued lifecycle events, then dispatches to the
// branch for the resolved multi-run mode. Both branches reuse the shared event,
// cancellation, and terminal helpers so enqueued and parallel execution stay one
// state machine instead of two divergent coordinators.
func (m *RunManager) runTaskMultiCoordinator(active *activeRun) error {
	if active == nil || active.taskMulti == nil {
		return errors.New("task multi run is not configured")
	}
	prepared := active.taskMulti
	total := len(prepared.items)
	if err := m.emitTaskMultiQueueStarted(active, prepared, total); err != nil {
		return err
	}
	if prepared.parallelTasks != nil {
		return m.runTaskMultiParallelTasks(active, prepared, total)
	}
	switch prepared.mode {
	case workspacecfg.TaskRunMultipleModeParallel:
		// Parallel mode re-emits item_queued with worktree metadata per child as it
		// is allocated, so the shared upfront seeding is skipped to avoid a second
		// item_queued event per child (which doubled --stream output). The started
		// event already seeds every item into the snapshot.
		return m.runTaskMultiParallelQueue(active, prepared, total)
	default:
		if err := m.emitTaskMultiItemsQueued(active, prepared, total); err != nil {
			return err
		}
		return m.runTaskMultiEnqueuedQueue(active, prepared, total)
	}
}

// emitTaskMultiQueueStarted emits the parent "queue started" lifecycle event
// shared by every scheduler branch. It records the resolved mode, requested
// slugs, and total item count.
func (m *RunManager) emitTaskMultiQueueStarted(active *activeRun, prepared *preparedTaskMulti, total int) error {
	return m.emitTaskMultiEvent(active, eventspkg.EventKindTaskRunMultipleStarted, kinds.TaskRunMultiplePayload{
		Mode:          prepared.mode,
		Status:        runStatusRunning,
		Slugs:         preparedTaskMultiSlugs(prepared.items),
		Total:         total,
		ParallelLimit: prepared.parallelLimit,
	})
}

// emitTaskMultiItemsQueued emits one ordered "item queued" event per prepared
// child. The enqueued branch uses it to seed all items before any child starts;
// the parallel branch skips it and instead re-emits item_queued with worktree
// metadata per child as it is allocated, relying on the started event for upfront
// snapshot seeding.
func (m *RunManager) emitTaskMultiItemsQueued(active *activeRun, prepared *preparedTaskMulti, total int) error {
	for idx, item := range prepared.items {
		if err := m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleItemQueued,
			item,
			idx,
			total,
			taskMultiItemStatusQueued,
			"",
			"",
		); err != nil {
			return err
		}
	}
	return nil
}

func (m *RunManager) runTaskMultiParallelTasks(active *activeRun, prepared *preparedTaskMulti, total int) error {
	if prepared.parallelTasks == nil {
		return errors.New("parallel task execution is not configured")
	}
	if len(prepared.items) != 1 {
		return fmt.Errorf("parallel task execution requires one workflow item, got %d", len(prepared.items))
	}
	base, err := m.resolveTaskMultiParallelBase(active, prepared)
	if err != nil {
		if cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, 0, total, err); cancelErr != nil {
			return errors.Join(err, cancelErr)
		}
		return err
	}
	integrationPath, err := planParallelIntegrationPath(
		m.worktreeAllocator.worktreesRoot,
		prepared.workspace.RootDir,
		active.runID,
	)
	if err != nil {
		return err
	}
	integrationBranch := parallelIntegrationBranch(active.runID)
	orchestrator := runparallel.NewParallelExecutionOrchestrator(
		parallelWorktreeLifecycle{
			allocator:           m.worktreeAllocator,
			workflowRootsBySlug: taskMultiWorkflowRootsBySlug(prepared.items),
		},
		parallelTaskLauncher{
			manager:           m,
			active:            active,
			prepared:          prepared,
			item:              prepared.items[0],
			integrationBranch: integrationBranch,
		},
		runparallel.WithRecoveryStrategy(m.recoveryStrategy),
		runparallel.WithRecoveryEventSink(parallelRecoveryEventSink{manager: m, active: active}),
		runparallel.WithEventEmitter(parallelEventEmitter{manager: m, active: active}),
	)
	_, err = orchestrator.Run(active.ctx, runparallel.ParallelPlan{
		RunID:             active.runID,
		WorkspaceRoot:     prepared.workspace.RootDir,
		BaseBranch:        base.Branch,
		BaseCommit:        base.Commit,
		IntegrationBranch: integrationBranch,
		IntegrationPath:   integrationPath,
		Waves:             prepared.parallelTasks.waves,
		Tasks:             prepared.parallelTasks.tasks,
		Config:            prepared.parallelTasks.config,
		Recovery:          prepared.items[0].recovery,
	})
	if err != nil {
		return err
	}
	return m.emitTaskMultiQueueCompleted(active, prepared, total)
}

// parallelEventEmitter bridges the daemon-neutral orchestrator emit seam onto the
// parent run journal + workspace event bus. Emit failures are observability-only,
// so they are logged rather than surfaced into the FSM control flow.
type parallelEventEmitter struct {
	manager *RunManager
	active  *activeRun
}

func (e parallelEventEmitter) EmitParallelPlanEvent(
	_ context.Context,
	payload kinds.TaskParallelPlanPayload,
) {
	if e.manager == nil || e.active == nil {
		return
	}
	if err := e.manager.emitTaskParallelPlanEvent(e.active, payload); err != nil {
		slog.Default().Warn(
			"daemon: emit parallel task plan event",
			"run_id", e.active.runID,
			"error", err.Error(),
		)
	}
}

func (e parallelEventEmitter) EmitParallelEvent(
	_ context.Context,
	kind eventspkg.EventKind,
	payload kinds.TaskParallelPayload,
) {
	if e.manager == nil || e.active == nil {
		return
	}
	if err := e.manager.emitTaskParallelEvent(e.active, kind, payload); err != nil {
		slog.Default().Warn(
			"daemon: emit parallel task event",
			"run_id", e.active.runID,
			"kind", string(kind),
			"error", err.Error(),
		)
	}
}

type parallelRecoveryEventSink struct {
	manager *RunManager
	active  *activeRun
}

func (s parallelRecoveryEventSink) Submit(ctx context.Context, event eventspkg.Event) error {
	if s.manager == nil || s.active == nil || s.active.scope == nil || s.active.scope.RunJournal() == nil {
		return nil
	}
	if strings.TrimSpace(event.RunID) == "" {
		event.RunID = s.active.runID
	}
	s.active.emitMu.Lock()
	defer s.active.emitMu.Unlock()
	if _, err := s.active.scope.RunJournal().SubmitWithSeq(detachContext(ctx), event); err != nil {
		return err
	}
	s.manager.publishWorkspaceEvent(ctx, apicore.WorkspaceEvent{
		WorkspaceID: s.active.workspaceID,
		RunID:       s.active.runID,
		Mode:        s.active.mode,
		Status:      runStatusRunning,
		Kind:        apicore.WorkspaceEventKindRunStatusChanged,
	})
	return nil
}

type parallelWorktreeLifecycle struct {
	allocator           *taskMultiWorktreeAllocator
	workflowRootsBySlug map[string]string
}

func taskMultiWorkflowRootsBySlug(items []preparedTaskMultiItem) map[string]string {
	roots := make(map[string]string, len(items))
	for idx := range items {
		slug := strings.TrimSpace(items[idx].slug)
		root := strings.TrimSpace(items[idx].workflowRoot)
		if slug == "" || root == "" {
			continue
		}
		roots[slug] = root
	}
	return roots
}

func (l parallelWorktreeLifecycle) CreateIntegrationBranch(
	ctx context.Context,
	spec runparallel.IntegrationSpec,
) error {
	if l.allocator == nil {
		return errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.CreateIntegrationBranch(
		ctx,
		spec.WorkspaceRoot,
		spec.IntegrationPath,
		spec.IntegrationBranch,
		spec.BaseRef,
	)
}

func (l parallelWorktreeLifecycle) CommitTask(ctx context.Context, spec runparallel.TaskCommitSpec) (string, error) {
	if l.allocator == nil {
		return "", errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.CommitTask(ctx, spec)
}

func (l parallelWorktreeLifecycle) CommitStaged(
	ctx context.Context,
	spec runparallel.StagedCommitSpec,
) (string, error) {
	if l.allocator == nil {
		return "", errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.CommitStaged(ctx, spec)
}

func (l parallelWorktreeLifecycle) SquashMerge(
	ctx context.Context,
	integrationPath string,
	worktreeRef string,
	message string,
) (runparallel.ConflictSet, error) {
	if l.allocator == nil {
		return runparallel.ConflictSet{}, errors.New("daemon: task worktree allocator is not configured")
	}
	conflicts, err := l.allocator.SquashMerge(ctx, integrationPath, worktreeRef, message)
	if err != nil {
		return runparallel.ConflictSet{}, err
	}
	return runparallel.ConflictSet{
		Files:       conflicts.Files,
		StagedFiles: conflicts.StagedFiles,
		Clean:       conflicts.Clean,
	}, nil
}

func (l parallelWorktreeLifecycle) Head(ctx context.Context, path string) (string, error) {
	if l.allocator == nil {
		return "", errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.Head(ctx, path)
}

func (l parallelWorktreeLifecycle) FastForward(
	ctx context.Context,
	workspaceRoot string,
	targetBranch string,
	integrationBranch string,
) error {
	if l.allocator == nil {
		return errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.FastForward(ctx, workspaceRoot, targetBranch, integrationBranch)
}

func (l parallelWorktreeLifecycle) SyncTaskArtifacts(
	ctx context.Context,
	workspaceRoot string,
	tasks []runparallel.TaskOutcome,
) error {
	return syncCompletedParallelTaskArtifacts(ctx, workspaceRoot, tasks, l.workflowRootsBySlug)
}

func (l parallelWorktreeLifecycle) DiscardIntegrationBranch(
	ctx context.Context,
	workspaceRoot string,
	integrationPath string,
	integrationBranch string,
) error {
	if l.allocator == nil {
		return errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.DiscardIntegrationBranch(ctx, workspaceRoot, integrationPath, integrationBranch)
}

func (l parallelWorktreeLifecycle) Remove(ctx context.Context, workspaceRoot string, path string) error {
	if l.allocator == nil {
		return errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.Remove(ctx, workspaceRoot, path)
}

func (l parallelWorktreeLifecycle) Prune(ctx context.Context, workspaceRoot string) error {
	if l.allocator == nil {
		return errors.New("daemon: task worktree allocator is not configured")
	}
	return l.allocator.Prune(ctx, workspaceRoot)
}

type parallelTaskLauncher struct {
	manager           *RunManager
	active            *activeRun
	prepared          *preparedTaskMulti
	item              preparedTaskMultiItem
	integrationBranch string
}

func (l parallelTaskLauncher) PrepareTask(
	ctx context.Context,
	spec runparallel.TaskLaunchSpec,
) (runparallel.PreparedTaskRun, error) {
	if l.manager == nil {
		return nil, errors.New("daemon: run manager is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &parallelPreparedTaskRun{launcher: l, spec: spec}, nil
}

type parallelPreparedTaskRun struct {
	launcher parallelTaskLauncher
	spec     runparallel.TaskLaunchSpec
	child    taskWorktreeChildRun
	result   runparallel.TaskRunResult
}

func (r *parallelPreparedTaskRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	child, err := r.launcher.manager.startTaskWorktreeChild(
		r.launcher.active,
		r.launcher.prepared,
		r.launcher.item,
		r.spec.Task.Number,
		taskMultiWorktreeBase{Branch: r.spec.Base.Branch, Commit: r.spec.Base.Commit},
	)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	r.child = child
	r.emitTaskStarted(ctx, child)
	return r.awaitChild(ctx, child)
}

func (r *parallelPreparedTaskRun) RestartFailed(
	ctx context.Context,
	failedJobIDs []string,
) (recovery.RunOutcome, error) {
	if len(failedJobIDs) == 0 {
		return recovery.RunOutcome{}, errors.New("parallel task recovery: no failed job IDs supplied")
	}
	if strings.TrimSpace(r.child.Allocation.Path) == "" {
		return recovery.RunOutcome{}, errors.New("parallel task recovery: missing task worktree allocation")
	}
	child, err := r.launcher.manager.startTaskWorktreeChildInAllocation(
		r.launcher.active,
		r.launcher.prepared,
		r.launcher.item,
		r.spec.Task.Number,
		r.child.Allocation,
	)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	r.child = child
	r.emitTaskStarted(ctx, child)
	return r.awaitChild(ctx, child)
}

func (r *parallelPreparedTaskRun) emitTaskStarted(ctx context.Context, child taskWorktreeChildRun) {
	if r == nil {
		return
	}
	taskID := strings.TrimSpace(string(r.spec.Task.ID))
	if taskID == "" && r.spec.Task.Number > 0 {
		taskID = fmt.Sprintf("task_%02d", r.spec.Task.Number)
	}
	parallelEventEmitter{
		manager: r.launcher.manager,
		active:  r.launcher.active,
	}.EmitParallelEvent(
		ctx,
		eventspkg.EventKindTaskParallelTaskStarted,
		kinds.TaskParallelPayload{
			WaveIndex:         r.spec.WaveIndex,
			WaveTotal:         r.spec.WaveTotal,
			TaskID:            taskID,
			Phase:             runStatusRunning,
			ChildRunID:        child.Run.RunID,
			WorktreePath:      child.Allocation.Path,
			IntegrationBranch: r.launcher.integrationBranch,
		},
	)
}

func (r *parallelPreparedTaskRun) Result() runparallel.TaskRunResult {
	return r.result
}

func (r *parallelPreparedTaskRun) FailedConfig() *model.RuntimeConfig {
	if r.child.RuntimeConfig == nil {
		return nil
	}
	return r.child.RuntimeConfig.Clone()
}

func (r *parallelPreparedTaskRun) awaitChild(
	ctx context.Context,
	child taskWorktreeChildRun,
) (recovery.RunOutcome, error) {
	childRow, err := r.launcher.manager.waitForTaskMultiChild(ctx, child.Run.RunID)
	if err != nil {
		var childCancelErr error
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			childCancelErr = r.launcher.manager.Cancel(detachContext(ctx), child.Run.RunID)
		}
		return recovery.RunOutcome{}, errors.Join(err, childCancelErr)
	}
	r.result = runparallel.TaskRunResult{
		Task:         r.spec.Task,
		RunID:        child.Run.RunID,
		WorktreePath: child.Allocation.Path,
		BaseBranch:   child.Allocation.BaseBranch,
		BaseCommit:   child.Allocation.BaseCommit,
	}
	outcome, readErr := readParallelTaskChildOutcome(child.Run.RunID)
	terminalErr := parallelTaskChildTerminalError(childRow, r.spec.Task.ID)
	if readErr != nil {
		if terminalErr != nil {
			return parallelTaskChildFallbackOutcome(childRow, r.spec.Task.ID, terminalErr), terminalErr
		}
		return recovery.RunOutcome{}, readErr
	}
	r.applyWorktreeScope(outcome)
	return outcome, terminalErr
}

func (r *parallelPreparedTaskRun) applyWorktreeScope(outcome recovery.RunOutcome) {
	scope, path, err := readParallelTaskWorktreeScope(outcome, r.spec.Task)
	r.result.ScopeArtifactPath = path
	if err != nil {
		r.result.ScopeSupported = false
		r.result.ScopeError = err.Error()
		return
	}
	r.result.ScopeSupported = scope.Supported
	r.result.ProducedPaths = append([]string(nil), scope.ProducedPaths...)
	r.result.PreExistingChangedPaths = append([]string(nil), scope.PreExistingChangedPaths...)
	r.result.ScopeError = strings.TrimSpace(scope.Error)
	if !scope.Supported && r.result.ScopeError == "" {
		r.result.ScopeError = strings.TrimSpace(scope.UnsupportedReason)
	}
}

func readParallelTaskWorktreeScope(
	outcome recovery.RunOutcome,
	task runparallel.TaskSpec,
) (worktree.Scope, string, error) {
	runID := strings.TrimSpace(outcome.RunID)
	if runID == "" {
		return worktree.Scope{}, "", errors.New("parallel task child outcome missing run id")
	}
	artifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		return worktree.Scope{}, "", err
	}
	candidates := parallelTaskWorktreeScopeSafeNames(outcome, task)
	var missing []string
	for _, safeName := range candidates {
		path := artifacts.JobArtifacts(safeName).WorktreeScopePath
		scope, readErr := worktree.ReadScope(path)
		if readErr == nil {
			return scope, path, nil
		}
		if errors.Is(readErr, os.ErrNotExist) {
			missing = append(missing, path)
			continue
		}
		return worktree.Scope{}, path, readErr
	}
	return worktree.Scope{}, "", fmt.Errorf(
		"parallel task child %s missing worktree scope artifact: %s",
		runID,
		strings.Join(missing, ", "),
	)
}

func parallelTaskWorktreeScopeSafeNames(outcome recovery.RunOutcome, task runparallel.TaskSpec) []string {
	candidates := make([]string, 0, len(outcome.Jobs)+4)
	for _, job := range outcome.Jobs {
		if safeName := strings.TrimSpace(job.SafeName); safeName != "" {
			candidates = append(candidates, safeName)
		}
	}
	if task.ID != "" {
		candidates = append(candidates, string(task.ID))
	}
	if task.Number > 0 {
		candidates = append(candidates, fmt.Sprintf("task-%02d", task.Number), fmt.Sprintf("task_%02d", task.Number))
	}
	candidates = append(candidates, runModeTask)
	return uniqueTaskWorktreeScopeSafeNames(candidates)
}

func uniqueTaskWorktreeScopeSafeNames(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func readParallelTaskChildOutcome(runID string) (recovery.RunOutcome, error) {
	artifacts, err := model.ResolveHomeRunArtifacts(runID)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	outcome, err := recovery.ReadRunOutcome(artifacts)
	if err != nil {
		return recovery.RunOutcome{}, err
	}
	return *outcome, nil
}

func parallelTaskChildTerminalError(row globaldb.Run, taskID runparallel.TaskID) error {
	switch row.Status {
	case runStatusCompleted:
		return nil
	case runStatusCancelled:
		return fmt.Errorf("parallel task child run %s for task %s was canceled", row.RunID, taskID)
	default:
		return fmt.Errorf(
			"parallel task child run %s for task %s ended with status %s: %s",
			row.RunID,
			taskID,
			row.Status,
			row.ErrorText,
		)
	}
}

func parallelTaskChildFallbackOutcome(
	row globaldb.Run,
	taskID runparallel.TaskID,
	cause error,
) recovery.RunOutcome {
	status := recoveryStatusForRunStatus(row.Status)
	outcome := recovery.RunOutcome{
		RunID:  strings.TrimSpace(row.RunID),
		Status: status,
	}
	if outcome.RunID != "" {
		if artifacts, err := model.ResolveHomeRunArtifacts(outcome.RunID); err == nil {
			outcome.ArtifactsDir = artifacts.RunDir
			outcome.ResultPath = artifacts.ResultPath
		}
	}
	if status != recovery.StatusFailed {
		return outcome
	}
	jobID := strings.TrimSpace(string(taskID))
	if jobID == "" {
		jobID = runModeTask
	}
	outcome.Jobs = []recovery.JobOutcome{{
		SafeName: jobID,
		Status:   recovery.StatusFailed,
		ExitCode: 1,
		Error:    parallelTaskChildFailureMessage(row, cause),
	}}
	return outcome
}

func parallelTaskChildFailureMessage(row globaldb.Run, cause error) string {
	message := strings.TrimSpace(row.ErrorText)
	if message != "" {
		return message
	}
	if cause != nil {
		return cause.Error()
	}
	return fmt.Sprintf("child run ended with status %s", row.Status)
}

func recoveryStatusForRunStatus(status string) recovery.RunStatus {
	switch status {
	case runStatusCompleted:
		return recovery.StatusSucceeded
	case runStatusCancelled:
		return recovery.StatusCanceled
	case runStatusFailed, runStatusCrashed:
		return recovery.StatusFailed
	default:
		return recovery.StatusUnknown
	}
}

func disabledParallelChildRecoveryConfig() workspacecfg.AgentRecoveryConfig {
	enabled := false
	return workspacecfg.AgentRecoveryConfig{Enabled: &enabled}
}

// emitTaskMultiQueueCompleted emits the terminal "queue completed" event for a
// fully successful queue. It is the shared queue-terminal helper for branches
// that complete every child.
func (m *RunManager) emitTaskMultiQueueCompleted(active *activeRun, prepared *preparedTaskMulti, total int) error {
	return m.emitTaskMultiEvent(active, eventspkg.EventKindTaskRunMultipleQueueCompleted, kinds.TaskRunMultiplePayload{
		Mode:   prepared.mode,
		Status: runStatusCompleted,
		Slugs:  preparedTaskMultiSlugs(prepared.items),
		Total:  total,
	})
}

// runTaskMultiEnqueuedQueue preserves the V1 sequential coordinator behavior: it
// starts one child at a time and waits for each child to reach a terminal status
// before starting the next. A canceled parent context or a failed child stops the
// queue and cancels the remaining queued siblings via the shared cancellation
// helper.
func (m *RunManager) runTaskMultiEnqueuedQueue(active *activeRun, prepared *preparedTaskMulti, total int) error {
	for idx, item := range prepared.items {
		if err := context.Cause(active.ctx); err != nil {
			if emitErr := m.cancelTaskMultiQueuedItems(active, prepared.items, idx, total, err); emitErr != nil {
				return errors.Join(err, emitErr)
			}
			return err
		}
		if err := m.runTaskMultiChildAt(active, prepared, item, idx, total); err != nil {
			return err
		}
	}
	return m.emitTaskMultiQueueCompleted(active, prepared, total)
}

// runTaskMultiParallelQueue runs the queued children concurrently, each in its
// own isolated git worktree, bounded by the resolved parallel limit. It resolves
// the parent workspace base branch and HEAD once, then fans out child workers up
// to the limit using a counting semaphore. Every worker goroutine is owned by the
// parent coordinator and joined via the WaitGroup before the parent reaches a
// terminal status. Execution is fail-late: a child that cannot start, fails, or
// crashes is recorded but does NOT cancel healthy siblings; the aggregate parent
// result is computed only after every active child settles. Parent cancellation
// stops launching new children, cancels in-flight children through the shared wait
// helper, and marks the not-started items canceled. Both modes reuse the shared
// queue lifecycle, item-event, and cancellation helpers so they stay one state
// machine.
func (m *RunManager) runTaskMultiParallelQueue(active *activeRun, prepared *preparedTaskMulti, total int) error {
	base, err := m.resolveTaskMultiParallelBase(active, prepared)
	if err != nil {
		if cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, 0, total, err); cancelErr != nil {
			return errors.Join(err, cancelErr)
		}
		return err
	}
	limit := prepared.parallelLimit
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	childErrs := make([]error, len(prepared.items))
	var wg sync.WaitGroup
	launched := 0
	var cancelCause error
	for idx := range prepared.items {
		if cause := context.Cause(active.ctx); cause != nil {
			cancelCause = cause
			break
		}
		select {
		case sem <- struct{}{}:
		case <-active.ctx.Done():
			cancelCause = context.Cause(active.ctx)
		}
		if cancelCause != nil {
			break
		}
		index := idx
		item := prepared.items[idx]
		wg.Add(1)
		launched++
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			childErrs[index] = m.runTaskMultiParallelChild(active, prepared, item, index, total, base)
		}()
	}
	wg.Wait()
	if cancelCause == nil {
		if cause := context.Cause(active.ctx); cause != nil {
			cancelCause = cause
		}
	}
	return m.finalizeTaskMultiParallel(active, prepared, total, childErrs, launched, cancelCause)
}

// runTaskMultiParallelChild starts one parallel child in its allocated worktree
// and waits for it to settle WITHOUT canceling siblings. Unlike the enqueued
// path, a start failure or non-completed terminal status is recorded and returned
// for aggregation while healthy siblings keep running (fail-late). Parent
// cancellation still cancels the in-flight child via the shared wait helper.
func (m *RunManager) runTaskMultiParallelChild(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
	base taskMultiWorktreeBase,
) error {
	childRun, err := m.startTaskMultiWorktreeChild(active, prepared, item, index, total, base)
	if err != nil {
		emitErr := m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			item,
			index,
			total,
			taskMultiItemStatusFailed,
			"",
			err.Error(),
		)
		return errors.Join(fmt.Errorf("start child %s: %w", item.slug, err), emitErr)
	}
	childRow, err := m.waitForTaskMultiChild(active.ctx, childRun.RunID)
	if err != nil {
		var childCancelErr error
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			childCancelErr = m.Cancel(detachContext(active.ctx), childRun.RunID)
		}
		emitErr := m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleItemCanceled,
			item,
			index,
			total,
			taskMultiItemStatusCanceled,
			childRun.RunID,
			err.Error(),
		)
		return errors.Join(fmt.Errorf("await child %s: %w", item.slug, err), childCancelErr, emitErr)
	}
	if emitErr := m.finishTaskMultiChild(active, item, index, total, childRow); emitErr != nil {
		return emitErr
	}
	if childRow.Status == runStatusCompleted {
		return nil
	}
	if childRow.Status == runStatusCancelled {
		return fmt.Errorf("task multi child run %s for %s was canceled", childRow.RunID, item.slug)
	}
	return fmt.Errorf(
		"task multi child run %s for %s ended with status %s: %s",
		childRow.RunID,
		item.slug,
		childRow.Status,
		childRow.ErrorText,
	)
}

// finalizeTaskMultiParallel computes the terminal parent result after every
// launched child has settled. When the parent was canceled it marks the
// not-started items canceled and returns the cancellation cause; otherwise it
// returns the aggregate child failure (if any) or emits the queue-completed event.
func (m *RunManager) finalizeTaskMultiParallel(
	active *activeRun,
	prepared *preparedTaskMulti,
	total int,
	childErrs []error,
	launched int,
	cancelCause error,
) error {
	if cancelCause != nil {
		cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, launched, total, cancelCause)
		return errors.Join(cancelCause, errors.Join(childErrs...), cancelErr)
	}
	if aggErr := aggregateTaskMultiParallelResult(prepared.items, childErrs); aggErr != nil {
		return aggErr
	}
	return m.emitTaskMultiQueueCompleted(active, prepared, total)
}

// aggregateTaskMultiParallelResult returns nil when every child completed
// successfully; otherwise it returns a single error naming the failed child slugs
// in queue order and wrapping the underlying child errors.
func aggregateTaskMultiParallelResult(items []preparedTaskMultiItem, childErrs []error) error {
	failedSlugs := make([]string, 0, len(childErrs))
	errs := make([]error, 0, len(childErrs))
	for idx, childErr := range childErrs {
		if childErr == nil {
			continue
		}
		slug := ""
		if idx < len(items) {
			slug = items[idx].slug
		}
		failedSlugs = append(failedSlugs, slug)
		errs = append(errs, childErr)
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf(
		"parallel multi-run failed for %d of %d children (%s): %w",
		len(errs),
		len(childErrs),
		strings.Join(failedSlugs, ", "),
		errors.Join(errs...),
	)
}

// resolveTaskMultiParallelBase resolves the parent workspace branch and HEAD
// commit once per parallel parent run via the worktree allocator. A detached
// parent checkout (or a workspace outside a git repository) surfaces as the
// parent run failure so no child worktrees are created from an ambiguous base.
func (m *RunManager) resolveTaskMultiParallelBase(
	active *activeRun,
	prepared *preparedTaskMulti,
) (taskMultiWorktreeBase, error) {
	if m.worktreeAllocator == nil {
		return taskMultiWorktreeBase{}, errors.New("daemon: task multi worktree allocator is not configured")
	}
	return m.worktreeAllocator.ResolveBase(active.ctx, prepared.workspace.RootDir)
}

func (m *RunManager) runTaskMultiChildAt(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
) error {
	childRun, err := m.startTaskMultiChild(active, prepared, item, index, total)
	if err != nil {
		return m.handleTaskMultiChildStartFailure(active, prepared, item, index, total, err)
	}
	return m.awaitTaskMultiChild(active, prepared, item, index, total, childRun)
}

// handleTaskMultiChildStartFailure marks a child that never started as failed and
// cancels the remaining queued siblings. It is shared by the enqueued and
// parallel branches so a start failure produces one consistent terminal shape.
func (m *RunManager) handleTaskMultiChildStartFailure(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
	startErr error,
) error {
	emitErr := m.emitTaskMultiItemEvent(
		active,
		eventspkg.EventKindTaskRunMultipleChildFailed,
		item,
		index,
		total,
		taskMultiItemStatusFailed,
		"",
		startErr.Error(),
	)
	cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, index+1, total, startErr)
	return errors.Join(startErr, emitErr, cancelErr)
}

// awaitTaskMultiChild waits for one started child to reach a terminal status and
// records the matching item event. A child failure or cancellation cancels the
// remaining queued siblings. It is shared by the enqueued and parallel branches.
func (m *RunManager) awaitTaskMultiChild(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
	childRun apicore.Run,
) error {
	childRow, err := m.waitForTaskMultiChild(active.ctx, childRun.RunID)
	if err != nil {
		var childCancelErr error
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			childCancelErr = m.Cancel(detachContext(active.ctx), childRun.RunID)
		}
		emitErr := m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleItemCanceled,
			item,
			index,
			total,
			taskMultiItemStatusCanceled,
			childRun.RunID,
			err.Error(),
		)
		cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, index+1, total, err)
		return errors.Join(err, childCancelErr, emitErr, cancelErr)
	}
	if err := m.finishTaskMultiChild(active, item, index, total, childRow); err != nil {
		return err
	}
	if childRow.Status == runStatusCompleted {
		return nil
	}
	if childRow.Status == runStatusCancelled && active.cancelWasRequested() {
		cause := context.Cause(active.ctx)
		if cause == nil {
			cause = context.Canceled
		}
		if emitErr := m.cancelTaskMultiQueuedItems(active, prepared.items, index+1, total, cause); emitErr != nil {
			return errors.Join(cause, emitErr)
		}
		return cause
	}
	childErr := fmt.Errorf(
		"task multi child run %s for %s ended with status %s: %s",
		childRow.RunID,
		item.slug,
		childRow.Status,
		childRow.ErrorText,
	)
	if emitErr := m.cancelTaskMultiQueuedItems(active, prepared.items, index+1, total, childErr); emitErr != nil {
		return errors.Join(childErr, emitErr)
	}
	return childErr
}

func (m *RunManager) startTaskMultiChild(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
) (apicore.Run, error) {
	runtimeCfg := item.runtimeCfg.Clone()
	if runtimeCfg == nil {
		return apicore.Run{}, errors.New("task multi child runtime config is required")
	}
	runtimeCfg.ParentRunID = active.runID
	childRun, err := m.startRun(active.ctx, startRunSpec{
		workspace:        prepared.workspace,
		workflowID:       cloneStringPtr(item.workflowID),
		workflowSlug:     item.slug,
		workflowRoot:     item.workflowRoot,
		mode:             runModeTask,
		presentationMode: prepared.presentationMode,
		parentRunID:      active.runID,
		runtimeCfg:       runtimeCfg,
		recovery:         item.recovery,
	})
	if err != nil {
		return apicore.Run{}, err
	}
	if err := m.emitTaskMultiItemEvent(
		active,
		eventspkg.EventKindTaskRunMultipleChildStarted,
		item,
		index,
		total,
		taskMultiItemStatusRunning,
		childRun.RunID,
		"",
	); err != nil {
		cancelErr := m.Cancel(detachContext(active.ctx), childRun.RunID)
		return apicore.Run{}, errors.Join(err, cancelErr)
	}
	return childRun, nil
}

// startTaskMultiWorktreeChild allocates an isolated git worktree for a parallel
// child, registers the worktree as its own workspace, remaps the child runtime
// onto that workspace, and launches the child task run under the parent run id.
// Worktree metadata is emitted before the child launches so snapshots and manual
// cleanup survive a crash between allocation and child start. The original parent
// stays registered under its own workspace; children are linked only by
// parent_run_id and the multi-run snapshot, not by a shared workspace id.
func (m *RunManager) startTaskMultiWorktreeChild(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	index int,
	total int,
	base taskMultiWorktreeBase,
) (apicore.Run, error) {
	if m.worktreeAllocator == nil {
		return apicore.Run{}, errors.New("daemon: task multi worktree allocator is not configured")
	}
	allocation, err := m.worktreeAllocator.Allocate(active.ctx, taskMultiWorktreeSpec{
		WorkspaceRoot: prepared.workspace.RootDir,
		ParentRunID:   active.runID,
		Slug:          item.slug,
		Index:         index,
		Base:          base,
	})
	if err != nil {
		return apicore.Run{}, fmt.Errorf("allocate worktree for %s: %w", item.slug, err)
	}
	if err := mirrorTaskMultiWorkflowArtifacts(item.workflowRoot, allocation.Path, item.slug); err != nil {
		cleanupErr := m.worktreeAllocator.Remove(detachContext(active.ctx), prepared.workspace.RootDir, allocation.Path)
		return apicore.Run{}, errors.Join(err, cleanupErr)
	}
	if err := m.emitTaskMultiEvent(
		active,
		eventspkg.EventKindTaskRunMultipleItemQueued,
		taskMultiWorktreeItemPayload(item, index, total, taskMultiItemStatusQueued, "", "", allocation),
	); err != nil {
		return apicore.Run{}, err
	}
	workspaceRow, workflowID, _, err := m.resolveWorkflowContext(detachContext(active.ctx), allocation.Path, item.slug)
	if err != nil {
		return apicore.Run{}, fmt.Errorf("register worktree workspace for %s: %w", item.slug, err)
	}
	// Align the runtime workspace root with the registered worktree workspace row
	// so database identity and runtime filesystem paths match (ADR-007).
	tasksDir, err := requireTaskMultiWorktreeTaskDir(workspaceRow.RootDir, item.slug)
	if err != nil {
		return apicore.Run{}, err
	}
	runtimeCfg, err := remapTaskMultiChildRuntime(item.runtimeCfg, workspaceRow.RootDir, item.slug, active.runID)
	if err != nil {
		return apicore.Run{}, err
	}
	childRun, err := m.startRun(active.ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     item.slug,
		workflowRoot:     tasksDir,
		mode:             runModeTask,
		presentationMode: prepared.presentationMode,
		parentRunID:      active.runID,
		runtimeCfg:       runtimeCfg,
		recovery:         item.recovery,
	})
	if err != nil {
		return apicore.Run{}, err
	}
	if err := m.emitTaskMultiEvent(
		active,
		eventspkg.EventKindTaskRunMultipleChildStarted,
		taskMultiWorktreeItemPayload(item, index, total, taskMultiItemStatusRunning, childRun.RunID, "", allocation),
	); err != nil {
		cancelErr := m.Cancel(detachContext(active.ctx), childRun.RunID)
		return apicore.Run{}, errors.Join(err, cancelErr)
	}
	return childRun, nil
}

// startTaskWorktreeChild launches one PRD task file as an isolated child run in
// its own git worktree. Unlike startTaskMultiWorktreeChild, this scopes the
// normal task run to exactly one task number and leaves the slug-scoped
// multi-run path unchanged.
func (m *RunManager) startTaskWorktreeChild(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	targetTaskNumber int,
	base taskMultiWorktreeBase,
) (taskWorktreeChildRun, error) {
	if targetTaskNumber <= 0 {
		return taskWorktreeChildRun{}, fmt.Errorf(
			"daemon: target task number must be positive, got %d",
			targetTaskNumber,
		)
	}
	if m.worktreeAllocator == nil {
		return taskWorktreeChildRun{}, errors.New("daemon: task worktree allocator is not configured")
	}
	allocation, err := m.worktreeAllocator.Allocate(active.ctx, taskMultiWorktreeSpec{
		WorkspaceRoot: prepared.workspace.RootDir,
		ParentRunID:   active.runID,
		Slug:          item.slug,
		Index:         targetTaskNumber,
		TaskNumber:    targetTaskNumber,
		Base:          base,
	})
	if err != nil {
		return taskWorktreeChildRun{}, fmt.Errorf(
			"allocate worktree for %s task %d: %w",
			item.slug,
			targetTaskNumber,
			err,
		)
	}
	if err := mirrorTaskMultiWorkflowArtifacts(item.workflowRoot, allocation.Path, item.slug); err != nil {
		cleanupErr := m.worktreeAllocator.Remove(
			detachContext(active.ctx),
			prepared.workspace.RootDir,
			allocation.Path,
		)
		return taskWorktreeChildRun{}, errors.Join(err, cleanupErr)
	}
	child, err := m.startTaskWorktreeChildInAllocation(active, prepared, item, targetTaskNumber, allocation)
	if err != nil {
		cleanupErr := m.worktreeAllocator.Remove(
			detachContext(active.ctx),
			prepared.workspace.RootDir,
			allocation.Path,
		)
		return taskWorktreeChildRun{}, errors.Join(err, cleanupErr)
	}
	return child, nil
}

func (m *RunManager) startTaskWorktreeChildInAllocation(
	active *activeRun,
	prepared *preparedTaskMulti,
	item preparedTaskMultiItem,
	targetTaskNumber int,
	allocation taskMultiWorktreeAllocation,
) (taskWorktreeChildRun, error) {
	workspaceRow, workflowID, _, err := m.resolveWorkflowContext(detachContext(active.ctx), allocation.Path, item.slug)
	if err != nil {
		return taskWorktreeChildRun{}, fmt.Errorf("register worktree workspace for %s task %d: %w",
			item.slug,
			targetTaskNumber,
			err,
		)
	}
	tasksDir, err := requireTaskMultiWorktreeTaskDir(workspaceRow.RootDir, item.slug)
	if err != nil {
		return taskWorktreeChildRun{}, err
	}
	runtimeCfg, err := remapTaskMultiChildRuntime(item.runtimeCfg, workspaceRow.RootDir, item.slug, active.runID)
	if err != nil {
		return taskWorktreeChildRun{}, err
	}
	target := targetTaskNumber
	runtimeCfg.TargetTaskNumber = &target
	runtimeCfg.Name = fmt.Sprintf("%s-task-%02d", item.slug, targetTaskNumber)
	childRun, err := m.startRun(active.ctx, startRunSpec{
		workspace:        workspaceRow,
		workflowID:       workflowID,
		workflowSlug:     item.slug,
		workflowRoot:     tasksDir,
		mode:             runModeTask,
		presentationMode: prepared.presentationMode,
		parentRunID:      active.runID,
		runtimeCfg:       runtimeCfg,
		recovery:         disabledParallelChildRecoveryConfig(),
	})
	if err != nil {
		return taskWorktreeChildRun{}, err
	}
	return taskWorktreeChildRun{Run: childRun, Allocation: allocation, RuntimeConfig: runtimeCfg}, nil
}

// remapTaskMultiChildRuntime clones base and repoints it at the worktree: the
// workspace root becomes the worktree path, the task directory becomes the slug
// directory inside the worktree, and ParentRunID links the child to the parent
// multi-run. All other runtime overrides are preserved.
func remapTaskMultiChildRuntime(
	base *model.RuntimeConfig,
	worktreePath string,
	slug string,
	parentRunID string,
) (*model.RuntimeConfig, error) {
	if base == nil {
		return nil, errors.New("daemon: task multi child runtime config is required")
	}
	trimmedPath := strings.TrimSpace(worktreePath)
	if trimmedPath == "" {
		return nil, errors.New("daemon: task multi worktree path is required")
	}
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, errors.New("daemon: task multi child slug is required")
	}
	remapped := base.Clone()
	if remapped == nil {
		return nil, errors.New("daemon: task multi child runtime config is required")
	}
	remapped.WorkspaceRoot = trimmedPath
	remapped.TasksDir = model.TaskDirectoryForWorkspace(trimmedPath, trimmedSlug)
	if strings.TrimSpace(remapped.WorkflowName) == "" {
		remapped.WorkflowName = trimmedSlug
	}
	remapped.ParentRunID = strings.TrimSpace(parentRunID)
	remapped.RunID = ""
	return remapped, nil
}

// requireTaskMultiWorktreeTaskDir resolves and validates the slug task directory
// inside an allocated worktree, returning a slug-specific error when the worktree
// base commit does not contain the task directory.
func requireTaskMultiWorktreeTaskDir(worktreePath string, slug string) (string, error) {
	tasksDir := model.TaskDirectoryForWorkspace(strings.TrimSpace(worktreePath), strings.TrimSpace(slug))
	if err := requireDirectory(tasksDir); err != nil {
		return "", fmt.Errorf(
			"task multi worktree %s is missing task directory for slug %q: %w",
			strings.TrimSpace(worktreePath),
			strings.TrimSpace(slug),
			err,
		)
	}
	return tasksDir, nil
}

// taskMultiWorktreeItemPayload builds a parent item event payload that carries
// worktree metadata alongside the standard item fields. The snapshot builder
// merges these additively, so emitting worktree fields before child launch keeps
// metadata recoverable even if the child run id does not exist yet.
func taskMultiWorktreeItemPayload(
	item preparedTaskMultiItem,
	index int,
	total int,
	status string,
	childRunID string,
	errorText string,
	allocation taskMultiWorktreeAllocation,
) kinds.TaskRunMultiplePayload {
	return kinds.TaskRunMultiplePayload{
		Slug:           item.slug,
		Index:          index,
		Total:          total,
		Status:         status,
		ChildRunID:     strings.TrimSpace(childRunID),
		Error:          strings.TrimSpace(errorText),
		WorktreePath:   allocation.Path,
		BaseBranch:     allocation.BaseBranch,
		BaseCommit:     allocation.BaseCommit,
		WorktreeStatus: allocation.WorktreeStatus,
	}
}

func (m *RunManager) finishTaskMultiChild(
	active *activeRun,
	item preparedTaskMultiItem,
	index int,
	total int,
	childRow globaldb.Run,
) error {
	switch childRow.Status {
	case runStatusCompleted:
		return m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			item,
			index,
			total,
			taskMultiItemStatusCompleted,
			childRow.RunID,
			"",
		)
	case runStatusCancelled:
		return m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleItemCanceled,
			item,
			index,
			total,
			taskMultiItemStatusCanceled,
			childRow.RunID,
			childRow.ErrorText,
		)
	default:
		return m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			item,
			index,
			total,
			taskMultiItemStatusFailed,
			childRow.RunID,
			childRow.ErrorText,
		)
	}
}

func (m *RunManager) waitForTaskMultiChild(ctx context.Context, runID string) (globaldb.Run, error) {
	trimmedRunID := strings.TrimSpace(runID)
	ticker := time.NewTicker(taskMultiChildPollInterval)
	defer ticker.Stop()

	for {
		row, err := m.globalDB.GetRun(detachContext(ctx), trimmedRunID)
		if err != nil {
			return globaldb.Run{}, fmt.Errorf("load child run %s: %w", trimmedRunID, err)
		}
		if isTerminalRunStatus(row.Status) {
			return row, nil
		}
		select {
		case <-ctx.Done():
			if cancelErr := m.Cancel(detachContext(ctx), runID); cancelErr != nil {
				return globaldb.Run{}, errors.Join(ctx.Err(), fmt.Errorf("cancel child run %s: %w", runID, cancelErr))
			}
			return globaldb.Run{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *RunManager) cancelTaskMultiQueuedItems(
	active *activeRun,
	items []preparedTaskMultiItem,
	startIndex int,
	total int,
	cause error,
) error {
	var err error
	for idx := startIndex; idx < len(items); idx++ {
		item := items[idx]
		err = errors.Join(err, m.emitTaskMultiItemEvent(
			active,
			eventspkg.EventKindTaskRunMultipleItemCanceled,
			item,
			idx,
			total,
			taskMultiItemStatusCanceled,
			"",
			errorString(cause),
		))
	}
	err = errors.Join(
		err,
		m.emitTaskMultiEvent(active, eventspkg.EventKindTaskRunMultipleQueueCanceled, kinds.TaskRunMultiplePayload{
			Mode:   active.taskMulti.mode,
			Status: taskMultiItemStatusCanceled,
			Slugs:  preparedTaskMultiSlugs(items),
			Total:  total,
			Error:  errorString(cause),
		}),
	)
	return err
}

func (m *RunManager) emitTaskMultiItemEvent(
	active *activeRun,
	kind eventspkg.EventKind,
	item preparedTaskMultiItem,
	index int,
	total int,
	status string,
	childRunID string,
	errorText string,
) error {
	return m.emitTaskMultiEvent(active, kind, kinds.TaskRunMultiplePayload{
		Slug:       item.slug,
		Index:      index,
		Total:      total,
		Status:     status,
		ChildRunID: strings.TrimSpace(childRunID),
		Error:      strings.TrimSpace(errorText),
	})
}

func (m *RunManager) emitTaskMultiEvent(
	active *activeRun,
	kind eventspkg.EventKind,
	payload kinds.TaskRunMultiplePayload,
) error {
	if active == nil || active.scope == nil || active.scope.RunJournal() == nil {
		return nil
	}
	// Serialize emission so concurrent parallel-mode child workers append item
	// events atomically and in per-item order.
	active.emitMu.Lock()
	defer active.emitMu.Unlock()
	payload.RunID = active.runID
	if payload.Mode == "" && active.taskMulti != nil {
		payload.Mode = active.taskMulti.mode
	}
	if err := submitSyntheticEvent(
		detachContext(active.ctx),
		active.scope.RunJournal(),
		active.runID,
		kind,
		payload,
	); err != nil {
		return err
	}
	m.publishWorkspaceEvent(active.ctx, apicore.WorkspaceEvent{
		WorkspaceID: active.workspaceID,
		RunID:       active.runID,
		Mode:        active.mode,
		Status:      runStatusRunning,
		Kind:        apicore.WorkspaceEventKindRunStatusChanged,
	})
	return nil
}

func (m *RunManager) emitTaskParallelPlanEvent(
	active *activeRun,
	payload kinds.TaskParallelPlanPayload,
) error {
	if active == nil || active.scope == nil || active.scope.RunJournal() == nil {
		return nil
	}
	active.emitMu.Lock()
	defer active.emitMu.Unlock()
	payload.RunID = active.runID
	if err := submitSyntheticEvent(
		detachContext(active.ctx),
		active.scope.RunJournal(),
		active.runID,
		eventspkg.EventKindTaskParallelPlanStarted,
		payload,
	); err != nil {
		return err
	}
	m.publishWorkspaceEvent(active.ctx, apicore.WorkspaceEvent{
		WorkspaceID: active.workspaceID,
		RunID:       active.runID,
		Mode:        active.mode,
		Status:      runStatusRunning,
		Kind:        apicore.WorkspaceEventKindRunStatusChanged,
	})
	return nil
}

// emitTaskParallelEvent appends one task.parallel.* event to the parent run
// journal and notifies the workspace bus, mirroring emitTaskMultiEvent. Emission
// is serialized through active.emitMu so concurrent wave workers append atomically.
func (m *RunManager) emitTaskParallelEvent(
	active *activeRun,
	kind eventspkg.EventKind,
	payload kinds.TaskParallelPayload,
) error {
	if active == nil || active.scope == nil || active.scope.RunJournal() == nil {
		return nil
	}
	active.emitMu.Lock()
	defer active.emitMu.Unlock()
	payload.RunID = active.runID
	if err := submitSyntheticEvent(
		detachContext(active.ctx),
		active.scope.RunJournal(),
		active.runID,
		kind,
		payload,
	); err != nil {
		return err
	}
	m.publishWorkspaceEvent(active.ctx, apicore.WorkspaceEvent{
		WorkspaceID: active.workspaceID,
		RunID:       active.runID,
		Mode:        active.mode,
		Status:      runStatusRunning,
		Kind:        apicore.WorkspaceEventKindRunStatusChanged,
	})
	return nil
}

func preparedTaskMultiSlugs(items []preparedTaskMultiItem) []string {
	slugs := make([]string, 0, len(items))
	for _, item := range items {
		slugs = append(slugs, item.slug)
	}
	return slugs
}

func newTaskMultiSnapshotBuilder() *taskMultiSnapshotBuilder {
	return &taskMultiSnapshotBuilder{
		index: make(map[string]int),
	}
}

func (b *taskMultiSnapshotBuilder) applyEvent(event eventspkg.Event) error {
	switch event.Kind {
	case eventspkg.EventKindTaskRunMultipleStarted:
		payload, err := decodeTaskMultiPayload(event)
		if err != nil {
			return err
		}
		for _, slug := range payload.Slugs {
			b.ensureItem(slug).Status = taskMultiItemStatusQueued
		}
	case eventspkg.EventKindTaskRunMultipleItemQueued,
		eventspkg.EventKindTaskRunMultipleChildStarted,
		eventspkg.EventKindTaskRunMultipleChildCompleted,
		eventspkg.EventKindTaskRunMultipleChildFailed,
		eventspkg.EventKindTaskRunMultipleItemCanceled:
		payload, err := decodeTaskMultiPayload(event)
		if err != nil {
			return err
		}
		applyTaskMultiItemMetadata(b.ensureItem(payload.Slug), payload)
	}
	return nil
}

// applyTaskMultiItemMetadata merges one parent-event payload into a snapshot
// item. Non-empty fields overwrite prior values so later events refine earlier
// state, while empty fields are ignored. This lets worktree metadata be recorded
// before a child run id exists (metadata-only updates emitted before child
// launch) and keeps older parent events without worktree fields compatible.
func applyTaskMultiItemMetadata(item *apicore.TaskRunMultipleItem, payload kinds.TaskRunMultiplePayload) {
	if status := strings.TrimSpace(payload.Status); status != "" {
		item.Status = status
	}
	if childRunID := strings.TrimSpace(payload.ChildRunID); childRunID != "" {
		item.RunID = childRunID
	}
	if errorText := strings.TrimSpace(payload.Error); errorText != "" {
		item.ErrorText = errorText
	}
	if worktreePath := strings.TrimSpace(payload.WorktreePath); worktreePath != "" {
		item.WorktreePath = worktreePath
	}
	if baseBranch := strings.TrimSpace(payload.BaseBranch); baseBranch != "" {
		item.BaseBranch = baseBranch
	}
	if baseCommit := strings.TrimSpace(payload.BaseCommit); baseCommit != "" {
		item.BaseCommit = baseCommit
	}
	if worktreeStatus := strings.TrimSpace(payload.WorktreeStatus); worktreeStatus != "" {
		item.WorktreeStatus = worktreeStatus
	}
}

func decodeTaskMultiPayload(event eventspkg.Event) (kinds.TaskRunMultiplePayload, error) {
	var payload kinds.TaskRunMultiplePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return kinds.TaskRunMultiplePayload{}, fmt.Errorf("daemon: decode %s payload: %w", event.Kind, err)
	}
	return payload, nil
}

func (b *taskMultiSnapshotBuilder) ensureItem(slug string) *apicore.TaskRunMultipleItem {
	trimmed := strings.TrimSpace(slug)
	if idx, ok := b.index[trimmed]; ok {
		return &b.items[idx]
	}
	b.items = append(b.items, apicore.TaskRunMultipleItem{
		Slug:   trimmed,
		Status: taskMultiItemStatusQueued,
	})
	idx := len(b.items) - 1
	b.index[trimmed] = idx
	return &b.items[idx]
}

func (b *taskMultiSnapshotBuilder) snapshotItems() []apicore.TaskRunMultipleItem {
	return append([]apicore.TaskRunMultipleItem(nil), b.items...)
}
