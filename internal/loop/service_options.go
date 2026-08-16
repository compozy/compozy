package loop

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
)

// DefaultsResolver resolves the `[loops.defaults.*]` layer for one workspace.
type DefaultsResolver func(context.Context, WorkspaceID) (LoopDefaults, error)

// InputDefaultsResolver resolves source-preserving configured inputs for one named Loop.
type InputDefaultsResolver func(context.Context, WorkspaceID, string) (InputDefaultLayers, error)

// Option configures a loop service.
type Option func(*service)

// WithLogger injects structured observability for post-commit service failures.
func WithLogger(logger *slog.Logger) Option {
	return func(s *service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithDefaults injects the `[loops.defaults.*]` layer consumed by the resolver.
func WithDefaults(defaults LoopDefaults) Option {
	return func(s *service) {
		s.defaults = defaults
	}
}

// WithParticipationResolver injects the canonical execution participation resolver.
func WithParticipationResolver(resolver participation.Resolver) Option {
	return func(s *service) {
		s.participationResolver = resolver
	}
}

// WithDefaultsResolver injects workspace-aware `[loops.defaults.*]` resolution.
func WithDefaultsResolver(resolver DefaultsResolver) Option {
	return func(s *service) {
		s.defaultsResolver = resolver
	}
}

// WithInputDefaultsResolver injects workspace-aware `[loops.inputs.<name>]` resolution.
func WithInputDefaultsResolver(resolver InputDefaultsResolver) Option {
	return func(s *service) {
		s.inputDefaultsResolver = resolver
	}
}

// WithRuntimeCatalog injects workspace-specific effective runtime validation.
func WithRuntimeCatalog(catalog WorkspaceRuntimeCatalog) Option {
	return func(s *service) {
		s.runtimeCatalog = catalog
	}
}

// WithClock injects a deterministic clock.
func WithClock(now func() time.Time) Option {
	return func(s *service) {
		s.now = now
	}
}

// WithRunIDFactory injects a deterministic run ID factory.
func WithRunIDFactory(factory func() (RunID, error)) Option {
	return func(s *service) {
		s.newRunID = factory
	}
}

// WithHookDispatcher injects loop hook dispatch at aggregate call sites.
func WithHookDispatcher(hooks HookDispatcher) Option {
	return func(s *service) {
		s.hooks = hooks
	}
}

// WithGoalRunActivator injects the post-commit activation path for Goal successor workers.
func WithGoalRunActivator(activator GoalRunActivator) Option {
	return func(s *service) {
		s.goalRunActivator = activator
	}
}

// WithCoordinatorRunActivator injects the post-commit activation path for non-Goal coordinator wakes.
func WithCoordinatorRunActivator(activator CoordinatorRunActivator) Option {
	return func(s *service) {
		s.coordinatorActivator = activator
	}
}

// WithGoalPromptLeaseRevoker injects post-commit cancellation for exact managed Goal prompt leases.
func WithGoalPromptLeaseRevoker(revoker GoalPromptLeaseRevoker) Option {
	return func(s *service) {
		s.goalLeaseRevoker = revoker
	}
}

// WithCancellationSessionController injects post-commit prompt and process cancellation.
func WithCancellationSessionController(controller CancellationSessionController) Option {
	return func(s *service) {
		s.cancellationSessions = controller
	}
}

// WithResponderPolicy injects the daemon-owned initiator-lineage policy.
func WithResponderPolicy(policy ResponderPolicy) Option {
	return func(s *service) {
		s.responderPolicy = policy
	}
}

func (s *service) resolveDefaults(ctx context.Context, ws WorkspaceID) (LoopDefaults, error) {
	if s.defaultsResolver == nil {
		return s.defaults, nil
	}
	return s.defaultsResolver(ctx, ws)
}

func (s *service) resolveInputDefaults(
	ctx context.Context,
	ws WorkspaceID,
	loopName string,
) (InputDefaultLayers, error) {
	if s.inputDefaultsResolver == nil {
		return InputDefaultLayers{}, nil
	}
	return s.inputDefaultsResolver(ctx, ws, loopName)
}

func (s *service) runtimeCatalogForWorkspace(
	ctx context.Context,
	ws WorkspaceID,
) (RuntimeCatalog, error) {
	if s.runtimeCatalog == nil {
		return nil, nil
	}
	catalog, err := s.runtimeCatalog.ForWorkspace(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace runtime catalog: %w", err)
	}
	if catalog == nil {
		return nil, fmt.Errorf("%w: workspace runtime catalog returned nil", ErrActionDependencyMissing)
	}
	return catalog, nil
}
