package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type networkWakePending struct {
	notification store.CommittedNetworkNotification
	key          string
	readyAt      time.Time
}

type networkWakeWorkResult struct {
	pending networkWakePending
	retry   bool
	err     error
}

func deriveNetworkWakeRunnerActor() (taskpkg.ActorContext, error) {
	actor, err := taskpkg.DeriveDaemonActorContext("network-wake-runner", "daemon.network_wake")
	if err != nil {
		return taskpkg.ActorContext{}, fmt.Errorf("daemon: derive network wake actor: %w", err)
	}
	return actor, nil
}

func makeNetworkWakePending(
	notifications []store.CommittedNetworkNotification,
	readyAt time.Time,
) []networkWakePending {
	pending := make([]networkWakePending, 0, len(notifications))
	for _, notification := range notifications {
		deadline := notification.ReadyAt.UTC()
		if deadline.IsZero() {
			deadline = readyAt
		}
		pending = append(pending, networkWakePending{
			notification: notification,
			key:          networkWakeDispatchKey(notification),
			readyAt:      deadline,
		})
	}
	return pending
}

func networkWakeDispatchKey(notification store.CommittedNetworkNotification) string {
	return strings.TrimSpace(notification.WorkspaceID) + "\x00" +
		strings.TrimSpace(notification.RecipientSessionID)
}

func (r *networkWakeRunner) loadDurableWakes(
	ctx context.Context,
) ([]store.CommittedNetworkNotification, error) {
	notifications, err := r.store.ListQueuedNetworkWakes(ctx, networkWakeScanLimit)
	if err != nil {
		return nil, fmt.Errorf("daemon: list queued network wakes: %w", err)
	}
	return notifications, nil
}

func (r *networkWakeRunner) run(
	ctx context.Context,
	actor taskpkg.ActorContext,
	initial []networkWakePending,
) error {
	results := make(chan networkWakeWorkResult, r.maxWorkers)
	pending := make([]networkWakePending, 0, len(initial))
	known := make(map[string]struct{}, len(initial))
	for _, item := range initial {
		addNetworkWakePending(&pending, known, item)
	}
	activeKeys := make(map[string]struct{}, r.maxWorkers)
	activeWorkers := 0
	ticker := time.NewTicker(networkWakePollInterval)
	defer ticker.Stop()

	for {
		pending, activeWorkers = r.dispatchNetworkWakes(
			ctx,
			actor,
			pending,
			activeKeys,
			activeWorkers,
			results,
		)
		retryTimer, retryReady := networkWakeRetryTimer(pending, r.now().UTC())
		select {
		case <-ctx.Done():
			stopNetworkWakeRetryTimer(retryTimer)
			r.cancelActive()
			r.workerWG.Wait()
			return nil
		case notification := <-r.notifications:
			stopNetworkWakeRetryTimer(retryTimer)
			for _, item := range makeNetworkWakePending(
				[]store.CommittedNetworkNotification{notification},
				r.now().UTC(),
			) {
				addNetworkWakePending(&pending, known, item)
			}
		case result := <-results:
			stopNetworkWakeRetryTimer(retryTimer)
			delete(activeKeys, result.pending.key)
			activeWorkers--
			r.logNetworkWakeResult(result)
			if result.retry && ctx.Err() == nil {
				result.pending.readyAt = r.now().UTC().Add(networkWakeRetryDelay)
				pending = append(pending, result.pending)
			} else {
				delete(known, strings.TrimSpace(result.pending.notification.TaskRunID))
			}
		case <-ticker.C:
			stopNetworkWakeRetryTimer(retryTimer)
			if !r.networkEnabled() {
				continue
			}
			notifications, err := r.loadDurableWakes(ctx)
			if err != nil {
				r.logger.Error("daemon.network_wake.scan_failed", "error", err)
				continue
			}
			for _, item := range makeNetworkWakePending(notifications, r.now().UTC()) {
				addNetworkWakePending(&pending, known, item)
			}
		case <-retryReady:
		}
	}
}

func (r *networkWakeRunner) dispatchNetworkWakes(
	ctx context.Context,
	actor taskpkg.ActorContext,
	pending []networkWakePending,
	activeKeys map[string]struct{},
	activeWorkers int,
	results chan<- networkWakeWorkResult,
) ([]networkWakePending, int) {
	if !r.networkEnabled() {
		return pending, activeWorkers
	}
	for activeWorkers < r.maxWorkers {
		index := nextRunnableNetworkWake(pending, activeKeys, r.now().UTC())
		if index < 0 {
			break
		}
		item := pending[index]
		pending = append(pending[:index], pending[index+1:]...)
		activeKeys[item.key] = struct{}{}
		activeWorkers++
		workerCtx, cancel := context.WithCancel(ctx)
		r.registerActive(strings.TrimSpace(item.notification.TaskRunID), cancel)
		r.workerWG.Go(func() {
			defer cancel()
			defer r.unregisterActive(strings.TrimSpace(item.notification.TaskRunID))
			retry, err := r.processNotification(workerCtx, actor, item.notification)
			results <- networkWakeWorkResult{pending: item, retry: retry, err: err}
		})
	}
	return pending, activeWorkers
}

func addNetworkWakePending(
	pending *[]networkWakePending,
	known map[string]struct{},
	item networkWakePending,
) {
	runID := strings.TrimSpace(item.notification.TaskRunID)
	if runID == "" {
		return
	}
	if _, exists := known[runID]; exists {
		return
	}
	known[runID] = struct{}{}
	*pending = append(*pending, item)
}

func nextRunnableNetworkWake(
	pending []networkWakePending,
	activeKeys map[string]struct{},
	now time.Time,
) int {
	for index, item := range pending {
		if item.readyAt.After(now) {
			continue
		}
		if _, busy := activeKeys[item.key]; busy {
			continue
		}
		return index
	}
	return -1
}

func networkWakeRetryTimer(pending []networkWakePending, now time.Time) (*time.Timer, <-chan time.Time) {
	var earliest time.Time
	for _, item := range pending {
		if item.readyAt.After(now) && (earliest.IsZero() || item.readyAt.Before(earliest)) {
			earliest = item.readyAt
		}
	}
	if earliest.IsZero() {
		return nil, nil
	}
	timer := time.NewTimer(earliest.Sub(now))
	return timer, timer.C
}

func stopNetworkWakeRetryTimer(timer *time.Timer) {
	if timer != nil {
		timer.Stop()
	}
}

func (r *networkWakeRunner) logNetworkWakeResult(result networkWakeWorkResult) {
	if result.err == nil {
		return
	}
	r.logger.Error(
		"daemon.network_wake.execution_failed",
		"task_run_id", result.pending.notification.TaskRunID,
		"recipient_session_id", result.pending.notification.RecipientSessionID,
		"error", result.err,
	)
}
