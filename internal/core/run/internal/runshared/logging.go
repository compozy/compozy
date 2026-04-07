package runshared

import (
	"io"
	"log/slog"

	"github.com/compozy/compozy/internal/core/model"
)

func RuntimeLogger(enabled bool) *slog.Logger {
	if !enabled {
		return SilentLogger()
	}
	return slog.Default()
}

func RuntimeLoggerFor(cfg *Config, useUI bool) *slog.Logger {
	if cfg == nil {
		return RuntimeLogger(false)
	}
	if cfg.Mode == model.ExecutionModeExec {
		return RuntimeLogger(cfg.Verbose)
	}
	return RuntimeLogger(!useUI)
}

func SilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
