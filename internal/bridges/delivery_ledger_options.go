package bridges

import (
	"context"
	"errors"
)

var (
	// ErrDeliveryReconciliationPending reports that durable boot recovery has not completed.
	ErrDeliveryReconciliationPending = errors.New("bridges: delivery reconciliation pending")
)

// WithDeliveryLedgerStore enables durable registration and acknowledgement checkpoints.
func WithDeliveryLedgerStore(store DeliveryLedgerStore) DeliveryBrokerOption {
	return func(b *Broker) {
		b.ledgerStore = store
	}
}

// WithDeliveryBrokerRegistrationGate keeps registrations closed until boot reconciliation completes.
func WithDeliveryBrokerRegistrationGate() DeliveryBrokerOption {
	return func(b *Broker) {
		b.registrationsReady = false
	}
}

// CompleteDeliveryReconciliation admits new prompt-delivery registrations.
func (b *Broker) CompleteDeliveryReconciliation() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.registrationsReady = true
	b.mu.Unlock()
}

// CheckPromptDeliveryAdmission rejects prompt side effects until reconciliation completes.
func (b *Broker) CheckPromptDeliveryAdmission(ctx context.Context) error {
	if b == nil {
		return errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return errors.New("bridges: delivery admission context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.registrationsReady {
		return ErrDeliveryReconciliationPending
	}
	return nil
}
