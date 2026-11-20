package sessiondb

import (
	"context"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

func queryInheritedTranscriptEntries(
	ctx context.Context, queries *sqlcgen.Queries, query transcript.PageQuery,
) ([]transcript.Entry, error) {
	through := *query.ThroughSequence
	throughStart := through
	if query.BeforeSequence > 0 {
		throughStart = min(throughStart, query.BeforeSequence-1)
	}
	entries := make([]transcript.Entry, 0, query.Limit+1)
	for len(entries) <= query.Limit {
		ask := query.Limit + 1 - len(entries)
		rows, err := queries.ListTranscriptEntriesThrough(ctx, sqlcgen.ListTranscriptEntriesThroughParams{
			ThroughStart: throughStart, RowLimit: int64(ask),
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			entry, err := inheritedTranscriptEntry(ctx, queries, row, through)
			if err != nil {
				return nil, err
			}
			if entry != nil {
				entries = append(entries, *entry)
			}
		}
		if len(rows) < ask {
			break
		}
		throughStart = rows[len(rows)-1].StartSequence - 1
	}
	return entries, nil
}

func inheritedTranscriptEntry(
	ctx context.Context, queries *sqlcgen.Queries, row sqlcgen.ListTranscriptEntriesThroughRow, through int64,
) (*transcript.Entry, error) {
	if row.UpdatedSequence <= through {
		entry, err := materializedTranscriptEntry(
			row.MessageJson, row.StartSequence, row.UpdatedSequence, row.EventType, row.MarkerJson,
		)
		return &entry, err
	}
	// Late tool updates can change an old entry. Fold only its events through the cut.
	stored, err := queries.ListTranscriptEntryEventsThrough(ctx, sqlcgen.ListTranscriptEntryEventsThroughParams{
		EntryKey: row.EntryKey, ThroughSequence: through,
	})
	if err != nil {
		return nil, err
	}
	identity := transcript.EntryIdentity{
		Key: row.EntryKey, Kind: transcript.EntryKind(row.Kind), LogicalID: row.LogicalID,
		TurnID: row.TurnID, BaseMessageID: row.BaseMessageID, MessageID: row.MessageID.String,
		StartSequence: row.StartSequence, UpdatedSequence: row.StartSequence,
	}
	events := make([]store.SessionEvent, 0, len(stored))
	for _, event := range stored {
		storedEvent, err := sessionEventFromSQLC(
			event.ID,
			event.Sequence,
			event.TurnID,
			event.Type,
			event.AgentName,
			event.Content,
			event.Archived,
			event.Timestamp,
			"",
		)
		if err != nil {
			return nil, err
		}
		identity.UpdatedSequence = storedEvent.Sequence
		events = append(events, storedEvent)
	}
	return transcript.ProjectAssignedEntry(events, identity)
}
