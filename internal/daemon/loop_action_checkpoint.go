package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	deathResumeCheckpointEventLimit = 12
	deathResumePartialTextLimit     = 512
)

func (r *loopActionRuntime) captureDeathResumeCheckpoint(
	ctx context.Context,
	sessionID string,
) (*looppkg.DeathResumeCheckpoint, error) {
	events, err := r.sessions.Events(ctx, sessionID, store.EventQuery{
		Limit:   deathResumeCheckpointEventLimit,
		Archive: store.EventArchiveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: read dead Loop action checkpoint: %w", err)
	}
	if len(events) == 0 {
		return nil, nil
	}
	checkpoint := &looppkg.DeathResumeCheckpoint{
		SessionID:     sessionID,
		EventStartSeq: events[0].Sequence,
		EventEndSeq:   events[len(events)-1].Sequence,
		Partials:      make([]looppkg.DeathResumePartial, 0, len(events)),
	}
	for _, event := range events {
		partial, found, err := deathResumePartialFromEvent(event)
		if err != nil {
			return nil, err
		}
		if found {
			checkpoint.Partials = append(checkpoint.Partials, partial)
		}
	}
	if err := checkpoint.Validate(sessionID); err != nil {
		return nil, fmt.Errorf("daemon: validate dead Loop action checkpoint: %w", err)
	}
	return checkpoint, nil
}

func deathResumePartialFromEvent(
	event store.SessionEvent,
) (looppkg.DeathResumePartial, bool, error) {
	agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		return looppkg.DeathResumePartial{}, false, fmt.Errorf(
			"daemon: decode dead Loop action event %d: %w",
			event.Sequence,
			err,
		)
	}
	if !deathResumeProgressEvent(agentEvent.Type) {
		return looppkg.DeathResumePartial{}, false, nil
	}
	text := firstNonEmpty(
		agentEvent.Text,
		agentEvent.Title,
		agentEvent.Error,
		strings.TrimSpace(agentEvent.Action+" "+agentEvent.Resource),
	)
	text = diagnostics.RedactAndBound(text, deathResumePartialTextLimit)
	if text == "" {
		return looppkg.DeathResumePartial{}, false, nil
	}
	return looppkg.DeathResumePartial{
		Sequence: event.Sequence,
		Type:     agentEvent.Type,
		Text:     text,
	}, true, nil
}

func deathResumeProgressEvent(eventType string) bool {
	switch eventType {
	case acp.EventTypeAgentMessage,
		acp.EventTypeThought,
		acp.EventTypeToolCall,
		acp.EventTypeToolResult,
		acp.EventTypePlan,
		acp.EventTypeError:
		return true
	default:
		return false
	}
}
