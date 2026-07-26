package store

import "github.com/compozy/agh/internal/network/participation"

// SessionNetworkState keeps immutable participation off the hot SessionInfo value.
type SessionNetworkState struct {
	NetworkSpec participation.Spec
}

// NetworkSpecSnapshot returns the immutable snapshot, defaulting uninitialized in-memory sessions to Local.
func (s SessionInfo) NetworkSpecSnapshot() participation.Spec {
	if s.SessionNetworkState == nil || s.NetworkSpec == (participation.Spec{}) {
		return participation.LocalSpec()
	}
	return s.NetworkSpec
}

// SetNetworkSpec replaces the immutable participation snapshot.
func (s *SessionInfo) SetNetworkSpec(spec participation.Spec) {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	s.SessionNetworkState = &SessionNetworkState{NetworkSpec: spec}
}
