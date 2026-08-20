package cmdpalette

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	providers       []ProviderRegistration
	clients         ClientDirectory
	bindings        BindingsResolver
	executor        ActionExecutor
	newID           func() string
	now             func() time.Time
	personalization PersonalizationStore
	policy          PersonalizationPolicy
	logger          *slog.Logger
	degradedOnce    sync.Once
	viewProviders   []ViewProviderRegistration
	dynamicViews    []DynamicViewProvider
	viewStreamEpoch string
	viewPrograms    ViewProgramProvider
	viewSessionMu   sync.Mutex
	viewSessions    map[string]*viewSession

	eventRecorder    EventRecorder
	eventMu          sync.Mutex
	eventSubscribers map[uint64]eventSubscription
	nextSubscriber   uint64

	flightMu sync.Mutex
	flights  map[string]struct{}
}

type Option func(*Service) error

func WithInvocationIDGenerator(generator func() string) Option {
	return func(service *Service) error {
		if generator == nil {
			return errors.New("cmd palette: invocation ID generator is required")
		}
		service.newID = generator
		return nil
	}
}

func WithEventRecorder(recorder EventRecorder) Option {
	return func(service *Service) error {
		if recorder == nil {
			return errors.New("cmd palette: event recorder is required")
		}
		service.eventRecorder = recorder
		return nil
	}
}

func WithClock(clock func() time.Time) Option {
	return func(service *Service) error {
		if clock == nil {
			return errors.New("cmd palette: clock is required")
		}
		service.now = clock
		return nil
	}
}

func WithPersonalizationStore(store PersonalizationStore) Option {
	return func(service *Service) error {
		if store == nil {
			return errors.New("cmd palette: personalization store is required")
		}
		service.personalization = store
		return nil
	}
}

func WithPersonalizationPolicy(policy PersonalizationPolicy) Option {
	return func(service *Service) error {
		if policy == nil {
			return errors.New("cmd palette: personalization policy is required")
		}
		service.policy = policy
		return nil
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) error {
		if logger == nil {
			return errors.New("cmd palette: logger is required")
		}
		service.logger = logger
		return nil
	}
}

func NewRegistry(
	providers []ProviderRegistration,
	clients ClientDirectory,
	bindings BindingsResolver,
	executor ActionExecutor,
	options ...Option,
) (*Service, error) {
	if len(providers) == 0 {
		return nil, errors.New("cmd palette: at least one provider is required")
	}
	if executor == nil {
		return nil, errors.New("cmd palette: action executor is required")
	}
	service := &Service{
		providers:        cloneProviderRegistrations(providers),
		clients:          clients,
		bindings:         bindings,
		executor:         executor,
		newID:            func() string { return "inv_" + uuid.NewString() },
		now:              time.Now,
		logger:           slog.Default(),
		viewStreamEpoch:  "vse_" + uuid.NewString(),
		viewSessions:     make(map[string]*viewSession),
		flights:          make(map[string]struct{}),
		eventSubscribers: make(map[uint64]eventSubscription),
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	if err := validateProviderRegistrations(service.providers); err != nil {
		return nil, err
	}
	return service, nil
}

var _ Registry = (*Service)(nil)

func validateProviderRegistrations(registrations []ProviderRegistration) error {
	seenSources := make(map[string]struct{}, len(registrations))
	seenCommands := make(map[CommandID]string)
	for _, registration := range registrations {
		sourceID := registration.Source.ID()
		if strings.TrimSpace(sourceID) == "" || registration.Provider == nil {
			return errors.New("cmd palette: provider source and implementation are required")
		}
		if _, exists := seenSources[sourceID]; exists {
			return fmt.Errorf("cmd palette: duplicate provider source %q", sourceID)
		}
		seenSources[sourceID] = struct{}{}
		if registration.Source.Kind == SourceKindExtension && registration.Source.Extension == "" {
			if _, ok := registration.Provider.(ContributionProvider); !ok {
				return errors.New("cmd palette: aggregate extension provider must supply contributions")
			}
			continue
		}
		provider, static := registration.Provider.(StaticProvider)
		if !static {
			continue
		}
		for _, descriptor := range provider.StaticCommands() {
			if err := validateProviderDescriptor(registration.Source, descriptor); err != nil {
				return err
			}
			if first, exists := seenCommands[descriptor.ID]; exists {
				return &DuplicateCommandIDError{ID: descriptor.ID, First: first, Second: sourceID}
			}
			seenCommands[descriptor.ID] = sourceID
		}
	}
	return nil
}

func validateProviderDescriptor(source Source, descriptor Descriptor) error {
	if descriptor.Source != source {
		return invalidDescriptor(
			"%s: descriptor source %q does not match provider %q",
			descriptor.ID,
			descriptor.Source.ID(),
			source.ID(),
		)
	}
	return ValidateDescriptor(descriptor)
}

func cloneProviderRegistrations(source []ProviderRegistration) []ProviderRegistration {
	return append([]ProviderRegistration(nil), source...)
}

func (s *Service) acquireFlight(workspaceID WorkspaceID, commandID CommandID) bool {
	key := string(workspaceID) + "\x00" + string(commandID)
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if _, exists := s.flights[key]; exists {
		return false
	}
	s.flights[key] = struct{}{}
	return true
}

func (s *Service) releaseFlight(workspaceID WorkspaceID, commandID CommandID) {
	key := string(workspaceID) + "\x00" + string(commandID)
	s.flightMu.Lock()
	delete(s.flights, key)
	s.flightMu.Unlock()
}
