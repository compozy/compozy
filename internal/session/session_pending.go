package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) createPendingInteraction(
	ctx context.Context,
	req store.PendingInteractionCreate,
) (store.SessionAttentionCommit, error) {
	if m.attentionStore == nil {
		return store.SessionAttentionCommit{}, errors.New("session: attention store is required")
	}
	if strings.TrimSpace(req.InteractionID) == "" {
		interactionID, err := m.newInteractionID()
		if err != nil {
			return store.SessionAttentionCommit{}, err
		}
		req.InteractionID = interactionID
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = m.now().UTC()
	}
	commit, err := m.attentionStore.CreatePendingInteraction(ctx, req)
	if err != nil {
		return store.SessionAttentionCommit{}, fmt.Errorf("session: create pending interaction: %w", err)
	}
	m.applyAttentionProjection(req.SessionID, commit.After)
	m.applyPendingInteractionCommit(req.SessionID, commit.Interaction)
	m.publishAttentionCommit(ctx, req.SessionID, commit)
	return commit, nil
}

func (m *Manager) transitionPendingInteraction(
	ctx context.Context,
	req store.PendingInteractionTransition,
) (store.SessionAttentionCommit, error) {
	if m.attentionStore == nil {
		return store.SessionAttentionCommit{}, errors.New("session: attention store is required")
	}
	if req.At.IsZero() {
		req.At = m.now().UTC()
	}
	commit, err := m.attentionStore.TransitionPendingInteraction(ctx, req)
	if err != nil {
		return store.SessionAttentionCommit{}, fmt.Errorf("session: transition pending interaction: %w", err)
	}
	if commit.Interaction == nil {
		return commit, nil
	}
	sessionID := commit.Interaction.SessionID
	m.applyAttentionProjection(sessionID, commit.After)
	m.applyPendingInteractionCommit(sessionID, commit.Interaction)
	m.publishAttentionCommit(ctx, sessionID, commit)
	return commit, nil
}

// PendingInteractions lists the sanitized canonical records for one session.
func (m *Manager) PendingInteractions(
	ctx context.Context,
	sessionID string,
	statuses []string,
) ([]store.PendingInteraction, error) {
	if m == nil || m.attentionStore == nil {
		return nil, errors.New("session: attention store is required")
	}
	if ctx == nil {
		return nil, errors.New("session: pending interactions context is required")
	}
	interactions, err := m.attentionStore.ListPendingInteractions(ctx, strings.TrimSpace(sessionID), statuses)
	if err != nil {
		return nil, fmt.Errorf("session: list pending interactions: %w", err)
	}
	return clonePendingInteractions(interactions), nil
}

