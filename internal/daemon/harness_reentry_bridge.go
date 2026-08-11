package daemon

import (
	"context"

	"errors"
	"fmt"
	"log/slog"

	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	harnessSummaryDetachedCompleted       = eventspkg.HarnessDetachedRunCompleted
	harnessSummarySyntheticReentryEmitted = eventspkg.HarnessSyntheticReentryEmitted
	harnessSummarySyntheticReentryDropped = eventspkg.HarnessSyntheticReentryDropped

	harnessReentryShutdownFinalizationTimeout = time.Second

	harnessTaskEventRunCompleted = eventspkg.TaskRunCompleted
	harnessTaskEventRunFailed    = eventspkg.TaskRunFailed
	harnessTaskEventRunCanceled  = eventspkg.TaskRunCanceled

	harnessReentryOutcomeEmitted = "emitted"
	harnessReentryOutcomeSilent  = "silent"
	harnessReentryOutcomeDropped = "dropped"

	harnessReentryReasonCompleted            = "task_run_completed"
	harnessReentryReasonFailed               = "task_run_failed"
	harnessReentryReasonCanceled             = "task_run_canceled"
	harnessReentryReasonPolicySilent         = "policy_silent"
	harnessReentryReasonTargetMissing        = "target_missing"
	harnessReentryReasonTargetInactivePrefix = "target_inactive"
	harnessReentryReasonDispatchFailed       = "synthetic_dispatch_failed"
	harnessReentryReasonEventMissing         = "synthetic_event_missing"
	harnessReentryReasonAlreadyRecorded      = "synthetic_event_already_recorded"
)

type harnessReentryStore interface {
	store.EventSummaryStore
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	ListTaskRunsByStatus(ctx context.Context, statuses []taskpkg.RunStatus) ([]taskpkg.Run, error)
	ListTaskEventRecords(ctx context.Context, query taskpkg.EventRecordQuery) ([]taskpkg.EventRecord, error)
	UpdateTaskRunMetadata(
		ctx context.Context,
		mutation taskpkg.RunMetadataMutation,
	) (taskpkg.Run, error)
}

type harnessReentrySessionManager interface {
	Status(ctx context.Context, id string) (*session.Info, error)
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

type harnessWakeTargetSnapshot struct {
	SessionID            string
	AgentName            string
	Type                 session.Type
	State                session.State
	WorkspaceID          string
	NetworkParticipation participation.Spec
	Missing              bool
}

type harnessReentryDecision struct {
	outcome          string
	reason           string
	target           harnessWakeTargetSnapshot
	syntheticMessage string
	syntheticMeta    acp.PromptSyntheticMeta
}

type harnessSyntheticWake struct {
	taskID            string
	runID             string
	targetSessionID   string
	targetAgentName   string
	targetWorkspaceID string
	reason            string
	summary           string
	completedAt       time.Time
	completionSeq     int64
	syntheticMessage  string
	syntheticMeta     acp.PromptSyntheticMeta
}

type harnessWakeQueue struct {
	dispatching bool
	items       []harnessSyntheticWake
}

type recoveredDetachedHarnessRun struct {
	run           taskpkg.Run
	completionSeq int64
	completedAt   time.Time
}

type harnessReentryBridge struct {
	ctx           context.Context
	cancel        context.CancelFunc
	workers       sync.WaitGroup
	resolver      *HarnessContextResolver
	recorder      *harnessLifecycleRecorder
	store         harnessReentryStore
	sessions      harnessReentrySessionManager
	heartbeatWake heartbeat.WakeService
	logger        *slog.Logger

	events chan taskpkg.EventRecord
	rescan chan struct{}

	processingMu sync.Mutex
	processing   map[string]struct{}

	queueMu sync.Mutex
	queues  map[string]*harnessWakeQueue
}

var _ taskpkg.EventObserver = (*harnessReentryBridge)(nil)

func newHarnessReentryBridge(
	ctx context.Context,
	resolver *HarnessContextResolver,
	recorder *harnessLifecycleRecorder,
	store harnessReentryStore,
	sessions harnessReentrySessionManager,
	logger *slog.Logger,
	options ...harnessReentryOption,
) (*harnessReentryBridge, error) {
	if ctx == nil {
		return nil, errors.New("daemon: harness reentry bridge context is required")
	}
	if resolver == nil {
		return nil, errors.New("daemon: harness reentry bridge requires a harness resolver")
	}
	if store == nil {
		return nil, errors.New("daemon: harness reentry bridge requires a task store")
	}
	if sessions == nil {
		return nil, errors.New("daemon: harness reentry bridge requires a session manager")
	}
	if logger == nil {
		logger = slog.Default()
	}

	bridgeCtx, cancel := context.WithCancel(ctx)
	bridge := &harnessReentryBridge{
		ctx:        bridgeCtx,
		cancel:     cancel,
		resolver:   resolver,
		recorder:   recorder,
		store:      store,
		sessions:   sessions,
		logger:     logger,
		events:     make(chan taskpkg.EventRecord, 256),
		rescan:     make(chan struct{}, 1),
		processing: make(map[string]struct{}),
		queues:     make(map[string]*harnessWakeQueue),
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(bridge); err != nil {
			cancel()
			return nil, err
		}
	}
	bridge.startWorker(bridge.run)
	return bridge, nil
}

type harnessReentryOption func(*harnessReentryBridge) error

func withHarnessHeartbeatWake(
	store any,
	sessions harnessReentrySessionManager,
	config compozyconfig.HeartbeatConfig,
) harnessReentryOption {
	return func(bridge *harnessReentryBridge) error {
		if bridge == nil || !config.Enabled {
			return nil
		}
		wakeStore, ok := store.(heartbeat.WakeStore)
		if !ok {
			return nil
		}
		healthReader, ok := sessions.(heartbeat.SessionHealthReader)
		if !ok {
			return nil
		}
		service, err := heartbeat.NewManagedWakeService(wakeStore, healthReader, bridge, config)
		if err != nil {
			return fmt.Errorf("daemon: create harness heartbeat wake service: %w", err)
		}
		bridge.heartbeatWake = service
		return nil
	}
}

func (b *harnessReentryBridge) shutdown() {
	if b == nil || b.cancel == nil {
		return
	}
	b.cancel()
	b.workers.Wait()
}

func (b *harnessReentryBridge) startWorker(worker func()) {
	b.workers.Go(func() {
		worker()
	})
}

func (b *harnessReentryBridge) OnTaskEvent(_ context.Context, record taskpkg.EventRecord) {
	if b == nil || !isDetachedHarnessTerminalTaskEvent(record.Event.EventType) {
		return
	}

	select {
	case <-b.ctx.Done():
		return
	case b.events <- record:
		return
	default:
	}

	b.logger.Warn(
		"daemon: detached harness reentry queue saturated; scheduling recovery rescan",
		"task_id", record.Event.TaskID,
		"run_id", record.Event.RunID,
		"event_type", record.Event.EventType,
	)
	b.requestRescan()
}

func (b *harnessReentryBridge) run() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case record := <-b.events:
			b.processTaskEvent(record)
		case <-b.rescan:
			if err := b.recoverPendingRuns(b.operationContext()); err != nil {
				b.logger.Error("daemon: recover detached harness runs after queue saturation", "error", err)
			}
		}
	}
}

