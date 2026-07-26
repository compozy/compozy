package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
)

func (d *Daemon) bootHooks(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: hook boot state is required")
	}

	nativeDecls, nativeExecutors := d.initializeHookObservers(state)
	providers := d.hookBindingProviders(state, nativeDecls)
	hooks := hookspkg.NewHooks(d.hookRuntimeOptions(state, nativeExecutors)...)
	hookBindings, err := d.newHookBindingPublisher(state, hooks, providers)
	if err != nil {
		hooks.Close()
		return err
	}
	if err := hookBindings.Sync(ctx); err != nil {
		hooks.Close()
		return fmt.Errorf("daemon: sync hook bindings: %w", err)
	}
	if hookAwareObserver, ok := state.observer.(interface {
		AttachHooks(observe.HookCatalogSource)
	}); ok {
		hookAwareObserver.AttachHooks(hooks)
	}
	state.notifier.setRuntime(hooks, state.observer, state.registry)
	cleanup.add(func(context.Context) error {
		hooks.Close()
		return nil
	})

	if state.skillsRegistry != nil {
		state.skillsCancel, state.skillsDone = startSkillsWatcher(
			ctx,
			state.skillsRegistry,
			state.cfg.Skills.PollInterval,
			workspaceSkillWatcherRoots(d.homePaths, state.registry),
			func(refreshCtx context.Context) error {
				if state.agentSkillResources != nil {
					if err := state.agentSkillResources.Sync(refreshCtx); err != nil {
						return err
					}
				}
				return hookBindings.Sync(refreshCtx)
			},
		)
		cleanup.add(func(cleanupCtx context.Context) error {
			return stopSkillsWatcher(cleanupCtx, state.skillsCancel, state.skillsDone)
		})
	}
	if state.loopResources != nil {
		state.loopsCancel, state.loopsDone = startLoopWatcher(
			ctx,
			state.cfg.Skills.PollInterval,
			[]string{d.homePaths.LoopsDir},
			workspaceLoopWatcherRoots(d.homePaths, state.registry),
			state.loopResources,
			state.logger,
		)
		cleanup.add(func(cleanupCtx context.Context) error {
			return stopLoopWatcher(cleanupCtx, state.loopsCancel, state.loopsDone)
		})
	}

	state.hooks = hooks
	state.hookDispatcher = hooks
	state.hookBindings = hookBindings
	attachParticipationResolverHooks(state.participationResolver, hooks)
	return nil
}

func (d *Daemon) hookBindingProviders(
	state *bootState,
	nativeDecls []hookspkg.HookDecl,
) []hookBindingDeclarationProvider {
	return []hookBindingDeclarationProvider{
		func(context.Context) ([]hookspkg.HookDecl, error) {
			return hookCloneDeclarations(nativeDecls), nil
		},
		configDeclarationProvider(state.registry, state.workspaceResolver, state.logger),
		agentDeclarationProvider(state.registry, state.workspaceResolver, state.logger),
		skillDeclarationProvider(
			state.skillsRegistry,
			state.registry,
			state.workspaceResolver,
			state.cfg.Skills.AllowedMarketplaceHooks,
			state.logger,
		),
		extensionDeclarationProvider(state.currentExtensionRuntime),
	}
}

func (d *Daemon) hookRuntimeOptions(
	state *bootState,
	nativeExecutors map[string]hookspkg.Executor,
) []hookspkg.Option {
	return []hookspkg.Option{
		hookspkg.WithLogger(state.logger),
		hookspkg.WithNow(d.now),
		hookspkg.WithDebugPatchAudit(strings.EqualFold(state.cfg.Log.Level, "debug")),
		hookspkg.WithExecutorResolver(daemonExecutorResolverWithSecrets(
			nativeExecutors,
			state.providerVault,
			state.processRegistry,
		)),
		hookspkg.WithTelemetrySink(state.hookTelemetrySinks),
	}
}

func (d *Daemon) newHookBindingPublisher(
	state *bootState,
	hooks *hookspkg.Hooks,
	providers []hookBindingDeclarationProvider,
) (hookBindingPublisher, error) {
	hookBindings := hookBindingPublisher(
		hookBindingPublisherFunc(func(reloadCtx context.Context) error {
			decls, err := chainDeclarationProviders(providers...)(reloadCtx)
			if err != nil {
				return err
			}
			nextState, err := hooks.BuildBindingState(decls)
			if err != nil {
				return err
			}
			return hooks.ApplyBindingState(nextState, 0)
		}),
	)
	if state.resourceKernel == nil || state.resourceCodecs == nil {
		return hookBindings, nil
	}

	hookCodec, err := resources.ResolveCodec[hookspkg.HookDecl](
		state.resourceCodecs,
		hookBindingResourceKind,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve hook binding codec: %w", err)
	}
	hookStore, err := newHookBindingStore(state.resourceKernel, hookCodec)
	if err != nil {
		return nil, fmt.Errorf("daemon: create hook binding store: %w", err)
	}
	return newHookBindingSourceSyncer(
		hookStore,
		hookCodec,
		hookBindingSyncActor(),
		state.logger,
		func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
			if state.resourceReconcile == nil {
				return nil
			}
			return state.resourceReconcile.Trigger(ctx, kind, reason)
		},
		providers...,
	), nil
}
