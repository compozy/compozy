package store

import "fmt"

type SteerDeliveryMode string

func SteerDeliveryModeValues() []string {
	return []string{
		string(SteerDeliveryInjected),
		string(SteerDeliveryPendingInjection),
		string(SteerDeliveryInterruptFallback),
	}
}

const (
	SteerDeliveryInjected          SteerDeliveryMode = "injected"
	SteerDeliveryPendingInjection  SteerDeliveryMode = "pending_injection"
	SteerDeliveryInterruptFallback SteerDeliveryMode = "interrupt_fallback"
)

func (m SteerDeliveryMode) Validate() error {
	switch m {
	case "", SteerDeliveryInjected, SteerDeliveryPendingInjection, SteerDeliveryInterruptFallback:
		return nil
	default:
		return fmt.Errorf("store: invalid steer delivery %q", m)
	}
}
