package task

import (
	"context"
	"encoding/json"

	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	defaultSchedulerDrainTimeout = 60 * time.Second
	schedulerDrainPollInterval   = 500 * time.Millisecond
)

// DefaultSchedulerDrainTimeout keeps API transports from duplicating scheduler policy.
func DefaultSchedulerDrainTimeout() time.Duration {
	return defaultSchedulerDrainTimeout
}

type taskPauseStore interface {
	PauseTask(context.Context, PauseMutation) (Task, error)
	ResumeTask(context.Context, ResumeMutation) (Task, error)
}

type schedulerControlStore interface {
	GetSchedulerPause(context.Context) (SchedulerPauseState, error)
	SetSchedulerPaused(context.Context, string, string) (SchedulerPauseState, error)
	SetSchedulerResumed(context.Context) (SchedulerPauseState, error)
	CountActiveTaskRunClaims(context.Context) (int, error)
	CountQueuedTaskRuns(context.Context, bool) (int, error)
	CountPausedTasks(context.Context) (int, error)
	CountEscalatingQueuedTaskRuns(context.Context) (int, error)
	CountNeedsAttentionTaskRuns(context.Context) (int, error)
	SchedulerBacklog(context.Context, SchedulerBacklogQuery) (SchedulerBacklog, error)
}

type eventSummaryStore interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type taskPausePayload struct {
	Manual         bool            `json:"manual"`
	ActorKind      ActorKind       `json:"actor_kind"`
	ActorID        string          `json:"actor_id,omitempty"`
	PreviousPaused bool            `json:"previous_paused"`
	Paused         bool            `json:"paused"`
	Reason         string          `json:"reason,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type taskResumePayload struct {
	Manual         bool            `json:"manual"`
	ActorKind      ActorKind       `json:"actor_kind"`
	ActorID        string          `json:"actor_id,omitempty"`
	PreviousPaused bool            `json:"previous_paused"`
	Paused         bool            `json:"paused"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type schedulerEventPayload struct {
	Manual          bool      `json:"manual"`
	ActorKind       ActorKind `json:"actor_kind"`
	ActorID         string    `json:"actor_id,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	PreviousPaused  bool      `json:"previous_paused"`
	Paused          bool      `json:"paused"`
	ActiveClaims    int       `json:"active_claims,omitempty"`
	RemainingClaims int       `json:"remaining_claims,omitempty"`
	TimedOut        bool      `json:"timed_out,omitempty"`
	StartedAt       time.Time `json:"started_at,omitzero"`
	CompletedAt     time.Time `json:"completed_at,omitzero"`
}

// PauseTask marks one task as paused for future scheduler and claim eligibility.
func (m *Service) PauseTask(
	ctx context.Context,
	id string,
	req PauseTaskRequest,
	actor ActorContext,
) (*Task, error) {
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizePauseTaskRequest(req)
	if err != nil {
		return nil, err
	}
	pauseStore, err := m.requireTaskPauseStore()
	if err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(id)
	previous, err := m.loadAuthorizedTask(ctx, m.store, taskID, actor)
	if err != nil {
		return nil, err
	}
	if isTerminalTaskStatus(previous.Status) {
		return nil, terminalTaskPauseError(previous)
	}
	if err := m.requireForceRunRate(actor, previous.ID); err != nil {
		return nil, err
	}
	updated, err := pauseStore.PauseTask(ctx, PauseMutation{
		TaskID:   previous.ID,
		Actor:    actorLabel(actor),
		Reason:   normalized.Reason,
		PausedAt: m.now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, updated.ID, "", taskEventPaused, actor, taskPausePayload{
		Manual:         true,
		ActorKind:      actor.Actor.Kind.Normalize(),
		ActorID:        actor.Actor.Ref,
		PreviousPaused: previous.Paused,
		Paused:         updated.Paused,
		Reason:         normalized.Reason,
		Metadata:       cloneRawJSON(normalized.Metadata),
	}); err != nil {
		return nil, err
	}
	return &updated, nil
}

// ResumeTask clears one task pause for future scheduler and claim eligibility.
func (m *Service) ResumeTask(
	ctx context.Context,
	id string,
	req ResumeTaskRequest,
	actor ActorContext,
) (*Task, error) {
	if err := m.requireForceRunAuthority(actor); err != nil {
		return nil, err
	}
	normalized, err := normalizeResumeTaskRequest(req)
	if err != nil {
		return nil, err
	}
	pauseStore, err := m.requireTaskPauseStore()
	if err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(id)
	previous, err := m.loadAuthorizedTask(ctx, m.store, taskID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.requireForceRunRate(actor, previous.ID); err != nil {
		return nil, err
	}
	updated, err := pauseStore.ResumeTask(ctx, ResumeMutation{TaskID: previous.ID, ResumedAt: m.now().UTC()})
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, updated.ID, "", taskEventResumed, actor, taskResumePayload{
		Manual:         true,
		ActorKind:      actor.Actor.Kind.Normalize(),
		ActorID:        actor.Actor.Ref,
		PreviousPaused: previous.Paused,
		Paused:         updated.Paused,
		Metadata:       cloneRawJSON(normalized.Metadata),
	}); err != nil {
		return nil, err
	}
	return &updated, nil
}

// SchedulerStatus returns scheduler-wide pause state and live queue pressure.
func (m *Service) SchedulerStatus(ctx context.Context, actor ActorContext) (SchedulerStatus, error) {
	if err := requireReadAuthority(actor); err != nil {
		return SchedulerStatus{}, err
	}
	controlStore, err := m.requireSchedulerControlStore()
	if err != nil {
		return SchedulerStatus{}, err
	}
	return m.schedulerStatus(ctx, controlStore, m.now().UTC())
}
