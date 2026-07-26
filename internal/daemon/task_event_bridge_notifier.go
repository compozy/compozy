package daemon

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/notifications"
	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	taskpkg "github.com/compozy/agh/internal/task"
)

const defaultBridgeTerminalNotificationQueueSize = 32

type taskEventObserverFanout struct {
	observers []taskpkg.EventObserver
	logger    *slog.Logger
}

var _ taskpkg.EventObserver = (*taskEventObserverFanout)(nil)

func newTaskEventObserverFanout(
	logger *slog.Logger,
	observers ...taskpkg.EventObserver,
) taskpkg.EventObserver {
	filtered := make([]taskpkg.EventObserver, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &taskEventObserverFanout{observers: filtered, logger: logger}
	}
}

func (f *taskEventObserverFanout) OnTaskEvent(ctx context.Context, record taskpkg.EventRecord) {
	if f == nil {
		return
	}
	for _, observer := range f.observers {
		if observer == nil {
			continue
		}
		func(target taskpkg.EventObserver) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger := f.logger
					if logger == nil {
						logger = slog.Default()
					}
					logger.Error(
						"daemon: task event observer panicked during fanout",
						"panic", recovered,
						"event_id", record.Event.ID,
						"task_id", record.Event.TaskID,
						"run_id", record.Event.RunID,
						"event_type", record.Event.EventType,
					)
				}
			}()
			target.OnTaskEvent(ctx, record)
		}(observer)
	}
}

type bridgeTerminalTaskNotificationObserver struct {
	notifier *bridgepkg.TerminalTaskNotifier
	presets  notificationPresetDispatcher
	tasks    bridgepkg.TerminalTaskEventReader
	logger   *slog.Logger
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	pending  map[string]struct{}
	backlog  []bridgeTerminalTaskNotificationWake
	queue    chan bridgeTerminalTaskNotificationWake
}

type notificationPresetDispatcher interface {
	Dispatch(ctx context.Context, event presetspkg.Event) (presetspkg.DispatchResult, error)
}

var _ taskpkg.EventObserver = (*bridgeTerminalTaskNotificationObserver)(nil)

type bridgeTerminalTaskNotificationWake struct {
	taskID     string
	pendingKey string
	eventID    string
	runID      string
	eventType  string
	record     taskpkg.EventRecord
}

func (d *Daemon) composeTaskEventObserver(
	state *bootState,
	store taskStore,
	reentry taskpkg.EventObserver,
) (taskpkg.EventObserver, *bridgeTerminalTaskNotificationObserver, *taskStatusProjectionObserver) {
	if state == nil {
		return reentry, nil, nil
	}
	bridgeEventObserver := newBridgeTerminalTaskNotificationObserver(
		state.bridges,
		store,
		state.bridges,
		state.bridges,
		state.bridges,
		state.notificationPresets,
		state.logger,
		d.now,
		state.cfg.Task.Orchestration.BridgeNotificationTimeout,
	)
	statusProjectionObserver := newTaskStatusProjectionObserver(
		store,
		state.notifier,
		withTaskStatusProjectionObserverLogger(state.logger),
		withTaskStatusProjectionObserverClock(d.now),
		withTaskStatusProjectionObserverQueueSize(state.cfg.Task.Orchestration.NetworkStatusQueueSize),
		withTaskStatusProjectionObserverTimeout(state.cfg.Task.Orchestration.NetworkStatusTimeout),
	)
	return newTaskEventObserverFanout(
		state.logger,
		reentry,
		bridgeEventObserver,
		statusProjectionObserver,
	), bridgeEventObserver, statusProjectionObserver
}

func newBridgeTerminalTaskNotificationObserver(
	subscriptions bridgepkg.BridgeTaskSubscriptionStore,
	taskEvents bridgepkg.TerminalTaskEventReader,
	instances bridgepkg.BridgeInstanceReader,
	cursors notifications.CursorStore,
	transport bridgepkg.DeliveryTransport,
	presets notificationPresetDispatcher,
	logger *slog.Logger,
	now func() time.Time,
	timeout time.Duration,
) *bridgeTerminalTaskNotificationObserver {
	if taskEvents == nil || cursors == nil || transport == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	observer := &bridgeTerminalTaskNotificationObserver{
		logger:  logger,
		timeout: timeout,
		tasks:   taskEvents,
		ctx:     ctx,
		cancel:  cancel,
		pending: make(map[string]struct{}),
		queue: make(
			chan bridgeTerminalTaskNotificationWake,
			defaultBridgeTerminalNotificationQueueSize,
		),
	}
	if subscriptions != nil && instances != nil {
		observer.notifier = bridgepkg.NewTerminalTaskNotifier(bridgepkg.TerminalTaskNotifierConfig{
			Subscriptions: subscriptions,
			TaskEvents:    taskEvents,
			Instances:     instances,
			Cursors:       cursors,
			Transport:     transport,
			Now:           now,
		})
	}
	if presets != nil {
		observer.presets = presets
	}
	if observer.notifier == nil && observer.presets == nil {
		cancel()
		return nil
	}
	observer.start()
	return observer
}

