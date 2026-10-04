package task

import (
	"context"
	"errors"
	"time"
)

// RunLeaseSettlementReservationStore fences recovery at the repository's mutation boundary.
type RunLeaseSettlementReservationStore interface {
	ReserveRunLeaseSettlement(context.Context, string, string, time.Time, ActorContext) (func(), error)
}

// ReserveRunLeaseSettlement retains an already-executed result's ownership while
// the daemon retries persistence. The caller keeps the admission timestamp and
// releases the reservation after settlement or shutdown grace.
func (m *Service) ReserveRunLeaseSettlement(
	ctx context.Context,
	runID, claimToken string,
	at time.Time,
	actor ActorContext,
) (func(), error) {
	if err := validateLeaseRunToken(runID, claimToken, "lease_settlement"); err != nil {
		return nil, err
	}
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	if actor.Actor.Kind.Normalize() != ActorKindDaemon {
		return nil, ErrPermissionDenied
	}
	reservations, ok := m.store.(RunLeaseSettlementReservationStore)
	if !ok {
		return nil, errors.New("task: store does not support lease settlement reservations")
	}
	return reservations.ReserveRunLeaseSettlement(ctx, runID, claimToken, at, actor)
}
