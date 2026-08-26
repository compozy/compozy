package daemon

import (
	"context"
	"errors"
)

func (d *Daemon) bootResourceWatchers(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	agentSkillResources := state.agentSkillResources
	hookBindings := state.hookBindings
	if state.skillsRegistry != nil {
		if hookBindings == nil {
			return errors.New("daemon: hook bindings are required before starting the skills watcher")
		}
		state.skillsCancel, state.skillsDone = startSkillsWatcher(
			ctx,
			state.skillsRegistry,
			state.cfg.Skills.PollInterval,
			workspaceSkillWatcherRoots(
				d.homePaths, state.registry, state.workspaceResolver, bootProfileCatalog{state: state},
			),
			workspaceAgentWatcherRoots(d.homePaths, state.registry),
			func(refreshCtx context.Context) error {
				if agentSkillResources != nil {
					if err := agentSkillResources.Sync(refreshCtx); err != nil {
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

	loopResources := state.loopResources
	if loopResources != nil {
		state.loopsCancel, state.loopsDone = startLoopWatcher(
			ctx,
			state.cfg.Skills.PollInterval,
			[]string{d.homePaths.LoopsDir},
			workspaceLoopWatcherRoots(d.homePaths, state.registry),
			loopResources,
			state.logger,
		)
		cleanup.add(func(cleanupCtx context.Context) error {
			return stopLoopWatcher(cleanupCtx, state.loopsCancel, state.loopsDone)
		})
	}
	return nil
}
