package daemon

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const windowManagerMaxSnapshotBytes = 16 * 1024 * 1024

type globalHotkeyFailureNotifier interface {
	NotifyGlobalHotkeyRegistrationFailed(
		context.Context,
		cmdpalette.WorkspaceID,
		cmdpalette.ClientID,
		cmdpalette.CommandID,
		string,
		string,
	)
}

var _ globalHotkeyFailureNotifier = (*cmdpalette.Service)(nil)

func (d *Daemon) bootDefaultWorkspaceAndWindowManager(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
	operatorHome string,
) error {
	if err := d.ensureDefaultWorkspace(ctx, state, operatorHome); err != nil {
		return err
	}
	resolver, err := newWindowManagerStoreWorkspaceResolver(state.workspaceResolver, state.logger)
	if err != nil {
		return err
	}
	engine, err := clientstate.Open(
		ctx,
		clientstate.DatabasePath(d.homePaths.HomeDir),
		resolver,
		clientstate.Limits{
			MaxValueBytes:       windowManagerMaxSnapshotBytes,
			MaxKeysPerWorkspace: clientstate.DefaultLimits().MaxKeysPerWorkspace,
		},
		clientstate.WithLogger(state.logger),
	)
	if err != nil {
		return fmt.Errorf("daemon: open window-manager store: %w", err)
	}
	cleanup.add(func(context.Context) error { return engine.Close() })
	repository, err := newWindowManagerRepository(engine, withWindowManagerRepositoryLogger(state.logger))
	if err != nil {
		return err
	}
	if state.windowLayoutCatalog == nil {
		state.windowLayoutCatalog = newResourceCatalog(windowmanager.CloneLayoutResource)
	}
	manager, err := windowmanager.NewService(
		repository,
		windowManagerWorkspaceAuthorizer{resolver: resolver},
		newWindowManagerLayoutRegistry(state.windowLayoutCatalog),
		windowManagerDefaults(state.cfg.WindowManager),
		windowmanager.WithLifecycleContext(ctx),
		windowmanager.WithEventObserver(newWindowManagerHookObserver(state)),
		windowmanager.WithClientUnregisteredObserver(
			func(ctx context.Context, workspaceID windowmanager.WorkspaceID, clientID windowmanager.ClientID) error {
				views, ok := state.cmdPalette.(cmdpalette.ViewSessionService)
				if !ok || views == nil {
					return nil
				}
				return views.CloseClientSessions(
					ctx,
					cmdpalette.WorkspaceID(workspaceID),
					cmdpalette.ClientID(clientID),
				)
			},
		),
		windowmanager.WithGlobalShortcutFailureObserver(
			func(
				ctx context.Context,
				workspaceID windowmanager.WorkspaceID,
				clientID windowmanager.ClientID,
				registration windowmanager.GlobalShortcutRegistration,
			) {
				notifier, ok := state.cmdPalette.(globalHotkeyFailureNotifier)
				if !ok || notifier == nil {
					return
				}
				notifier.NotifyGlobalHotkeyRegistrationFailed(
					ctx,
					cmdpalette.WorkspaceID(workspaceID),
					cmdpalette.ClientID(clientID),
					cmdpalette.CommandID(registration.CommandID),
					registration.IntendedChord,
					registration.Reason,
				)
			},
		),
		windowmanager.WithWorkspaceConfigResolver(
			windowManagerWorkspaceConfigResolver{resolver: state.workspaceResolver},
		),
	)
	if err != nil {
		return fmt.Errorf("daemon: create window manager: %w", err)
	}
	cleanup.add(func(context.Context) error { return manager.Close() })
	state.windowManagerStoreResolver = resolver
	state.windowManagerStore = engine
	state.windowManagerRepository = repository
	state.windowManager = manager
	return nil
}

