package gateway

import "errors"

var (
	// ErrDeviceUnauthenticated masks unknown, malformed, mismatched, and revoked credentials.
	ErrDeviceUnauthenticated = errors.New("gateway device unauthenticated")
	// ErrDeviceNotFound reports a device identifier outside the operator inventory.
	ErrDeviceNotFound = errors.New("gateway device not found")
	// ErrDeviceRevoked reports an epoch fence invalidated by revocation.
	ErrDeviceRevoked = errors.New("gateway device revoked")
	// ErrPairingInvalid masks malformed and unknown pairing artifacts.
	ErrPairingInvalid = errors.New("gateway pairing invalid")
	// ErrPairingExpired reports a known artifact whose TTL elapsed.
	ErrPairingExpired = errors.New("gateway pairing expired")
	// ErrPairingSpent reports a known artifact that already admitted one device.
	ErrPairingSpent = errors.New("gateway pairing spent")
	// ErrPairingLimit reports that the bounded pending-artifact set is full.
	ErrPairingLimit = errors.New("gateway pairing limit reached")
	// ErrPairingForbidden reports an attempt to mint or redeem from an untrusted tier.
	ErrPairingForbidden = errors.New("gateway pairing forbidden")
	// ErrStreamTicketInvalid masks unknown, malformed, expired, spent, and revoked-device tickets.
	ErrStreamTicketInvalid = errors.New("gateway stream ticket invalid")
	// ErrAuthRateLimited reports too many failed authentication attempts from one source.
	ErrAuthRateLimited = errors.New("gateway authentication rate limited")
	// ErrProviderTrustStale reports that a persisted provider confirmation no longer matches the live registry.
	ErrProviderTrustStale = errors.New("gateway provider trust stale")
	// ErrProviderNotFound reports a provider name outside the active tier inventory.
	ErrProviderNotFound = errors.New("gateway provider not found")
	// ErrDigestConfirmationRequired reports a provider control digest awaiting operator consent.
	ErrDigestConfirmationRequired = errors.New("gateway digest confirmation required")
	// ErrIngressSubjectNotFound reports an unknown webhook or bridge ingress subject.
	ErrIngressSubjectNotFound = errors.New("gateway ingress subject not found")
	// ErrIngressTargetUnavailable reports a subject whose daemon-local ingress target is invalid.
	ErrIngressTargetUnavailable = errors.New("gateway ingress target unavailable")
	// ErrIngressForbidden reports an ingress mutation outside the caller's workspace boundary.
	ErrIngressForbidden = errors.New("gateway ingress forbidden")
	// ErrIngressRateLimited reports a public sender exceeding one endpoint/source budget.
	ErrIngressRateLimited = errors.New("gateway ingress rate limited")
	// ErrLocalOnlyOperation reports an operation unavailable through a remote profile.
	ErrLocalOnlyOperation = errors.New("gateway local-only operation")
)
