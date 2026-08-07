package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	defaultPairingTTL                   = 5 * time.Minute
	defaultPairingLimit                 = 32
	defaultStreamTicketTTL              = 30 * time.Second
	defaultStreamTicketLimit            = 256
	defaultConnectionTerminationTimeout = 5 * time.Second
)

// DeviceService owns pairing, authentication, device inventory, tickets, and live revocation.
type DeviceService struct {
	store       DeviceStore
	pairings    *PairingStore
	tickets     *streamTicketStore
	connections ConnectionRegistry
	mutations   *mutationGate
	now         func() time.Time
	entropy     io.Reader
}

type DeviceServiceOption func(*deviceServiceConfig) error

type deviceServiceConfig struct {
	now               func() time.Time
	entropy           io.Reader
	pairingTTL        time.Duration
	pairingLimit      int
	streamTicketTTL   time.Duration
	streamTicketLimit int
	connections       ConnectionRegistry
}

func NewDeviceService(store DeviceStore, opts ...DeviceServiceOption) (*DeviceService, error) {
	if store == nil {
		return nil, errors.New("gateway: device store is required")
	}
	config := deviceServiceConfig{
		now: func() time.Time { return time.Now().UTC() }, entropy: defaultEntropy(),
		pairingTTL: defaultPairingTTL, pairingLimit: defaultPairingLimit,
		streamTicketTTL: defaultStreamTicketTTL, streamTicketLimit: defaultStreamTicketLimit,
		connections: NewConnectionRegistry(),
	}
	for _, option := range opts {
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	pairings, err := newPairingStore(config.pairingLimit, config.pairingTTL, config.now, config.entropy)
	if err != nil {
		return nil, err
	}
	tickets, err := newStreamTicketStore(
		config.streamTicketLimit,
		config.streamTicketTTL,
		config.now,
		config.entropy,
	)
	if err != nil {
		return nil, err
	}
	return &DeviceService{
		store: store, pairings: pairings, tickets: tickets,
		connections: config.connections, mutations: newMutationGate(), now: config.now, entropy: config.entropy,
	}, nil
}

func WithDeviceClock(now func() time.Time) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if now == nil {
			return errors.New("gateway: device clock is required")
		}
		config.now = now
		return nil
	}
}

func WithDeviceEntropy(entropy io.Reader) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if entropy == nil {
			return errors.New("gateway: device entropy is required")
		}
		config.entropy = entropy
		return nil
	}
}

func WithPairingLimits(maxPending int, ttl time.Duration) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		config.pairingLimit = maxPending
		config.pairingTTL = ttl
		return nil
	}
}

func WithStreamTicketLimits(maxEntries int, ttl time.Duration) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		config.streamTicketLimit = maxEntries
		config.streamTicketTTL = ttl
		return nil
	}
}

func WithConnectionRegistry(registry ConnectionRegistry) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if registry == nil {
			return errors.New("gateway: connection registry is required")
		}
		config.connections = registry
		return nil
	}
}

func (s *DeviceService) MintPairing(_ context.Context, req PairingRequest) (PairingArtifact, error) {
	if req.Source != PairingSourceLocal && req.Source != PairingSourcePrivate {
		return PairingArtifact{}, ErrPairingForbidden
	}
	return s.pairings.Mint(req.Source)
}

// Active reports that the daemon initialized the authentication authority.
func (s *DeviceService) Active(context.Context) (bool, error) {
	return true, nil
}

func (s *DeviceService) RedeemPairing(ctx context.Context, req RedeemRequest) (IssuedCredential, error) {
	if req.Source != PairingSourceLocal && req.Source != PairingSourcePrivate {
		return IssuedCredential{}, ErrPairingForbidden
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || !validActorKind(req.Kind) {
		return IssuedCredential{}, ErrPairingInvalid
	}

	var issued IssuedCredential
	err := s.pairings.Redeem(req.Artifact, func(pairingSource PairingSource) error {
		credential := strings.TrimSpace(req.Credential)
		if credential == "" {
			var generateErr error
			credential, generateErr = generateDeviceCredential(s.entropy)
			if generateErr != nil {
				return generateErr
			}
		} else if !validOpaqueSecret(credential, deviceCredentialPrefix) {
			return ErrPairingInvalid
		}
		deviceID, generateErr := generateDeviceID(s.entropy)
		if generateErr != nil {
			return generateErr
		}
		now := s.now()
		device := DeviceSession{
			ID: deviceID, Name: name, ActorKind: req.Kind, PairingOrigin: string(pairingSource),
			CreatedAt: now,
		}
		if createErr := s.store.CreateDevice(ctx, StoredDeviceSession{
			Session: device, TokenHash: hashCredential(credential),
		}); createErr != nil {
			return fmt.Errorf("gateway: create paired device: %w", createErr)
		}
		issued = IssuedCredential{Device: device, Credential: credential}
		return nil
	})
	if err != nil {
		return IssuedCredential{}, err
	}
	return issued, nil
}

func (s *DeviceService) Authenticate(ctx context.Context, credential string) (DeviceSession, error) {
	stored, err := s.store.FindDeviceByTokenHash(ctx, hashCredential(credential), s.now())
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			constantTimeCredentialMatch(credential, "")
			return DeviceSession{}, ErrDeviceUnauthenticated
		}
		return DeviceSession{}, fmt.Errorf("gateway: find device credential: %w", err)
	}
	valid := validOpaqueSecret(credential, deviceCredentialPrefix)
	matched := constantTimeCredentialMatch(credential, stored.TokenHash)
	if !valid || !matched || !stored.Session.RevokedAt.IsZero() {
		return DeviceSession{}, ErrDeviceUnauthenticated
	}
	return stored.Session, nil
}

