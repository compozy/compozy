package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

type pendingInteractionRow struct {
	interactionID     string
	sessionID         string
	kind              string
	providerRequestID string
	turnID            string
	title             string
	payloadJSON       string
	status            string
	createdAt         string
	resolvedAt        sql.NullString
	resolution        string
	resolvedBy        string
}

func getPendingInteraction(
	ctx context.Context,
	exec globalSQLExecutor,
	interactionID string,
) (store.PendingInteraction, error) {
	row := exec.QueryRowContext(ctx, `SELECT interaction_id, session_id, kind, provider_request_id,
turn_id, title, payload_json, status, created_at, resolved_at, resolution, resolved_by
FROM session_pending_interactions WHERE interaction_id = ?`, interactionID)
	interaction, err := scanPendingInteraction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return store.PendingInteraction{}, fmt.Errorf("%w: %s", store.ErrPendingInteractionNotFound, interactionID)
	}
	return interaction, err
}

func scanPendingInteraction(scanner rowScanner) (store.PendingInteraction, error) {
	var row pendingInteractionRow
	if err := scanner.Scan(
		&row.interactionID,
		&row.sessionID,
		&row.kind,
		&row.providerRequestID,
		&row.turnID,
		&row.title,
		&row.payloadJSON,
		&row.status,
		&row.createdAt,
		&row.resolvedAt,
		&row.resolution,
		&row.resolvedBy,
	); err != nil {
		return store.PendingInteraction{}, err
	}
	var payload store.PendingInteractionPayload
	if err := json.Unmarshal([]byte(row.payloadJSON), &payload); err != nil {
		return store.PendingInteraction{}, fmt.Errorf("store: decode pending interaction payload: %w", err)
	}
	createdAt, err := store.ParseTimestamp(row.createdAt)
	if err != nil {
		return store.PendingInteraction{}, fmt.Errorf("store: parse pending interaction created_at: %w", err)
	}
	interaction := store.PendingInteraction{
		InteractionID:     strings.TrimSpace(row.interactionID),
		SessionID:         strings.TrimSpace(row.sessionID),
		Kind:              strings.TrimSpace(row.kind),
		ProviderRequestID: strings.TrimSpace(row.providerRequestID),
		TurnID:            strings.TrimSpace(row.turnID),
		Title:             row.title,
		Payload:           payload,
		Status:            strings.TrimSpace(row.status),
		CreatedAt:         createdAt,
		Resolution:        row.resolution,
		ResolvedBy:        row.resolvedBy,
	}
	if row.resolvedAt.Valid && strings.TrimSpace(row.resolvedAt.String) != "" {
		resolvedAt, parseErr := store.ParseTimestamp(row.resolvedAt.String)
		if parseErr != nil {
			return store.PendingInteraction{}, fmt.Errorf("store: parse pending interaction resolved_at: %w", parseErr)
		}
		interaction.ResolvedAt = &resolvedAt
	}
	return store.SanitizePendingInteraction(interaction), nil
}

func pendingInteractionFromCreate(req store.PendingInteractionCreate) store.PendingInteraction {
	normalized := req.Normalize()
	return store.PendingInteraction{
		InteractionID:     normalized.InteractionID,
		SessionID:         normalized.SessionID,
		Kind:              normalized.Kind,
		ProviderRequestID: normalized.ProviderRequestID,
		TurnID:            normalized.TurnID,
		Title:             normalized.Title,
		Payload:           normalized.Payload,
		Status:            store.PendingInteractionStatusPending,
		CreatedAt:         normalized.CreatedAt,
	}
}