func (b *harnessReentryBridge) recover(ctx context.Context) error {
	return b.recoverPendingRuns(ctx)
}

func (b *harnessReentryBridge) recoverPendingRuns(ctx context.Context) error {
	if b == nil {
		return errors.New("daemon: harness reentry bridge is required")
	}
	if ctx == nil {
		return errors.New("daemon: harness reentry recovery context is required")
	}

	recovered, err := b.loadRecoveredDetachedHarnessRuns(ctx)
	if err != nil {
		return err
	}

	for i := range recovered {
		item := &recovered[i]
		if err := b.processTerminalRun(
			item.run.TaskID,
			item.run.ID,
			item.completionSeq,
			item.completedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func (b *harnessReentryBridge) latestDetachedTerminalSequence(
	ctx context.Context,
	taskID string,
	runID string,
) (int64, time.Time, error) {
	records, err := b.store.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: strings.TrimSpace(taskID)})
	if err != nil {
		return 0, time.Time{}, err
	}

	var matched *taskpkg.EventRecord
	for i := range records {
		record := records[i]
		if strings.TrimSpace(record.Event.RunID) != strings.TrimSpace(runID) {
			continue
		}
		if !isDetachedHarnessTerminalTaskEvent(record.Event.EventType) {
			continue
		}
		if matched == nil || matched.Sequence < record.Sequence {
			item := record
			matched = &item
		}
	}
	if matched == nil {
		return 0, time.Time{}, nil
	}
	return matched.Sequence, matched.Event.Timestamp, nil
}

func (b *harnessReentryBridge) processTaskEvent(record taskpkg.EventRecord) {
	if err := b.processTerminalRun(
		record.Event.TaskID,
		record.Event.RunID,
		record.Sequence,
		record.Event.Timestamp,
	); err != nil {
		b.logger.Error(
			"daemon: process detached harness terminal run",
			"task_id", record.Event.TaskID,
			"run_id", record.Event.RunID,
			"event_type", record.Event.EventType,
			"error", err,
		)
	}
}

func (b *harnessReentryBridge) processTerminalRun(
	taskID string,
	runID string,
	completionSequence int64,
	completedAt time.Time,
) error {
	if b == nil {
		return errors.New("daemon: harness reentry bridge is required")
	}

	opCtx := b.operationContext()
	run, err := b.store.GetTaskRun(opCtx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	if !isDetachedHarnessTerminalRun(run.Status.Normalize()) {
		return nil
	}

	metadata, ok, err := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if detachedHarnessReentryProcessed(metadata.Reentry) {
		return nil
	}
	if !b.claimProcessing(run.ID) {
		return nil
	}

	task, err := b.store.GetTask(opCtx, strings.TrimSpace(taskID))
	if err != nil {
		b.releaseProcessing(run.ID)
		return err
	}

	if completedAt.IsZero() {
		completedAt = run.EndedAt
	}
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}

	target := b.resolveWakeTargetSnapshot(metadata)
	agentName := b.resolveSummaryAgentName(target, metadata)
	decision := b.evaluateDecision(task, run, metadata, target)
	b.writeEventSummary(
		target.SessionID,
		agentName,
		harnessSummaryDetachedCompleted,
		detachedCompletionSummary(task.ID, run, metadata),
		completedAt,
	)

	return b.applyWakeDecision(
		task,
		run,
		target,
		agentName,
		completionSequence,
		completedAt,
		decision,
	)
}
