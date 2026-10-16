package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

// Each older projection version names the entries whose projection the next version changed.
const (
	// whitespaceProjectionVersion lost text whitespace; version 2 preserves it (#670).
	whitespaceProjectionVersion = 1
	// sessionMessageProjectionVersion dropped real-provider session-message cards; version 3 projects them (#742).
	sessionMessageProjectionVersion = 2
)

const projectionUpgradeBatchSize = 512

// upgradableProjectionVersion reports a stored projection this build can repair in place.
func upgradableProjectionVersion(version int64) bool {
	return version >= whitespaceProjectionVersion && version < transcript.ProjectionVersion
}

// upgradeTranscriptProjection replays the entries each version after the stored one changed.
func upgradeTranscriptProjection(ctx context.Context, db *sql.DB, sessionID string) error {
	row, err := sqlcgen.New(db).GetTranscriptProjectionState(ctx)
	if err != nil {
		return fmt.Errorf("store: read transcript projection version: %w", err)
	}
	if row.ProjectionVersion == transcript.ProjectionVersion {
		return nil
	}
	return store.ExecuteWrite(ctx, db, func(ctx context.Context, tx *store.WriteTx) error {
		return upgradeTranscriptProjectionTx(ctx, tx, sessionID)
	})
}

func upgradeTranscriptProjectionTx(ctx context.Context, tx *store.WriteTx, sessionID string) error {
	queries := sqlcgen.New(tx)
	row, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return fmt.Errorf("store: read transcript projection version: %w", err)
	}
	if row.ProjectionVersion == transcript.ProjectionVersion {
		return nil
	}
	if !upgradableProjectionVersion(row.ProjectionVersion) {
		return fmt.Errorf("%w: stored version %d, supported version %d",
			transcript.ErrProjectionIncompatible, row.ProjectionVersion, transcript.ProjectionVersion)
	}

	keys := make(map[string]struct{})
	if row.ProjectionVersion <= whitespaceProjectionVersion {
		if err := collectWhitespaceProjectionEntryKeys(ctx, queries, keys); err != nil {
			return err
		}
	}
	if row.ProjectionVersion <= sessionMessageProjectionVersion {
		if err := collectSessionMessageProjectionEntryKeys(ctx, queries, keys); err != nil {
			return err
		}
	}
	orderedKeys := slices.AppendSeq(make([]string, 0, len(keys)), maps.Keys(keys))
	slices.Sort(orderedKeys)

	resolver := projectionSQLResolver{db: tx}
	for _, key := range orderedKeys {
		identity, found, resolveErr := resolver.EntryIdentity(ctx, key)
		if resolveErr != nil {
			return resolveErr
		}
		if !found || identity.Kind != transcript.EntryKindAssistant {
			return fmt.Errorf("%w: upgraded event references missing assistant entry %q",
				transcript.ErrProjectionCorrupt, key)
		}
		events, loadErr := loadAssignedEvents(ctx, tx, sessionID, key)
		if loadErr != nil {
			return loadErr
		}
		entry, projectErr := transcript.ProjectAssignedEntry(events, identity)
		if projectErr != nil {
			return fmt.Errorf("store: reproject upgraded entry %q: %w", key, projectErr)
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
		return fmt.Errorf("store: finish transcript projection upgrade: %w", err)
	}
	return nil
}

func collectWhitespaceProjectionEntryKeys(
	ctx context.Context,
	queries *sqlcgen.Queries,
	keys map[string]struct{},
) error {
	var afterSequence int64
	for {
		textEvents, err := queries.ListTranscriptTextEventsForUpgrade(
			ctx,
			sqlcgen.ListTranscriptTextEventsForUpgradeParams{
				AfterSequence: afterSequence,
				RowLimit:      projectionUpgradeBatchSize,
			},
		)
		if err != nil {
			return fmt.Errorf("store: read transcript text events for upgrade: %w", err)
		}
		for _, event := range textEvents {
			if needsWhitespaceProjectionRepair(event.Content) {
				keys[event.TranscriptEntryKey] = struct{}{}
			}
			afterSequence = event.Sequence
		}
		if len(textEvents) < projectionUpgradeBatchSize {
			return nil
		}
	}
}

// collectSessionMessageProjectionEntryKeys names the entries holding a session-prompt call. A
// call's result is routed to the entry of its last tool_call, so these entries hold every card.
func collectSessionMessageProjectionEntryKeys(
	ctx context.Context,
	queries *sqlcgen.Queries,
	keys map[string]struct{},
) error {
	var afterSequence int64
	for {
		calls, err := queries.ListTranscriptSessionPromptEventsForUpgrade(
			ctx,
			sqlcgen.ListTranscriptSessionPromptEventsForUpgradeParams{
				AfterSequence: afterSequence,
				RowLimit:      projectionUpgradeBatchSize,
			},
		)
		if err != nil {
			return fmt.Errorf("store: read transcript session-prompt calls for upgrade: %w", err)
		}
		for _, call := range calls {
			keys[call.TranscriptEntryKey] = struct{}{}
			afterSequence = call.Sequence
		}
		if len(calls) < projectionUpgradeBatchSize {
			return nil
		}
	}
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
	missing, err := sqlcgen.New(reader.db).ListMissingUnarchivedCompactionEntries(ctx)
	if err != nil {
		return nil, closeReadOnlySessionDBAfterOpenError(reader.db,
			fmt.Errorf("store: inspect restored compaction entries: %w", err))
	}
	if row.ProjectionVersion == transcript.ProjectionVersion && len(missing) == 0 {
		return reader, nil
	}
	if !upgradableProjectionVersion(row.ProjectionVersion) && row.ProjectionVersion != transcript.ProjectionVersion {
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
