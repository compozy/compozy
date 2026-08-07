package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Service composes exposure policy with device authentication for API transports.
type Service struct {
	policy  Policy
	devices *DeviceService
	ingress *ingressManager
}

// ServiceOption adds optional gateway capabilities to the transport facade.
type ServiceOption func(*Service) error

// WithIngress configures workspace-authorized public ingress management.
func WithIngress(
	store IngressStore,
	access workspaceaccess.Policy,
	events IngressEventSink,
	now func() time.Time,
) ServiceOption {
	return func(service *Service) error {
		manager, err := newIngressManager(store, service.policy, access, events, now)
		if err != nil {
			return err
		}
		service.ingress = manager
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

func (s *Service) Status(ctx context.Context) (Status, error) {
	status, err := s.policy.Status(ctx)
	if err != nil || s.ingress == nil {
		return status, err
	}
	bindings, err := s.ingress.store.ListIngressBindings(ctx)
	if err != nil {
		return Status{}, err
	}
	status.Bindings = make([]IngressBindingStatus, 0, len(bindings))
	readWorkspaceID, scopedRead := ingressReadWorkspace(ctx)
	for _, binding := range bindings {
		if scopedRead && binding.Scope == IngressScopeWorkspace && binding.WorkspaceID != readWorkspaceID {
			continue
		}
		projection, projectErr := s.ingress.project(ctx, binding.Subject)
		if errors.Is(projectErr, ErrIngressSubjectNotFound) {
			continue
		}
		if projectErr != nil {
			return Status{}, projectErr
		}
		status.Bindings = append(status.Bindings, projection)
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

func (s *Service) RenameDevice(ctx context.Context, deviceID, name string) (DeviceSession, error) {
	return s.devices.RenameDevice(ctx, deviceID, name)
}

func (s *Service) MintStreamTicket(ctx context.Context, deviceID string) (StreamTicket, error) {
	return s.devices.MintStreamTicket(ctx, deviceID)
}

func (s *Service) ResolveIngressSubject(ctx context.Context, ref IngressSubjectRef) (IngressSubject, error) {
	if s == nil || s.ingress == nil {
		return IngressSubject{}, ErrIngressSubjectNotFound
	}
	return s.ingress.store.ResolveIngressSubject(ctx, ref)
}

func (s *Service) BindIngress(ctx context.Context, req IngressBindRequest) (IngressBinding, error) {
	if s == nil || s.ingress == nil {
		return IngressBinding{}, ErrIngressSubjectNotFound
	}
	return s.ingress.bind(ctx, req)
}

func (s *Service) UnbindIngress(ctx context.Context, req IngressUnbindRequest) (UnbindResult, error) {
	if s == nil || s.ingress == nil {
		return UnbindResult{}, ErrIngressSubjectNotFound
	}
	return s.ingress.unbind(ctx, req)
}

func (s *Service) ProjectIngress(ctx context.Context, ref IngressSubjectRef) (IngressProjection, error) {
	if s == nil || s.ingress == nil {
		return IngressProjection{}, ErrIngressSubjectNotFound
	}
	return s.ingress.project(ctx, ref)
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

func (s *Service) Connections() ConnectionRegistry {
	return s.devices.Connections()
}

func (s *Service) Acquire(tier Tier, surface Surface) (func(), error) {
	return s.policy.Acquire(tier, surface)
}

var _ DeviceAuthenticator = (*Service)(nil)
var _ AdmissionController = (*Service)(nil)