func installWorkspaceRemovalPreparer(state *bootState, sessions SessionManager) error {
	preparer, ok := sessions.(workspaceRemovalPreparer)
	if !ok {
		return errMissingWorkspaceRemovalPreparation
	}
	if state.windowManagerStoreResolver == nil || state.windowManagerStore == nil ||
		state.windowManagerRepository == nil || state.windowManager == nil {
		return errors.New("daemon: window-manager removal dependencies are required")
	}
	state.workspaceResolver.SetUnregisterPreparer(
		func(ctx context.Context, workspace workspacepkg.Workspace) (workspacepkg.UnregisterPreparation, error) {
			attentionPreparation, err := prepareAttentionWorkspaceRemoval(state, workspace.ID)
			if err != nil {
				return nil, err
			}
			sessionPreparation, err := preparer.PrepareWorkspaceRemoval(ctx, workspace.ID)
			if err != nil {
				return nil, err
			}
			if sessionPreparation == nil {
				return nil, errors.New("daemon: session workspace removal preparation is required")
			}
			windowPreparation, err := state.windowManagerStoreResolver.prepareRemoval(
				workspace,
				state.windowManagerStore,
				state.windowManager,
				state.windowManagerRepository,
			)
			if err != nil {
				rollbackErr := sessionPreparation.Rollback(context.WithoutCancel(ctx))
				return nil, errors.Join(err, rollbackErr)
			}
			return workspaceRemovalPreparation{
				windowManager: windowPreparation,
				session:       sessionPreparation,
				attention:     attentionPreparation,
				deadEntities:  state.deadEntities,
				mcpTools:      state.mcpToolProvider,
				workspaceID:   workspace.ID,
			}, nil
		},
	)
	return nil
}

type workspaceRemovalPreparation struct {
	windowManager workspacepkg.UnregisterPreparation
	session       workspacepkg.UnregisterPreparation
	attention     workspacepkg.UnregisterPreparation
	deadEntities  *deadentity.Service
	mcpTools      workspaceMCPStateRetirer
	workspaceID   string
}

type workspaceMCPStateRetirer interface {
	ForgetWorkspace(workspaceID string)
}

func (p workspaceRemovalPreparation) BeforeDelete(ctx context.Context) error {
	if err := p.windowManager.BeforeDelete(ctx); err != nil {
		return err
	}
	if err := p.session.BeforeDelete(ctx); err != nil {
		return err
	}
	return p.attention.BeforeDelete(ctx)
}

func (p workspaceRemovalPreparation) Commit(ctx context.Context) error {
	windowManagerErr := p.windowManager.Commit(ctx)
	sessionErr := p.session.Commit(ctx)
	attentionErr := p.attention.Commit(ctx)
	if p.deadEntities != nil {
		p.deadEntities.ForgetWorkspace(p.workspaceID)
	}
	if p.mcpTools != nil {
		p.mcpTools.ForgetWorkspace(p.workspaceID)
	}
	return errors.Join(windowManagerErr, sessionErr, attentionErr)
}

func (p workspaceRemovalPreparation) Rollback(ctx context.Context) error {
	return errors.Join(
		p.attention.Rollback(ctx),
		p.session.Rollback(ctx),
		p.windowManager.Rollback(ctx),
	)
}

