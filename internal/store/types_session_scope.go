package store

// SessionScopeState keeps the string-backed scope off the hot SessionInfo value.
type SessionScopeState struct {
	Scope SessionScope
}

// NewSessionScopeState returns a normalized scope snapshot.
func NewSessionScopeState(scope SessionScope) *SessionScopeState {
	if scope == "" {
		scope = SessionScopeWorkspace
	}
	return &SessionScopeState{Scope: scope}
}

// ScopeValue returns the catalog scope, defaulting zero values to workspace scope.
func (s SessionInfo) ScopeValue() SessionScope {
	if s.ScopeState == nil || s.ScopeState.Scope == "" {
		return SessionScopeWorkspace
	}
	return s.ScopeState.Scope
}

// SetScope replaces the catalog scope snapshot.
func (s *SessionInfo) SetScope(scope SessionScope) {
	s.ScopeState = NewSessionScopeState(scope)
}
