package session

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var deriveReplayPayloadTypes = []string{
	acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry, acp.EventTypeAgentMessage, acp.EventTypeThought,
	acp.EventTypeToolCall, acp.EventTypeToolResult, events.TranscriptMarkerCreated, events.TranscriptMarkerRedacted,
}

func eventsAfter(events []store.SessionEvent, sequence int64) []store.SessionEvent {
	return events[sort.Search(len(events), func(i int) bool { return events[i].Sequence > sequence }):]
}

func readDeriveReplayWindow(
	ctx context.Context, recorder EventReadCloser, baseline []transcript.Message, shapes []store.SessionEvent,
	covered, cut int64, budget int,
) ([]transcript.Message, int, error) {
	if cut < covered {
		messages, err := assembleDeriveMessages(baseline, shapes, covered, cut)
		return messages, 0, err
	}
	turns := deriveTurns(shapes)
	carried := carriedTurnEvents(shapes, turns, covered, cut)
	layout, err := transcript.ReplayLayout(ctx, carried)
	if err != nil {
		return nil, 0, err
	}
	messages := append([]transcript.Message(nil), baseline...)
	for _, position := range layout {
		messages = append(messages, position.Message)
	}
	if len(layout) == 0 {
		return transcript.Prune(messages, transcript.PruneOptions{Dedup: true}), 0, nil
	}
	// A canonical message, even after capping, cannot encode smaller than the empty shape.
	// Keep one more than can fit so omitted shapes cannot influence the chosen suffix.
	maxMessages := budget/replayMessageBytes(transcript.Message{}) + 1
	first := max(0, len(layout)-maxMessages)
	tail := carried[sort.Search(len(carried), func(i int) bool { return carried[i].Sequence >= layout[first].Sequence }):]
	if err := fillDeriveReplayPayloads(ctx, recorder, tail, shapes, cut); err != nil {
		return nil, 0, err
	}
	hydrated, err := transcript.AssembleContext(ctx, tail)
	if err != nil {
		return nil, 0, err
	}
	if err := applyDeriveReplayPayloads(messages, hydrated, len(baseline)+first); err != nil {
		return nil, 0, err
	}
	if first > 0 {
		if err := hydrateDeriveFirstUser(ctx, recorder, messages, len(baseline), first); err != nil {
			return nil, 0, err
		}
		// Earlier messages matter only for their order, the pinned request, and the omission count.
		first += len(baseline)
		copy(messages[first:], transcript.Prune(messages[first:], transcript.PruneOptions{Dedup: true}))
		return messages, first, nil
	}
	return transcript.Prune(messages, transcript.PruneOptions{Dedup: true}), 0, nil
}

func fillDeriveReplayPayloads(
	ctx context.Context, recorder EventReadCloser, carried, shapes []store.SessionEvent, cut int64,
) error {
	if len(carried) == 0 {
		return nil
	}
	limit := 0
	for _, shape := range eventsAfter(shapes, carried[0].Sequence-1) {
		if shape.Sequence > cut {
			break
		}
		if slices.Contains(deriveReplayPayloadTypes, shape.Type) {
			limit++
		}
	}
	if limit == 0 {
		return nil
	}
	body, err := recorder.Query(ctx, store.EventQuery{
		Archive: store.EventArchiveUnarchived, Types: deriveReplayPayloadTypes, BeforeSequence: cut + 1, Limit: limit,
	})
	if err != nil {
		return fmt.Errorf("session: read carried event payloads: %w", err)
	}
	for i, j := 0, 0; i < len(carried) && j < len(body); {
		switch {
		case carried[i].Sequence < body[j].Sequence:
			i++
		case carried[i].Sequence > body[j].Sequence:
			j++
		default:
			carried[i] = body[j]
			i++
			j++
		}
	}
	return nil
}

func applyDeriveReplayPayloads(messages, hydrated []transcript.Message, first int) error {
	type key struct {
		id   string
		role transcript.Role
	}
	byMessage := make(map[key]transcript.Message, len(hydrated))
	for _, message := range hydrated {
		byMessage[key{message.ID, message.Role}] = message
	}
	for index := first; index < len(messages); index++ {
		message := messages[index]
		full, ok := byMessage[key{message.ID, message.Role}]
		if !ok {
			return fmt.Errorf("session: replay message %q is missing from its event window", message.ID)
		}
		messages[index] = full
	}
	return nil
}

func hydrateDeriveFirstUser(
	ctx context.Context, recorder EventReadCloser, messages []transcript.Message, baseline, first int,
) error {
	index := slices.IndexFunc(
		messages,
		func(message transcript.Message) bool { return message.Role == transcript.RoleUser },
	)
	if index < baseline || index >= baseline+first {
		return nil
	}
	rows, err := recorder.Query(ctx, store.EventQuery{
		Archive: store.EventArchiveUnarchived, ID: messages[index].ID, Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("session: read pinned first replay message: %w", err)
	}
	hydrated, err := transcript.AssembleContext(ctx, rows)
	if err != nil {
		return err
	}
	if len(hydrated) != 1 || hydrated[0].Role != transcript.RoleUser {
		return fmt.Errorf("session: missing pinned replay message %q", messages[index].ID)
	}
	messages[index] = hydrated[0]
	return nil
}
