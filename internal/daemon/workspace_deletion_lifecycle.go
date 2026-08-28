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

func shutdownWorkspaceAndTerminals(
	ctx context.Context,
	targets *shutdownTargets,
	errs *[]error,
) {
	if targets.workspaceFinalizer != nil {
		appendWrappedError(
			errs,
			"daemon: drain workspace deletion finalization",
			targets.workspaceFinalizer.DrainUnregisters(ctx),
		)
	}
	if targets.terminals != nil {
		appendWrappedError(errs, "daemon: shutdown terminal runtime", targets.terminals.Shutdown(ctx))
	}
}
