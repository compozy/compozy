package inputqueue

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type steerDeliveryStore interface {
	CancelPendingSessionSteer(context.Context, string, string, time.Time) error
	SettlePendingSessionSteer(
		context.Context,
		string,
		string,
		store.SteerDeliveryMode,
		time.Time,
	) (store.SessionInputQueueEntry, bool, error)
	ReserveSessionSteer(context.Context, string, string, time.Time) (store.SessionInputQueueEntry, bool, error)
	ResolveSessionSteer(
		context.Context,
		string,
		string,
		store.SteerDeliveryMode,
		time.Time,
	) (store.SessionInputQueueEntry, error)
}

func (s *Service) CancelPendingSteer(ctx context.Context, sessionID, entryID string) error {
	owner, ok := s.store.(steerDeliveryStore)
	if !ok {
		return errors.New("inputqueue: steer delivery store is required")
	}
	return owner.CancelPendingSessionSteer(ctx, sessionID, entryID, s.now())
}

func (s *Service) SettlePendingSteer(
	ctx context.Context, sessionID, entryID string, delivery store.SteerDeliveryMode,
) (store.SessionInputQueueEntry, bool, error) {
	owner, ok := s.store.(steerDeliveryStore)
	if !ok {
		return store.SessionInputQueueEntry{}, false, errors.New("inputqueue: steer delivery store is required")
	}
	return owner.SettlePendingSessionSteer(ctx, sessionID, entryID, delivery, s.now())
}

func (s *Service) ReserveSteer(
	ctx context.Context,
	sessionID, entryID string,
) (store.SessionInputQueueEntry, bool, error) {
	owner, ok := s.store.(steerDeliveryStore)
	if !ok {
		return store.SessionInputQueueEntry{}, false, errors.New("inputqueue: steer delivery store is required")
	}
	return owner.ReserveSessionSteer(ctx, sessionID, entryID, s.now())
}

func (s *Service) ResolveSteer(
	ctx context.Context, sessionID, entryID string, delivery store.SteerDeliveryMode,
) (store.SessionInputQueueEntry, error) {
	owner, ok := s.store.(steerDeliveryStore)
	if !ok {
		return store.SessionInputQueueEntry{}, errors.New("inputqueue: steer delivery store is required")
	}
	return owner.ResolveSessionSteer(ctx, sessionID, entryID, delivery, s.now())
}
