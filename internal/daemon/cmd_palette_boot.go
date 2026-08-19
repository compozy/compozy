package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
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
	executor := &cmdPaletteActionExecutor{
		tools: state.toolRegistry, approvalTokens: state.toolApprovals,
		windowManager: state.windowManager, approvalTTL: state.cfg.Tools.Policy.ApprovalTimeout(),
		now: d.now,
	}
	eventRecorder := &cmdPaletteEventRecorder{writer: state.registry, logger: d.logger}
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
	registry, err := cmdpalette.NewRegistry(
		[]cmdpalette.ProviderRegistration{{
			Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore}, Provider: provider,
		}},
		&cmdPaletteClientDirectory{windowManager: state.windowManager}, nil, executor,
		cmdpalette.WithEventRecorder(eventRecorder), cmdpalette.WithClock(d.now),
		cmdpalette.WithPersonalizationStore(personalizationStore), cmdpalette.WithLogger(d.logger),
	)
	if err != nil {
		return fmt.Errorf("daemon: create command palette registry: %w", err)
	}
	state.cmdPalette = registry
	state.approvalCoordinator = coordinator
	return nil
}
