package gateway

import (
	"context"
	"time"
)

// ActorKind identifies the operator client represented by a device session.
type ActorKind string

const (
	ActorKindOperatorDevice ActorKind = "operator_device"
	ActorKindCLIProfile     ActorKind = "cli_profile"
)

// PairingSource is the listener trust boundary for pairing operations.
type PairingSource string

const (
	PairingSourceLocal   PairingSource = "local"
	PairingSourcePrivate PairingSource = "private"
	PairingSourcePublic  PairingSource = "public"
)

// PairingRequest asks a trusted surface to mint a one-time artifact.
type PairingRequest struct {
	Source PairingSource
}

// PairingArtifact is returned only to the already-trusted minting caller.
type PairingArtifact struct {
	Artifact   string
	ManualCode string
	ExpiresAt  time.Time
}

// RedeemRequest exchanges one artifact or manual code for one named device session.
type RedeemRequest struct {
	Artifact string
	// ManualCode is the hand-typed 6-digit alias of Artifact; either one spends the same entry.
	ManualCode string
	Name       string
	Kind       ActorKind
	Source     PairingSource
	// Credential is optional for browser redemption. CLI callers provision it locally first.
	Credential string
}

// IssuedCredential contains the only server response that may carry a raw device credential.
type IssuedCredential struct {
	Device     DeviceSession
	Credential string
}

// RevokeResult distinguishes a state change from an idempotent no-op.
type RevokeResult struct {
	Device   DeviceSession
	Changed  bool
	Canceled int
}

// RemoveResult reports the deleted device record and the lifecycle work it required.
type RemoveResult struct {
	Device   DeviceSession
	Revoked  bool
	Canceled int
}

// StoredDeviceSession is the daemon-private persistence record.
type StoredDeviceSession struct {
	Session   DeviceSession
	TokenHash string
}

// DeviceMutationEvent identifies one durable device lifecycle transition.
type DeviceMutationEvent struct {
	Device    DeviceSession
	ActorKind string
	ActorID   string
	RemovedAt time.Time
}

// DeviceEventSink records durable device lifecycle events before the record changes.
type DeviceEventSink interface {
	RecordDeviceRemoved(context.Context, DeviceMutationEvent) error
}

// DeviceStore owns durable device-session transitions.
type DeviceStore interface {
	CreateDevice(context.Context, StoredDeviceSession) error
	FindDeviceByTokenHash(context.Context, string, time.Time) (StoredDeviceSession, error)
	ListDevices(context.Context) ([]DeviceSession, error)
	GetDevice(context.Context, string) (DeviceSession, error)
	RenameDevice(context.Context, string, string) (DeviceSession, error)
	RenameDeviceForActor(context.Context, string, uint64, string, string) (DeviceSession, error)
	RevokeDevice(context.Context, string, time.Time) (DeviceSession, bool, error)
	RevokeDeviceForActor(context.Context, string, uint64, string, time.Time) (DeviceSession, bool, error)
	DeleteDevice(context.Context, string) (DeviceSession, error)
	DeleteDeviceForActor(context.Context, string, uint64, string) (DeviceSession, error)
	RevalidateDeviceEpoch(context.Context, string, uint64) error
}

// DeviceAuthenticator authenticates request credentials and single-use stream tickets.
type DeviceAuthenticator interface {
	Authenticate(context.Context, string) (DeviceSession, error)
	ConsumeStreamTicket(context.Context, string) (DeviceSession, error)
	AcquireMutation(context.Context, string, uint64) (context.Context, error)
	RevalidateForCommit(context.Context, string, uint64) error
}
