package sessiondb

import (
	"context"
	"errors"

	"github.com/compozy/agh/internal/store"
)

const historyEventChunkSize = 256

type eventQueryFunc func(context.Context, store.EventQuery) ([]store.SessionEvent, error)

func queryTurnHistory(
	ctx context.Context,
	query store.EventQuery,
	queryEvents eventQueryFunc,
) ([]store.TurnHistory, error) {
	if queryEvents == nil {
		return nil, errors.New("store: history event query function is required")
	}
	if query.Limit <= 0 {
		return queryAllTurnHistory(ctx, query, queryEvents)
	}
	return queryBoundedTurnHistory(ctx, query, queryEvents)
}

func queryAllTurnHistory(
	ctx context.Context,
	query store.EventQuery,
	queryEvents eventQueryFunc,
) ([]store.TurnHistory, error) {
	queryForEvents := query
	queryForEvents.Limit = 0
	queryForEvents.AfterSequence = 0
	events, err := queryEvents(ctx, queryForEvents)
	if err != nil {
		return nil, err
	}
	return groupedTurnHistory(events, query), nil
}

func queryBoundedTurnHistory(
	ctx context.Context,
	query store.EventQuery,
	queryEvents eventQueryFunc,
) ([]store.TurnHistory, error) {
	queryForEvents := query
	queryForEvents.Limit = historyChunkLimit(query.Limit)
	queryForEvents.AfterSequence = 0
	upperBound := query.BeforeSequence
	events := make([]store.SessionEvent, 0, queryForEvents.Limit)
	historyQuery := query
	historyQuery.Limit = 0
	for {
		queryForEvents.BeforeSequence = upperBound
		chunk, err := queryEvents(ctx, queryForEvents)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		events = append(chunk, events...)
		turns := groupedTurnHistory(events, historyQuery)
		if len(turns) > query.Limit || len(chunk) < queryForEvents.Limit {
			return limitTurnHistory(turns, query.Limit), nil
		}
		upperBound = chunk[0].Sequence
	}
	return limitTurnHistory(groupedTurnHistory(events, historyQuery), query.Limit), nil
}

func groupedTurnHistory(events []store.SessionEvent, query store.EventQuery) []store.TurnHistory {
	turns := make([]store.TurnHistory, 0)
	indexByTurnID := make(map[string]int, len(events))
	for _, event := range events {
		if idx, ok := indexByTurnID[event.TurnID]; ok {
			turns[idx].Events = append(turns[idx].Events, event)
			continue
		}

		indexByTurnID[event.TurnID] = len(turns)
		turns = append(turns, store.TurnHistory{
			TurnID: event.TurnID,
			Events: []store.SessionEvent{event},
		})
	}
	return limitTurnHistory(filterTurnHistoryAfterSequence(turns, query.AfterSequence), query.Limit)
}

func limitTurnHistory(turns []store.TurnHistory, limit int) []store.TurnHistory {
	if limit <= 0 || len(turns) <= limit {
		return turns
	}
	return append([]store.TurnHistory(nil), turns[len(turns)-limit:]...)
}

func filterTurnHistoryAfterSequence(turns []store.TurnHistory, afterSequence int64) []store.TurnHistory {
	if afterSequence <= 0 {
		return turns
	}
	filtered := make([]store.TurnHistory, 0, len(turns))
	for _, turn := range turns {
		minSequence, maxSequence := turnSequenceBounds(turn.Events)
		if maxSequence <= afterSequence {
			continue
		}
		if minSequence <= afterSequence {
			continue
		}
		filtered = append(filtered, turn)
	}
	return filtered
}

func turnSequenceBounds(events []store.SessionEvent) (int64, int64) {
	if len(events) == 0 {
		return 0, 0
	}
	minSequence := events[0].Sequence
	maxSequence := events[0].Sequence
	for _, event := range events[1:] {
		if event.Sequence < minSequence {
			minSequence = event.Sequence
		}
		if event.Sequence > maxSequence {
			maxSequence = event.Sequence
		}
	}
	return minSequence, maxSequence
}

func historyChunkLimit(turnLimit int) int {
	chunkLimit := turnLimit * 8
	if chunkLimit < historyEventChunkSize {
		return historyEventChunkSize
	}
	return chunkLimit
}
