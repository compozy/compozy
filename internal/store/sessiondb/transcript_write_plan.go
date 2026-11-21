package sessiondb

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

var errTranscriptChanged = errors.New("store: transcript changed while preparing append")

// transcriptWritePlan contains only ready-to-write rows. Event assignment,
// reduction and JSON encoding run on a read snapshot, before BEGIN IMMEDIATE.
type transcriptWritePlan struct {
	before       transcript.ProjectionState
	after        transcript.ProjectionState
	nextSequence int64
	events       []store.SessionEvent
	entryKeys    []string
	entries      []preparedTranscriptEntry
	toolRoutes   map[string]string
	existing     bool
}

type preparedTranscriptEntry struct {
	row   sqlcgen.UpsertTranscriptEntryParams
	parts []sqlcgen.UpsertTranscriptEntryPartParams
}

func (s *SessionDB) writeProjectedEvents(
	ctx context.Context, events []store.SessionEvent, ifAbsent bool,
) ([]store.SessionEvent, bool, error) {
	for ctx.Err() == nil {
		s.transcriptFolds.begin()
		plan, err := s.prepareTranscriptWrite(ctx, events, ifAbsent)
		if err == nil && plan.existing {
			return plan.events, false, nil
		}
		if err == nil {
			err = s.commitTranscriptWrite(ctx, plan)
		}
		if err == nil {
			s.transcriptFolds.commit()
			return plan.events, true, nil
		}
		// A second handle can append, archive, clear or rewind between preparation
		// and publication. Discard the speculative folds and prepare a new snapshot.
		s.transcriptFolds.begin()
		if !errors.Is(err, errTranscriptChanged) {
			return nil, false, err
		}
	}
	return nil, false, ctx.Err()
}

func (s *SessionDB) prepareTranscriptWrite(
	ctx context.Context, events []store.SessionEvent, ifAbsent bool,
) (plan transcriptWritePlan, err error) {
	tx, err := s.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return plan, fmt.Errorf("store: begin transcript preparation: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	if ifAbsent {
		existing, found, err := s.findIdenticalEvent(ctx, tx, events[0])
		if err != nil {
			return plan, err
		}
		if found {
			return transcriptWritePlan{events: []store.SessionEvent{existing}, existing: true}, nil
		}
	}
	plan.before, err = loadProjectionState(ctx, tx)
	if err != nil {
		return plan, err
	}
	plan.nextSequence, err = nextEventSequence(ctx, tx)
	if err != nil {
		return plan, err
	}
	projector, err := transcript.NewProjector(plan.before, projectionSQLResolver{db: tx})
	if err != nil {
		return plan, err
	}
	plan.events = slices.Clone(events)
	plan.entryKeys = make([]string, len(events))
	assigned := make(map[string][]store.SessionEvent)
	affected := make(map[string]struct{})
	for i := range plan.events {
		event := &plan.events[i]
		event.Sequence = plan.nextSequence + int64(i)
		if event.Archived {
			continue
		}
		assignment, err := projector.Assign(ctx, *event)
		if err != nil {
			return plan, err
		}
		key := assignment.Entry.Key
		plan.entryKeys[i] = key
		assigned[key] = append(assigned[key], *event)
		affected[key] = struct{}{}
		for _, completed := range assignment.CompletedKeys {
			affected[completed] = struct{}{}
		}
	}
	plan.entries, err = s.prepareTranscriptEntries(ctx, tx, projector, affected, assigned)
	plan.after = projector.State()
	plan.toolRoutes = projector.ToolRoutes()
	return plan, err
}

func (s *SessionDB) findIdenticalEvent(
	ctx context.Context, db projectionDB, event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	row, err := sqlcgen.New(db).GetEventByID(ctx, event.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionEvent{}, false, nil
	}
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	existing, err := sessionEventFromSQLC(row.ID, row.Sequence, row.TurnID, row.Type,
		row.AgentName, row.Content, row.Archived, row.Timestamp, s.owner.SessionID)
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	if !sessionEventsHaveIdenticalIdentity(existing, event) {
		return store.SessionEvent{}, false, fmt.Errorf("%w: %s", ErrEventIdentityCollision, event.ID)
	}
	return existing, true, nil
}

