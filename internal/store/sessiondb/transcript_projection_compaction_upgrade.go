package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

// restoreCompactionTranscriptProjection retains the ledger's assigned keys instead of rerouting surviving entries.
func restoreCompactionTranscriptProjection(ctx context.Context, db *sql.DB, sessionID string) (retErr error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin compaction projection upgrade: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			retErr = errors.Join(retErr, fmt.Errorf("store: rollback compaction projection upgrade: %w", err))
		}
	}()
	queries := sqlcgen.New(tx)
	keys, err := queries.ListMissingUnarchivedCompactionEntries(ctx)
	if err != nil {
		return fmt.Errorf("store: find restored compaction entries: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	state, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return fmt.Errorf("store: read restored projection state: %w", err)
	}
	if state.ProjectionVersion != transcript.ProjectionVersion &&
		state.ProjectionVersion != whitespaceProjectionVersion {
		return fmt.Errorf("%w: stored version %d", transcript.ErrProjectionIncompatible, state.ProjectionVersion)
	}
	for _, key := range keys {
		if err := restoreCompactionTranscriptEntry(ctx, tx, sessionID, key, keys); err != nil {
			return err
		}
	}
	if err := queries.AdvanceTranscriptProjectionGeneration(ctx); err != nil {
		return fmt.Errorf("store: advance restored compaction generation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit compaction projection upgrade: %w", err)
	}
	return nil
}

func restoreCompactionTranscriptEntry(
	ctx context.Context,
	tx *sql.Tx,
	sessionID, key string,
	restoredKeys []string,
) error {
	events, err := loadAssignedEvents(ctx, tx, sessionID, key)
	if err != nil {
		return err
	}
	events = slices.DeleteFunc(events, func(event store.SessionEvent) bool { return event.Archived })
	identity, toolRoutes, err := rebuildCompactionEntryIdentity(ctx, tx, sessionID, key, events)
	if err != nil {
		return err
	}
	candidate := identity
	candidate.MessageID = candidate.BaseMessageID
	projected, err := transcript.ProjectAssignedEntry(events, candidate)
	if err != nil {
		return fmt.Errorf("store: inspect restored entry %q: %w", key, err)
	}
	if projected != nil {
		identity.MessageID, err = allocateProjectionMessageID(ctx, tx, key, identity.BaseMessageID)
		if err != nil {
			return err
		}
	}
	entry, err := transcript.ProjectAssignedEntry(events, identity)
	if err != nil {
		return fmt.Errorf("store: reproject restored entry %q: %w", key, err)
	}
	if err := persistProjectionEntry(ctx, tx, identity, entry); err != nil {
		return err
	}
	for toolKey := range toolRoutes {
		if routed, found, err := (projectionSQLResolver{db: tx}).ToolEntryIdentity(ctx, toolKey); err != nil {
			return err
		} else if found && !slices.Contains(restoredKeys, routed.Key) {
			continue
		}
		if err := sqlcgen.New(tx).UpsertTranscriptToolRoute(ctx, sqlcgen.UpsertTranscriptToolRouteParams{
			ToolKey: toolKey, EntryKey: key,
		}); err != nil {
			return fmt.Errorf("store: restore transcript tool route %q: %w", toolKey, err)
		}
	}
	return nil
}

func rebuildCompactionEntryIdentity(
	ctx context.Context,
	tx *sql.Tx,
	sessionID, key string,
	events []store.SessionEvent,
) (transcript.EntryIdentity, map[string]string, error) {
	prefix, _, ok := strings.Cut(key, ":s")
	generation, err := strconv.ParseInt(strings.TrimPrefix(prefix, "g"), 10, 64)
	if !ok || err != nil || generation < 0 {
		return transcript.EntryIdentity{}, nil, fmt.Errorf(
			"%w: invalid restored entry key %q",
			transcript.ErrProjectionCorrupt,
			key,
		)
	}
	projector, err := transcript.NewProjector(transcript.ProjectionState{
		Version: transcript.ProjectionVersion, Generation: generation,
	}, nil)
	if err != nil {
		return transcript.EntryIdentity{}, nil, err
	}
	for _, event := range events {
		assignment, err := projector.Assign(ctx, event)
		if err != nil {
			return transcript.EntryIdentity{}, nil, err
		}
		if assignment.Entry.Key != key {
			return transcript.EntryIdentity{}, nil, fmt.Errorf(
				"%w: inconsistent restored entry %q",
				transcript.ErrProjectionCorrupt,
				key,
			)
		}
	}
	identity, found := projector.Identity(key)
	if !found {
		return transcript.EntryIdentity{}, nil, fmt.Errorf(
			"%w: missing restored identity %q",
			transcript.ErrProjectionCorrupt,
			key,
		)
	}
	toolRoutes := projector.ToolRoutes()
	// A later boundary can complete the entry even when rewind now excludes that boundary from history.
	boundary, err := sqlcgen.New(tx).GetNextTranscriptBoundaryEventForUpgrade(ctx, key)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return transcript.EntryIdentity{}, nil, fmt.Errorf("store: load restored entry boundary: %w", err)
	}
	if err == nil && strings.HasPrefix(boundary.TranscriptEntryKey, prefix+":") {
		event, err := sessionEventFromSQLC(boundary.ID, boundary.Sequence, boundary.TurnID,
			boundary.Type, boundary.AgentName, boundary.Content, boundary.Archived, boundary.Timestamp, sessionID)
		if err != nil {
			return transcript.EntryIdentity{}, nil, err
		}
		assignment, err := projector.Assign(ctx, event)
		if err != nil {
			return transcript.EntryIdentity{}, nil, err
		}
		if slices.Contains(assignment.CompletedKeys, key) {
			identity, found = projector.Identity(key)
		}
		if !found {
			return transcript.EntryIdentity{}, nil, fmt.Errorf(
				"%w: missing completed identity %q",
				transcript.ErrProjectionCorrupt,
				key,
			)
		}
	}
	return identity, toolRoutes, nil
}
