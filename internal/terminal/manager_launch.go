package terminal

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/subprocess"
)

type terminalLaunch struct {
	spec        ProcSpec
	info        Info
	origin      Actor
	settings    Settings
	nonce       string
	titlePinned bool
	startLabel  string
	// worktreeRelease ends the worktree usage lease; the launch owns it once called.
	worktreeRelease func()
}

// launchTerminal starts an owned terminal with workspace settings and actor-bound process identity.
func (m *Service) launchTerminal(
	ctx context.Context,
	launch *terminalLaunch,
) (*session, terminalKey, error) {
	release, err := m.prepareEnvironment(ctx, launch.info.WS, launch.origin, &launch.spec)
	if err != nil {
		releaseWorktreeNow(launch.worktreeRelease)
		return nil, terminalKey{}, err
	}
	transferred := false
	defer func() {
		if !transferred {
			release()
		}
	}()
	proc, err := m.startTerminalProcess(ctx, launch.origin, launch.spec)
	if err != nil {
		releaseWorktreeNow(launch.worktreeRelease)
		return nil, terminalKey{}, fmt.Errorf("terminal: start %s: %w", launch.startLabel, err)
	}
	profileName := m.eventProfileName(ctx, launch.info.ProfileID)
	item := newSession(
		ctx,
		m,
		proc,
		launch.info,
		launch.origin,
		launch.settings,
		launch.nonce,
		profileName,
		launch.spec.Cols,
		launch.spec.Rows,
		launch.titlePinned,
	)
	processRecord, err := m.processRegistration(ctx, item, launch.spec)
	if err != nil {
		cleanupErr := cleanupUnregisteredProcess(ctx, proc)
		m.releaseWorktreeAfterExit(proc, launch.worktreeRelease)
		return nil, terminalKey{}, errors.Join(err, cleanupErr)
	}
	item.processRecord = processRecord
	item.environmentRelease = release
	item.worktreeRelease = launch.worktreeRelease
	key := terminalKey{
		workspaceID: launch.info.WS,
		profileID:   launch.info.ProfileID,
		id:          launch.info.ID,
	}
	if err := m.insert(key, item); err != nil {
		cleanupErr := cleanupRegisteredProcess(ctx, proc, processRecord, err)
		m.releaseWorktreeAfterExit(proc, launch.worktreeRelease)
		return nil, terminalKey{}, cleanupErr
	}
	transferred = true
	return item, key, nil
}

// startTerminalProcess keeps open and exec launches on the same actor-bound priority policy.
func (m *Service) startTerminalProcess(ctx context.Context, actor Actor, spec ProcSpec) (Proc, error) {
	spec.Logger = m.logger
	if actor.Kind == ActorKindAgent {
		spec.Niceness = m.agentNiceness
	}
	proc, err := m.pty.Start(ctx, spec)
	if err != nil {
		return nil, err
	}
	if actor.Kind == ActorKindAgent {
		subprocess.ApplyLaunchNiceness(m.logger, proc.ProcessGroupID(), m.agentNiceness)
	}
	return proc, nil
}
