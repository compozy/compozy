package loop

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	watchpkg "github.com/compozy/compozy/internal/loop/watch"
	"golang.org/x/text/unicode/norm"
)

// AdmissionClaim is one workspace-scoped watch redelivery tombstone.
type AdmissionClaim struct {
	WorkspaceID      WorkspaceID
	LoopName         string
	SourceKey        string
	EventKey         string
	LoopRunID        RunID
	ClaimedAt        time.Time
	ExpiresAt        time.Time
	SuppressedCount  int
	LastSuppressedAt *time.Time
}

// AdmissionIdentity is the stable source-delivery key claimed with run creation.
type AdmissionIdentity struct {
	SourceKey string
	EventKey  string
	Horizon   time.Duration
}

// NormalizeAdmissionIdentity canonicalizes the workspace-local suppression identity.
func NormalizeAdmissionIdentity(identity AdmissionIdentity) (AdmissionIdentity, error) {
	if !utf8.ValidString(identity.SourceKey) {
		return AdmissionIdentity{}, fmt.Errorf("%w: admission source_key must be valid UTF-8", ErrValidation)
	}
	identity.SourceKey = norm.NFC.String(strings.TrimSpace(identity.SourceKey))
	if identity.SourceKey == "" || len([]byte(identity.SourceKey)) > 256 {
		return AdmissionIdentity{}, fmt.Errorf("%w: admission source_key must contain 1 to 256 bytes", ErrValidation)
	}
	eventKey, err := watchpkg.NormalizeEventKey(identity.EventKey)
	if err != nil {
		return AdmissionIdentity{}, fmt.Errorf("%w: admission event_key: %v", ErrValidation, err)
	}
	identity.EventKey = eventKey
	if identity.Horizon <= 0 {
		return AdmissionIdentity{}, fmt.Errorf("%w: admission tombstone horizon must be positive", ErrValidation)
	}
	return identity, nil
}