// RecoverPendingInteractions marks requests from a dead turn as restart-orphaned.
func (m *Manager) RecoverPendingInteractions(ctx context.Context, sessionID string) (int, error) {
	if m == nil || m.attentionStore == nil {
		return 0, nil
	}
	if ctx == nil {
		return 0, errors.New("session: pending interaction recovery context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return 0, errors.New("session: pending interaction recovery session id is required")
	}
	if _, active := m.Get(target); active {
		return 0, nil
	}
	interactions, err := m.attentionStore.ListPendingInteractions(
		ctx,
		target,
		[]string{store.PendingInteractionStatusPending},
	)
	if err != nil {
		return 0, fmt.Errorf("session: list pending interactions during restart recovery: %w", err)
	}
	orphaned := 0
	for _, interaction := range interactions {
		commit, err := m.transitionPendingInteraction(ctx, store.PendingInteractionTransition{
			InteractionID: interaction.InteractionID,
			Status:        store.PendingInteractionStatusOrphaned,
			At:            m.now().UTC(),
		})
		if err != nil {
			return orphaned, fmt.Errorf(
				"session: orphan interaction %q during restart recovery: %w",
				interaction.InteractionID,
				err,
			)
		}
		if commit.Interaction != nil && commit.Interaction.Status == store.PendingInteractionStatusOrphaned {
			orphaned++
		}
	}
	return orphaned, nil
}

func (m *Manager) hydrateSessionAttention(ctx context.Context, target *Session) error {
	if m.attentionStore == nil || target == nil {
		return nil
	}
	projection, err := m.attentionStore.GetSessionAttention(ctx, target.ID)
	if err != nil {
		return fmt.Errorf("session: hydrate attention projection for %q: %w", target.ID, err)
	}
	interactions, err := m.attentionStore.ListPendingInteractions(ctx, target.ID, nil)
	if err != nil {
		return fmt.Errorf("session: hydrate pending interactions for %q: %w", target.ID, err)
	}
	target.mu.Lock()
	applyAttentionProjectionLocked(target, projection)
	target.PendingInteractions = clonePendingInteractions(interactions)
	target.mu.Unlock()
	return nil
}

func (m *Manager) hydrateSessionInfoAttention(ctx context.Context, info *Info) error {
	if m.attentionStore == nil || info == nil {
		return nil
	}
	projection, err := m.attentionStore.GetSessionAttention(ctx, info.ID)
	if err != nil {
		return fmt.Errorf("session: hydrate attention projection for %q: %w", info.ID, err)
	}
	applyAttentionToInfo(info, projection)
	return nil
}

func (m *Manager) refreshPendingInteractions(ctx context.Context, sessionID string) error {
	active, ok := m.Get(sessionID)
	if !ok {
		return nil
	}
	interactions, err := m.attentionStore.ListPendingInteractions(ctx, sessionID, nil)
	if err != nil {
		return fmt.Errorf("session: refresh pending interactions for %q: %w", sessionID, err)
	}
	active.mu.Lock()
	active.PendingInteractions = clonePendingInteractions(interactions)
	active.mu.Unlock()
	return nil
}

func (m *Manager) applyAttentionProjection(sessionID string, projection store.SessionAttention) {
	active, ok := m.Get(sessionID)
	if !ok {
		return
	}
	active.mu.Lock()
	applyAttentionProjectionLocked(active, projection)
	active.mu.Unlock()
}

func (m *Manager) applyPendingInteractionCommit(
	sessionID string,
	committed *store.PendingInteraction,
) {
	if committed == nil {
		return
	}
	active, ok := m.Get(sessionID)
	if !ok {
		return
	}
	interaction := store.SanitizePendingInteraction(*committed)
	active.mu.Lock()
	next := make([]store.PendingInteraction, 0, len(active.PendingInteractions)+1)
	replaced := false
	for _, existing := range active.PendingInteractions {
		if existing.InteractionID == interaction.InteractionID {
			replaced = true
			if pendingInteractionIsActionable(interaction.Status) {
				next = append(next, interaction)
			}
			continue
		}
		if pendingInteractionIsActionable(existing.Status) {
			next = append(next, existing)
		}
	}
	if !replaced && pendingInteractionIsActionable(interaction.Status) {
		next = append(next, interaction)
	}
	sort.Slice(next, func(left, right int) bool {
		if next[left].CreatedAt.Equal(next[right].CreatedAt) {
			return next[left].InteractionID < next[right].InteractionID
		}
		return next[left].CreatedAt.Before(next[right].CreatedAt)
	})
	active.PendingInteractions = clonePendingInteractions(next)
	active.mu.Unlock()
}

func pendingInteractionIsActionable(status string) bool {
	switch status {
	case store.PendingInteractionStatusPending, store.PendingInteractionStatusOrphaned:
		return true
	default:
		return false
	}
}

func applyAttentionProjectionLocked(target *Session, projection store.SessionAttention) {
	target.PendingPermissionCount = projection.PendingPermissionCount
	target.PendingClarifyCount = projection.PendingClarifyCount
	target.AttentionRevision = projection.AttentionRevision
	target.LastSettledRevision = projection.LastSettledRevision
	target.LastSeenRevision = projection.LastSeenRevision
	target.LastSeenAt = cloneTimePointer(projection.LastSeenAt)
	target.AttentionChangedAt = cloneTimePointer(projection.AttentionChangedAt)
}

func clonePendingInteractions(values []store.PendingInteraction) []store.PendingInteraction {
	if len(values) == 0 {
		return []store.PendingInteraction{}
	}
	result := make([]store.PendingInteraction, len(values))
	for index := range values {
		result[index] = store.SanitizePendingInteraction(values[index])
		result[index].Payload.Choices = append([]string(nil), result[index].Payload.Choices...)
		result[index].Payload.Decisions = append([]string(nil), result[index].Payload.Decisions...)
		result[index].ResolvedAt = cloneTimePointer(result[index].ResolvedAt)
	}
	return result
}
