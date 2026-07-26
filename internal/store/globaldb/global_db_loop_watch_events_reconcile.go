package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	watchEventsMatchedEvent       = "loop.watch_events.matched"
	watchEventsWakeEnqueuedEvent  = "loop.watch_events.wake_enqueued"
	watchEventsWakeCoalescedEvent = "loop.watch_events.wake_coalesced"
	watchEventsWakeErrorEvent     = "loop.watch_events.wake_error"

	watchEventsRecoverySessionID = "loop-watch-events-recovery"
	watchEventsDaemonAgentName   = "daemon"
	watchEventsGapReason         = "watch_events_gap"

	watchEventsContentWorkspaceIDKey = "workspace_id"
	watchEventsContentIdempotencyKey = "idempotency_key"
	watchEventsContentRunIDKey       = "run_id"
	watchEventsContentErrorKey       = "error"
)

// EnqueueWatchEventsGapWakes scans parked watch-events nodes for durable
// cursor gaps and enqueues idempotent coordinator wakes.
func (g *LoopRepo) EnqueueWatchEventsGapWakes(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
) ([]taskpkg.Run, error) {
	normalizedOrigin, now, err := g.normalizeWatchEventsGapScan(ctx, origin, now)
	if err != nil {
		return nil, err
	}
	parked, err := g.watchEvents.ListParkedWatchEventSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	return g.enqueueWatchEventsGapWakesForSubscriptions(ctx, parked, normalizedOrigin, now)
}

// EnqueueWatchEventsGapWakesPage checks one bounded page of parked
// subscriptions and returns the durable cursor for the next scheduler cycle.
func (g *LoopRepo) EnqueueWatchEventsGapWakesPage(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
	after looppkg.ParkedWatchEventScanCursor,
	limit int,
) ([]taskpkg.Run, looppkg.ParkedWatchEventScanCursor, error) {
	normalizedOrigin, now, err := g.normalizeWatchEventsGapScan(ctx, origin, now)
	if err != nil {
		return nil, after, err
	}
	parked, err := g.watchEvents.ListParkedWatchEventSubscriptionsPage(ctx, after, limit)
	if err != nil {
		return nil, after, err
	}
	if len(parked) == 0 && !parkedWatchEventScanCursorEmpty(after) {
		after = looppkg.ParkedWatchEventScanCursor{}
		parked, err = g.watchEvents.ListParkedWatchEventSubscriptionsPage(ctx, after, limit)
		if err != nil {
			return nil, after, err
		}
	}

	enqueued := make([]taskpkg.Run, 0)
	var scanErr error
	for _, subscription := range parked {
		runs, enqueueErr := g.enqueueWatchEventsGapWakesForSubscriptions(
			ctx,
			[]looppkg.ParkedWatchEventSubscription{subscription},
			normalizedOrigin,
			now,
		)
		enqueued = append(enqueued, runs...)
		after = parkedWatchEventScanCursor(subscription)
		if enqueueErr != nil {
			scanErr = errors.Join(
				scanErr,
				fmt.Errorf(
					"store: recover parked watch-events subscription %q/%q: %w",
					subscription.LoopRunID,
					subscription.NodeID,
					enqueueErr,
				),
			)
		}
	}
	return enqueued, after, scanErr
}

func (g *LoopRepo) normalizeWatchEventsGapScan(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
) (taskpkg.Origin, time.Time, error) {
	if err := g.checkReady(ctx, "enqueue watch-events gap wakes"); err != nil {
		return taskpkg.Origin{}, time.Time{}, err
	}
	normalizedOrigin, err := normalizeLoopCoordinatorReconcileOrigin(origin)
	if err != nil {
		return taskpkg.Origin{}, time.Time{}, err
	}
	return normalizedOrigin, g.normalizeLoopCoordinatorReconcileTime(now), nil
}

