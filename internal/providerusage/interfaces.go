package providerusage

import (
	"context"
	"time"
)

// Target is one profile-scoped provider route whose usage is collected.
type Target struct {
	ProfileID        string
	ProviderID       string // canonical Compozy provider ID
	ProviderName     string // catalog display name
	RouteFingerprint string // daemon-derived; never caller-supplied
	Status           Status // Empty means eligible for collection.
	Error            *UsageError
	Route            ResolvedRoute
}

// ResolvedRoute is the read-only launch context a collector may use.
type ResolvedRoute struct {
	Env            []string          // effective provider env (home/config dir), secrets excluded
	NativeCommand  string            // resolved native CLI path when the provider has one ("" if absent)
	BoundSecrets   map[string]string // slot -> value, only slots declared by the provider
	KeychainAccess bool              // operator consent recorded in config
}

// TargetResolver lists the enabled targets for one profile.
type TargetResolver interface {
	Targets(ctx context.Context, profileID string) ([]Target, error)
}

// CredentialIdentity fences publication and keys cooldowns. Never serialized, logged, or exposed.
type CredentialIdentity struct {
	Digest    [32]byte `json:"-"` // HMAC-SHA256(process-random key, source kind || credential bytes)
	AccountID string   `json:"-"` // upstream account ID when readable without network; "" otherwise
}

// Same reports ownership equality: AccountIDs when both are non-empty (so an owner-CLI token renewal
// for the same account is not an ownership change), otherwise Digests.

// CredentialProbe reads the route's credential without network access.
type CredentialProbe interface {
	Identity(ctx context.Context, t Target, intent Intent) (CredentialIdentity, error) // *UsageError
}

// Collector produces one snapshot for a target using the credential the probe identified.
type Collector interface {
	ProviderID() string
	CredentialProbe
	// Errors may carry fenced partial data without MeasuredAt; same-owner last-good data takes precedence.
	Collect(ctx context.Context, t Target, intent Intent, cred CredentialIdentity) (Snapshot, error) // *UsageError
}

type Intent string

const (
	IntentBackground Intent = "background" // No Keychain reads; honors cooldowns.
	IntentExplicit   Intent = "explicit"   // Local operator action; may read Keychain.
)

// Service is the daemon-owned read model.
type Service interface {
	Envelope(ctx context.Context, profileID string) (Envelope, error)
	Refresh(ctx context.Context, req RefreshRequest) (Envelope, error)
	Subscribe(profileID string) (<-chan Revision, func())
}

type RefreshRequest struct {
	ProfileID   string
	ProviderIDs []string      // empty = all targets
	MaxAge      time.Duration // refresh only snapshots older than max(MaxAge, 30s floor)
	Wait        time.Duration // caller wait for results (default 15s, max 30s); never bounds the flight
	Intent      Intent        // decided by the API boundary from principal + requested intent
}

type Revision struct {
	Revision uint64 `json:"revision"`
	BootID   string `json:"boot_id"`
}

// PublicationValidator validates one-shot credentials without loading them again.
type PublicationValidator interface {
	ValidateIdentity(ctx context.Context, t Target, captured CredentialIdentity) error
}

type CredentialCache interface {
	// An empty profile ID clears every cached profile.
	ClearCredentials(profileID string)
}
