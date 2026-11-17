package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
)

type sessionWakeSessionManager interface {
	Status(context.Context, string) (*session.Info, error)
	PromptSynthetic(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

type sessionWakeSessionResolver func() sessionWakeSessionManager

type sessionWakeInstanceReader interface {
	Get(context.Context, string) (agentinstance.Instance, error)
}

type sessionWakeBridge struct {
	drainCtx  context.Context
	resolve   sessionWakeSessionResolver
	logger    *slog.Logger
	workers   *ownedWorkerGroup
	instances sessionWakeInstanceReader
}

var _ session.SpawnWakeNotifier = (*sessionWakeBridge)(nil)

func (bridge *sessionWakeBridge) configureInstances(state *bootState) {
	if provider, ok := state.registry.(instanceRepositoryProvider); ok {
		bridge.instances = provider.Instances()
	}
}

func newSessionWakeBridge(
	ctx context.Context,
	resolve sessionWakeSessionResolver,
	logger *slog.Logger,
) (*sessionWakeBridge, error) {
	if ctx == nil {
		return nil, errors.New("daemon: session wake bridge context is required")
	}
	if resolve == nil {
		return nil, errors.New("daemon: session wake bridge resolver is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	drainCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	return &sessionWakeBridge{
		drainCtx: drainCtx,
		resolve:  resolve,
		logger:   logger,
		workers:  newOwnedWorkerGroup(cancel),
	}, nil
}

func (bridge *sessionWakeBridge) WakeSpawnCreator(
	ctx context.Context,
	creatorSessionID string,
	event session.SpawnWakeEvent,
) error {
	if bridge == nil || bridge.resolve == nil || bridge.workers == nil {
		return errors.New("daemon: session wake bridge is not initialized")
	}
	if ctx == nil {
		return errors.New("daemon: session wake context is required")
	}
	creatorID := strings.TrimSpace(creatorSessionID)
	if creatorID == "" {
		return session.ErrSpawnWakeSessionNotLive
	}
	normalized := event.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	complete, admitted := bridge.workers.Begin()
	if !admitted {
		return fmt.Errorf("%w: daemon session wake bridge is stopping", session.ErrSpawnWakeRetryable)
	}
	deliveryCtx := context.WithoutCancel(ctx)
	sessions := bridge.resolve()
	if sessions == nil {
		complete()
		return session.ErrSpawnWakeSessionNotLive
	}
	originalCreatorID := creatorID
	creatorID, creator, err := bridge.creatorForWake(deliveryCtx, sessions, creatorID, normalized.Automatic)
	if err != nil || !sessionWakeCreatorLive(creator, normalized.Automatic) {
		complete()
		if err != nil && !errors.Is(err, session.ErrSessionNotFound) &&
			!errors.Is(err, session.ErrSessionNotActive) {
			return fmt.Errorf("daemon: read session wake creator %q: %w", creatorID, err)
		}
		return fmt.Errorf("%w: %s", session.ErrSpawnWakeSessionNotLive, creatorID)
	}
	// The in-memory attention cache does not survive restart or eviction. Reuse
	// the durable admission identity so replaying the same revision costs no turn.
	alreadyAdmitted, ledgerErr := childWakeAdmitted(
		deliveryCtx,
		sessions,
		creatorID,
		normalized.WakeEventID,
		originalCreatorID,
	)
	if ledgerErr != nil {
		complete()
		// No dispatch was attempted. A retry rechecks the durable identity before
		// enqueueing, including when this failed read hid an earlier admission.
		return fmt.Errorf("%w: %w", session.ErrSpawnWakeRetryable, ledgerErr)
	}
	if alreadyAdmitted {
		complete()
		return nil
	}

	events, err := sessions.PromptSynthetic(deliveryCtx, creatorID, session.SyntheticPromptOpts{
		MessageID: normalized.WakeEventID,
		Message:   sessionWakePromptMessage(normalized),
		Metadata: acp.PromptSyntheticMeta{
			Kind:           sessionWakeKind(normalized),
			ChildSessionID: normalized.ChildSessionID,
			ChildAgentName: normalized.ChildAgentName,
			Badge:          string(normalized.Badge),
			Reason:         string(normalized.Reason),
			Summary:        normalized.Detail,
			WakeEventID:    normalized.WakeEventID,
		},
		SkipIfBusy:              false,
		InterruptIfAgentWaiting: false,
	})
	if err != nil {
		complete()
		if errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, session.ErrSessionNotActive) {
			return fmt.Errorf("%w: %s", session.ErrSpawnWakeSessionNotLive, creatorID)
		}
		return fmt.Errorf("daemon: deliver session wake to %q: %w", creatorID, err)
	}
	bridge.drainWakeEvents(creatorID, normalized, events, complete)
	return nil
}

func sessionWakeCreatorLive(creator *session.Info, automatic bool) bool {
	return creator != nil && creator.State == session.StateActive && (!automatic || creator.Later == nil)
}

func (bridge *sessionWakeBridge) creatorForWake(
	ctx context.Context, sessions sessionWakeSessionManager, id string, automatic bool,
) (string, *session.Info, error) {
	creator, err := sessions.Status(ctx, id)
	if err != nil || creator == nil || !automatic || creator.InstanceID == "" || bridge.instances == nil {
		return id, creator, err
	}
	instance, err := bridge.instances.Get(ctx, creator.InstanceID)
	if err != nil {
		return id, nil, err
	}
	if instance.ArchivedAt != nil || instance.ProfileID != creator.ProfileID ||
		instance.WorkspaceID != creator.WorkspaceID || instance.MainSessionID == "" {
		return id, nil, session.ErrSpawnWakeSessionNotLive
	}
	if instance.MainSessionID == id {
		return id, creator, nil
	}
	current, err := sessions.Status(ctx, instance.MainSessionID)
	if err == nil && current != nil && (current.InstanceID != creator.InstanceID ||
		current.WorkspaceID != creator.WorkspaceID || current.ProfileID != creator.ProfileID ||
		current.ArchivedAt != nil || current.Later != nil) {
		return id, nil, session.ErrSpawnWakeSessionNotLive
	}
	return instance.MainSessionID, current, err
}

func childWakeAdmitted(
	ctx context.Context,
	sessions sessionWakeSessionManager,
	creatorID, wakeID, originalCreatorID string,
) (bool, error) {
	ledger, ok := sessions.(monitorWakeLedger)
	if !ok {
		return false, nil
	}
	admitted, err := ledger.SyntheticWakeAdmitted(ctx, creatorID, wakeID)
	if err != nil {
		return false, fmt.Errorf("daemon: read child wake admission for %q: %w", creatorID, err)
	}
	if !admitted && originalCreatorID != creatorID {
		return ledger.SyntheticWakeAdmitted(ctx, originalCreatorID, wakeID)
	}
	return admitted, nil
}

func sessionWakePromptMessage(event session.SpawnWakeEvent) string {
	child := event.ChildSessionID
	if event.ChildAgentName != "" {
		child += " (" + event.ChildAgentName + ")"
	}
	lines := []string{
		"Session wake: child session " + child + " reported " + string(event.Badge) + ".",
		"Reason: " + string(event.Reason) + ".",
		sessionWakeAction(event),
	}
	if event.EndedEarly {
		// The answer and detail are already redacted and bounded; sanitizing the whole
		// message would cut the answer to the short wake bound, so only the header is.
		lines[0] = session.SanitizeSpawnWakeText(lines[0])
		lines = append(lines, "Detail: "+event.Detail)
		if event.FinalAnswer != "" {
			lines = append(lines, "Final answer:\n---\n"+event.FinalAnswer)
		}
		return strings.Join(lines, "\n")
	}
	if event.Detail != "" {
		lines = append(lines, "Detail: "+event.Detail)
	}
	return session.SanitizeSpawnWakeText(strings.Join(lines, "\n"))
}

func sessionWakeKind(event session.SpawnWakeEvent) string {
	if event.Automatic {
		return session.SpawnWakeKindChildSettled
	}
	if event.EndedEarly {
		return session.SpawnWakeKindEndedEarly
	}
	return ""
}

func sessionWakeAction(event session.SpawnWakeEvent) string {
	if event.EndedEarly {
		if event.FinalAnswer == "" {
			return "The final answer is unavailable. Inspect the child with compozy__session_history or " +
				"compozy__session_events before deciding whether to prompt it again or stop it with compozy__session_stop."
		}
		return "Review its answer and decide whether to prompt it again or stop it with compozy__session_stop."
	}
	switch event.Badge {
	case session.BadgeWaitingForInput:
		return "Use compozy__session_clarify_answer or compozy__session_events to act."
	case session.BadgeWaitingForAuth:
		return "Use compozy__session_approve or compozy__session_events to act."
	case session.BadgeDone:
		return "Read its answer with compozy__session_history limit 1; notify_on_complete returns answers inline."
	default:
		return "Use compozy__session_events to inspect the child."
	}
}

func (bridge *sessionWakeBridge) drainWakeEvents(
	creatorSessionID string,
	event session.SpawnWakeEvent,
	events <-chan acp.AgentEvent,
	complete func(),
) {
	if events == nil {
		complete()
		return
	}
	drainCtx := bridge.drainCtx
	go func() {
		defer complete()
		for {
			select {
			case <-drainCtx.Done():
				return
			case agentEvent, ok := <-events:
				if !ok {
					return
				}
				if agentEvent.Type == acp.EventTypeError {
					bridge.logger.Warn(
						"session.spawn_wake.agent_error",
						"creator_session_id", strings.TrimSpace(creatorSessionID),
						"child_session_id", event.ChildSessionID,
						"wake_event_id", event.WakeEventID,
						"reason", event.Reason,
					)
				}
			}
		}
	}()
}

func (bridge *sessionWakeBridge) shutdown(ctx context.Context) error {
	if bridge == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: session wake bridge shutdown context is required")
	}
	done := bridge.workers.Stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown session wake bridge: %w", ctx.Err())
	}
}
