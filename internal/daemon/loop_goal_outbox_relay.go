package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	"golang.org/x/sync/errgroup"
)

const (
	goalSessionOutboxBatchSize    = 50
	goalSessionOutboxPollInterval = 100 * time.Millisecond
	loopSessionCleanupConcurrency = 64
	loopSessionCleanupStopTimeout = 5 * time.Second
)

type goalSessionOutboxStore interface {
	ClaimGoalSessionOutbox(context.Context, int) ([]goalpkg.SessionOutboxEvent, error)
	AcknowledgeGoalSessionOutbox(context.Context, string, time.Time) error
}

type loopSessionCleanupStore interface {
	ClaimLoopSessionCleanup(context.Context, int) ([]looppkg.SessionCleanupObligation, error)
	AcknowledgeLoopSessionCleanup(context.Context, string, time.Time) error
}

type loopSessionCleanupStopper interface {
	StopWithCause(context.Context, string, session.StopCause, string) error
}

type goalSessionEventAppender interface {
	AppendSessionEventIfAbsent(context.Context, session.GoalEvent) error
}

type goalSessionOutboxRelay struct {
	store        goalSessionOutboxStore
	appender     goalSessionEventAppender
	cleanupStore loopSessionCleanupStore
	stopper      loopSessionCleanupStopper
	logger       *slog.Logger
	now          func() time.Time
	batchSize    int
	pollInterval time.Duration
}

type goalSnapshotChangedPayload struct {
	SessionID      string                     `json:"session_id"`
	RunID          *string                    `json:"run_id"`
	BoundSessionID *string                    `json:"bound_session_id"`
	Revision       int64                      `json:"revision"`
	Cause          goalpkg.SessionOutboxCause `json:"cause"`
}

