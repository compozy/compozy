package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) permissionEventHandler(target *Session) func(acp.AgentEvent) {
	if m.attentionStore == nil {
		return nil
	}
	return func(event acp.AgentEvent) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(m.fallbackLifecycleContext()), defaultLifecycleTimeout)
		defer cancel()
		if _, err := m.applyAttentionAgentEvent(ctx, target, event); err != nil {
			m.sessionLogger(target).ErrorContext(ctx, "session: persist permission lifecycle failed",
				"request_id", event.RequestIDValue(), "error", err)
		}
	}
}

func terminalInteractionTransition(
	interaction store.PendingInteraction,
	at time.Time,
	reason string,
) store.PendingInteractionTransition {
	status := store.PendingInteractionStatusCanceled
	if interaction.ExpiresAt != nil && !at.Before(*interaction.ExpiresAt) {
		status, reason = store.PendingInteractionStatusTimedOut, "expired"
	}
	return store.PendingInteractionTransition{
		InteractionID: interaction.InteractionID, Status: status, Resolution: reason,
		ResolvedBy: sessionSystemActorKind, At: at,
	}
}

func (m *Manager) cancelSessionInteractions(ctx context.Context, sessionID, reason string) error {
	target, active := m.Get(sessionID)
	if active {
		target.interactionLifecycleMu.Lock()
	}
	commits, err := m.commitCanceledSessionInteractions(ctx, sessionID, reason)
	if active {
		target.interactionLifecycleMu.Unlock()
	}
	for _, commit := range commits {
		m.publishAttentionCommit(ctx, sessionID, commit)
	}
	return err
}

func (m *Manager) commitCanceledSessionInteractions(
	ctx context.Context,
	sessionID, reason string,
) ([]store.SessionAttentionCommit, error) {
	if m.attentionStore == nil {
		return nil, nil
	}
	interactions, err := m.attentionStore.ListPendingInteractions(ctx, sessionID, nil)
	if err != nil {
		return nil, err
	}
	var commits []store.SessionAttentionCommit
	for _, interaction := range interactions {
		commit, err := m.commitTransitionPendingInteraction(
			ctx,
			terminalInteractionTransition(interaction, m.now().UTC(), reason),
		)
		if err != nil {
			return commits, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func (m *Manager) applyPendingPermissionEvent(
	ctx context.Context,
	target *Session,
	event acp.AgentEvent,
	requestID string,
) (store.SessionAttentionCommit, error) {
	info := target.Info()
	if info.State == StateStopping || info.State == StateStopped {
		return store.SessionAttentionCommit{}, nil
	}
	if m.attentionStore != nil {
		interactions, err := m.PendingInteractions(ctx, target.ID, []string{
			store.PendingInteractionStatusPending,
			store.PendingInteractionStatusOrphaned,
			store.PendingInteractionStatusResolved,
			store.PendingInteractionStatusTimedOut,
			store.PendingInteractionStatusCanceled,
		})
		if err != nil {
			return store.SessionAttentionCommit{}, err
		}
		for _, existing := range interactions {
			if existing.Kind == store.PendingInteractionKindPermission && existing.ProviderRequestID == requestID &&
				existing.TurnID == event.TurnID &&
				(pendingInteractionIsActionable(existing.Status) || existing.CreatedAt.Equal(event.Timestamp)) {
				return store.SessionAttentionCommit{}, nil
			}
		}
	}
	payload, title := permissionInteractionPayload(event)
	commit, err := m.commitCreatePendingInteraction(ctx, store.PendingInteractionCreate{
		SessionID:         target.ID,
		Kind:              store.PendingInteractionKindPermission,
		ProviderRequestID: requestID,
		TurnID:            event.TurnID,
		Title:             title,
		Payload:           payload,
		CreatedAt:         event.Timestamp,
		ExpiresAt:         event.ExpiresAtValue(),
	})
	if errors.Is(err, store.ErrPendingInteractionConflict) {
		return store.SessionAttentionCommit{}, m.refreshPendingInteractions(ctx, target.ID)
	}
	return commit, err
}

func permissionResolutionError(interaction store.PendingInteraction, at time.Time) error {
	if interaction.Status == store.PendingInteractionStatusTimedOut ||
		(pendingInteractionIsActionable(interaction.Status) && interaction.ExpiresAt != nil &&
			!at.Before(*interaction.ExpiresAt)) {
		return ErrPendingPermissionExpired
	}
	if interaction.Status == store.PendingInteractionStatusCanceled {
		return ErrPendingPermissionCanceled
	}
	return nil
}
