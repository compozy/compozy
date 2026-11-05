package store

import "fmt"

type SteerDeliveryMode string

// SteerDeliveryModeValues supplies the canonical delivery vocabulary to public schemas.
func SteerDeliveryModeValues() []string {
	return []string{
		string(SteerDeliveryInjected),
		string(SteerDeliveryPendingInjection),
		string(SteerDeliveryPendingBoundary),
		string(SteerDeliveryAfterTurn),
		string(SteerDeliveryInterruptFallback),
	}
}

const (
	SteerDeliveryInjected          SteerDeliveryMode = "injected"
	SteerDeliveryPendingBoundary   SteerDeliveryMode = "pending_boundary"
	SteerDeliveryAfterTurn         SteerDeliveryMode = "after_turn"
	SteerDeliveryPendingInjection  SteerDeliveryMode = "pending_injection"
	SteerDeliveryInterruptFallback SteerDeliveryMode = "interrupt_fallback"
)

// Validate accepts known delivery modes and the unset value used before negotiation.
func (m SteerDeliveryMode) Validate() error {
	switch m {
	case "",
		SteerDeliveryPendingBoundary,
		SteerDeliveryAfterTurn,
		SteerDeliveryInjected,
		SteerDeliveryPendingInjection,
		SteerDeliveryInterruptFallback:
		return nil
	default:
		return fmt.Errorf("store: invalid steer delivery %q", m)
	}
}
