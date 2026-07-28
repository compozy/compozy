package daemon

import (
	"context"
	"fmt"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozylogger "github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/redact"
)

func (d *Daemon) bootConfig(state *bootState, cleanup *bootCleanup) error {
	cfg, err := d.loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("daemon: validate config: %w", err)
	}
	redact.SnapshotEnabled(cfg.Redact.Enabled)
	if err := compozyconfig.EnsureHomeLayout(d.homePaths); err != nil {
		return fmt.Errorf("daemon: ensure home layout: %w", err)
	}
	if _, _, err := compozyconfig.EnsureBootstrapAgent(d.homePaths); err != nil {
		return fmt.Errorf("daemon: ensure bootstrap agent: %w", err)
	}
	logger := d.logger
	closeLogger := d.closeLogger
	if logger == nil {
		logger, closeLogger, err = compozylogger.New(
			compozylogger.WithLevel(cfg.Log.Level),
			compozylogger.WithFile(d.homePaths.LogFile),
			compozylogger.WithFileRotation(compozylogger.FileRotationConfig{
				MaxSizeMB:       cfg.Log.MaxSizeMB,
				MaxBackups:      cfg.Log.MaxBackups,
				MaxAgeDays:      cfg.Log.MaxAgeDays,
				CompressBackups: cfg.Log.CompressBackups,
			}),
			compozylogger.WithMirrorToStderr(compozylogger.MirrorToStderrEnabled(os.Getenv)),
		)
		if err != nil {
			return fmt.Errorf("daemon: create logger: %w", err)
		}
	} else {
		logger = compozylogger.WithRedaction(logger)
	}
	if closeLogger == nil {
		closeLogger = func() error { return nil }
	}

	state.cfg = cfg
	state.logger = logger
	state.closeLogger = closeLogger
	cleanup.add(func(context.Context) error {
		return closeLogger()
	})
	return nil
}

func loadConfigFromHome(
	homePaths compozyconfig.HomePaths,
	getenv func(string) string,
) (compozyconfig.Config, error) {
	cfg := compozyconfig.DefaultWithHome(homePaths)
	if err := compozyconfig.ApplyConfigOverlayFile(homePaths.ConfigFile, &cfg); err != nil {
		return compozyconfig.Config{}, fmt.Errorf("daemon: load global config: %w", err)
	}
	compozyconfig.ApplyMarketplaceCatalogEnvironment(&cfg, getenv)

	socketPath, err := compozyconfig.ResolvePath(cfg.Daemon.Socket)
	if err != nil {
		return compozyconfig.Config{}, fmt.Errorf("daemon: normalize daemon socket path: %w", err)
	}
	if strings.TrimSpace(socketPath) != "" {
		cfg.Daemon.Socket = socketPath
	}

	if err := cfg.Validate(); err != nil {
		return compozyconfig.Config{}, fmt.Errorf("daemon: validate config: %w", err)
	}

	return cfg, nil
}
