package daemon

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
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
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	entry, ok, err := s.queue.ConsumeSessionSteer(ctx, sessionID, now)
	if err != nil || !ok {
		return acp.SteerInput{}, ok, err
	}
	return acp.SteerInput{
		Text:            entry.Text,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
	}, true, nil
}
