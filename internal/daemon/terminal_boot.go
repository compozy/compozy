package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminaljournal "github.com/compozy/compozy/internal/terminal/journal"
)

func (d *Daemon) bootTerminal(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: terminal state is required")
	}
	if state.workspaceResolver == nil {
		return errors.New("daemon: terminal workspace resolver is required")
	}
	databasePool, err := workspacedb.NewPool(func(resolveCtx context.Context, workspaceID string) (string, error) {
		resolved, resolveErr := state.workspaceResolver.Resolve(resolveCtx, workspaceID)
		if resolveErr != nil {
			return "", resolveErr
		}
		canonicalID := strings.TrimSpace(resolved.WorkspaceID)
		if canonicalID == "" {
			canonicalID = strings.TrimSpace(resolved.ID)
		}
		if canonicalID != strings.TrimSpace(workspaceID) {
			return "", fmt.Errorf("daemon: resolved workspace id %q does not match %q", canonicalID, workspaceID)
		}
		return resolved.RootDir, nil
	})
	if err != nil {
		return fmt.Errorf("daemon: create workspace database pool: %w", err)
	}
	cleanup.add(databasePool.Close)
	journal, err := terminaljournal.New(terminaljournal.Options{
		Databases: databasePool, HomeDir: d.homePaths.HomeDir, Logger: state.logger, Now: d.now,
	})
	if err != nil {
		return fmt.Errorf("daemon: create terminal journal: %w", err)
	}
	options := []terminalpkg.Option{
		terminalpkg.WithProcessRegistry(state.processRegistry),
		terminalpkg.WithLogger(state.logger),
		terminalpkg.WithClock(d.now),
		terminalpkg.WithJournal(journal),
		terminalpkg.WithMarkerConsumer(journal),
	}
	if state.terminalPermissions == nil {
		state.terminalPermissions = newTerminalPermissionBridge()
	}
	options = append(
		options,
		terminalpkg.WithTypingGrantAuthorizer(state.terminalPermissions),
		terminalpkg.WithExecAuthorizer(state.terminalPermissions),
	)
	options = append(options, terminalpkg.WithWorkspaceResolver(state.workspaceResolver))
	if state.profiles != nil {
		options = append(options, terminalpkg.WithProfileNameResolver(state.profiles))
	}
	if state.profiles != nil {
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
	if state.notifier != nil {
		state.notifier.setTerminalRuntime(manager)
	}
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
