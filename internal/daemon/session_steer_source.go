package daemon

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type sessionSteerSource struct {
	queue store.SessionInputQueueStore
	now   func() time.Time
}

var _ acp.SteerSource = sessionSteerSource{}

func (s sessionSteerSource) ConsumeSteer(
	ctx context.Context,
	sessionID string,
) (acp.SteerInput, bool, error) {
	if s.queue == nil {
		return acp.SteerInput{}, false, nil
	}
	entry, ok, err := s.queue.ConsumeSessionSteer(ctx, sessionID, s.currentTime())
	if err != nil || !ok {
		return acp.SteerInput{}, ok, err
	}
	return acp.SteerInput{
		Text:            entry.Text,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
		MessageID:       entry.MessageID,
		TurnID:          entry.TurnID,
		EventID:         entry.EventID,
	}, true, nil
}

func (s sessionSteerSource) CompleteSteer(
	ctx context.Context,
	sessionID string,
	queueEntryID string,
) error {
	if s.queue == nil {
		return nil
	}
	return s.queue.MarkSessionInputSent(ctx, sessionID, queueEntryID, s.currentTime())
}

func (s sessionSteerSource) FailSteer(
	ctx context.Context,
	sessionID string,
	queueEntryID string,
	summary string,
) error {
	if s.queue == nil {
		return nil
	}
	return s.queue.MarkSessionInputFailed(ctx, sessionID, queueEntryID, summary, s.currentTime())
}

func (s sessionSteerSource) currentTime() time.Time {
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	return now
}
