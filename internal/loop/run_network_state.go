package loop

import "github.com/compozy/agh/internal/network/participation"

// RunNetworkState keeps immutable participation off the hot loop Run value.
type RunNetworkState struct {
	NetworkSpec participation.Spec `json:"network_spec"`
}

// NetworkSpecSnapshot returns the immutable snapshot, defaulting uninitialized in-memory runs to Local.
func (r Run) NetworkSpecSnapshot() participation.Spec {
	if r.RunNetworkState == nil || r.NetworkSpec == (participation.Spec{}) {
		return participation.LocalSpec()
	}
	return r.NetworkSpec
}

// SetNetworkSpec replaces the immutable participation snapshot.
func (r *Run) SetNetworkSpec(spec participation.Spec) {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	r.RunNetworkState = &RunNetworkState{NetworkSpec: spec}
}
