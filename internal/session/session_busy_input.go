package session

import (
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// BusyInputState describes the live session's follow-up behavior.
type BusyInputState struct {
	DefaultMode     string                  `json:"default_mode"`
	SteerCapability config.SteerCapability  `json:"steer_capability"`
	SteerDelivery   store.SteerDeliveryMode `json:"steer_delivery,omitempty"`
}

func (s *Session) busyInputStateLocked(caps acp.Caps) *BusyInputState {
	if s.followUpMode == nil {
		return nil
	}
	capability := caps.SteerCapability
	if capability == "" {
		capability = config.SteerCapabilityNone
	}
	return &BusyInputState{
		DefaultMode: s.followUpMode(), SteerCapability: capability, SteerDelivery: s.steerDelivery,
	}
}
