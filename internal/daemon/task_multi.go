package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
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
	items            []preparedTaskMultiItem
}

type preparedTaskMultiItem struct {
	slug         string
	workflowID   *string
	workflowRoot string
	runtimeCfg   *model.RuntimeConfig
}

type taskMultiSnapshotBuilder struct {
	items []apicore.TaskRunMultipleItem
	index map[string]int
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
		row, workflowID, runtimeCfg, childPresentationMode, err := m.prepareTaskStart(
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
	return &preparedTaskMulti{
		workspace:        workspaceRow,
		mode:             mode,
		presentationMode: presentationMode,
		items:            items,
	}, nil
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
		return "", taskMultiValidationProblem(
			"unsupported_run_multiple_mode",
			"parallel run_multiple mode is not supported by the daemon; use enqueued",
			"mode",
		)
	default:
		return "", taskMultiValidationProblem(
			"invalid_run_multiple_mode",
			"run_multiple mode must be enqueued",
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

func (m *RunManager) runTaskMultiCoordinator(active *activeRun) error {
	if active == nil || active.taskMulti == nil {
		return errors.New("task multi run is not configured")
	}
	prepared := active.taskMulti
	total := len(prepared.items)
	if err := m.emitTaskMultiEvent(active, eventspkg.EventKindTaskRunMultipleStarted, kinds.TaskRunMultiplePayload{
		Mode:   prepared.mode,
		Status: runStatusRunning,
		Slugs:  preparedTaskMultiSlugs(prepared.items),
		Total:  total,
	}); err != nil {
		return err
	}
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
	return m.emitTaskMultiEvent(active, eventspkg.EventKindTaskRunMultipleQueueCompleted, kinds.TaskRunMultiplePayload{
		Mode:   prepared.mode,
		Status: runStatusCompleted,
		Slugs:  preparedTaskMultiSlugs(prepared.items),
		Total:  total,
	})
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
		cancelErr := m.cancelTaskMultiQueuedItems(active, prepared.items, index+1, total, err)
		return errors.Join(err, emitErr, cancelErr)
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
		item := b.ensureItem(payload.Slug)
		item.Status = strings.TrimSpace(payload.Status)
		if childRunID := strings.TrimSpace(payload.ChildRunID); childRunID != "" {
			item.RunID = childRunID
		}
		if errorText := strings.TrimSpace(payload.Error); errorText != "" {
			item.ErrorText = errorText
		}
	}
	return nil
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
