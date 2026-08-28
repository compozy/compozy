package daemon

import (
	"context"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
)

// daemonBootCallsStoreSupport keeps the general daemon boot fake compatible
// with the fail-closed calls composition contract. Unexpected call mutations
// still reach the nil embedded interfaces and panic instead of being hidden.
type daemonBootCallsStoreSupport struct {
	callspkg.Store
	callspkg.MailboxStore
}

func (daemonBootCallsStoreSupport) ReconcileActivations(context.Context, time.Time) ([]string, error) {
	return nil, nil
}

func (daemonBootCallsStoreSupport) ListQueuedActivationRunIDs(context.Context, int) ([]string, error) {
	return nil, nil
}

func (daemonBootCallsStoreSupport) ListDueCalls(context.Context, time.Time, int) ([]callspkg.CallRecord, error) {
	return nil, nil
}

func (daemonBootCallsStoreSupport) ListPendingDeliveries(
	context.Context,
	string,
	int,
) ([]callspkg.DeliveryRecord, error) {
	return nil, nil
}

func (daemonBootCallsStoreSupport) FenceSessionReap(
	context.Context,
	callspkg.SessionReapFence,
) (callspkg.SessionReapFenceResult, error) {
	return callspkg.SessionReapFenceResult{Allowed: true}, nil
}

func (daemonBootCallsStoreSupport) FailPendingDeliveriesForRecipient(
	context.Context,
	string,
	string,
	time.Time,
) error {
	return nil
}

func (daemonBootCallsStoreSupport) FinalizeReapedSession(
	context.Context,
	string,
	string,
	time.Time,
) error {
	return nil
}

func (*fakeSessionManager) QueuedInputDeliveryStatus(
	context.Context,
	string,
	string,
) (session.InputDeliveryStatus, error) {
	return session.InputDeliveryStatus{}, nil
}
