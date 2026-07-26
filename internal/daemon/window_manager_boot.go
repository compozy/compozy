package daemon

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"

	"github.com/compozy/agh/internal/clientstate"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const windowManagerMaxSnapshotBytes = 16 * 1024 * 1024

func (d *Daemon) bootDefaultWorkspaceAndWindowManager(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if err := d.ensureDefaultWorkspace(ctx, state); err != nil {
		return err
	}
	resolver, err := newWindowManagerStoreWorkspaceResolver(state.workspaceResolver, state.logger)
	if err != nil {
		return err
	}
	engine, err := clientstate.Open(
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
	repository, err := newWindowManagerRepository(engine)
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
		windowmanager.WithEventObserver(newWindowManagerHookObserver(state)),
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
			}, nil
		},
	)
	return nil
}

type workspaceRemovalPreparation struct {
	windowManager workspacepkg.UnregisterPreparation
	session       workspacepkg.UnregisterPreparation
}

func (p workspaceRemovalPreparation) BeforeDelete(ctx context.Context) error {
	if err := p.windowManager.BeforeDelete(ctx); err != nil {
		return err
	}
	return p.session.BeforeDelete(ctx)
}

func (p workspaceRemovalPreparation) Commit(ctx context.Context) error {
	return errors.Join(p.windowManager.Commit(ctx), p.session.Commit(ctx))
}

func (p workspaceRemovalPreparation) Rollback(ctx context.Context) error {
	return errors.Join(p.windowManager.Rollback(ctx), p.session.Rollback(ctx))
}

func windowManagerDefaults(cfg aghconfig.WindowManagerConfig) windowmanager.Config {
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
		Shortcuts: cloneWindowManagerShortcuts(cfg.Shortcuts),
	}
}

func windowManagerConfig(defaults windowmanager.Config) (aghconfig.WindowManagerConfig, error) {
	inner, err := windowManagerInteger("gaps.inner", defaults.Gaps.Inner, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	top, err := windowManagerInteger("gaps.top", defaults.Gaps.Top, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	right, err := windowManagerInteger("gaps.right", defaults.Gaps.Right, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	bottom, err := windowManagerInteger("gaps.bottom", defaults.Gaps.Bottom, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	left, err := windowManagerInteger("gaps.left", defaults.Gaps.Left, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	edgeBand, err := windowManagerInteger("snap.edge_band", defaults.Snap.EdgeBand, 4, 128)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	cornerReach, err := windowManagerInteger("snap.corner_reach", defaults.Snap.CornerReach, 16, 512)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	exitSlack, err := windowManagerInteger("snap.exit_slack", defaults.Snap.ExitSlack, 0, 64)
	if err != nil {
		return aghconfig.WindowManagerConfig{}, err
	}
	return aghconfig.WindowManagerConfig{
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
		DesktopTransition:   string(defaults.DesktopTransition),
		Gaps: aghconfig.WindowManagerGapsConfig{
			Inner: inner, Top: top, Right: right, Bottom: bottom, Left: left,
		},
		Snap: aghconfig.WindowManagerSnapConfig{
			EdgeBand: edgeBand, CornerReach: cornerReach, ExitSlack: exitSlack,
			RepeatRatios: append([]float64(nil), defaults.Snap.RepeatRatios...),
		},
		Bindings: aghconfig.WindowManagerBindingConfig{
			TopCenter: defaults.Bindings.TopCenter, BottomCenter: defaults.Bindings.BottomCenter,
		},
		Shortcuts: cloneWindowManagerShortcuts(defaults.Shortcuts),
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

func cloneWindowManagerShortcuts(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
