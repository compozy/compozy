package acp

import (
	"context"
	"errors"
	"io"

	"github.com/compozy/compozy/internal/store"
)

func (a *activePromptState) failIngest(cause error) {
	if errors.Is(cause, ErrIngestClosed) {
		return
	}
	a.ingest.Close(cause)
	if a.cancel != nil {
		a.cancel()
	}
}

func (p *AgentProcess) forwardIngestEvents(active *activePromptState) {
	defer close(active.events)
	// Process death must not discard events already accepted for durable append.
	// The session manager owns this consumer until the closed gate is drained.
	for {
		event, err := active.ingest.Next(context.Background())
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			event = AgentEvent{
				Type: EventTypeError, SessionID: p.SessionID, TurnID: active.turnID,
				Timestamp: timeNowUTC(), Error: err.Error(),
				Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: err.Error()},
			}
		}
		active.events <- event
		if err != nil {
			return
		}
	}
}

// IngestStats exposes the active transport backlog to session supervision.
func (p *AgentProcess) IngestStats() IngestStats {
	if p == nil {
		return IngestStats{}
	}
	active := p.currentPrompt()
	if active == nil || active.ingest == nil {
		return IngestStats{}
	}
	return active.ingest.Stats()
}
