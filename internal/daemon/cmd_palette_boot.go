package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (d *Daemon) bootCmdPalette(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil || state.registry == nil {
		return errors.New("daemon: command palette registry state is required")
	}
	approvalStore, ok := state.registry.(toolspkg.ApprovalPendingStore)
	if !ok {
		return errors.New("daemon: global registry does not support pending tool approvals")
	}
	personalizationStore, ok := state.registry.(cmdpalette.PersonalizationStore)
	if !ok {
		return errors.New("daemon: global registry does not support command palette personalization")
	}
	provider, err := corecmds.New()
	if err != nil {
		return fmt.Errorf("daemon: build core command palette provider: %w", err)
	}
	extensionProvider := &extensionCmdPaletteProvider{
		runtime: state.currentExtensionRuntime,
		tools:   state.toolRegistry,
		patches: newViewPatchHub(),
	}
	state.viewPatches = extensionProvider
	cleanup.add(func(context.Context) error {
		extensionProvider.CloseViewPatches()
		return nil
	})
	executor := &cmdPaletteActionExecutor{
		tools: state.toolRegistry, approvalTokens: state.toolApprovals,
		windowManagers: state.windowManagers, approvalTTL: state.cfg.Tools.Policy.ApprovalTimeout(),
		now: d.now,
	}
	eventRecorder := &cmdPaletteEventRecorder{writer: state.registry, logger: state.logger}
	coordinator, err := toolspkg.NewApprovalCoordinator(approvalStore, executor)
	if err != nil {
		return fmt.Errorf("daemon: create tool approval coordinator: %w", err)
	}
	if err := coordinator.Recover(ctx); err != nil {
		closeErr := coordinator.Close()
		return errors.Join(fmt.Errorf("daemon: recover tool approvals: %w", err), closeErr)
	}
	cleanup.add(func(context.Context) error { return coordinator.Close() })
	executor.approvals = coordinator
	var registry *cmdpalette.Service
	bindings := &cmdPaletteBindingsResolver{
		workspaces: state.workspaceResolver,
		loadGlobal: d.loadConfig,
		loadProfile: func(profileName, workspaceRoot string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(
				d.homePaths,
				compozyconfig.WithProfile(profileName),
				compozyconfig.WithWorkspaceRoot(workspaceRoot),
			)
		},
		catalog: func() cmdpalette.BindableCatalog { return registry },
		logger:  state.logger,
	}
	registry, err = cmdpalette.NewRegistry(
		[]cmdpalette.ProviderRegistration{{
			Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore}, Provider: provider,
		}, {
			Source: cmdpalette.Source{Kind: cmdpalette.SourceKindExtension}, Provider: extensionProvider,
		}},
		&cmdPaletteClientDirectory{windowManagers: state.windowManagers}, bindings, executor,
		cmdpalette.WithEventRecorder(eventRecorder), cmdpalette.WithClock(d.now),
		cmdpalette.WithPersonalizationStore(personalizationStore),
		cmdpalette.WithPersonalizationPolicy(bindings), cmdpalette.WithLogger(state.logger),
		cmdpalette.WithDynamicViewProvider(extensionProvider),
		cmdpalette.WithViewProgramProvider(extensionProvider),
	)
	if err != nil {
		return fmt.Errorf("daemon: create command palette registry: %w", err)
	}
	state.cmdPalette = registry
	state.approvalCoordinator = coordinator
	state.deps.CmdPalette = registry
	state.deps.ApprovalCoordinator = coordinator
	return nil
}