func (o *bridgeTerminalTaskNotificationObserver) OnTaskEvent(
	_ context.Context,
	record taskpkg.EventRecord,
) {
	if o == nil || strings.TrimSpace(record.Event.TaskID) == "" {
		return
	}
	if o.notifier == nil && o.presets == nil {
		return
	}
	if o.notifier != nil && isBridgeTerminalNotificationWake(record.Event.EventType) {
		o.enqueue(record)
		return
	}
	if o.presets == nil {
		return
	}
	o.enqueue(record)
}

func (o *bridgeTerminalTaskNotificationObserver) shutdown() {
	if o == nil {
		return
	}
	if o.cancel != nil {
		o.cancel()
	}
	o.wg.Wait()
}

func (o *bridgeTerminalTaskNotificationObserver) start() {
	if o == nil {
		return
	}
	o.wg.Go(func() {
		for {
			select {
			case <-o.ctx.Done():
				return
			case wake := <-o.queue:
				o.processWake(wake)
			}
		}
	})
}

func (o *bridgeTerminalTaskNotificationObserver) enqueue(record taskpkg.EventRecord) {
	if o == nil {
		return
	}
	wake := bridgeTerminalTaskNotificationWake{
		taskID:    strings.TrimSpace(record.Event.TaskID),
		eventID:   strings.TrimSpace(record.Event.ID),
		runID:     strings.TrimSpace(record.Event.RunID),
		eventType: strings.TrimSpace(record.Event.EventType),
		record:    record,
	}
	if wake.taskID == "" {
		return
	}
	wake.pendingKey = wake.taskID
	if o.presets != nil {
		wake.pendingKey = wake.eventID
		if wake.pendingKey == "" {
			wake.pendingKey = strings.Join([]string{wake.taskID, wake.runID, wake.eventType}, ":")
		}
	}
	if wake.pendingKey == "" {
		return
	}
	o.mu.Lock()
	if _, exists := o.pending[wake.pendingKey]; exists {
		o.mu.Unlock()
		return
	}
	o.pending[wake.pendingKey] = struct{}{}
	o.backlog = append(o.backlog, wake)
	o.mu.Unlock()
	o.drainQueue()
}

func (o *bridgeTerminalTaskNotificationObserver) drainQueue() {
	if o == nil {
		return
	}
	for {
		o.mu.Lock()
		if len(o.backlog) == 0 {
			o.mu.Unlock()
			return
		}
		wake := o.backlog[0]
		select {
		case o.queue <- wake:
			o.backlog = o.backlog[1:]
			o.mu.Unlock()
		default:
			o.mu.Unlock()
			return
		}
	}
}

func (o *bridgeTerminalTaskNotificationObserver) processWake(
	wake bridgeTerminalTaskNotificationWake,
) {
	defer func() {
		o.mu.Lock()
		delete(o.pending, wake.pendingKey)
		o.mu.Unlock()
		o.drainQueue()
	}()

	notifyCtx := context.Background()
	cancel := func() {}
	if o.timeout > 0 {
		notifyCtx, cancel = context.WithTimeout(notifyCtx, o.timeout)
	}
	defer cancel()

	if o.notifier != nil && isBridgeTerminalNotificationWake(wake.eventType) {
		o.processTerminalWake(notifyCtx, wake)
	}
	if o.presets != nil {
		o.processPresetWake(notifyCtx, wake)
	}
}

func (o *bridgeTerminalTaskNotificationObserver) processTerminalWake(
	ctx context.Context,
	wake bridgeTerminalTaskNotificationWake,
) {
	sweep, err := o.notifier.DeliverDue(
		ctx,
		bridgepkg.BridgeTaskSubscriptionQuery{TaskID: wake.taskID},
	)
	if err != nil {
		o.logTerminalWakeFailure(wake, sweep, err)
		return
	}
	if sweep.Delivered > 0 || sweep.Suppressed > 0 || sweep.Deferred > 0 || sweep.Failed > 0 {
		o.logTerminalWakeSuccess(wake, sweep)
	}
}

