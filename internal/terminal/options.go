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

// WorkspaceResolver resolves an operator-supplied workspace ID or path.
type WorkspaceResolver interface {
	Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error)
}

// ProfileWorkspaceResolver resolves a workspace using profile-owned aliases.
type ProfileWorkspaceResolver interface {
	ResolveForProfile(ctx context.Context, idOrPath string, profileName string) (workspacepkg.ResolvedWorkspace, error)
}

// ProfileNameResolver projects a stable profile ID to its display name.
type ProfileNameResolver interface {
	ProfileName(ctx context.Context, profileID string) (string, error)
}

// SettingsProvider returns the effective terminal policy for a workspace and profile.
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

// Option configures one terminal Service dependency.
type Option func(*Service) error

// WithPTY replaces the platform process starter.
func WithPTY(starter pty.PTY) Option {
	return func(service *Service) error {
		if starter == nil {
			return errors.New("terminal: pty starter is required")
		}
		service.pty = starter
		return nil
	}
}

// WithWorkspaceResolver sets the workspace boundary resolver.
func WithWorkspaceResolver(resolver WorkspaceResolver) Option {
	return func(service *Service) error {
		if resolver == nil {
			return errors.New("terminal: workspace resolver is required")
		}
		service.workspaces = resolver
		return nil
	}
}

// WithProfileNameResolver sets the profile-name projection used in events.
func WithProfileNameResolver(resolver ProfileNameResolver) Option {
	return func(service *Service) error {
		if resolver == nil {
			return errors.New("terminal: profile name resolver is required")
		}
		service.profileNames = resolver
		return nil
	}
}

// WithSettingsProvider sets the effective terminal policy provider.
func WithSettingsProvider(provider SettingsProvider) Option {
	return func(service *Service) error {
		if provider == nil {
			return errors.New("terminal: settings provider is required")
		}
		service.settings = provider
		return nil
	}
}

// WithProfileGuard sets the profile availability guard.
func WithProfileGuard(guard ProfileGuard) Option {
	return func(service *Service) error {
		if guard == nil {
			return errors.New("terminal: profile guard is required")
		}
		service.profiles = guard
		return nil
	}
}

// WithTypingGrantAuthorizer sets the agent input grant authority.
func WithTypingGrantAuthorizer(authorizer TypingGrantAuthorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return errors.New("terminal: typing grant authorizer is required")
		}
		service.typingGrants = authorizer
		return nil
	}
}

// WithExecAuthorizer sets the command approval authority.
func WithExecAuthorizer(authorizer ExecAuthorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return errors.New("terminal: exec authorizer is required")
		}
		service.execApprovals = authorizer
		return nil
	}
}

// WithProcessRegistry registers spawned terminal processes with toolruntime.
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

// WithEventBus sets the terminal lifecycle event bus.
func WithEventBus(bus *EventBus) Option {
	return func(service *Service) error {
		if bus == nil {
			return errors.New("terminal: event bus is required")
		}
		service.events = bus
		return nil
	}
}

// WithJournal sets the terminal audit journal.
func WithJournal(journal Journal) Option {
	return func(service *Service) error {
		if journal == nil {
			return errors.New("terminal: journal is required")
		}
		service.journal = journal
		return nil
	}
}

// WithMarkerConsumer sets the authenticated shell-marker consumer.
func WithMarkerConsumer(consumer MarkerConsumer) Option {
	return func(service *Service) error {
		if consumer == nil {
			return errors.New("terminal: marker consumer is required")
		}
		service.markers = consumer
		return nil
	}
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) error {
		if logger == nil {
			return errors.New("terminal: logger is required")
		}
		service.logger = logger
		return nil
	}
}

// WithClock sets the clock used for terminal lifecycle timestamps.
func WithClock(now func() time.Time) Option {
	return func(service *Service) error {
		if now == nil {
			return errors.New("terminal: clock is required")
		}
		service.now = now
		return nil
	}
}

// WithEntropy sets the cryptographic entropy reader used for identifiers.
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