func (s *DeviceService) ListDevices(ctx context.Context) ([]DeviceSession, error) {
	devices, err := s.store.ListDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("gateway: list devices: %w", err)
	}
	if devices == nil {
		return []DeviceSession{}, nil
	}
	return devices, nil
}

func (s *DeviceService) RenameDevice(ctx context.Context, deviceID, name string) (DeviceSession, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DeviceSession{}, errors.New("gateway: device name is required")
	}
	var (
		device DeviceSession
		err    error
	)
	if actor, ok := DeviceFromContext(ctx); ok {
		device, err = s.store.RenameDeviceForActor(ctx, actor.ID, actor.RevokeEpoch, deviceID, name)
	} else {
		device, err = s.store.RenameDevice(ctx, deviceID, name)
	}
	if err != nil {
		return DeviceSession{}, fmt.Errorf("gateway: rename device: %w", err)
	}
	return device, nil
}

func (s *DeviceService) RevokeDevice(ctx context.Context, deviceID string) (RevokeResult, error) {
	operationCtx := ctx
	if releaseMutationForDevice(ctx, deviceID) {
		operationCtx = context.WithoutCancel(ctx)
	}
	finishRevocation, err := s.mutations.beginRevocation(operationCtx, deviceID)
	if err != nil {
		return RevokeResult{}, err
	}
	revocationCommitted := false
	defer func() { finishRevocation(revocationCommitted) }()

	var (
		device   DeviceSession
		changed  bool
		storeErr error
	)
	now := s.now()
	if actor, ok := DeviceFromContext(operationCtx); ok {
		device, changed, storeErr = s.store.RevokeDeviceForActor(
			operationCtx,
			actor.ID,
			actor.RevokeEpoch,
			deviceID,
			now,
		)
	} else {
		device, changed, storeErr = s.store.RevokeDevice(operationCtx, deviceID, now)
	}
	if storeErr != nil {
		return RevokeResult{}, fmt.Errorf("gateway: revoke device: %w", storeErr)
	}
	revocationCommitted = true
	terminationCtx, cancelTermination := context.WithTimeout(operationCtx, defaultConnectionTerminationTimeout)
	defer cancelTermination()
	canceled, cancelErr := s.connections.CancelDevice(terminationCtx, deviceID)
	result := RevokeResult{Device: device, Changed: changed, Canceled: canceled}
	if cancelErr != nil {
		return result, fmt.Errorf("gateway: terminate revoked device connections: %w", cancelErr)
	}
	return result, nil
}

func (s *DeviceService) MintStreamTicket(ctx context.Context, deviceID string) (StreamTicket, error) {
	device, ok := DeviceFromContext(ctx)
	if ok {
		if device.ID != deviceID {
			return StreamTicket{}, ErrDeviceUnauthenticated
		}
		if err := s.store.RevalidateDeviceEpoch(ctx, device.ID, device.RevokeEpoch); err != nil {
			return StreamTicket{}, ErrDeviceUnauthenticated
		}
	} else {
		var err error
		device, err = s.store.GetDevice(ctx, deviceID)
		if err != nil {
			return StreamTicket{}, fmt.Errorf("gateway: get stream-ticket device: %w", err)
		}
	}
	if !device.RevokedAt.IsZero() {
		return StreamTicket{}, ErrDeviceUnauthenticated
	}
	return s.tickets.mint(device)
}

func (s *DeviceService) ConsumeStreamTicket(ctx context.Context, ticket string) (DeviceSession, error) {
	entry, err := s.tickets.consume(ticket)
	if err != nil {
		return DeviceSession{}, err
	}
	if err := s.store.RevalidateDeviceEpoch(ctx, entry.deviceID, entry.epoch); err != nil {
		return DeviceSession{}, ErrStreamTicketInvalid
	}
	device, err := s.store.GetDevice(ctx, entry.deviceID)
	if err != nil || !device.RevokedAt.IsZero() {
		return DeviceSession{}, ErrStreamTicketInvalid
	}
	return device, nil
}

func (s *DeviceService) RevalidateForCommit(ctx context.Context, deviceID string, epoch uint64) error {
	if err := s.mutations.validate(deviceID); err != nil {
		return fmt.Errorf("gateway: validate device mutation gate: %w", err)
	}
	if err := s.store.RevalidateDeviceEpoch(ctx, deviceID, epoch); err != nil {
		return fmt.Errorf("gateway: revalidate device epoch: %w", err)
	}
	return nil
}

// AcquireMutation admits one remote mutation and carries its revocation-aware lease in the returned context.
func (s *DeviceService) AcquireMutation(
	ctx context.Context,
	deviceID string,
	epoch uint64,
) (context.Context, error) {
	mutationCtx, err := s.mutations.acquire(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if err := s.store.RevalidateDeviceEpoch(context.WithoutCancel(mutationCtx), deviceID, epoch); err != nil {
		ReleaseMutation(mutationCtx)
		return nil, fmt.Errorf("gateway: acquire device mutation: %w", err)
	}
	return mutationCtx, nil
}

func (s *DeviceService) Connections() ConnectionRegistry {
	return s.connections
}

func generateDeviceID(entropy io.Reader) (string, error) {
	random := make([]byte, 16)
	if _, err := io.ReadFull(entropy, random); err != nil {
		return "", fmt.Errorf("gateway: generate device ID: %w", err)
	}
	return "dev_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func validActorKind(kind ActorKind) bool {
	return kind == ActorKindOperatorDevice || kind == ActorKindCLIProfile
}

var _ DeviceAuthenticator = (*DeviceService)(nil)
var _ AuthGate = (*DeviceService)(nil)