func windowManagerDefaults(cfg compozyconfig.WindowManagerConfig) windowmanager.Config {
	return windowmanager.Config{
		NewWindowPolicy:     windowmanager.NewWindowPolicy(cfg.NewWindowPolicy),
		SmallViewportPolicy: windowmanager.SmallViewportPolicy(cfg.SmallViewportPolicy),
		FocusPolicy:         windowmanager.FocusPolicy(cfg.FocusPolicy),
		FocusWrap:           cfg.FocusWrap,
		FocusFollowsPointer: cfg.FocusFollowsPointer,
		RaiseOnFocus:        cfg.RaiseOnFocus,
		DragAwayPolicy:      windowmanager.DragAwayPolicy(cfg.DragAwayPolicy),
		GroupMoveModifier:   cfg.GroupMoveModifier,
		SwapModifier:        cfg.SwapModifier,
		HistoryLimit:        cfg.HistoryLimit,
		NavStackLimit:       cfg.NavStackLimit,
		ClosedEntryLimit:    cfg.ClosedEntryLimit,
		DesktopTransition:   windowmanager.DesktopTransition(cfg.DesktopTransition),
		Gaps: windowmanager.GapsConfig{
			Inner: float64(cfg.Gaps.Inner), Top: float64(cfg.Gaps.Top),
			Right: float64(cfg.Gaps.Right), Bottom: float64(cfg.Gaps.Bottom), Left: float64(cfg.Gaps.Left),
		},
		Snap: windowmanager.SnapConfig{
			EdgeBand: float64(cfg.Snap.EdgeBand), CornerReach: float64(cfg.Snap.CornerReach),
			ExitSlack: float64(cfg.Snap.ExitSlack), RepeatRatios: append([]float64(nil), cfg.Snap.RepeatRatios...),
		},
		Bindings: windowmanager.BindingsConfig{
			TopCenter: cfg.Bindings.TopCenter, BottomCenter: cfg.Bindings.BottomCenter,
		},
		Shortcuts: windowmanager.CloneShortcutMap(cfg.Shortcuts),
	}
}

func windowManagerConfig(defaults windowmanager.Config) (compozyconfig.WindowManagerConfig, error) {
	inner, err := windowManagerInteger("gaps.inner", defaults.Gaps.Inner, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	top, err := windowManagerInteger("gaps.top", defaults.Gaps.Top, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	right, err := windowManagerInteger("gaps.right", defaults.Gaps.Right, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	bottom, err := windowManagerInteger("gaps.bottom", defaults.Gaps.Bottom, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	left, err := windowManagerInteger("gaps.left", defaults.Gaps.Left, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	edgeBand, err := windowManagerInteger("snap.edge_band", defaults.Snap.EdgeBand, 4, 128)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	cornerReach, err := windowManagerInteger("snap.corner_reach", defaults.Snap.CornerReach, 16, 512)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	exitSlack, err := windowManagerInteger("snap.exit_slack", defaults.Snap.ExitSlack, 0, 64)
	if err != nil {
		return compozyconfig.WindowManagerConfig{}, err
	}
	return compozyconfig.WindowManagerConfig{
		NewWindowPolicy:     string(defaults.NewWindowPolicy),
		SmallViewportPolicy: string(defaults.SmallViewportPolicy),
		FocusPolicy:         string(defaults.FocusPolicy),
		FocusWrap:           defaults.FocusWrap,
		FocusFollowsPointer: defaults.FocusFollowsPointer,
		RaiseOnFocus:        defaults.RaiseOnFocus,
		DragAwayPolicy:      string(defaults.DragAwayPolicy),
		GroupMoveModifier:   defaults.GroupMoveModifier,
		SwapModifier:        defaults.SwapModifier,
		HistoryLimit:        defaults.HistoryLimit,
		NavStackLimit:       defaults.NavStackLimit,
		ClosedEntryLimit:    defaults.ClosedEntryLimit,
		DesktopTransition:   string(defaults.DesktopTransition),
		Gaps: compozyconfig.WindowManagerGapsConfig{
			Inner: inner, Top: top, Right: right, Bottom: bottom, Left: left,
		},
		Snap: compozyconfig.WindowManagerSnapConfig{
			EdgeBand: edgeBand, CornerReach: cornerReach, ExitSlack: exitSlack,
			RepeatRatios: append([]float64(nil), defaults.Snap.RepeatRatios...),
		},
		Bindings: compozyconfig.WindowManagerBindingConfig{
			TopCenter: defaults.Bindings.TopCenter, BottomCenter: defaults.Bindings.BottomCenter,
		},
		Shortcuts: windowmanager.CloneShortcutMap(defaults.Shortcuts),
	}, nil
}

func windowManagerInteger(path string, value float64, minimum int, maximum int) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value ||
		value < float64(minimum) || value > float64(maximum) {
		return 0, fmt.Errorf(
			"daemon: active window-manager %s must be an integer between %d and %d, got %v",
			path,
			minimum,
			maximum,
			value,
		)
	}
	return int(value), nil
}
