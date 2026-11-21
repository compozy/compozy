package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

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
// also fenced by generation, which changes atomically on archive, clear, and rewind.
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
		c.folds = slices.Delete(c.folds, maxCachedTranscriptFolds, len(c.folds))
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
			*folds = slices.Delete(*folds, index, index+1)
			return fold
		}
	}
	return nil
}

func (s *SessionDB) prepareAssistantEntry(
	ctx context.Context,
	tx projectionDB,
	generation int64,
	identity transcript.EntryIdentity,
	added []store.SessionEvent,
) (*transcript.Entry, int, []sqlcgen.UpsertTranscriptEntryPartParams, error) {
	fold, err := s.extendCachedFold(ctx, tx, generation, identity)
	if err != nil {
		return nil, 0, nil, err
	}
	if fold == nil {
		fold, added, err = s.restorePartFold(ctx, tx, identity, added)
		if err != nil {
			return nil, 0, nil, err
		}
	}
	changes, err := fold.AppendPartChanges(added)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("store: extend prepared transcript entry %q: %w", identity.Key, err)
	}
	s.transcriptFolds.stage(fold)
	message, count := fold.PartHeader()
	if count == 0 {
		return nil, 0, nil, nil
	}
	parts, err := prepareProjectionParts(identity.Key, identity.UpdatedSequence, changes)
	return &transcript.Entry{Message: message}, count, parts, err
}

func (s *SessionDB) extendCachedFold(
	ctx context.Context,
	tx projectionDB,
	generation int64,
	identity transcript.EntryIdentity,
) (*transcript.EntryFold, error) {
	fold := s.transcriptFolds.take(generation, identity)
	if fold == nil {
		return nil, nil
	}
	queries := sqlcgen.New(tx)
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

func (s *SessionDB) restorePartFold(
	ctx context.Context,
	tx projectionDB,
	identity transcript.EntryIdentity,
	added []store.SessionEvent,
) (*transcript.EntryFold, []store.SessionEvent, error) {
	fold, err := transcript.NewEntryPartFold(identity)
	if err != nil {
		return nil, nil, fmt.Errorf("store: start transcript entry %q: %w", identity.Key, err)
	}
	count, err := sqlcgen.New(tx).GetTranscriptEntryPartCount(ctx, identity.Key)
	if errors.Is(err, sql.ErrNoRows) {
		return fold, added, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("store: read transcript entry %q part count: %w", identity.Key, err)
	}
	events, err := loadAssignedEvents(ctx, tx, s.owner.SessionID, identity.Key)
	if err != nil {
		return nil, nil, err
	}
	// An inline snapshot needs its full set of part rows on the first new write.
	if count == 0 {
		return fold, append(events, added...), nil
	}
	if err := fold.Append(events); err != nil {
		return nil, nil, fmt.Errorf("store: restore transcript entry %q: %w", identity.Key, err)
	}
	return fold, added, nil
}
