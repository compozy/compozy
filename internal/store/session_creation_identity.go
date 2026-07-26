package store

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrSessionCreationIdentityMismatch rejects adoption of a same-ID session with different creation truth.
	ErrSessionCreationIdentityMismatch = errors.New("store: session creation identity mismatch")
)

// SessionCreationIdentity is the immutable idempotency witness for one created session.
type SessionCreationIdentity struct {
	CreationProfileRef string
	PolicySpecDigest   string
	CreationDigest     string
}

// Validate requires the complete non-secret creation identity triple.
func (i SessionCreationIdentity) Validate() error {
	if strings.TrimSpace(i.CreationProfileRef) == "" {
		return fmt.Errorf("store: session creation profile ref is required")
	}
	if strings.TrimSpace(i.PolicySpecDigest) == "" {
		return fmt.Errorf("store: session policy spec digest is required")
	}
	if strings.TrimSpace(i.CreationDigest) == "" {
		return fmt.Errorf("store: session creation digest is required")
	}
	return nil
}

// SessionCreationRegistration reports whether an atomic registration created or matched a row.
type SessionCreationRegistration struct {
	SessionID string
	Created   bool
	Identity  SessionCreationIdentity
}
