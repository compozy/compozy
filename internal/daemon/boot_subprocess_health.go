package daemon

import (
	"fmt"

	taskpkg "github.com/compozy/agh/internal/task"
)

func bootSubprocessHealthEscalator(
	state *bootState,
	store taskStore,
	manager *taskpkg.Service,
) error {
	actor, err := taskpkg.DeriveDaemonActorContext(
		"subprocess_health",
		"daemon.subprocess_health",
	)
	if err != nil {
		return fmt.Errorf("daemon: derive subprocess health escalation actor: %w", err)
	}
	escalator, err := newSubprocessHealthEscalator(
		store,
		escalationActorAdapter{manager: manager, actor: actor},
		state.cfg.Daemon.SubprocessHealthEscalationThreshold,
		state.logger,
	)
	if err != nil {
		return fmt.Errorf("daemon: create subprocess health escalator: %w", err)
	}
	state.subprocessHealth = escalator
	if state.notifier != nil {
		state.notifier.setSubprocessHealthRuntime(escalator)
	}
	return nil
}
