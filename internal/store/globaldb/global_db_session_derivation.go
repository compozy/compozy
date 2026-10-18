package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SessionDerivationStore = (*SessionRepo)(nil)

// RegisterDerivedSession writes the child's catalog row, its creation identity, and the
// derive receipt with its immutable outcome in one transaction. A receipt already
// recorded for the same (workspace_id, idempotency_key) rolls the whole transaction
// back with store.ErrSessionDerivationExists.
func (g *SessionRepo) RegisterDerivedSession(
	ctx context.Context,
	info store.SessionInfo,
	identity store.SessionCreationIdentity,
	receipt store.SessionDerivationReceipt,
) error {
	if err := g.checkReady(ctx, "register derived session"); err != nil {
		return err
	}
	if err := info.Validate(); err != nil {
		return err
	}
	if err := identity.Validate(); err != nil {
		return err
	}
	params, err := sessionDerivationInsertParams(info, receipt, g.now())
	if err != nil {
		return err
	}
	normalized := info
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	return g.withImmediateTransaction(ctx, "register derived session", func(exec globalSQLExecutor) error {
		affected, err := sqlcgen.New(exec).InsertSessionDerivation(ctx, params)
		if err != nil {
			return fmt.Errorf("store: insert session derivation %q: %w", params.IdempotencyKey, err)
		}
		if affected != 1 {
			return fmt.Errorf(
				"%w: workspace %q key %q",
				store.ErrSessionDerivationExists,
				params.WorkspaceID,
				params.IdempotencyKey,
			)
		}
		if _, err := g.registerSessionCreationTx(ctx, exec, normalized, identity); err != nil {
			return err
		}
		return nil
	})
}

