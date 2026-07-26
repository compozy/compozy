package core

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

const (
	sessionStreamKeepAliveInterval = 20 * time.Second
	sessionStreamKeepAliveComment  = "keepalive"
)

type sessionEventStreamSubscriber interface {
	SubscribeSessionEvents(
		ctx context.Context,
		sessionID string,
		afterSequence int64,
	) (<-chan store.SessionEvent, func(), error)
}

type sessionEventWakeSubscriber interface {
	SubscribeSessionEventWakes(
		ctx context.Context,
		sessionID string,
	) (<-chan store.SessionEvent, func(), error)
}

type sessionEventStreamSubscription struct {
	events <-chan store.SessionEvent
	cancel func()
}

func (s sessionEventStreamSubscription) active() bool {
	return s.events != nil
}

func (s sessionEventStreamSubscription) cancelIfActive() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (h *BaseHandlers) subscribeSessionEventStream(
	ctx context.Context,
	sessionID string,
	afterSequence int64,
	frameMode string,
) (sessionEventStreamSubscription, error) {
	if frameMode == contract.SessionStreamFrameTranscript {
		wakeSubscriber, ok := h.Sessions.(sessionEventWakeSubscriber)
		if !ok || wakeSubscriber == nil {
			return sessionEventStreamSubscription{}, nil
		}
		events, cancel, err := wakeSubscriber.SubscribeSessionEventWakes(ctx, sessionID)
		return newSessionEventStreamSubscription(events, cancel, err)
	}

	subscriber, ok := h.Sessions.(sessionEventStreamSubscriber)
	if !ok || subscriber == nil {
		return sessionEventStreamSubscription{}, nil
	}
	events, cancel, err := subscriber.SubscribeSessionEvents(ctx, sessionID, afterSequence)
	return newSessionEventStreamSubscription(events, cancel, err)
}

func newSessionEventStreamSubscription(
	events <-chan store.SessionEvent,
	cancel func(),
	err error,
) (sessionEventStreamSubscription, error) {
	if err != nil {
		return sessionEventStreamSubscription{}, err
	}
	if cancel == nil {
		cancel = func() {}
	}
	return sessionEventStreamSubscription{events: events, cancel: cancel}, nil
}

func (h *BaseHandlers) writeKeepAlive(writer FlushWriter) bool {
	if err := WriteSSEComment(writer, sessionStreamKeepAliveComment); err != nil {
		h.logSSEWriteFailure("keepalive", err)
		return false
	}
	return true
}
