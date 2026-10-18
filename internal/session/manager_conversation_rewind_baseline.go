package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func refreshStaleConversationRewindBaseline(
	ctx context.Context,
	recorder EventRecorder,
) (store.ConversationRewindState, bool, error) {
	reader, ok := recorder.(store.ConversationRewindReader)
	if !ok {
		return store.ConversationRewindState{}, false, nil
	}
	state, found, err := reader.ConversationRewindState(ctx)
	if err != nil {
		return store.ConversationRewindState{}, false, fmt.Errorf(
			"session: read conversation rewind replay state: %w",
			err,
		)
	}
	if !found || !state.BaselineStale {
		return state, found, nil
	}
	refresher, ok := recorder.(store.ConversationRewindBaselineRefresher)
	if !ok {
		return store.ConversationRewindState{}, true, errors.New(
			"session: event recorder cannot refresh stale conversation rewind baseline",
		)
	}
	messages, err := assembleConversationRewindPrefix(ctx, recorder, state.CoveredThroughSequence)
	if err != nil {
		return store.ConversationRewindState{}, true, err
	}
	baseline, err := json.Marshal(messages)
	if err != nil {
		return store.ConversationRewindState{}, true, fmt.Errorf(
			"session: marshal refreshed conversation rewind baseline: %w", err,
		)
	}
	if err := refresher.RefreshConversationRewindBaseline(
		ctx, state.CoveredThroughSequence, string(baseline),
	); err != nil {
		return store.ConversationRewindState{}, true, fmt.Errorf(
			"session: refresh stale conversation rewind baseline: %w", err,
		)
	}
	state.MessagesJSON = string(baseline)
	state.BaselineStale = false
	return state, true, nil
}

func assembleConversationRewindPrefix(
	ctx context.Context,
	recorder store.EventReader,
	coveredThrough int64,
) ([]transcript.Message, error) {
	events, err := recorder.Query(ctx, store.EventQuery{
		BeforeSequence: coveredThrough + 1, Archive: store.EventArchiveUnarchived,
	})
	if err != nil {
		return nil, fmt.Errorf("session: query stale conversation rewind prefix: %w", err)
	}
	messages, err := transcript.Assemble(events)
	if err != nil {
		return nil, fmt.Errorf("session: assemble stale conversation rewind prefix: %w", err)
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	return messages, nil
}
