package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
)

// Service composes exposure policy with device authentication for API transports.
type Service struct {
	policy             Policy
	devices            *DeviceService
	bindingProjections func(context.Context) ([]ingress.Projection, error)
}

// ServiceOption adds optional gateway capabilities to the transport facade.
type ServiceOption func(*Service) error

// WithBindingProjections supplies ingress data for the gateway status payload.
func WithBindingProjections(list func(context.Context) ([]ingress.Projection, error)) ServiceOption {
	return func(service *Service) error {
		service.bindingProjections = list
		return nil
	}
}

func NewService(policy Policy, devices *DeviceService, options ...ServiceOption) (*Service, error) {
	if policy == nil {
		return nil, errors.New("gateway: policy is required")
	}
	if devices == nil {
		return nil, errors.New("gateway: device service is required")
	}
	service := &Service{policy: policy, devices: devices}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

// ReconfigureDeviceLimits applies live artifact limits to the active device service.
func (s *Service) ReconfigureDeviceLimits(
	pairingMaxPending int,
	pairingTTL time.Duration,
	streamTicketTTL time.Duration,
) error {
	if s == nil || s.devices == nil {
		return errors.New("gateway: service is required")
	}
	return s.devices.ReconfigureLimits(pairingMaxPending, pairingTTL, streamTicketTTL)
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	status, err := s.policy.Status(ctx)
	if err != nil || s.bindingProjections == nil {
		return status, err
	}
	status.Bindings, err = s.bindingProjections(ctx)
	if err != nil {
		return Status{}, err
	}
	return status, nil
}

func (s *Service) SetSurfaceExposure(ctx context.Context, req SurfaceExposureRequest) (Status, error) {
	return s.policy.Transition(ctx, TransitionRequest{
		Target: TargetSurface, Tier: req.Tier, Surface: req.Surface,
		Desired: req.Desired, ExpectedGeneration: req.ExpectedGeneration, Consent: req.Consent,
	})
}

func (s *Service) EnableProvider(ctx context.Context, req ProviderEnableRequest) (Status, error) {
	return s.policy.Transition(ctx, TransitionRequest{
		Target: TargetProvider, Tier: req.Tier, Provider: req.Provider,
		Desired: DesiredEnabled, ExpectedGeneration: req.ExpectedGeneration,
	})
}

func (s *Service) DisableProvider(ctx context.Context, tier Tier, providerName string) (Status, error) {
	status, err := s.policy.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	for _, provider := range status.Providers {
		if provider.Tier != tier || provider.Name != providerName {
			continue
		}
		if provider.Desired == DesiredDisabled {
			status.Changed = false
			return status, nil
		}
		return s.policy.Transition(ctx, TransitionRequest{
			Target: TargetProvider, Tier: tier, Provider: ProviderIdentity{Name: provider.Name},
			Desired: DesiredDisabled, ExpectedGeneration: provider.Generation,
		})
	}
	return Status{}, ErrProviderNotFound
}

func (s *Service) MintPairing(ctx context.Context, req PairingRequest) (PairingArtifact, error) {
	return s.devices.MintPairing(ctx, req)
}

func (s *Service) RedeemPairing(ctx context.Context, req RedeemRequest) (IssuedCredential, error) {
	return s.devices.RedeemPairing(ctx, req)
}

func (s *Service) ListDevices(ctx context.Context) ([]DeviceSession, error) {
	return s.devices.ListDevices(ctx)
}

func (s *Service) RevokeDevice(ctx context.Context, deviceID string) (RevokeResult, error) {
	return s.devices.RevokeDevice(ctx, deviceID)
}

func (s *Service) RemoveDevice(ctx context.Context, deviceID string) (RemoveResult, error) {
	return s.devices.RemoveDevice(ctx, deviceID)
}

func (s *Service) RenameDevice(ctx context.Context, deviceID, name string) (DeviceSession, error) {
	return s.devices.RenameDevice(ctx, deviceID, name)
}

func (s *Service) MintStreamTicket(ctx context.Context, deviceID string) (remotehost.StreamTicket, error) {
	return s.devices.MintStreamTicket(ctx, deviceID)
}

func (s *Service) Authenticate(ctx context.Context, credential string) (DeviceSession, error) {
	return s.devices.Authenticate(ctx, credential)
}

func (s *Service) ConsumeStreamTicket(ctx context.Context, ticket string) (DeviceSession, error) {
	return s.devices.ConsumeStreamTicket(ctx, ticket)
}

func (s *Service) RevalidateForCommit(ctx context.Context, deviceID string, epoch uint64) error {
	return s.devices.RevalidateForCommit(ctx, deviceID, epoch)
}

func (s *Service) AcquireMutation(ctx context.Context, deviceID string, epoch uint64) (context.Context, error) {
	return s.devices.AcquireMutation(ctx, deviceID, epoch)
}

func (s *Service) Connections() remotehost.ConnectionRegistry {
	return s.devices.Connections()
}

func (s *Service) Acquire(tier Tier, surface Surface) (func(), error) {
	return s.policy.Acquire(tier, surface)
}

var _ DeviceAuthenticator = (*Service)(nil)
var _ AdmissionController = (*Service)(nil)
