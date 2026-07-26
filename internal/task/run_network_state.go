package task

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// RunNetworkState keeps immutable participation and wake-only correlation off the hot Run value.
type RunNetworkState struct {
	NetworkSpec            participation.Spec `json:"network_spec"`
	NetworkWakeID          string             `json:"network_wake_id,omitempty"`
	NetworkTargetSessionID string             `json:"network_target_session_id,omitempty"`
	NetworkOwnerKey        string             `json:"network_owner_key,omitempty"`
}

// NetworkSpecSnapshot returns the immutable snapshot, defaulting uninitialized in-memory runs to Local.
func (r Run) NetworkSpecSnapshot() participation.Spec {
	if r.RunNetworkState == nil || r.NetworkSpec == (participation.Spec{}) {
		return participation.LocalSpec()
	}
	return r.NetworkSpec
}

// NetworkWakeCorrelation returns the optional durable wake correlation tuple.
func (r Run) NetworkWakeCorrelation() (wakeID, targetSessionID, ownerKey string) {
	if r.RunNetworkState == nil {
		return "", "", ""
	}
	return r.NetworkWakeID, r.NetworkTargetSessionID, r.NetworkOwnerKey
}

// SetNetworkState replaces the immutable participation and wake correlation state.
func (r *Run) SetNetworkState(
	spec participation.Spec,
	wakeID string,
	targetSessionID string,
	ownerKey string,
) {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	r.RunNetworkState = &RunNetworkState{
		NetworkSpec:            spec,
		NetworkWakeID:          strings.TrimSpace(wakeID),
		NetworkTargetSessionID: strings.TrimSpace(targetSessionID),
		NetworkOwnerKey:        strings.TrimSpace(ownerKey),
	}
}