func (r *goalSessionOutboxRelay) Run(ctx context.Context) {
	if r == nil || ctx == nil {
		return
	}
	interval := r.pollInterval
	if interval <= 0 {
		interval = goalSessionOutboxPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.DeliverPending(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.runtimeLogger().WarnContext(ctx, "daemon: relay Goal session outbox", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *goalSessionOutboxRelay) DeliverPending(ctx context.Context) error {
	if r == nil || r.store == nil || r.appender == nil {
		return errors.New("daemon: Goal session outbox relay is unavailable")
	}
	projectionErr := r.deliverPendingProjections(ctx)
	cleanupErr := r.deliverPendingCleanups(ctx)
	return errors.Join(projectionErr, cleanupErr)
}

func (r *goalSessionOutboxRelay) deliverPendingProjections(ctx context.Context) error {
	events, err := r.store.ClaimGoalSessionOutbox(ctx, r.claimBatchSize())
	if err != nil {
		return fmt.Errorf("daemon: claim Goal session outbox: %w", err)
	}
	var deliveryErrs []error
	for _, event := range events {
		projection, err := goalSessionProjectionEvent(event)
		if err != nil {
			deliveryErrs = append(deliveryErrs, err)
			continue
		}
		if err := r.appender.AppendSessionEventIfAbsent(ctx, projection); err != nil {
			deliveryErrs = append(deliveryErrs, fmt.Errorf("append Goal session event %q: %w", event.EventID, err))
			continue
		}
		deliveredAt := r.currentTime()
		if deliveredAt.Before(event.CreatedAt) {
			deliveredAt = event.CreatedAt
		}
		if err := r.store.AcknowledgeGoalSessionOutbox(ctx, event.EventID, deliveredAt); err != nil {
			deliveryErrs = append(
				deliveryErrs,
				fmt.Errorf("acknowledge Goal session event %q: %w", event.EventID, err),
			)
		}
	}
	return errors.Join(deliveryErrs...)
}

func (r *goalSessionOutboxRelay) deliverPendingCleanups(ctx context.Context) error {
	if r.cleanupStore == nil || r.stopper == nil {
		return nil
	}
	obligations, err := r.cleanupStore.ClaimLoopSessionCleanup(ctx, r.claimBatchSize())
	if err != nil {
		return fmt.Errorf("daemon: claim Loop session cleanup: %w", err)
	}
	var deliveryErrs []error
	var deliveryErrsMu sync.Mutex
	var group errgroup.Group
	group.SetLimit(loopSessionCleanupConcurrency)
	for _, obligation := range obligations {
		group.Go(func() error {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loopSessionCleanupStopTimeout)
			defer cancel()
			err := r.stopper.StopWithCause(
				stopCtx,
				obligation.SessionID,
				session.CauseUserRequested,
				"Loop session cleanup: "+string(obligation.Cause),
			)
			if err != nil && !errors.Is(err, session.ErrSessionNotFound) &&
				!errors.Is(err, session.ErrSessionNotActive) {
				wrapped := fmt.Errorf("stop Loop run-owned session %q: %w", obligation.SessionID, err)
				r.logCleanupFailure(ctx, obligation, "stop", wrapped)
				deliveryErrsMu.Lock()
				deliveryErrs = append(deliveryErrs, wrapped)
				deliveryErrsMu.Unlock()
				return nil
			}
			completedAt := r.currentTime()
			if completedAt.Before(obligation.CreatedAt) {
				completedAt = obligation.CreatedAt
			}
			ackCtx, ackCancel := context.WithTimeout(context.WithoutCancel(ctx), loopSessionCleanupStopTimeout)
			defer ackCancel()
			if err := r.cleanupStore.AcknowledgeLoopSessionCleanup(
				ackCtx,
				obligation.CleanupID,
				completedAt,
			); err != nil {
				wrapped := fmt.Errorf("acknowledge Loop session cleanup %q: %w", obligation.CleanupID, err)
				r.logCleanupFailure(ctx, obligation, "acknowledge", wrapped)
				deliveryErrsMu.Lock()
				deliveryErrs = append(deliveryErrs, wrapped)
				deliveryErrsMu.Unlock()
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		deliveryErrs = append(deliveryErrs, err)
	}
	return errors.Join(deliveryErrs...)
}

func (r *goalSessionOutboxRelay) claimBatchSize() int {
	if r.batchSize > 0 {
		return r.batchSize
	}
	return goalSessionOutboxBatchSize
}

func (r *goalSessionOutboxRelay) logCleanupFailure(
	ctx context.Context,
	obligation looppkg.SessionCleanupObligation,
	phase string,
	err error,
) {
	age := max(r.currentTime().Sub(obligation.CreatedAt), 0)
	r.runtimeLogger().WarnContext(
		ctx,
		"daemon: Loop session cleanup delivery failed",
		"lane", "session-cleanup",
		"phase", phase,
		"cleanup_id", obligation.CleanupID,
		"workspace_id", obligation.WorkspaceID,
		"loop_run_id", obligation.LoopRunID,
		"session_id", obligation.SessionID,
		"source_kind", obligation.SourceKind,
		"source_id", obligation.SourceID,
		"source_epoch", obligation.SourceEpoch,
		"cause", obligation.Cause,
		"age", age,
		"error", err,
	)
}

func goalSessionProjectionEvent(event goalpkg.SessionOutboxEvent) (session.GoalEvent, error) {
	payload := goalSnapshotChangedPayload{
		SessionID: strings.TrimSpace(event.OriginSessionID), BoundSessionID: cloneGoalSessionID(event.BoundSessionID),
		Revision: event.ID, Cause: event.Cause,
	}
	if event.Cause != goalpkg.SessionOutboxCauseClear {
		runID := strings.TrimSpace(string(event.LoopRunID))
		payload.RunID = &runID
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return session.GoalEvent{}, fmt.Errorf("marshal Goal session projection: %w", err)
	}
	projection := session.GoalEvent{
		EventID: strings.TrimSpace(event.EventID), SessionID: payload.SessionID,
		SyntheticTurnID: "goal-snapshot:" + payload.SessionID, AgentName: string(SessionClassSystem),
		Type: session.EventTypeGoalSnapshotChanged, Content: content, CreatedAt: event.CreatedAt,
	}
	if err := projection.Validate(); err != nil {
		return session.GoalEvent{}, err
	}
	return projection, nil
}

func cloneGoalSessionID(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.TrimSpace(*value)
	return &cloned
}

func (r *goalSessionOutboxRelay) currentTime() time.Time {
	if r != nil && r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}

func (r *goalSessionOutboxRelay) runtimeLogger() *slog.Logger {
	if r != nil && r.logger != nil {
		return r.logger
	}
	return slog.Default()
}
