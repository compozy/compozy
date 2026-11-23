package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/remotehost"
)

const (
	defaultPairingTTL   = 5 * time.Minute
	defaultPairingLimit = 32
)

// DeviceService owns pairing, authentication, device inventory, tickets, and live revocation.
type DeviceService struct {
	store       DeviceStore
	pairings    *pairingStore
	tickets     *remotehost.StreamTicketStore
	connections remotehost.ConnectionRegistry
	mutations   *remotehost.MutationGate
	events      DeviceEventSink
	now         func() time.Time
	entropy     io.Reader
}

type DeviceServiceOption func(*deviceServiceConfig) error

type deviceServiceConfig struct {
	mutations         *remotehost.MutationGate
	now               func() time.Time
	entropy           io.Reader
	pairingTTL        time.Duration
	pairingLimit      int
	streamTicketTTL   time.Duration
	streamTicketLimit int
	connections       remotehost.ConnectionRegistry
	events            DeviceEventSink
}

func NewDeviceService(store DeviceStore, opts ...DeviceServiceOption) (*DeviceService, error) {
	if store == nil {
		return nil, errors.New("gateway: device store is required")
	}
	config := deviceServiceConfig{
		now: func() time.Time { return time.Now().UTC() }, entropy: defaultEntropy(),
		pairingTTL: defaultPairingTTL, pairingLimit: defaultPairingLimit,
		streamTicketTTL: remotehost.DefaultStreamTicketTTL, streamTicketLimit: remotehost.DefaultStreamTicketLimit,
		connections: remotehost.NewConnectionRegistry(),
	}
	for _, option := range opts {
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	if config.mutations == nil {
		config.mutations = remotehost.NewMutationGate(store.RevalidateDeviceEpoch)
	}
	pairings, err := newPairingStore(config.pairingLimit, config.pairingTTL, config.now, config.entropy)
	if err != nil {
		return nil, err
	}
	tickets, err := remotehost.NewStreamTicketStore(
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
		connections: config.connections, mutations: config.mutations,
		events: config.events, now: config.now, entropy: config.entropy,
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

func WithMutationGate(gate *remotehost.MutationGate) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if gate == nil {
			return errors.New("gateway: mutation gate is required")
		}
		config.mutations = gate
		return nil
	}
}

func WithConnectionRegistry(registry remotehost.ConnectionRegistry) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if registry == nil {
			return errors.New("gateway: connection registry is required")
		}
		config.connections = registry
		return nil
	}
}

// WithDeviceEvents configures the durable sink for device lifecycle events.
func WithDeviceEvents(events DeviceEventSink) DeviceServiceOption {
	return func(config *deviceServiceConfig) error {
		if events == nil {
			return errors.New("gateway: device event sink is required")
		}
		config.events = events
		return nil
	}
}

// ReconfigureLimits applies live pairing and stream-ticket limits to new artifacts.
func (s *DeviceService) ReconfigureLimits(
	pairingMaxPending int,
	pairingTTL time.Duration,
	streamTicketTTL time.Duration,
) error {
	if s == nil || s.pairings == nil || s.tickets == nil {
		return errors.New("gateway: device service is required")
	}
	if pairingMaxPending <= 0 || pairingTTL <= 0 || streamTicketTTL <= 0 {
		return errors.New("gateway: positive device limits are required")
	}
	if err := s.pairings.reconfigure(pairingMaxPending, pairingTTL); err != nil {
		return fmt.Errorf("gateway: reconfigure pairing limits: %w", err)
	}
	if err := s.tickets.Reconfigure(streamTicketTTL); err != nil {
		return fmt.Errorf("gateway: reconfigure stream ticket limits: %w", err)
	}
	return nil
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
	err := s.pairings.Redeem(req.Artifact, req.ManualCode, func(pairingSource PairingSource) error {
		credential := strings.TrimSpace(req.Credential)
		if credential == "" {
			var generateErr error
			credential, generateErr = generateDeviceCredential(s.entropy)
			if generateErr != nil {
				return generateErr
			}
		} else if !remotehost.ValidOpaqueSecret(credential, deviceCredentialPrefix) {
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
			Session: device, TokenHash: remotehost.HashCredential(credential),
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
	stored, err := s.store.FindDeviceByTokenHash(ctx, remotehost.HashCredential(credential), s.now())
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			remotehost.ConstantTimeCredentialMatch(credential, "")
			return DeviceSession{}, ErrDeviceUnauthenticated
		}
		return DeviceSession{}, fmt.Errorf("gateway: find device credential: %w", err)
	}
	valid := remotehost.ValidOpaqueSecret(credential, deviceCredentialPrefix)
	matched := remotehost.ConstantTimeCredentialMatch(credential, stored.TokenHash)
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
	return s.revokeSession(ctx, deviceID, func(operationCtx context.Context) (DeviceSession, bool, error) {
		if actor, ok := DeviceFromContext(operationCtx); ok {
			return s.store.RevokeDeviceForActor(operationCtx, actor.ID, actor.RevokeEpoch, deviceID, s.now())
		}
		return s.store.RevokeDevice(operationCtx, deviceID, s.now())
	})
}

func (s *DeviceService) MintStreamTicket(ctx context.Context, deviceID string) (remotehost.StreamTicket, error) {
	device, ok := DeviceFromContext(ctx)
	if ok {
		if device.ID != deviceID {
			return remotehost.StreamTicket{}, ErrDeviceUnauthenticated
		}
		if err := s.store.RevalidateDeviceEpoch(ctx, device.ID, device.RevokeEpoch); err != nil {
			return remotehost.StreamTicket{}, ErrDeviceUnauthenticated
		}
	} else {
		var err error
		device, err = s.store.GetDevice(ctx, deviceID)
		if err != nil {
			return remotehost.StreamTicket{}, fmt.Errorf("gateway: get stream-ticket device: %w", err)
		}
	}
	if !device.RevokedAt.IsZero() {
		return remotehost.StreamTicket{}, ErrDeviceUnauthenticated
	}
	return s.tickets.Mint(device.ID, device.RevokeEpoch)
}

func (s *DeviceService) ConsumeStreamTicket(ctx context.Context, ticket string) (DeviceSession, error) {
	entry, err := s.tickets.Consume(ticket)
	if err != nil {
		return DeviceSession{}, err
	}
	if err := s.store.RevalidateDeviceEpoch(ctx, entry.DeviceID, entry.Epoch); err != nil {
		return DeviceSession{}, remotehost.ErrStreamTicketInvalid
	}
	device, err := s.store.GetDevice(ctx, entry.DeviceID)
	if err != nil || !device.RevokedAt.IsZero() {
		return DeviceSession{}, remotehost.ErrStreamTicketInvalid
	}
	return device, nil
}

func (s *DeviceService) RevalidateForCommit(ctx context.Context, deviceID string, epoch uint64) error {
	return s.mutations.RevalidateForCommit(ctx, deviceID, epoch)
}

func (s *DeviceService) AcquireMutation(ctx context.Context, deviceID string, epoch uint64) (context.Context, error) {
	return s.mutations.AcquireMutation(ctx, deviceID, epoch)
}

func (s *DeviceService) Connections() remotehost.ConnectionRegistry {
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
