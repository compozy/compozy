package store

import "github.com/compozy/compozy/internal/network/participation"

// SessionMetaPlacementState keeps scope and network placement off the hot SessionMeta value.
// Embedding preserves the flat metadata JSON contract.
type SessionMetaPlacementState struct {
	Scope                SessionScope        `json:"scope"`
	NetworkParticipation *participation.Spec `json:"network_participation"`
}

// NewSessionMetaPlacement returns one immutable metadata placement snapshot.
func NewSessionMetaPlacement(scope SessionScope, spec participation.Spec) *SessionMetaPlacementState {
	if scope == "" {
		scope = SessionScopeWorkspace
	}
	return &SessionMetaPlacementState{
		Scope:                scope,
		NetworkParticipation: participation.CloneSpec(spec),
	}
}

// ScopeValue returns the durable scope, defaulting old in-memory zero values to workspace scope.
func (m SessionMeta) ScopeValue() SessionScope {
	if m.SessionMetaPlacementState == nil || m.Scope == "" {
		return SessionScopeWorkspace
	}
	return m.Scope
}

// SetPlacement replaces the immutable scope and network participation snapshot.
func (m *SessionMeta) SetPlacement(scope SessionScope, spec participation.Spec) {
	m.SessionMetaPlacementState = NewSessionMetaPlacement(scope, spec)
}
