package daemon

import (
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/session"
)

var _ loopManagedInputSessionManager = (*session.Manager)(nil)
var _ loopManagedInputLifecycleInstaller = (*session.Manager)(nil)

func newLoopGoalProductionRuntime(
	goalStore loopGoalProductionStore,
	sessions SessionManager,
	globalWorkspacePath string,
	policyGate *loopSessionPolicyGate,
	now func() time.Time,
	logger *slog.Logger,
	worktrees loopActionWorktrees,
) *loopGoalRuntime {
	managedInputs, managedOK := sessions.(loopManagedInputSessionManager)
	binder := &loopActionSessionBinder{
		sessions:            sessions,
		bindings:            goalStore,
		bindingAllocator:    goalStore,
		prompts:             goalStore,
		creationStore:       goalStore,
		managedInputs:       managedInputs,
		globalWorkspacePath: globalWorkspacePath,
		policyGate:          policyGate,
		worktrees:           worktrees,
		now:                 now,
	}
	lifecycle := &loopGoalManagedInputLifecycle{store: goalStore, binder: binder, now: now}
	if installer, ok := sessions.(loopManagedInputLifecycleInstaller); ok && managedOK {
		installer.SetManagedInputLifecycle(lifecycle)
	} else if logger != nil {
		logger.Warn(
			"daemon: managed Goal session lifecycle is unavailable",
			"dependency", "session_manager",
		)
	}
	contextRuntime := &loopGoalContextRuntime{sessions: sessions}
	recovery := &loopGoalPromptRecovery{
		store:    goalStore,
		sessions: sessions,
		binder:   binder,
	}
	return &loopGoalRuntime{
		loopActionSessionBinder: binder,
		loopGoalContextRuntime:  contextRuntime,
		loopGoalPromptRecovery:  recovery,
	}
}
