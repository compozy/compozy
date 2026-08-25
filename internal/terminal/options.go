package terminal

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type WorkspaceResolver interface {
	Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error)
}

type ProfileWorkspaceResolver interface {
	ResolveForProfile(ctx context.Context, idOrPath string, profileName string) (workspacepkg.ResolvedWorkspace, error)
}

type ProfileNameResolver interface {
	ProfileName(ctx context.Context, profileID string) (string, error)
}

type SettingsProvider func(context.Context, string, string) (Settings, error)

type processCheckpoint interface {
	Checkpoint(context.Context, toolruntime.ProcessCheckpoint) error
	Complete(context.Context, toolruntime.ProcessCompletion) error
}

type processRegister func(context.Context, toolruntime.RegisterConfig) (processCheckpoint, error)

// TypingGrantAuthorizer evaluates the existing profile-scoped approval store
// before an agent can deliver bytes to a terminal.
type TypingGrantAuthorizer interface {
	AuthorizeTerminalInput(context.Context, Actor, Info) error
}

// ExecAuthorizer obtains a caller-owned approval decision for an agent command.
type ExecAuthorizer interface {
	AuthorizeTerminalExec(context.Context, ExecRequest, CommandClassification) (string, error)
}

type Option func(*Service) error

func WithPTY(starter pty.PTY) Option {
	return func(service *Service) error {
		if starter == nil {
			return errors.New("terminal: pty starter is required")
		}
		service.pty = starter
		return nil
	}
}

func WithWorkspaceResolver(resolver WorkspaceResolver) Option {
	return func(service *Service) error {
		if resolver == nil {
			return errors.New("terminal: workspace resolver is required")
		}
		service.workspaces = resolver
		return nil
	}
}

func WithProfileNameResolver(resolver ProfileNameResolver) Option {
	return func(service *Service) error {
		if resolver == nil {
			return errors.New("terminal: profile name resolver is required")
		}
		service.profileNames = resolver
		return nil
	}
}

func WithSettingsProvider(provider SettingsProvider) Option {
	return func(service *Service) error {
		if provider == nil {
			return errors.New("terminal: settings provider is required")
		}
		service.settings = provider
		return nil
	}
}

func WithProfileGuard(guard ProfileGuard) Option {
	return func(service *Service) error {
		if guard == nil {
			return errors.New("terminal: profile guard is required")
		}
		service.profiles = guard
		return nil
	}
}

func WithTypingGrantAuthorizer(authorizer TypingGrantAuthorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return errors.New("terminal: typing grant authorizer is required")
		}
		service.typingGrants = authorizer
		return nil
	}
}

func WithExecAuthorizer(authorizer ExecAuthorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return errors.New("terminal: exec authorizer is required")
		}
		service.execApprovals = authorizer
		return nil
	}
}

func WithProcessRegistry(registry *toolruntime.Registry) Option {
	return func(service *Service) error {
		if registry == nil {
			return errors.New("terminal: process registry is required")
		}
		service.registerProcess = func(
			ctx context.Context,
			config toolruntime.RegisterConfig,
		) (processCheckpoint, error) {
			return registry.Register(ctx, config)
		}
		return nil
	}
}

func withProcessRegister(register processRegister) Option {
	return func(service *Service) error {
		if register == nil {
			return errors.New("terminal: process register function is required")
		}
		service.registerProcess = register
		return nil
	}
}

func WithEventBus(bus *EventBus) Option {
	return func(service *Service) error {
		if bus == nil {
			return errors.New("terminal: event bus is required")
		}
		service.events = bus
		return nil
	}
}

func WithJournal(journal Journal) Option {
	return func(service *Service) error {
		if journal == nil {
			return errors.New("terminal: journal is required")
		}
		service.journal = journal
		return nil
	}
}

func WithMarkerConsumer(consumer MarkerConsumer) Option {
	return func(service *Service) error {
		if consumer == nil {
			return errors.New("terminal: marker consumer is required")
		}
		service.markers = consumer
		return nil
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) error {
		if logger == nil {
			return errors.New("terminal: logger is required")
		}
		service.logger = logger
		return nil
	}
}

func WithClock(now func() time.Time) Option {
	return func(service *Service) error {
		if now == nil {
			return errors.New("terminal: clock is required")
		}
		service.now = now
		return nil
	}
}

func WithEntropy(entropy io.Reader) Option {
	return func(service *Service) error {
		if entropy == nil {
			return errors.New("terminal: entropy source is required")
		}
		service.entropy = entropy
		return nil
	}
}

func withInputRequestTTL(ttl time.Duration) Option {
	return func(service *Service) error {
		if ttl <= 0 {
			return errors.New("terminal: input request ttl must be positive")
		}
		service.inputRequestTTL = ttl
		return nil
	}
}

func defaultServiceOptions(service *Service) {
	service.pty = pty.New()
	service.settings = func(context.Context, string, string) (Settings, error) {
		return DefaultSettings(), nil
	}
	service.events = NewEventBus(nil)
	service.markers = noopMarkerConsumer{}
	service.logger = slog.Default()
	service.now = func() time.Time { return time.Now().UTC() }
	service.entropy = rand.Reader
	service.inputRequestTTL = inputRequestTTL
}

type noopMarkerConsumer struct{}

func (noopMarkerConsumer) ConsumeMarkerFacts(context.Context, Info, []MarkerFacts) {}
