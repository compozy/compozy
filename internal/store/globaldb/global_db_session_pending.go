package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// CreatePendingInteraction persists the canonical record and count projection together.
func (g *SessionRepo) CreatePendingInteraction(
	ctx context.Context,
	req store.PendingInteractionCreate,
) (commit store.SessionAttentionCommit, err error) {
	if err := g.checkReady(ctx, "create pending interaction"); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	payloadJSON, err := json.Marshal(normalized.Payload)
	if err != nil {
		return store.SessionAttentionCommit{}, fmt.Errorf("store: encode pending interaction payload: %w", err)
	}
	err = g.withImmediateTransaction(ctx, "create pending interaction", func(exec globalSQLExecutor) error {
		before, readErr := getSessionAttention(ctx, exec, normalized.SessionID)
		if readErr != nil {
			return readErr
		}
		// dynamic-sql: this is the canonical side-table insert paired with its projection update below.
		_, insertErr := exec.ExecContext(ctx, `INSERT INTO session_pending_interactions (
interaction_id, session_id, kind, provider_request_id, turn_id, title, payload_json,
status, created_at, resolution, resolved_by
) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, '', '')`,
			normalized.InteractionID,
			normalized.SessionID,
			normalized.Kind,
			normalized.ProviderRequestID,
			normalized.TurnID,
			normalized.Title,
			string(payloadJSON),
			store.FormatTimestamp(normalized.CreatedAt),
		)
		if insertErr != nil {
			if isSQLiteUniqueConstraint(insertErr) || isSQLitePrimaryKeyConstraint(insertErr) {
				return fmt.Errorf("%w: %s", store.ErrPendingInteractionConflict, normalized.ProviderRequestID)
			}
			return fmt.Errorf("store: insert pending interaction: %w", insertErr)
		}
		if updateErr := bumpPendingInteractionProjection(
			ctx,
			exec,
			normalized.SessionID,
			normalized.Kind,
			1,
			normalized.CreatedAt,
		); updateErr != nil {
			return updateErr
		}
		after, readErr := getSessionAttention(ctx, exec, normalized.SessionID)
		if readErr != nil {
			return readErr
		}
		interaction := pendingInteractionFromCreate(normalized)
		commit = store.SessionAttentionCommit{
			Before: before, After: after, Interaction: &interaction,
			Outcome: store.PendingInteractionStatusPending,
		}
		return nil
	})
	return commit, err
}

// TransitionPendingInteraction moves one canonical record and updates its projection atomically.
func (g *SessionRepo) TransitionPendingInteraction(
	ctx context.Context,
	req store.PendingInteractionTransition,
) (commit store.SessionAttentionCommit, err error) {
	if err := g.checkReady(ctx, "transition pending interaction"); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	err = g.withImmediateTransaction(ctx, "transition pending interaction", func(exec globalSQLExecutor) error {
		transitioned, transitionErr := transitionPendingInteraction(ctx, exec, normalized)
		if transitionErr != nil {
			return transitionErr
		}
		commit = transitioned
		return nil
	})
	return commit, err
}

func transitionPendingInteraction(
	ctx context.Context,
	exec globalSQLExecutor,
	normalized store.PendingInteractionTransition,
) (store.SessionAttentionCommit, error) {
	interaction, err := getPendingInteraction(ctx, exec, normalized.InteractionID)
	if err != nil {
		return store.SessionAttentionCommit{}, err
	}
	before, err := getSessionAttention(ctx, exec, interaction.SessionID)
	if err != nil {
		return store.SessionAttentionCommit{}, err
	}
	if interaction.Status != store.PendingInteractionStatusPending &&
		interaction.Status != store.PendingInteractionStatusOrphaned {
		return store.SessionAttentionCommit{
			Before: before, After: before, Interaction: &interaction,
			Outcome: store.PendingInteractionOutcomeAlreadyResolved,
		}, nil
	}
	if err := prepareOrphanResolutionInput(ctx, exec, interaction, normalized); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	if err := updatePendingInteraction(ctx, exec, normalized); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	delta := 0
	if normalized.Status != store.PendingInteractionStatusOrphaned {
		delta = -1
	}
	if err := bumpPendingInteractionProjection(
		ctx,
		exec,
		interaction.SessionID,
		interaction.Kind,
		delta,
		normalized.At,
	); err != nil {
		return store.SessionAttentionCommit{}, err
	}
	updated, err := getPendingInteraction(ctx, exec, normalized.InteractionID)
	if err != nil {
		return store.SessionAttentionCommit{}, err
	}
	after, err := getSessionAttention(ctx, exec, interaction.SessionID)
	if err != nil {
		return store.SessionAttentionCommit{}, err
	}
	outcome := normalized.Status
	if interaction.Status == store.PendingInteractionStatusOrphaned &&
		normalized.Status == store.PendingInteractionStatusResolved {
		outcome = store.PendingInteractionOutcomeResolvedAfterRestart
	}
	return store.SessionAttentionCommit{
		Before: before, After: after, Interaction: &updated, Outcome: outcome,
	}, nil
}