// SessionDerivationReceipt loads one receipt by its workspace-scoped idempotency key.
func (g *SessionRepo) SessionDerivationReceipt(
	ctx context.Context,
	workspaceID string,
	idempotencyKey string,
) (store.SessionDerivationReceipt, bool, error) {
	if err := g.checkReady(ctx, "get session derivation receipt"); err != nil {
		return store.SessionDerivationReceipt{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if workspaceID == "" || idempotencyKey == "" {
		return store.SessionDerivationReceipt{}, false, errors.New(
			"store: session derivation workspace id and idempotency key are required",
		)
	}
	row, err := g.queries.GetSessionDerivation(ctx, sqlcgen.GetSessionDerivationParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionDerivationReceipt{}, false, nil
	}
	if err != nil {
		return store.SessionDerivationReceipt{}, false, fmt.Errorf(
			"store: read session derivation %q: %w", idempotencyKey, err,
		)
	}
	receipt, err := sessionDerivationReceiptFromRow(row)
	if err != nil {
		return store.SessionDerivationReceipt{}, false, err
	}
	return receipt, true, nil
}

// MarkDerivationChildDeleted tombstones every receipt whose child is sessionID. The
// receipts themselves are never deleted, so a retry keeps its recorded answer.
func (g *SessionRepo) MarkDerivationChildDeleted(ctx context.Context, childSessionID string, at time.Time) error {
	if err := g.checkReady(ctx, "mark derivation child deleted"); err != nil {
		return err
	}
	return markDerivationChildDeleted(ctx, g.queries, childSessionID, at)
}

func markDerivationChildDeleted(
	ctx context.Context,
	queries *sqlcgen.Queries,
	childSessionID string,
	at time.Time,
) error {
	childSessionID = strings.TrimSpace(childSessionID)
	if childSessionID == "" {
		return errors.New("store: session derivation child session id is required")
	}
	if at.IsZero() {
		return errors.New("store: session derivation child deletion time is required")
	}
	if err := queries.MarkSessionDerivationChildDeleted(ctx, sqlcgen.MarkSessionDerivationChildDeletedParams{
		ChildDeletedAt: sql.NullString{String: store.FormatTimestamp(at), Valid: true},
		ChildSessionID: childSessionID,
	}); err != nil {
		return fmt.Errorf("store: tombstone session derivations for child %q: %w", childSessionID, err)
	}
	return nil
}

func sessionDerivationInsertParams(
	info store.SessionInfo,
	receipt store.SessionDerivationReceipt,
	now time.Time,
) (sqlcgen.InsertSessionDerivationParams, error) {
	if err := receipt.Validate(); err != nil {
		return sqlcgen.InsertSessionDerivationParams{}, err
	}
	if strings.TrimSpace(receipt.ChildSessionID) != strings.TrimSpace(info.ID) {
		return sqlcgen.InsertSessionDerivationParams{}, fmt.Errorf(
			"store: session derivation child %q does not match registered session %q",
			receipt.ChildSessionID, info.ID,
		)
	}
	if strings.TrimSpace(receipt.WorkspaceID) != strings.TrimSpace(info.WorkspaceID) ||
		strings.TrimSpace(receipt.ProfileID) != strings.TrimSpace(info.ProfileID) {
		return sqlcgen.InsertSessionDerivationParams{}, errors.New(
			"store: session derivation scope does not match the registered session",
		)
	}
	if receipt.ChildDeletedAt != nil {
		return sqlcgen.InsertSessionDerivationParams{}, errors.New(
			"store: a new session derivation cannot carry a child deletion time",
		)
	}
	outcomeJSON, err := json.Marshal(receipt.Outcome)
	if err != nil {
		return sqlcgen.InsertSessionDerivationParams{}, fmt.Errorf("store: encode session derivation outcome: %w", err)
	}
	createdAt := receipt.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return sqlcgen.InsertSessionDerivationParams{
		WorkspaceID:        strings.TrimSpace(receipt.WorkspaceID),
		IdempotencyKey:     strings.TrimSpace(receipt.IdempotencyKey),
		ProfileID:          strings.TrimSpace(receipt.ProfileID),
		RequestFingerprint: strings.TrimSpace(receipt.RequestFingerprint),
		SourceSessionID:    strings.TrimSpace(receipt.SourceSessionID),
		ChildSessionID:     strings.TrimSpace(receipt.ChildSessionID),
		Kind:               string(receipt.Kind),
		OutcomeJson:        string(outcomeJSON),
		CreatedAt:          store.FormatTimestamp(createdAt),
	}, nil
}

func sessionDerivationReceiptFromRow(row sqlcgen.SessionDerivation) (store.SessionDerivationReceipt, error) {
	var outcome store.SessionDerivationOutcome
	if err := json.Unmarshal([]byte(row.OutcomeJson), &outcome); err != nil {
		return store.SessionDerivationReceipt{}, fmt.Errorf(
			"store: decode session derivation outcome %q: %w", row.IdempotencyKey, err,
		)
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return store.SessionDerivationReceipt{}, fmt.Errorf(
			"store: parse session derivation created at %q: %w", row.IdempotencyKey, err,
		)
	}
	receipt := store.SessionDerivationReceipt{
		WorkspaceID:        row.WorkspaceID,
		ProfileID:          row.ProfileID,
		IdempotencyKey:     row.IdempotencyKey,
		RequestFingerprint: row.RequestFingerprint,
		SourceSessionID:    row.SourceSessionID,
		ChildSessionID:     row.ChildSessionID,
		Kind:               store.LineageKind(row.Kind),
		Outcome:            outcome,
		CreatedAt:          createdAt,
	}
	if row.ChildDeletedAt.Valid && strings.TrimSpace(row.ChildDeletedAt.String) != "" {
		deletedAt, parseErr := store.ParseTimestamp(row.ChildDeletedAt.String)
		if parseErr != nil {
			return store.SessionDerivationReceipt{}, fmt.Errorf(
				"store: parse session derivation child deleted at %q: %w", row.IdempotencyKey, parseErr,
			)
		}
		receipt.ChildDeletedAt = &deletedAt
	}
	return receipt, nil
}
