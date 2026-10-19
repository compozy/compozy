package daemon

import (
	"context"
	"math"

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
	history, err := reader.History(ctx, sessionID, nativeSessionPageProbe(query))
	if err != nil {
		return nil, nativeSessionPage{}, err
	}
	page := nativeSessionPage{Limit: query.Limit}
	if len(history) <= query.Limit {
		return history, page, nil
	}
	page.HasMore = true
	history = history[len(history)-query.Limit:]
	page.NextBeforeSequence = nativeTurnFirstSequence(history[0])
	return history, page, nil
}

// readNativeSessionHistoryForwardPage cuts the oldest turns after the cursor from the store's
// unbounded after-cursor read, the same read an omitted limit performed before paging existed: the
// store excludes every turn that began at or before the cursor only when it sees the whole log.
func readNativeSessionHistoryForwardPage(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	query store.EventQuery,
) ([]store.TurnHistory, nativeSessionPage, error) {
	unbounded := query
	unbounded.Limit = 0
	history, err := reader.History(ctx, sessionID, unbounded)
	if err != nil {
		return nil, nativeSessionPage{}, err
	}
	page := nativeSessionPage{Limit: query.Limit}
	if len(history) <= query.Limit {
		return history, page, nil
	}
	history = history[:query.Limit]
	page.HasMore = true
	page.NextAfterSequence = nativeTurnLastSequence(history[len(history)-1])
	return history, page, nil
}

func nativeTurnLastSequence(turn store.TurnHistory) int64 {
	var last int64
	for _, event := range turn.Events {
		last = max(last, event.Sequence)
	}
	return last
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