func prepareOrphanResolutionInput(
	ctx context.Context,
	exec globalSQLExecutor,
	interaction store.PendingInteraction,
	transition store.PendingInteractionTransition,
) error {
	if transition.OrphanInput == nil {
		return nil
	}
	if interaction.Status != store.PendingInteractionStatusOrphaned {
		return errors.New("store: orphan input requires an orphaned interaction")
	}
	if transition.OrphanInput.SessionID != interaction.SessionID {
		return errors.New("store: orphan input session does not own interaction")
	}
	return insertOrphanResolutionInput(ctx, exec, *transition.OrphanInput)
}

func updatePendingInteraction(
	ctx context.Context,
	exec globalSQLExecutor,
	transition store.PendingInteractionTransition,
) error {
	resolvedAt := any(nil)
	if transition.Status != store.PendingInteractionStatusOrphaned {
		resolvedAt = store.FormatTimestamp(transition.At)
	}
	// dynamic-sql: lifecycle status and resolution are mutated inside the canonical transaction.
	result, err := exec.ExecContext(ctx, `UPDATE session_pending_interactions
SET status = ?, resolved_at = ?, resolution = ?, resolved_by = ?
WHERE interaction_id = ? AND status IN ('pending', 'orphaned')`,
		transition.Status,
		resolvedAt,
		transition.Resolution,
		transition.ResolvedBy,
		transition.InteractionID,
	)
	if err != nil {
		return fmt.Errorf("store: transition pending interaction: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: read pending interaction affected rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", store.ErrPendingInteractionAlreadyResolved, transition.InteractionID)
	}
	return nil
}

// ListPendingInteractions returns sanitized durable interactions for one session.
func (g *SessionRepo) ListPendingInteractions(
	ctx context.Context,
	sessionID string,
	statuses []string,
) (interactions []store.PendingInteraction, err error) {
	if err := g.checkReady(ctx, "list pending interactions"); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, errors.New("store: session id is required")
	}
	normalizedStatuses, err := normalizeInteractionStatuses(statuses)
	if err != nil {
		return nil, err
	}
	encodedStatuses, err := json.Marshal(normalizedStatuses)
	if err != nil {
		return nil, fmt.Errorf("store: encode interaction statuses: %w", err)
	}
	const query = `SELECT interaction_id, session_id, kind, provider_request_id, turn_id,
title, payload_json, status, created_at, resolved_at, resolution, resolved_by
FROM session_pending_interactions
WHERE session_id = ? AND status IN (SELECT CAST(value AS TEXT) FROM json_each(?))
ORDER BY created_at ASC, interaction_id ASC`
	rows, queryErr := g.db.QueryContext(ctx, query, target, string(encodedStatuses))
	if queryErr != nil {
		return nil, fmt.Errorf("store: query pending interactions: %w", queryErr)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close pending interaction rows: %w", closeErr))
		}
	}()
	interactions = make([]store.PendingInteraction, 0)
	for rows.Next() {
		interaction, scanErr := scanPendingInteraction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		interactions = append(interactions, interaction)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate pending interactions: %w", rowsErr)
	}
	return interactions, nil
}

func bumpPendingInteractionProjection(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	kind string,
	delta int,
	at time.Time,
) error {
	permissionDelta, clarifyDelta := 0, 0
	switch kind {
	case store.PendingInteractionKindPermission:
		permissionDelta = delta
	case store.PendingInteractionKindClarify:
		clarifyDelta = delta
	default:
		return fmt.Errorf("store: invalid pending interaction kind %q", kind)
	}
	// dynamic-sql: both count projections and the causal revision move with the side-table row.
	result, err := exec.ExecContext(ctx, `UPDATE sessions SET
pending_permission_count = MAX(0, pending_permission_count + ?),
pending_clarify_count = MAX(0, pending_clarify_count + ?),
attention_revision = attention_revision + 1,
attention_changed_at = ?
WHERE id = ?`, permissionDelta, clarifyDelta, store.FormatTimestamp(at), sessionID)
	if err != nil {
		return fmt.Errorf("store: update pending interaction projection: %w", err)
	}
	return requireOneAffected(result, sessionID)
}

func insertOrphanResolutionInput(
	ctx context.Context,
	exec globalSQLExecutor,
	input store.SessionInputQueueInsert,
) error {
	normalized := input.Normalize()
	count, err := countPendingSessionInputs(ctx, exec, normalized.SessionID)
	if err != nil {
		return err
	}
	if count >= normalized.QueueCap {
		return fmt.Errorf(
			"%w: session %s cap %d",
			store.ErrSessionInputQueueFull,
			normalized.SessionID,
			normalized.QueueCap,
		)
	}
	if _, err := insertSessionInputQueueEntry(ctx, exec, normalized); err != nil {
		return err
	}
	return nil
}

func normalizeInteractionStatuses(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		return []string{
			store.PendingInteractionStatusPending,
			store.PendingInteractionStatusOrphaned,
		}, nil
	}
	result := make([]string, 0, len(statuses))
	seen := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		normalized := strings.TrimSpace(status)
		if !store.ValidPendingInteractionStatus(normalized) {
			return nil, fmt.Errorf("%w: %q", store.ErrPendingInteractionStatusInvalid, normalized)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}