func (g *LoopRepo) enqueueWatchEventsGapWakesForSubscriptions(
	ctx context.Context,
	parked []looppkg.ParkedWatchEventSubscription,
	origin taskpkg.Origin,
	now time.Time,
) ([]taskpkg.Run, error) {
	enqueued := make([]taskpkg.Run, 0)
	for _, subscription := range parked {
		hasGap, err := g.parkedWatchEventSubscriptionHasGap(ctx, subscription)
		if err != nil {
			return enqueued, err
		}
		if !hasGap {
			continue
		}
		if err := g.writeWatchEventsGapEvent(
			ctx,
			subscription,
			taskpkg.Run{},
			watchEventsMatchedEvent,
			string(eventspkg.OutcomeInfo),
			now,
			nil,
		); err != nil {
			return enqueued, err
		}
		run, added, wakeErr := g.EnqueueLoopCoordinatorWake(
			ctx,
			subscription.LoopRunID,
			watchEventsCoordinatorWakeKey(subscription.LoopRunID, subscription.NodeID),
			origin,
			now,
		)
		if err := normalizeWatchEventsGapWakeError(wakeErr); err != nil {
			if writeErr := g.writeWatchEventsGapEvent(
				ctx,
				subscription,
				taskpkg.Run{},
				watchEventsWakeErrorEvent,
				string(eventspkg.OutcomeFailure),
				now,
				err,
			); writeErr != nil {
				return enqueued, errors.Join(err, writeErr)
			}
			return enqueued, err
		}
		if added {
			enqueued = append(enqueued, run)
		}
		eventType := watchEventsWakeEnqueuedEvent
		if !added {
			eventType = watchEventsWakeCoalescedEvent
		}
		if err := g.writeWatchEventsGapEvent(
			ctx,
			subscription,
			run,
			eventType,
			string(eventspkg.OutcomeInfo),
			now,
			wakeErr,
		); err != nil {
			return enqueued, err
		}
	}
	return enqueued, nil
}

func parkedWatchEventScanCursor(
	subscription looppkg.ParkedWatchEventSubscription,
) looppkg.ParkedWatchEventScanCursor {
	return looppkg.ParkedWatchEventScanCursor{
		WorkspaceID: strings.TrimSpace(subscription.WorkspaceID),
		LoopRunID:   strings.TrimSpace(subscription.LoopRunID),
		NodeID:      strings.TrimSpace(subscription.NodeID),
	}
}

func parkedWatchEventScanCursorEmpty(cursor looppkg.ParkedWatchEventScanCursor) bool {
	return strings.TrimSpace(cursor.WorkspaceID) == "" &&
		strings.TrimSpace(cursor.LoopRunID) == "" &&
		strings.TrimSpace(cursor.NodeID) == ""
}

func (g *LoopRepo) parkedWatchEventSubscriptionHasGap(
	ctx context.Context,
	subscription looppkg.ParkedWatchEventSubscription,
) (bool, error) {
	query, err := watchEventsGapQuery(subscription)
	if err != nil {
		return false, err
	}
	cursors, err := g.watchEvents.ReadCursors(ctx, query)
	if err != nil {
		return false, err
	}
	for stream, cursor := range cursors {
		if cursor > subscription.Cursors[stream] {
			return true, nil
		}
	}
	return false, nil
}

