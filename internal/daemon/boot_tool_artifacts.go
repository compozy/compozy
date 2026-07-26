package daemon

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (d *Daemon) bootToolArtifacts(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	retention := toolspkg.ToolArtifactRetention{
		MaxCount: state.cfg.Tools.Artifacts.MaxCount,
		MaxBytes: state.cfg.Tools.Artifacts.MaxBytes,
		MaxAge:   state.cfg.Tools.Artifacts.MaxAge,
	}
	store, err := toolspkg.OpenFilesystemToolArtifactStore(ctx, d.homePaths.ToolArtifactsDir, retention)
	if err != nil {
		return fmt.Errorf("daemon: open tool artifact store: %w", err)
	}
	sweeper := toolspkg.NewToolArtifactSweeper(
		store,
		toolspkg.ToolArtifactSweepInterval,
		func(err error) {
			state.logger.Error("tool artifact retention sweep failed", "error", err)
		},
	)
	if err := sweeper.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start tool artifact retention: %w", err)
	}
	cleanup.add(sweeper.Shutdown)
	state.toolArtifacts = store
	state.deps.ToolArtifacts = store
	state.runtimeWorkers.toolArtifacts = sweeper
	return nil
}
