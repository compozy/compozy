package daemon

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func (d *Daemon) bootTerminal(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: terminal state is required")
	}
	options := []terminalpkg.Option{
		terminalpkg.WithProcessRegistry(state.processRegistry),
		terminalpkg.WithLogger(state.logger),
		terminalpkg.WithClock(d.now),
	}
	if state.workspaceResolver != nil {
		options = append(options, terminalpkg.WithWorkspaceResolver(state.workspaceResolver))
	}
	if state.profiles != nil {
		options = append(options, terminalpkg.WithProfileNameResolver(state.profiles))
	}
	if state.profiles != nil && state.workspaceResolver != nil {
		options = append(
			options,
			terminalpkg.WithProfileGuard(state.profiles),
			terminalpkg.WithSettingsProvider(terminalSettingsProvider(state)),
		)
	} else {
		settings := terminalSettings(state.cfg.Terminal)
		options = append(options, terminalpkg.WithSettingsProvider(
			func(context.Context, string, string) (terminalpkg.Settings, error) { return settings, nil },
		))
	}
	manager, err := terminalpkg.NewManager(options...)
	if err != nil {
		return fmt.Errorf("daemon: create terminal manager: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start terminal manager: %w", err)
	}
	cleanup.add(manager.Shutdown)
	state.terminals = manager
	return nil
}

func terminalSettingsProvider(state *bootState) terminalpkg.SettingsProvider {
	return func(ctx context.Context, workspaceID, profileID string) (terminalpkg.Settings, error) {
		if state == nil || state.workspaceResolver == nil || state.profiles == nil {
			return terminalpkg.Settings{}, errors.New("daemon: terminal settings dependencies are unavailable")
		}
		profileName, err := state.profiles.ProfileName(ctx, profileID)
		if err != nil {
			return terminalpkg.Settings{}, err
		}
		resolved, err := state.workspaceResolver.ResolveForProfile(ctx, workspaceID, profileName)
		if err != nil {
			return terminalpkg.Settings{}, fmt.Errorf("daemon: resolve terminal workspace profile %q: %w", profileName, err)
		}
		return terminalSettings(resolved.Config.Terminal), nil
	}
}

func terminalSettings(config compozyconfig.TerminalConfig) terminalpkg.Settings {
	return terminalpkg.Settings{
		DefaultShell: config.DefaultShell, ShellIntegration: config.ShellIntegration,
		ScrollbackBytes: config.ScrollbackBytes, DetachedTTL: config.DetachedTTL,
		ExitRetention: config.ExitRetention, Recording: config.Recording,
		RecordingRetentionDays: config.RecordingRetentionDays,
		MaxPerWorkspace:        config.MaxPerWorkspace, MaxPerDaemon: config.MaxPerDaemon,
		MaxSubscribers: config.MaxSubscribers,
	}
}
