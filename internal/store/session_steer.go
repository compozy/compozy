package store

import "fmt"

type SteerDeliveryMode string

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
