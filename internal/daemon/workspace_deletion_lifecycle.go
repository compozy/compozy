package daemon

import (
	"context"
	"fmt"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func configureWorkspaceDeletionLifecycle(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
	cleanup *bootCleanup,
) error {
	if err := installWorkspaceRemovalPreparer(state, sessions); err != nil {
		return err
	}
	cleanup.add(state.workspaceResolver.DrainUnregisters)
	if err := state.workspaceResolver.ResumeUnregisters(ctx); err != nil {
		return fmt.Errorf("daemon: resume workspace deletion finalization: %w", err)
	}
	return nil
}

func newWorkspaceRuntimeState(resolver *workspacepkg.Resolver) workspaceRuntimeState {
	return workspaceRuntimeState{workspaceResolver: resolver, workspaceFinalizer: resolver}
}

// drainWorkspaceDeletions joins workspace deletion finalization, which stops the sessions it removes,
// before the daemon takes its own session snapshot.
func drainWorkspaceDeletions(ctx context.Context, targets *shutdownTargets, errs *[]error) {
	if targets.workspaceFinalizer != nil {
		appendWrappedError(
			errs,
			"daemon: drain workspace deletion finalization",
			targets.workspaceFinalizer.DrainUnregisters(ctx),
		)
	}
}
