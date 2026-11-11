package providerusage

import (
	"context"
	"time"
)

// Target is one profile-scoped provider route whose usage is collected.
type Target struct {
	ProfileID              string
	ProviderID             string // canonical Compozy provider ID
	CollectorID            string // collector family; empty means ProviderID (aliases resolve via runtime_provider)
	ProviderName           string // catalog display name
	AuthCommandFingerprint string
	RouteFingerprint       string // daemon-derived; never caller-supplied
	Status                 Status // Empty means eligible for collection.
	Error                  *UsageError
	Route                  ResolvedRoute
}

// Collector returns the collector family that owns this target.
func (t Target) Collector() string {
	if t.CollectorID != "" {
		return t.CollectorID
	}
	return t.ProviderID
}

// ResolvedRoute is the read-only launch context a collector may use.
type ResolvedRoute struct {
	Env            []string          // effective provider env (home/config dir), secrets excluded
	NativeCommand  string            // resolved native CLI path when the provider has one ("" if absent)
	BoundSecrets   map[string]string // slot -> value, only slots declared by the provider
	KeychainAccess bool              // operator consent recorded in config
}

// AccountSelector is implemented by native-CLI collectors whose account is chosen by a route
// variable. SelectorKey names the variable an alias's own launch command must assign for env, after
// the collector's precedence: the dedicated account variable even when the lookup would fall back
// to its HOME-derived default (Claude, Codex, Kimi CLI), or the variable a home-based collector reads
// (Cursor, Gemini). It must share that precedence with the credential lookup. "" means the
// collector has no such variable.
type AccountSelector interface {
	SelectorKey(env []string) string
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
	Wait        time.Duration // caller wait for results (zero returns immediately, max 30s); never bounds the flight
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

// TargetCredentialCache evicts the cached credential of one target that left the catalog.
type TargetCredentialCache interface {
	ForgetTarget(t Target)
}

type CredentialCache interface {
	// An empty profile ID clears every cached profile.
	ClearCredentials(profileID string)
}
