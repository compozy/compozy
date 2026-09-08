package terminal

import (
	"context"
	"errors"
	"fmt"
)

type terminalLaunch struct {
	spec        ProcSpec
	info        Info
	origin      Actor
	settings    Settings
	nonce       string
	titlePinned bool
	startLabel  string
}

func (m *Service) launchTerminal(
	ctx context.Context,
	launch terminalLaunch,
) (*session, terminalKey, error) {
	proc, err := m.pty.Start(ctx, launch.spec)
	if err != nil {
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
		return nil, terminalKey{}, errors.Join(err, cleanupUnregisteredProcess(ctx, proc))
	}
	item.processRecord = processRecord
	key := terminalKey{
		workspaceID: launch.info.WS,
		profileID:   launch.info.ProfileID,
		id:          launch.info.ID,
	}
	if err := m.insert(key, item); err != nil {
		return nil, terminalKey{}, cleanupRegisteredProcess(ctx, proc, processRecord, err)
	}
	return item, key, nil
}
