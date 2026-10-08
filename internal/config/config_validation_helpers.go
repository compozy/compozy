package config

import (
	"errors"
	"fmt"
)

func normalizeConfigPaths(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	socket, err := expandUserPath(cfg.Daemon.Socket)
	if err != nil {
		return fmt.Errorf("expand daemon.socket: %w", err)
	}
	cfg.Daemon.Socket = socket

	return nil
}
