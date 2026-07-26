package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/agh/internal/transcript"
)

type projectionDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type projectionSQLResolver struct {
	db projectionDB
}

func (r projectionSQLResolver) EntryIdentity(
	ctx context.Context,
	key string,
) (transcript.EntryIdentity, bool, error) {
	row, err := sqlcgen.New(r.db).GetTranscriptEntryIdentity(ctx, key)
	return optionalProjectionIdentity(
		row.EntryKey,
		row.Kind,
		row.LogicalID,
		row.TurnID,
		row.BaseMessageID,
		row.MessageID,
		row.StartSequence,
		row.UpdatedSequence,
		row.Complete,
		err,
	)
}

func (r projectionSQLResolver) ToolEntryIdentity(
	ctx context.Context,
	toolKey string,
) (transcript.EntryIdentity, bool, error) {
	row, err := sqlcgen.New(r.db).GetTranscriptToolEntryIdentity(ctx, toolKey)
	return optionalProjectionIdentity(
		row.EntryKey,
		row.Kind,
		row.LogicalID,
		row.TurnID,
		row.BaseMessageID,
		row.MessageID,
		row.StartSequence,
		row.UpdatedSequence,
		row.Complete,
		err,
	)
}

func (r projectionSQLResolver) LatestAssistantIdentity(
	ctx context.Context,
	turnID string,
) (transcript.EntryIdentity, bool, error) {
	row, err := sqlcgen.New(r.db).GetLatestAssistantTranscriptIdentity(
		ctx,
		sqlcgen.GetLatestAssistantTranscriptIdentityParams{
			Kind:   string(transcript.EntryKindAssistant),
			TurnID: turnID,
		},
	)
	return optionalProjectionIdentity(
		row.EntryKey,
		row.Kind,
		row.LogicalID,
		row.TurnID,
		row.BaseMessageID,
		row.MessageID,
		row.StartSequence,
		row.UpdatedSequence,
		row.Complete,
		err,
	)
}

func optionalProjectionIdentity(
	key string,
	kind string,
	logicalID string,
	turnID string,
	baseMessageID string,
	messageID string,
	startSequence int64,
	updatedSequence int64,
	complete int64,
	err error,
) (transcript.EntryIdentity, bool, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return transcript.EntryIdentity{}, false, nil
		}
		return transcript.EntryIdentity{}, false, fmt.Errorf("store: query transcript identity: %w", err)
	}
	return transcript.EntryIdentity{
		Key:             key,
		Kind:            transcript.EntryKind(kind),
		LogicalID:       logicalID,
		TurnID:          turnID,
		BaseMessageID:   baseMessageID,
		MessageID:       messageID,
		StartSequence:   startSequence,
		UpdatedSequence: updatedSequence,
		Complete:        complete != 0,
	}, true, nil
}

func loadProjectionState(ctx context.Context, db projectionDB) (transcript.ProjectionState, error) {
	row, err := sqlcgen.New(db).GetTranscriptProjectionState(ctx)
	if err != nil {
		return transcript.ProjectionState{}, fmt.Errorf("%w: load state: %v", transcript.ErrProjectionCorrupt, err)
	}
	state := transcript.ProjectionState{
		Version:        int(row.ProjectionVersion),
		Generation:     row.Generation,
		ActiveEntryKey: row.ActiveEntryKey,
	}
	if state.Version != transcript.ProjectionVersion {
		return transcript.ProjectionState{}, fmt.Errorf(
			"%w: stored version %d, supported version %d",
			transcript.ErrProjectionIncompatible,
			state.Version,
			transcript.ProjectionVersion,
		)
	}
	return state, nil
}

func persistProjectionState(ctx context.Context, db projectionDB, state transcript.ProjectionState) error {
	if err := sqlcgen.New(db).UpdateTranscriptProjectionState(ctx, sqlcgen.UpdateTranscriptProjectionStateParams{
		ProjectionVersion: int64(state.Version),
		Generation:        state.Generation,
		ActiveEntryKey:    state.ActiveEntryKey,
	}); err != nil {
		return fmt.Errorf("store: update transcript projection state: %w", err)
	}
	return nil
}

func persistProjectionEntry(
	ctx context.Context,
	db projectionDB,
	identity transcript.EntryIdentity,
	entry *transcript.Entry,
) error {
	var messageJSON sql.NullString
	var eventType string
	var markerJSON sql.NullString
	if entry != nil {
		encoded, err := json.Marshal(entry.Message)
		if err != nil {
			return fmt.Errorf("store: marshal transcript message: %w", err)
		}
		messageJSON = sql.NullString{String: string(encoded), Valid: true}
		eventType = entry.EventType
		if entry.Marker != nil {
			encodedMarker, markerErr := json.Marshal(entry.Marker)
			if markerErr != nil {
				return fmt.Errorf("store: marshal transcript marker: %w", markerErr)
			}
			markerJSON = sql.NullString{String: string(encodedMarker), Valid: true}
		}
	}
	if err := sqlcgen.New(db).UpsertTranscriptEntry(ctx, sqlcgen.UpsertTranscriptEntryParams{
		EntryKey:        identity.Key,
		Kind:            string(identity.Kind),
		LogicalID:       identity.LogicalID,
		TurnID:          identity.TurnID,
		BaseMessageID:   identity.BaseMessageID,
		MessageID:       identity.MessageID,
		StartSequence:   identity.StartSequence,
		UpdatedSequence: identity.UpdatedSequence,
		Complete:        boolToSQLite(identity.Complete),
		MessageJson:     messageJSON,
		EventType:       eventType,
		MarkerJson:      markerJSON,
	}); err != nil {
		return fmt.Errorf("store: upsert transcript entry %q: %w", identity.Key, err)
	}
	return nil
}

func allocateProjectionMessageID(
	ctx context.Context,
	db projectionDB,
	entryKey string,
	base string,
) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "message"
	}
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		count, err := sqlcgen.New(db).CountTranscriptMessageIDConflicts(
			ctx,
			sqlcgen.CountTranscriptMessageIDConflictsParams{
				MessageID: candidate,
				EntryKey:  entryKey,
			},
		)
		if err != nil {
			return "", fmt.Errorf("store: test transcript message id %q: %w", candidate, err)
		}
		if count == 0 {
			return candidate, nil
		}
	}
}

func loadAssignedEvents(
	ctx context.Context,
	db projectionDB,
	sessionID string,
	entryKey string,
) (events []store.SessionEvent, err error) {
	rows, err := sqlcgen.New(db).ListEventsForTranscriptEntry(ctx, entryKey)
	if err != nil {
		return nil, fmt.Errorf("store: query events for transcript entry %q: %w", entryKey, err)
	}
	events = make([]store.SessionEvent, 0, len(rows))
	for _, row := range rows {
		event, scanErr := sessionEventFromSQLC(
			row.ID,
			row.Sequence,
			row.TurnID,
			row.Type,
			row.AgentName,
			row.Content,
			row.Archived,
			row.Timestamp,
			sessionID,
		)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	return events, nil
}
