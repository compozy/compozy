package daemon

import (
	"context"
	"fmt"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	aghlogger "github.com/compozy/agh/internal/logger"
	"github.com/compozy/agh/internal/redact"
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
	if err := aghconfig.EnsureHomeLayout(d.homePaths); err != nil {
		return fmt.Errorf("daemon: ensure home layout: %w", err)
	}
	if _, _, err := aghconfig.EnsureBootstrapAgent(d.homePaths); err != nil {
		return fmt.Errorf("daemon: ensure bootstrap agent: %w", err)
	}
	logger := d.logger
	closeLogger := d.closeLogger
	if logger == nil {
		logger, closeLogger, err = aghlogger.New(
			aghlogger.WithLevel(cfg.Log.Level),
			aghlogger.WithFile(d.homePaths.LogFile),
			aghlogger.WithFileRotation(aghlogger.FileRotationConfig{
				MaxSizeMB:       cfg.Log.MaxSizeMB,
				MaxBackups:      cfg.Log.MaxBackups,
				MaxAgeDays:      cfg.Log.MaxAgeDays,
				CompressBackups: cfg.Log.CompressBackups,
			}),
			aghlogger.WithMirrorToStderr(aghlogger.MirrorToStderrEnabled(os.Getenv)),
		)
		if err != nil {
			return fmt.Errorf("daemon: create logger: %w", err)
		}
	} else {
		logger = aghlogger.WithRedaction(logger)
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

func loadConfigFromHome(homePaths aghconfig.HomePaths) (aghconfig.Config, error) {
	cfg := aghconfig.DefaultWithHome(homePaths)
	if err := aghconfig.ApplyConfigOverlayFile(homePaths.ConfigFile, &cfg); err != nil {
		return aghconfig.Config{}, fmt.Errorf("daemon: load global config: %w", err)
	}

	socketPath, err := aghconfig.ResolvePath(cfg.Daemon.Socket)
	if err != nil {
		return aghconfig.Config{}, fmt.Errorf("daemon: normalize daemon socket path: %w", err)
	}
	if strings.TrimSpace(socketPath) != "" {
		cfg.Daemon.Socket = socketPath
	}

	if err := cfg.Validate(); err != nil {
		return aghconfig.Config{}, fmt.Errorf("daemon: validate config: %w", err)
	}

	return cfg, nil
}
