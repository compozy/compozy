package daemon

import (
	"cmp"
	"context"
	"math"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

const (
	// nativeSessionEventsDefaultLimit bounds one session event page when the caller omits limit.
	nativeSessionEventsDefaultLimit = 50
	// nativeSessionHistoryDefaultLimit bounds one turn history page when the caller omits limit.
	nativeSessionHistoryDefaultLimit = 10

	nativeToolsPageKey        = "page"
	nativeToolsEventsPageKey  = "events_page"
	nativeToolsHistoryPageKey = "history_page"
)

// nativeSessionPage reports whether a bounded session read left more rows and how to continue.
type nativeSessionPage struct {
	Limit              int   `json:"limit"`
	HasMore            bool  `json:"has_more"`
	NextAfterSequence  int64 `json:"next_after_sequence,omitzero"`
	NextBeforeSequence int64 `json:"next_before_sequence,omitzero"`
}

type nativeSessionPageReader interface {
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	History(ctx context.Context, id string, query store.EventQuery) ([]store.TurnHistory, error)
}

func withNativeSessionLimit(query store.EventQuery, defaultLimit int) store.EventQuery {
	if query.Limit <= 0 {
		query.Limit = defaultLimit
	}
	return query
}

// nativeSessionPageProbe reads one extra row so has_more is exact instead of inferred from a full page.
func nativeSessionPageProbe(query store.EventQuery) store.EventQuery {
	if query.Limit < math.MaxInt {
		query.Limit++
	}
	return query
}

// readNativeSessionEventPage returns one event page: ascending after the cursor when after_sequence
// is set, otherwise the newest events before before_sequence (or the end of the log).
func readNativeSessionEventPage(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.SessionEvent, nativeSessionPage, error) {
	events, err := reader.Events(ctx, sessionID, nativeSessionPageProbe(query))
	if err != nil {
		return nil, nativeSessionPage{}, err
	}
	page := nativeSessionPage{Limit: query.Limit}
	if len(events) <= query.Limit {
		return events, page, nil
	}
	page.HasMore = true
	if query.AfterSequence > 0 || query.Forward {
		events = events[:query.Limit]
		page.NextAfterSequence = events[len(events)-1].Sequence
		return events, page, nil
	}
	events = events[len(events)-query.Limit:]
	page.NextBeforeSequence = events[0].Sequence
	return events, page, nil
}

// readNativeSessionHistoryPage returns the oldest turns starting after after_sequence when that
// cursor is set, otherwise the newest turns of one bounded window; older turns continue through
// before_sequence.
func readNativeSessionHistoryPage(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.TurnHistory, nativeSessionPage, error) {
	if query.AfterSequence > 0 || query.Forward {
		return readNativeSessionHistoryForwardPage(ctx, reader, sessionID, query)
	}
	return readNativeSessionHistoryBackwardPage(ctx, reader, sessionID, query)
}

func readNativeSessionHistoryBackwardPage(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.TurnHistory, nativeSessionPage, error) {
	turns := make([]store.TurnHistory, 0)
	seen := make(map[string]bool)
	scan := query
	scan.Limit = nativeSessionEventsDefaultLimit
	for {
		chunk, err := reader.Events(ctx, sessionID, scan)
		if err != nil {
			return nil, nativeSessionPage{}, err
		}
		for _, event := range chunk {
			if seen[event.TurnID] {
				continue
			}
			seen[event.TurnID] = true
			firstQuery := query
			firstQuery.Forward, firstQuery.BeforeSequence = true, 0
			firstQuery.TurnID, firstQuery.Limit = event.TurnID, 1
			first, err := reader.Events(ctx, sessionID, firstQuery)
			if err != nil {
				return nil, nativeSessionPage{}, err
			}
			if len(first) != 0 {
				turns = append(turns, store.TurnHistory{TurnID: event.TurnID, Events: first})
			}
		}
		slices.SortFunc(turns, func(left, right store.TurnHistory) int {
			return cmp.Compare(left.Events[0].Sequence, right.Events[0].Sequence)
		})
		if len(chunk) < scan.Limit || (len(turns) > query.Limit &&
			chunk[0].Sequence <= turns[len(turns)-query.Limit].Events[0].Sequence) {
			break
		}
		scan.BeforeSequence = chunk[0].Sequence
	}
	page := nativeSessionPage{Limit: query.Limit}
	if len(turns) > query.Limit {
		turns = turns[len(turns)-query.Limit:]
		page.HasMore = true
		page.NextBeforeSequence = nativeTurnFirstSequence(turns[0])
	}
	for index := range turns {
		turnQuery := query
		turnQuery.Forward, turnQuery.BeforeSequence = true, 0
		turnQuery.TurnID, turnQuery.Limit = turns[index].TurnID, nativeSessionEventsDefaultLimit
		events, err := readNativeSessionTurnEvents(ctx, reader, sessionID, turnQuery)
		if err != nil {
			return nil, nativeSessionPage{}, err
		}
		turns[index].Events = events
	}
	return turns, page, nil
}

// Forward history is ordered by turn start, even when events from different turns interleave.
func readNativeSessionHistoryForwardPage(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.TurnHistory, nativeSessionPage, error) {
	page := nativeSessionPage{Limit: query.Limit}
	turns := make([]store.TurnHistory, 0)
	seen := make(map[string]bool)
	scan := query
	scan.Forward = true
	scan.Limit = nativeSessionEventsDefaultLimit
	for {
		events, err := reader.Events(ctx, sessionID, scan)
		if err != nil {
			return nil, nativeSessionPage{}, err
		}
		for _, event := range events {
			if seen[event.TurnID] {
				continue
			}
			seen[event.TurnID] = true
			firstQuery := scan
			firstQuery.TurnID, firstQuery.AfterSequence, firstQuery.Limit = event.TurnID, 0, 1
			first, err := reader.Events(ctx, sessionID, firstQuery)
			if err != nil {
				return nil, nativeSessionPage{}, err
			}
			if len(first) == 0 || first[0].Sequence <= query.AfterSequence {
				continue
			}
			if len(turns) == query.Limit {
				page.HasMore = true
				page.NextAfterSequence = nativeTurnFirstSequence(turns[len(turns)-1])
				return turns, page, nil
			}
			turnQuery := scan
			turnQuery.TurnID, turnQuery.AfterSequence = event.TurnID, 0
			turnEvents, err := readNativeSessionTurnEvents(ctx, reader, sessionID, turnQuery)
			if err != nil {
				return nil, nativeSessionPage{}, err
			}
			turns = append(turns, store.TurnHistory{TurnID: event.TurnID, Events: turnEvents})
		}
		if len(events) < scan.Limit {
			return turns, page, nil
		}
		scan.AfterSequence = events[len(events)-1].Sequence
	}
}

func readNativeSessionTurnEvents(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	events := make([]store.SessionEvent, 0)
	for {
		chunk, err := reader.Events(ctx, sessionID, query)
		if err != nil {
			return nil, err
		}
		events = append(events, chunk...)
		if len(chunk) < query.Limit {
			return events, nil
		}
		query.AfterSequence = chunk[len(chunk)-1].Sequence
	}
}

func nativeTurnFirstSequence(turn store.TurnHistory) int64 {
	if len(turn.Events) == 0 {
		return 0
	}
	first := turn.Events[0].Sequence
	for _, event := range turn.Events[1:] {
		first = min(first, event.Sequence)
	}
	return first
}
