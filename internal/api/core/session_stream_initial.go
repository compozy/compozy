package core

import (
	"context"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) sessionStreamInitialEvents(
	ctx context.Context,
	sessionID string,
	query store.EventQuery,
	options sessionStreamOptions,
) ([]store.SessionEvent, error) {
	if options.frameMode == contract.SessionStreamFrameTranscript {
		event, err := h.Sessions.LatestSessionEventByType(
			ctx,
			sessionID,
			session.EventTypeGoalSnapshotChanged,
		)
		if err != nil {
			return nil, err
		}
		if event != nil && event.Sequence > query.AfterSequence {
			return []store.SessionEvent{*event}, nil
		}
		return []store.SessionEvent{}, nil
	}
	if !sessionStreamNeedsInitialEvents(options) {
		return nil, nil
	}
	return h.Sessions.Events(ctx, sessionID, query)
}

func sessionStreamNeedsInitialEvents(options sessionStreamOptions) bool {
	return options.frameMode == contract.SessionStreamFrameRaw
}
