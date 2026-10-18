package store

import "strings"

// SessionAcceptedRoute is the binding ACP accepted, committed together with
// acp_session_id. It carries no command text: the command is identified only by its
// sha256 fingerprint, so resume can match a configured route without persisting secrets.
type SessionAcceptedRoute struct {
	// Attempt is the chain index that was accepted: 0 is the primary route.
	Attempt            int    `json:"attempt"`
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	AuthMode           string `json:"auth_mode"`
	HomePolicy         string `json:"home_policy"`
	CommandFingerprint string `json:"command_fingerprint"`
}

// Normalize returns a trimmed copy of the record.
func (r SessionAcceptedRoute) Normalize() SessionAcceptedRoute {
	return SessionAcceptedRoute{
		Attempt:            max(r.Attempt, 0),
		Provider:           strings.TrimSpace(r.Provider),
		Model:              strings.TrimSpace(r.Model),
		AuthMode:           strings.TrimSpace(r.AuthMode),
		HomePolicy:         strings.TrimSpace(r.HomePolicy),
		CommandFingerprint: strings.TrimSpace(r.CommandFingerprint),
	}
}

// CloneSessionAcceptedRoute isolates an accepted-route record.
func CloneSessionAcceptedRoute(route *SessionAcceptedRoute) *SessionAcceptedRoute {
	if route == nil {
		return nil
	}
	return new(route.Normalize())
}
