// Package gateway owns durable exposure policy and reconciliation contracts.
package gateway

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

var (
	// ErrExposureRefused reports a fail-closed policy refusal.
	ErrExposureRefused = errors.New("gateway exposure refused")
	// ErrConsentRequired reports that public operator UI consent is missing.
	ErrConsentRequired = errors.New("gateway consent required")
	// ErrGenerationConflict reports an optimistic concurrency conflict.
	ErrGenerationConflict = errors.New("gateway generation conflict")
	// ErrStaleGeneration reports that an external effect completed for obsolete intent.
	ErrStaleGeneration = errors.New("gateway stale generation")
	// ErrTierProviderConflict reports that a tier already has an enabled provider.
	ErrTierProviderConflict = errors.New("gateway tier already has an enabled provider")
)

// RefusalError carries an operator-readable cause and correction.
type RefusalError struct {
	Cause string
	Fix   string
}

func (e RefusalError) Error() string {
	return fmt.Sprintf("%v: %s; fix: %s", ErrExposureRefused, e.Cause, e.Fix)
}

// Unwrap allows callers to match ErrExposureRefused.
func (e RefusalError) Unwrap() error { return ErrExposureRefused }

// Tier identifies one independently managed reachability tier.
type Tier string

const (
	TierPrivate Tier = "private"
	TierPublic  Tier = "public"
)

// Validate reports unsupported tiers.
func (t Tier) Validate() error {
	switch t {
	case TierPrivate, TierPublic:
		return nil
	default:
		return fmt.Errorf("gateway: invalid tier %q", t)
	}
}

// Surface identifies a route inventory whose reachability is managed as a unit.
type Surface string

const (
	SurfaceOperatorUI     Surface = "operator_ui"
	SurfaceWebhookIngress Surface = "webhook_ingress"
)

// Validate reports unsupported surfaces.
func (s Surface) Validate() error {
	switch s {
	case SurfaceOperatorUI, SurfaceWebhookIngress:
		return nil
	default:
		return fmt.Errorf("gateway: invalid surface %q", s)
	}
}

// DesiredState records operator intent.
type DesiredState string

const (
	DesiredEnabled  DesiredState = "enabled"
	DesiredDisabled DesiredState = "disabled"
)

// Validate reports unsupported desired states.
func (s DesiredState) Validate() error {
	switch s {
	case DesiredEnabled, DesiredDisabled:
		return nil
	default:
		return fmt.Errorf("gateway: invalid desired state %q", s)
	}
}

// ProviderObservedState records the last reconciled provider state.
type ProviderObservedState string

const (
	ProviderDown         ProviderObservedState = "down"
	ProviderEstablishing ProviderObservedState = "establishing"
	ProviderUp           ProviderObservedState = "up"
	ProviderDegraded     ProviderObservedState = "degraded"
)

// SurfaceObservedState records whether a surface is actually served.
type SurfaceObservedState string

const (
	SurfaceOff SurfaceObservedState = "off"
	SurfaceOn  SurfaceObservedState = "on"
)

// ProviderHealth describes provider reachability without exposing secrets.
type ProviderHealth string

const (
	HealthDown     ProviderHealth = "down"
	HealthHealthy  ProviderHealth = "healthy"
	HealthDegraded ProviderHealth = "degraded"
)

// TransitionTarget identifies exactly one durable state row.
type TransitionTarget string

const (
	TargetProvider TransitionTarget = "provider"
	TargetSurface  TransitionTarget = "surface"
)

// TransitionRequest changes one provider activation or surface exposure row.
type TransitionRequest struct {
	Target             TransitionTarget
	Tier               Tier
	Desired            DesiredState
	ExpectedGeneration uint64
	Provider           ProviderIdentity
	Surface            Surface
	Consent            bool
}

// ProviderIdentity records live-registry provider identity at transition time.
type ProviderIdentity struct {
	Name            string
	InstallSource   string
	DigestConfirmed string
	ConfirmedAt     time.Time
}

// Validate ensures a provider identity can be persisted safely.
func (p ProviderIdentity) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("gateway: provider name is required")
	}
	if strings.TrimSpace(p.InstallSource) == "" {
		return errors.New("gateway: provider install source is required")
	}
	return nil
}

func (p ProviderIdentity) normalized() ProviderIdentity {
	p.Name = strings.TrimSpace(p.Name)
	p.InstallSource = strings.TrimSpace(p.InstallSource)
	return p
}

// ProviderActivation is the durable desired/observed provider state for one tier.
type ProviderActivation struct {
	ProviderName string
	Tier         Tier
	Desired      DesiredState
	Observed     ProviderObservedState
	Generation   uint64
	LastHealthAt time.Time
	LastError    string
}

// SurfaceExposure is the durable desired/observed surface state for one tier.
type SurfaceExposure struct {
	Surface     Surface
	Tier        Tier
	Desired     DesiredState
	Observed    SurfaceObservedState
	Generation  uint64
	ConsentedAt time.Time
}

// DeviceSession is the non-secret device state used by status projection.
type DeviceSession struct {
	ID            string
	Name          string
	ActorKind     ActorKind
	PairingOrigin string
	RevokeEpoch   uint64
	CreatedAt     time.Time
	LastSeenAt    time.Time
	RevokedAt     time.Time
}

// Snapshot is the complete operator-global durable gateway state.
type Snapshot struct {
	Providers []ProviderActivation
	Surfaces  []SurfaceExposure
	Devices   []DeviceSession
}

// ExposurePlan groups the desired state by independently reconciled tier.
type ExposurePlan struct {
	Tiers []TierPlan
}

// TierPlan is the desired reachability for one tier.
type TierPlan struct {
	Tier     Tier
	Provider *ProviderActivation
	Surfaces []SurfaceExposure
	Enabled  bool
}

// Reachability is provider output that remains untrusted until verification.
type Reachability struct {
	Tier      Tier
	Endpoints []AdvertisedEndpoint
	Health    ProviderHealth
	Reason    string
}

// RuntimeTier is the in-memory listener and advertisement state.
type RuntimeTier struct {
	Tier       Tier
	Bound      netip.AddrPort
	Endpoints  []AdvertisedEndpoint
	Surfaces   []Surface
	Advertised bool
}

// Refusal is the latest fail-closed explanation exposed in status.
type Refusal struct {
	Cause string
	Fix   string
}

// Status is the user- and agent-readable gateway projection.
type Status struct {
	Enabled   bool
	Changed   bool
	Tiers     []TierStatus
	Surfaces  []SurfaceStatus
	Providers []ProviderStatus
	Addresses []AddressStatus
	Devices   []DeviceSession
	Bindings  []IngressBindingStatus
	Refusal   *Refusal
}

// TierStatus summarizes desired and runtime state for a tier.
type TierStatus struct {
	Tier            Tier
	Desired         DesiredState
	Observed        string
	ListenerAddress string
	Advertised      bool
}

// SurfaceStatus exposes desired versus observed surface state.
type SurfaceStatus struct {
	Surface    Surface
	Tier       Tier
	Desired    DesiredState
	Observed   SurfaceObservedState
	Generation uint64
}

// ProviderStatus exposes provider health for one tier.
type ProviderStatus struct {
	Name       string
	Tier       Tier
	Desired    DesiredState
	Observed   ProviderObservedState
	Generation uint64
	Health     ProviderHealth
	Cause      string
}

// AddressStatus exposes only verified runtime addresses as live.
type AddressStatus struct {
	Tier    Tier
	Address string
	Live    bool
}
