package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	redactpkg "github.com/compozy/compozy/internal/redact"
)

// DefaultTaskStarvationAge is the queued age past which a claimable run is treated as
// starved by the scheduler convergence backstop and the scheduler status surface.
const DefaultTaskStarvationAge = 2 * time.Minute

// WithStarvationAge overrides the queued-age threshold used by scheduler-status starvation
// counts so the status surface agrees with the scheduler's own threshold.
func WithStarvationAge(age time.Duration) Option {
	return func(opts *managerOptions) {
		opts.starvationAge = age
	}
}

type runStarvedPayload struct {
	QueuedAt                     time.Time           `json:"queued_at,omitzero"`
	QueuedAgeMS                  int64               `json:"queued_age_ms"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation"`
}

// RunNeedsAttentionEventPayload is the canonical audit payload for a run that requires intervention.
type RunNeedsAttentionEventPayload struct {
	PreviousStatus               RunStatus           `json:"previous_status"`
	Status                       RunStatus           `json:"status"`
	SessionID                    string              `json:"session_id,omitempty"`
	Diagnostic                   string              `json:"diagnostic,omitempty"`
	QueuedAt                     time.Time           `json:"queued_at,omitzero"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation"`
}

// RecordRunStarved emits the canonical task.run_starved event for one starved queued run.
// The scheduler stays observational; run-event authority remains in the task service.
func (m *Service) RecordRunStarved(
	ctx context.Context,
	runID string,
	queuedAt time.Time,
	age time.Duration,
	actor ActorContext,
) error {
	run, err := m.store.GetTaskRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	return m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunStarved, actor, runStarvedPayload{
		QueuedAt:                     queuedAt,
		QueuedAgeMS:                  age.Milliseconds(),
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
	})
}

// MarkRunNeedsAttention transitions a nonterminal run to needs_attention via a CAS store mutation
// and records the canonical event. It is idempotent: a run already in needs_attention is
// returned unchanged. The diagnostic must never embed a raw claim token.
func (m *Service) MarkRunNeedsAttention(
	ctx context.Context,
	runID string,
	diagnostic string,
	actor ActorContext,
) (Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return Run{}, err
	}
	diagnostic = strings.TrimSpace(diagnostic)
	if redactpkg.ContainsRawClaimToken(diagnostic) {
		return Run{}, fmt.Errorf("task: needs_attention diagnostic must not embed a claim token")
	}
	if err := ValidateDiagnosticSize(diagnostic, "task_run.needs_attention.diagnostic"); err != nil {
		return Run{}, err
	}

	run, _, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return Run{}, err
	}
	previousStatus := run.Status.Normalize()
	if previousStatus == TaskRunStatusNeedsAttention {
		return run, nil
	}
	if !runStatusAllowsNeedsAttention(previousStatus) {
		return Run{}, fmt.Errorf(
			"%w: run %q is %s; only nonterminal runs can be marked needs_attention",
			ErrInvalidStatusTransition,
			run.ID,
			previousStatus,
		)
	}
	payload := RunNeedsAttentionEventPayload{
		PreviousStatus:               previousStatus,
		Status:                       TaskRunStatusNeedsAttention,
		SessionID:                    run.SessionID,
		Diagnostic:                   diagnostic,
		QueuedAt:                     run.QueuedAt,
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
	}
	if err := m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunNeedsAttention,
		actor,
		payload,
	); err != nil {
		return Run{}, err
	}
	commandID, err := m.newID("terminal-command")
	if err != nil {
		return Run{}, fmt.Errorf("task: generate terminal command id: %w", err)
	}
	eventIDs, err := m.reserveTaskEventIDs(1)
	if err != nil {
		return Run{}, err
	}
	command, err := newNeedsAttentionTerminalRunCommand(
		commandID,
		eventIDs,
		run,
		diagnostic,
		actor,
		m.now().UTC(),
	)
	if err != nil {
		return Run{}, err
	}
	settlement, err := m.executeTerminalRunCommand(ctx, command)
	if err != nil {
		return Run{}, err
	}
	if err := m.publishTerminalRunCommandSettlement(ctx, &settlement); err != nil {
		return settlement.run, err
	}
	return settlement.run, nil
}

func runStatusAllowsNeedsAttention(status RunStatus) bool {
	switch status.Normalize() {
	case TaskRunStatusQueued,
		TaskRunStatusClaimed,
		TaskRunStatusStarting,
		TaskRunStatusRunning:
		return true
	default:
		return false
	}
}