func watchEventsGapQuery(
	subscription looppkg.ParkedWatchEventSubscription,
) (looppkg.WatchEventsQuery, error) {
	streams := map[string]int64{}
	kindSet := map[string]struct{}{}
	for _, ref := range subscription.Subscriptions {
		contract, ok := subscription.Contracts[hooks.HookEvent(strings.TrimSpace(ref.Kind))]
		if !ok {
			return looppkg.WatchEventsQuery{}, fmt.Errorf(
				"%w: watch-events kind is unsupported: %q",
				looppkg.ErrValidation,
				ref.Kind,
			)
		}
		matchedCursor := false
		for stream, cursor := range subscription.Cursors {
			if looppkg.WatchEventsBaseStream(stream) != contract.Stream {
				continue
			}
			streams[stream] = cursor
			matchedCursor = true
		}
		if !matchedCursor {
			return looppkg.WatchEventsQuery{}, fmt.Errorf(
				"%w: watch-events cursor for stream %q is required",
				looppkg.ErrValidation,
				contract.Stream,
			)
		}
		for _, ledgerType := range contract.LedgerTypes {
			trimmed := strings.TrimSpace(ledgerType)
			if trimmed != "" {
				kindSet[trimmed] = struct{}{}
			}
		}
	}
	kinds := make([]string, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	return looppkg.WatchEventsQuery{
		WorkspaceID: strings.TrimSpace(subscription.WorkspaceID),
		Streams:     streams,
		Kinds:       kinds,
		Limit:       looppkg.LoopMaxFanoutWidth,
	}, nil
}

func (g *LoopRepo) writeWatchEventsGapEvent(
	ctx context.Context,
	subscription looppkg.ParkedWatchEventSubscription,
	run taskpkg.Run,
	eventType string,
	outcome string,
	now time.Time,
	cause error,
) error {
	content, err := json.Marshal(map[string]any{
		columnLoopRunID:                  strings.TrimSpace(subscription.LoopRunID),
		columnLoopName:                   strings.TrimSpace(subscription.LoopName),
		loopRunEventPayloadKeyNodeID:     strings.TrimSpace(subscription.NodeID),
		watchEventsContentWorkspaceIDKey: strings.TrimSpace(subscription.WorkspaceID),
		loopRunEventPayloadKeyGeneration: subscription.Generation,
		watchEventsContentIdempotencyKey: watchEventsCoordinatorWakeKey(subscription.LoopRunID, subscription.NodeID),
		watchEventsContentRunIDKey:       strings.TrimSpace(run.ID),
		watchEventsContentErrorKey:       watchEventsErrorString(cause),
	})
	if err != nil {
		return fmt.Errorf("store: marshal watch-events gap event: %w", err)
	}
	return g.observe.WriteEventSummary(ctx, store.EventSummary{
		SessionID:   watchEventsRecoverySessionID,
		WorkspaceID: strings.TrimSpace(subscription.WorkspaceID),
		Type:        eventType,
		AgentName:   watchEventsDaemonAgentName,
		Outcome:     outcome,
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			RunID:           strings.TrimSpace(run.ID),
			SchedulerReason: watchEventsGapReason,
			ActorKind:       string(taskpkg.ActorKindDaemon),
			ActorID:         watchEventsRecoverySessionID,
		},
		Summary:   watchEventsGapSummary(eventType, subscription),
		Timestamp: now.UTC(),
	})
}

func watchEventsGapSummary(eventType string, subscription looppkg.ParkedWatchEventSubscription) string {
	switch eventType {
	case watchEventsMatchedEvent:
		return "watch-events gap matched for " + strings.TrimSpace(subscription.NodeID)
	case watchEventsWakeCoalescedEvent:
		return "watch-events gap wake coalesced for " + strings.TrimSpace(subscription.NodeID)
	case watchEventsWakeErrorEvent:
		return "watch-events gap wake failed for " + strings.TrimSpace(subscription.NodeID)
	default:
		return "watch-events gap wake enqueued for " + strings.TrimSpace(subscription.NodeID)
	}
}

func watchEventsCoordinatorWakeKey(loopRunID string, nodeID string) string {
	return fmt.Sprintf(
		"loop.coordinator.watch_events.%s.%s",
		strings.TrimSpace(loopRunID),
		strings.TrimSpace(nodeID),
	)
}

func normalizeWatchEventsGapWakeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, taskpkg.ErrInvalidStatusTransition), errors.Is(err, taskpkg.ErrConflict):
		return nil
	default:
		return err
	}
}

func watchEventsErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