func (s *SessionDB) prepareTranscriptEntries(
	ctx context.Context, db projectionDB, projector *transcript.Projector,
	affected map[string]struct{}, assigned map[string][]store.SessionEvent,
) ([]preparedTranscriptEntry, error) {
	identities := make([]transcript.EntryIdentity, 0, len(affected))
	for key := range affected {
		identity, ok := projector.Identity(key)
		if !ok {
			return nil, fmt.Errorf("%w: missing affected identity %q", transcript.ErrProjectionCorrupt, key)
		}
		identities = append(identities, identity)
	}
	slices.SortFunc(identities, func(a, b transcript.EntryIdentity) int {
		return cmp.Compare(a.StartSequence, b.StartSequence)
	})
	entries := make([]preparedTranscriptEntry, 0, len(identities))
	reserved := make(map[string]struct{})
	for _, identity := range identities {
		candidate := identity
		if candidate.MessageID == "" {
			candidate.MessageID = candidate.BaseMessageID
		}
		entry, err := s.prepareAssignedEntry(ctx, db, projector.State().Generation, candidate, assigned[identity.Key])
		if err != nil {
			return nil, err
		}
		if entry.row.MessageJson.Valid && identity.MessageID == "" {
			identity.MessageID, err = allocateProjectionMessageID(
				ctx,
				db,
				identity.Key,
				identity.BaseMessageID,
				reserved,
			)
			if err != nil {
				return nil, err
			}
			if identity.MessageID != candidate.MessageID {
				entry, err = s.prepareAssignedEntry(
					ctx,
					db,
					projector.State().Generation,
					identity,
					assigned[identity.Key],
				)
				if err != nil {
					return nil, err
				}
			}
		}
		if identity.MessageID != "" {
			reserved[identity.MessageID] = struct{}{}
		}
		entry.row.MessageID = identity.MessageID
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *SessionDB) prepareAssignedEntry(
	ctx context.Context, db projectionDB, generation int64,
	identity transcript.EntryIdentity, added []store.SessionEvent,
) (preparedTranscriptEntry, error) {
	if identity.Kind == transcript.EntryKindAssistant {
		entry, count, parts, err := s.prepareAssistantEntry(ctx, db, generation, identity, added)
		if err != nil {
			return preparedTranscriptEntry{}, err
		}
		row, err := prepareProjectionEntry(&s.projectionEncoder, identity, entry)
		row.PartCount = int64(count)
		return preparedTranscriptEntry{row: row, parts: parts}, err
	}
	events, err := loadAssignedEvents(ctx, db, s.owner.SessionID, identity.Key)
	if err != nil {
		return preparedTranscriptEntry{}, err
	}
	entry, err := transcript.ProjectAssignedEntry(append(events, added...), identity)
	if err != nil {
		return preparedTranscriptEntry{}, fmt.Errorf("store: prepare transcript entry %q: %w", identity.Key, err)
	}
	row, err := prepareProjectionEntry(&s.projectionEncoder, identity, entry)
	return preparedTranscriptEntry{row: row}, err
}

func (s *SessionDB) commitTranscriptWrite(ctx context.Context, plan transcriptWritePlan) error {
	return store.ExecuteWriteOperation(
		ctx,
		s.db,
		"append session events and transcript",
		func(ctx context.Context, tx *store.WriteTx) error {
			state, err := loadProjectionState(ctx, tx)
			if err != nil {
				return err
			}
			next, err := nextEventSequence(ctx, tx)
			if err != nil {
				return err
			}
			if state != plan.before || next != plan.nextSequence {
				return errTranscriptChanged
			}
			for i, event := range plan.events {
				if err := insertSessionEvent(ctx, tx, event, plan.entryKeys[i]); err != nil {
					return err
				}
			}
			queries := sqlcgen.New(tx)
			for _, entry := range plan.entries {
				if err := queries.UpsertTranscriptEntry(ctx, entry.row); err != nil {
					return fmt.Errorf("store: upsert prepared transcript entry %q: %w", entry.row.EntryKey, err)
				}
				for _, part := range entry.parts {
					if err := queries.UpsertTranscriptEntryPart(ctx, part); err != nil {
						return fmt.Errorf("store: write part of transcript entry %q: %w", entry.row.EntryKey, err)
					}
				}
			}
			for toolKey, entryKey := range plan.toolRoutes {
				if err := queries.UpsertTranscriptToolRoute(ctx, sqlcgen.UpsertTranscriptToolRouteParams{
					ToolKey: toolKey, EntryKey: entryKey,
				}); err != nil {
					return fmt.Errorf("store: upsert transcript tool route %q: %w", toolKey, err)
				}
			}
			return persistProjectionState(ctx, tx, plan.after)
		},
	)
}
