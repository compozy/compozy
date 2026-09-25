package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

const whitespaceProjectionVersion = 1
const whitespaceUpgradeBatchSize = 512

// upgradeTranscriptWhitespaceProjection replays entries whose text whitespace changed in version 1.
func upgradeTranscriptWhitespaceProjection(ctx context.Context, db *sql.DB, sessionID string) (retErr error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin transcript whitespace upgrade: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			retErr = errors.Join(retErr, fmt.Errorf("store: rollback transcript whitespace upgrade: %w", rollbackErr))
		}
	}()

	queries := sqlcgen.New(tx)
	row, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return fmt.Errorf("store: read transcript projection version: %w", err)
	}
	if row.ProjectionVersion == transcript.ProjectionVersion {
		return nil
	}
	if row.ProjectionVersion != whitespaceProjectionVersion {
		return fmt.Errorf("%w: stored version %d, supported version %d",
			transcript.ErrProjectionIncompatible, row.ProjectionVersion, transcript.ProjectionVersion)
	}

	orderedKeys, err := whitespaceProjectionEntryKeys(ctx, queries)
	if err != nil {
		return err
	}

	resolver := projectionSQLResolver{db: tx}
	for _, key := range orderedKeys {
		identity, found, resolveErr := resolver.EntryIdentity(ctx, key)
		if resolveErr != nil {
			return resolveErr
		}
		if !found || identity.Kind != transcript.EntryKindAssistant {
			return fmt.Errorf("%w: whitespace event references missing assistant entry %q",
				transcript.ErrProjectionCorrupt, key)
		}
		events, loadErr := loadAssignedEvents(ctx, tx, sessionID, key)
		if loadErr != nil {
			return loadErr
		}
		entry, projectErr := transcript.ProjectAssignedEntry(events, identity)
		if projectErr != nil {
			return fmt.Errorf("store: reproject whitespace entry %q: %w", key, projectErr)
		}
		if err := persistProjectionEntry(ctx, tx, identity, entry); err != nil {
			return err
		}
	}

	if err := queries.UpdateTranscriptProjectionState(ctx, sqlcgen.UpdateTranscriptProjectionStateParams{
		ProjectionVersion: transcript.ProjectionVersion,
		Generation:        row.Generation,
		ActiveEntryKey:    row.ActiveEntryKey,
	}); err != nil {
		return fmt.Errorf("store: finish transcript whitespace upgrade: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit transcript whitespace upgrade: %w", err)
	}
	return nil
}

func whitespaceProjectionEntryKeys(ctx context.Context, queries *sqlcgen.Queries) ([]string, error) {
	keys := make(map[string]struct{})
	var afterSequence int64
	for {
		textEvents, err := queries.ListTranscriptTextEventsForUpgrade(
			ctx,
			sqlcgen.ListTranscriptTextEventsForUpgradeParams{
				AfterSequence: afterSequence,
				RowLimit:      whitespaceUpgradeBatchSize,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("store: read transcript text events for upgrade: %w", err)
		}
		for _, event := range textEvents {
			if needsWhitespaceProjectionRepair(event.Content) {
				keys[event.TranscriptEntryKey] = struct{}{}
			}
			afterSequence = event.Sequence
		}
		if len(textEvents) < whitespaceUpgradeBatchSize {
			break
		}
	}
	orderedKeys := make([]string, 0, len(keys))
	for key := range keys {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	return orderedKeys, nil
}

func needsWhitespaceProjectionRepair(content string) bool {
	if content != "" && strings.TrimSpace(content) == "" {
		return true
	}
	var payload struct {
		Text         string `json:"text"`
		AuthoredText string `json:"authored_text"`
		Content      struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return content != strings.TrimSpace(content)
	}
	for _, text := range []string{payload.Text, payload.AuthoredText, payload.Content.Text} {
		if text != "" && strings.TrimSpace(text) == "" {
			return true
		}
	}
	return false
}

// OpenSessionDBReadOnlyWithProjectionUpgrade repairs legacy projections before returning a reader.
func OpenSessionDBReadOnlyWithProjectionUpgrade(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (*ReadOnlySessionDB, error) {
	reader, err := OpenSessionDBReadOnly(ctx, owner, path)
	if err != nil {
		return nil, err
	}
	row, err := sqlcgen.New(reader.db).GetTranscriptProjectionState(ctx)
	if err != nil {
		return nil, closeReadOnlySessionDBAfterOpenError(reader.db,
			fmt.Errorf("store: inspect read-only transcript projection version: %w", err))
	}
	if row.ProjectionVersion == transcript.ProjectionVersion {
		return reader, nil
	}
	if row.ProjectionVersion != whitespaceProjectionVersion {
		return nil, closeReadOnlySessionDBAfterOpenError(reader.db,
			fmt.Errorf("%w: stored version %d, supported version %d",
				transcript.ErrProjectionIncompatible, row.ProjectionVersion, transcript.ProjectionVersion))
	}
	if err := reader.Close(ctx); err != nil {
		return nil, fmt.Errorf("store: close legacy transcript reader before upgrade: %w", err)
	}
	writer, err := OpenSessionDB(ctx, owner, path)
	if err != nil {
		return nil, fmt.Errorf("store: open legacy transcript for upgrade: %w", err)
	}
	if err := writer.Close(ctx); err != nil {
		return nil, fmt.Errorf("store: close upgraded transcript writer: %w", err)
	}
	return OpenSessionDBReadOnly(ctx, owner, path)
}
