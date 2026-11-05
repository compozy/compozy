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

type staleDispatchStore interface {
	FailStaleDispatchingSessionInputs(context.Context, time.Time) (int, error)
}

// FailStaleDispatches settles leases left by a previous daemon process.
func (s *Service) FailStaleDispatches(ctx context.Context) (int, error) {
	owner, ok := s.store.(staleDispatchStore)
	if !ok {
		return 0, errors.New("inputqueue: stale dispatch store is required")
	}
	return owner.FailStaleDispatchingSessionInputs(ctx, s.now())
}

// boundarySteerStore owns durable supersession and its recovery read.
type boundarySteerStore interface {
	SupersedeSessionBoundarySteers(context.Context, string, string, string, time.Time) ([]string, error)
	ListSupersededSessionSteers(context.Context, string, string) ([]store.SessionInputQueueEntry, error)
}

// SupersedeBoundarySteers retires guidance superseded by an empty operator interrupt.
func (s *Service) SupersedeBoundarySteers(
	ctx context.Context,
	sessionID, turnID, replacementID string,
) ([]string, error) {
	owner, ok := s.store.(boundarySteerStore)
	if !ok {
		return nil, nil
	}
	return owner.SupersedeSessionBoundarySteers(ctx, sessionID, turnID, replacementID, s.now())
}

// ListSupersededSteers returns a bounded page for idempotent transcript repair.
func (s *Service) ListSupersededSteers(
	ctx context.Context,
	sessionID, afterID string,
) ([]store.SessionInputQueueEntry, error) {
	owner, ok := s.store.(boundarySteerStore)
	if !ok {
		return nil, nil
	}
	return owner.ListSupersededSessionSteers(ctx, sessionID, afterID)
}
