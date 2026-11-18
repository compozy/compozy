package sessiondb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
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
	var encoder projectionMessageEncoder
	params, err := prepareProjectionEntry(&encoder, identity, entry)
	if err != nil {
		return err
	}
	if err := sqlcgen.New(db).UpsertTranscriptEntry(ctx, params); err != nil {
		return fmt.Errorf("store: upsert transcript entry %q: %w", identity.Key, err)
	}
	return nil
}

// maxRetainedProjectionBuffer bounds the encode buffer a session writer keeps
// between appends. Larger messages still encode correctly but release their
// buffer afterwards instead of pinning it for the session lifetime.
const maxRetainedProjectionBuffer = 4 << 20

// projectionMessageEncoder encodes transcript messages into a reused buffer.
// Every append rewrites the whole active assistant message, so a fresh
// encoder buffer per append would regrow to the full message size each time.
// The zero value is ready to use; it is not safe for concurrent use.
type projectionMessageEncoder struct {
	buf bytes.Buffer
}

// encode returns the same bytes as json.Marshal(message): MarshalWrite with
// the v1 default options omits the top-level newline exactly like Marshal.
func (e *projectionMessageEncoder) encode(message *transcript.UIMessage) (string, error) {
	e.buf.Reset()
	err := jsonv2.MarshalWrite(&e.buf, message, json.DefaultOptionsV1())
	var encoded string
	if err == nil {
		encoded = e.buf.String()
	}
	if e.buf.Cap() > maxRetainedProjectionBuffer {
		e.buf = bytes.Buffer{}
	}
	return encoded, err
}

func prepareProjectionEntry(
	encoder *projectionMessageEncoder,
	identity transcript.EntryIdentity,
	entry *transcript.Entry,
) (sqlcgen.UpsertTranscriptEntryParams, error) {
	var messageJSON sql.NullString
	var eventType string
	var markerJSON sql.NullString
	if entry != nil {
		encoded, err := encoder.encode(&entry.Message)
		if err != nil {
			return sqlcgen.UpsertTranscriptEntryParams{}, fmt.Errorf("store: marshal transcript message: %w", err)
		}
		messageJSON = sql.NullString{String: encoded, Valid: true}
		eventType = entry.EventType
		if entry.Marker != nil {
			encodedMarker, markerErr := json.Marshal(entry.Marker)
			if markerErr != nil {
				return sqlcgen.UpsertTranscriptEntryParams{}, fmt.Errorf(
					"store: marshal transcript marker: %w",
					markerErr,
				)
			}
			markerJSON = sql.NullString{String: string(encodedMarker), Valid: true}
		}
	}
	return sqlcgen.UpsertTranscriptEntryParams{
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
	}, nil
}

func allocateProjectionMessageID(
	ctx context.Context,
	db projectionDB,
	entryKey string,
	base string,
	reserved map[string]struct{},
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
		if _, used := reserved[candidate]; used {
			continue
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
