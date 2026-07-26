package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
)

const (
	goalSessionOutboxBatchSize    = 50
	goalSessionOutboxPollInterval = 100 * time.Millisecond
)

type goalSessionOutboxStore interface {
	ClaimGoalSessionOutbox(context.Context, int) ([]goalpkg.SessionOutboxEvent, error)
	AcknowledgeGoalSessionOutbox(context.Context, string, time.Time) error
}

type goalSessionCleanupStore interface {
	ClaimGoalSessionCleanup(context.Context, int) ([]goalpkg.SessionCleanupObligation, error)
	AcknowledgeGoalSessionCleanup(context.Context, string, time.Time) error
}

type goalSessionCleanupStopper interface {
	Stop(context.Context, string) error
}

type goalSessionEventAppender interface {
	AppendSessionEventIfAbsent(context.Context, session.GoalEvent) error
}

type goalSessionOutboxRelay struct {
	store        goalSessionOutboxStore
	appender     goalSessionEventAppender
	cleanupStore goalSessionCleanupStore
	stopper      goalSessionCleanupStopper
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
	obligations, err := r.cleanupStore.ClaimGoalSessionCleanup(ctx, r.claimBatchSize())
	if err != nil {
		return fmt.Errorf("daemon: claim Goal session cleanup: %w", err)
	}
	var deliveryErrs []error
	for _, obligation := range obligations {
		if err := r.stopper.Stop(ctx, obligation.SessionID); err != nil &&
			!errors.Is(err, session.ErrSessionNotFound) && !errors.Is(err, session.ErrSessionNotActive) {
			wrapped := fmt.Errorf("stop Goal run-owned session %q: %w", obligation.SessionID, err)
			r.logCleanupFailure(ctx, obligation, "stop", wrapped)
			deliveryErrs = append(deliveryErrs, wrapped)
			continue
		}
		completedAt := r.currentTime()
		if completedAt.Before(obligation.CreatedAt) {
			completedAt = obligation.CreatedAt
		}
		if err := r.cleanupStore.AcknowledgeGoalSessionCleanup(
			ctx,
			obligation.CleanupID,
			completedAt,
		); err != nil {
			wrapped := fmt.Errorf("acknowledge Goal session cleanup %q: %w", obligation.CleanupID, err)
			r.logCleanupFailure(ctx, obligation, "acknowledge", wrapped)
			deliveryErrs = append(deliveryErrs, wrapped)
		}
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
	obligation goalpkg.SessionCleanupObligation,
	phase string,
	err error,
) {
	age := max(r.currentTime().Sub(obligation.CreatedAt), 0)
	r.runtimeLogger().WarnContext(
		ctx,
		"daemon: Goal session cleanup delivery failed",
		"lane", "session-cleanup",
		"phase", phase,
		"cleanup_id", obligation.CleanupID,
		"workspace_id", obligation.WorkspaceID,
		"loop_run_id", obligation.LoopRunID,
		"session_id", obligation.SessionID,
		"binding_handle", obligation.Handle,
		"binding_epoch", obligation.BindingEpoch,
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
