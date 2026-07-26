package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticitems "github.com/compozy/agh/internal/diagnostics"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

func terminalTaskPauseError(record Task) error {
	status := record.Status.Normalize()
	item := diagnosticitems.NewItem(
		"task.pause."+diagnosticcontract.CodeTaskRunAlreadyTerminal,
		diagnosticcontract.CodeTaskRunAlreadyTerminal,
		diagnosticcontract.CategoryTask,
		"Task is already terminal",
		fmt.Sprintf("Task %s is %s and cannot be paused.", record.ID, status),
		diagnosticcontract.SeverityInfo,
		diagnosticcontract.FreshnessLive,
		diagnosticitems.WithSuggestedCommand(fmt.Sprintf("agh task inspect %s", record.ID)),
		diagnosticitems.WithEvidence(map[string]any{
			taskEvidenceIDKey: record.ID,
			"task_status":     string(status),
		}),
	)
	return diagnosticitems.NewStructuredError(item, ErrInvalidStatusTransition)
}

func detachedSchedulerDrainContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	drainWindow := timeout
	if drainWindow <= 0 {
		drainWindow = schedulerDrainPollInterval
	}
	return context.WithTimeout(context.WithoutCancel(ctx), drainWindow+schedulerDrainPollInterval)
}

func (m *Service) waitForSchedulerDrain(
	ctx context.Context,
	controlStore schedulerControlStore,
	timeout time.Duration,
	startedAt time.Time,
) (SchedulerDrainResult, error) {
	deadline := startedAt.Add(timeout)
	for {
		status, err := m.schedulerStatus(ctx, controlStore, m.now().UTC())
		if err != nil {
			return SchedulerDrainResult{}, err
		}
		if status.ActiveClaimCount == 0 {
			completedAt := m.now().UTC()
			return SchedulerDrainResult{
				Status:          status,
				Completed:       true,
				RemainingClaims: 0,
				StartedAt:       startedAt,
				CompletedAt:     completedAt,
			}, nil
		}
		if timeout == 0 || !m.now().UTC().Before(deadline) {
			completedAt := m.now().UTC()
			return SchedulerDrainResult{
				Status:          status,
				Completed:       false,
				TimedOut:        status.ActiveClaimCount > 0,
				RemainingClaims: status.ActiveClaimCount,
				StartedAt:       startedAt,
				CompletedAt:     completedAt,
			}, nil
		}
		wait := schedulerDrainPollInterval
		if remaining := deadline.Sub(m.now().UTC()); remaining > 0 && remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return SchedulerDrainResult{}, fmt.Errorf("task: drain scheduler: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (m *Service) requireTaskPauseStore() (taskPauseStore, error) {
	pauseStore, ok := m.store.(taskPauseStore)
	if !ok {
		return nil, errors.New("task: store does not support task pause controls")
	}
	return pauseStore, nil
}

func (m *Service) requireSchedulerControlStore() (schedulerControlStore, error) {
	controlStore, ok := m.store.(schedulerControlStore)
	if !ok {
		return nil, errors.New("task: store does not support scheduler controls")
	}
	return controlStore, nil
}

func (m *Service) recordSchedulerEventBestEffort(
	ctx context.Context,
	eventType string,
	actor ActorContext,
	payload schedulerEventPayload,
) {
	writer, ok := m.store.(eventSummaryStore)
	if !ok {
		return
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return
	}
	summary := store.EventSummary{
		Type:      eventType,
		Outcome:   string(eventspkg.OutcomeFor(eventType)),
		Content:   content,
		Summary:   schedulerEventSummary(eventType, payload),
		Timestamp: m.now().UTC(),
		EventCorrelation: store.EventCorrelation{
			ActorKind:       string(actor.Actor.Kind.Normalize()),
			ActorID:         actor.Actor.Ref,
			SchedulerReason: payload.Reason,
		},
	}
	eventCtx := context.Background()
	if ctx != nil {
		eventCtx = context.WithoutCancel(ctx)
	}
	if err := writer.WriteEventSummary(eventCtx, summary); err != nil {
		return
	}
}

func schedulerEventSummary(eventType string, payload schedulerEventPayload) string {
	switch eventType {
	case eventspkg.SchedulerPaused:
		return "scheduler paused"
	case eventspkg.SchedulerResumed:
		return "scheduler resumed"
	case eventspkg.SchedulerDrainStarted:
		return "scheduler drain started"
	case eventspkg.SchedulerDrainCompleted:
		if payload.TimedOut {
			return fmt.Sprintf("scheduler drain timed out with %d active claims", payload.RemainingClaims)
		}
		return "scheduler drain completed"
	default:
		return strings.TrimSpace(eventType)
	}
}