func (o *bridgeTerminalTaskNotificationObserver) processPresetWake(
	ctx context.Context,
	wake bridgeTerminalTaskNotificationWake,
) {
	event := presetEventFromTaskRecord(ctx, o.tasks, wake.record, o.log())
	result, err := o.presets.Dispatch(ctx, event)
	if err != nil {
		o.logPresetWakeFailure(wake, result, err)
		return
	}
	if result.Delivered > 0 || result.Suppressed > 0 || result.Failed > 0 {
		o.logPresetWakeSuccess(wake, result)
	}
}

func (o *bridgeTerminalTaskNotificationObserver) logTerminalWakeFailure(
	wake bridgeTerminalTaskNotificationWake,
	sweep bridgepkg.TerminalTaskNotificationSweep,
	err error,
) {
	o.log().Warn(
		"daemon: bridge terminal task notification delivery failed",
		"error", err,
		"event_id", wake.eventID,
		"task_id", wake.taskID,
		"run_id", wake.runID,
		"event_type", wake.eventType,
		"subscriptions", sweep.Subscriptions,
		"delivered", sweep.Delivered,
		"suppressed", sweep.Suppressed,
		"deferred", sweep.Deferred,
		"failed", sweep.Failed,
	)
}

func (o *bridgeTerminalTaskNotificationObserver) logTerminalWakeSuccess(
	wake bridgeTerminalTaskNotificationWake,
	sweep bridgepkg.TerminalTaskNotificationSweep,
) {
	o.log().Debug(
		"daemon: bridge terminal task notification sweep complete",
		"event_id", wake.eventID,
		"task_id", wake.taskID,
		"run_id", wake.runID,
		"event_type", wake.eventType,
		"subscriptions", sweep.Subscriptions,
		"delivered", sweep.Delivered,
		"suppressed", sweep.Suppressed,
		"deferred", sweep.Deferred,
		"failed", sweep.Failed,
	)
}

func (o *bridgeTerminalTaskNotificationObserver) logPresetWakeFailure(
	wake bridgeTerminalTaskNotificationWake,
	result presetspkg.DispatchResult,
	err error,
) {
	o.log().Warn(
		"daemon: notification preset delivery failed",
		"error", err,
		"event_id", wake.eventID,
		"task_id", wake.taskID,
		"run_id", wake.runID,
		"event_type", wake.eventType,
		"matched", result.Matched,
		"delivered", result.Delivered,
		"suppressed", result.Suppressed,
		"skipped", result.Skipped,
		"failed", result.Failed,
	)
}

func (o *bridgeTerminalTaskNotificationObserver) logPresetWakeSuccess(
	wake bridgeTerminalTaskNotificationWake,
	result presetspkg.DispatchResult,
) {
	o.log().Debug(
		"daemon: notification preset sweep complete",
		"event_id", wake.eventID,
		"task_id", wake.taskID,
		"run_id", wake.runID,
		"event_type", wake.eventType,
		"matched", result.Matched,
		"delivered", result.Delivered,
		"suppressed", result.Suppressed,
		"skipped", result.Skipped,
		"failed", result.Failed,
	)
}

func presetEventFromTaskRecord(
	ctx context.Context,
	tasks bridgepkg.TerminalTaskEventReader,
	record taskpkg.EventRecord,
	logger *slog.Logger,
) presetspkg.Event {
	event := presetspkg.Event{
		ID:        strings.TrimSpace(record.Event.ID),
		Type:      strings.TrimSpace(record.Event.EventType),
		AgentName: strings.TrimSpace(record.Event.Actor.Ref),
		TaskID:    strings.TrimSpace(record.Event.TaskID),
		RunID:     strings.TrimSpace(record.Event.RunID),
		Outcome:   eventspkg.OutcomeFor(record.Event.EventType),
		Sequence:  record.Sequence,
		Payload:   append([]byte(nil), record.Event.Payload...),
		Timestamp: record.Event.Timestamp,
	}
	if tasks != nil && event.TaskID != "" {
		taskRecord, err := tasks.GetTask(ctx, event.TaskID)
		if err == nil {
			event.WorkspaceID = strings.TrimSpace(taskRecord.WorkspaceID)
			event.Summary = strings.TrimSpace(taskRecord.Title)
		} else if logger != nil {
			logger.Debug(
				"daemon: notification preset could not enrich task event",
				"task_id", event.TaskID,
				"event_id", event.ID,
				"error", err,
			)
		}
	}
	return event
}

func (o *bridgeTerminalTaskNotificationObserver) log() *slog.Logger {
	if o != nil && o.logger != nil {
		return o.logger
	}
	return slog.Default()
}

func isBridgeTerminalNotificationWake(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case harnessTaskEventRunCompleted,
		"task.run_failed",
		"task.run_canceled",
		"task.run_review_approved",
		"task.canceled":
		return true
	default:
		return false
	}
}
