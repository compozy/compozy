package sessiondb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

// maxCachedTranscriptFolds bounds retained builders per session. Appends almost
// always extend the active assistant entry; the second slot absorbs late tool
// results routed to the entry the active one replaced.
const maxCachedTranscriptFolds = 2

// transcriptFoldCache keeps incremental assistant-entry reductions between
// appends. It is owned by the session writer goroutine and needs no lock.
// Folds touched by a write attempt are staged and published only after that
// transaction commits; a fold taken by a failed attempt is discarded. Reuse is
// also validated against the durable assigned-event prefix, so an archive or a
// rewind can only cause a full replay, never a stale entry.
type transcriptFoldCache struct {
	generation int64
	folds      []*transcript.EntryFold
	staged     []*transcript.EntryFold
}

func (c *transcriptFoldCache) reset() {
	c.generation = 0
	c.folds = nil
	c.staged = nil
}

// begin starts one write attempt, dropping folds staged by a failed attempt.
func (c *transcriptFoldCache) begin() {
	c.staged = nil
}

// commit publishes the folds staged by the committed write.
func (c *transcriptFoldCache) commit() {
	c.folds = append(c.staged, c.folds...)
	c.staged = nil
	if len(c.folds) > maxCachedTranscriptFolds {
		c.folds = c.folds[:maxCachedTranscriptFolds]
	}
}

func (c *transcriptFoldCache) take(generation int64, identity transcript.EntryIdentity) *transcript.EntryFold {
	if c.generation != generation {
		c.reset()
		c.generation = generation
		return nil
	}
	if fold := takeMatchingFold(&c.staged, identity); fold != nil {
		return fold
	}
	return takeMatchingFold(&c.folds, identity)
}

func (c *transcriptFoldCache) stage(fold *transcript.EntryFold) {
	c.staged = append([]*transcript.EntryFold{fold}, c.staged...)
}

func takeMatchingFold(folds *[]*transcript.EntryFold, identity transcript.EntryIdentity) *transcript.EntryFold {
	for index, fold := range *folds {
		if fold.Matches(identity) {
			*folds = append((*folds)[:index], (*folds)[index+1:]...)
			return fold
		}
	}
	return nil
}

// writeWithTranscriptFolds runs one projected write and publishes its folds
// only when the transaction commits.
func (s *SessionDB) writeWithTranscriptFolds(
	ctx context.Context,
	fn func(context.Context, *store.WriteTx) error,
) error {
	err := store.ExecuteWrite(ctx, s.db, func(ctx context.Context, tx *store.WriteTx) error {
		s.transcriptFolds.begin()
		return fn(ctx, tx)
	})
	if err != nil {
		s.transcriptFolds.begin()
		return err
	}
	s.transcriptFolds.commit()
	return nil
}

// projectAssistantEntry reduces one assistant entry, replaying only events the
// cached fold has not applied when its durable prefix is unchanged.
func (s *SessionDB) projectAssistantEntry(
	ctx context.Context,
	tx *store.WriteTx,
	generation int64,
	identity transcript.EntryIdentity,
) (*transcript.Entry, error) {
	fold, err := s.extendCachedFold(ctx, tx, generation, identity)
	if err != nil {
		return nil, err
	}
	if fold == nil {
		fold, err = s.replayFold(ctx, tx, identity)
		if err != nil {
			return nil, err
		}
	}
	s.transcriptFolds.stage(fold)
	return fold.Entry(identity), nil
}

func (s *SessionDB) extendCachedFold(
	ctx context.Context,
	tx *store.WriteTx,
	generation int64,
	identity transcript.EntryIdentity,
) (*transcript.EntryFold, error) {
	fold := s.transcriptFolds.take(generation, identity)
	if fold == nil {
		return nil, nil
	}
	queries := sqlcgen.New(tx)
	bounds, err := queries.GetTranscriptEntryEventBounds(ctx, sqlcgen.GetTranscriptEntryEventBoundsParams{
		TranscriptEntryKey: identity.Key,
		ThroughSequence:    fold.LastSequence(),
	})
	if err != nil {
		return nil, fmt.Errorf("store: read transcript entry %q bounds: %w", identity.Key, err)
	}
	if bounds.EventCount != int64(fold.Applied()) || bounds.MaxSequence != fold.LastSequence() {
		return nil, nil
	}
	rows, err := queries.ListEventsForTranscriptEntryAfter(ctx, sqlcgen.ListEventsForTranscriptEntryAfterParams{
		TranscriptEntryKey: identity.Key,
		AfterSequence:      fold.LastSequence(),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query new events for transcript entry %q: %w", identity.Key, err)
	}
	tail := make([]store.SessionEvent, 0, len(rows))
	for _, row := range rows {
		event, err := sessionEventFromSQLC(
			row.ID, row.Sequence, row.TurnID, row.Type, row.AgentName,
			row.Content, row.Archived, row.Timestamp, s.owner.SessionID,
		)
		if err != nil {
			return nil, err
		}
		tail = append(tail, event)
	}
	if err := fold.Append(tail); err != nil {
		return nil, nil
	}
	return fold, nil
}

func (s *SessionDB) replayFold(
	ctx context.Context,
	tx *store.WriteTx,
	identity transcript.EntryIdentity,
) (*transcript.EntryFold, error) {
	events, err := loadAssignedEvents(ctx, tx, s.owner.SessionID, identity.Key)
	if err != nil {
		return nil, err
	}
	fold, err := transcript.NewEntryFold(identity)
	if err != nil {
		return nil, fmt.Errorf("store: project transcript entry %q: %w", identity.Key, err)
	}
	if err := fold.Append(events); err != nil {
		return nil, fmt.Errorf("store: project transcript entry %q: %w", identity.Key, err)
	}
	return fold, nil
}
