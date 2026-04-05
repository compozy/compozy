package run

import (
	"io"
	"log/slog"

	"github.com/compozy/compozy/internal/core/model"
)

func runtimeLogger(enabled bool) *slog.Logger {
	if !enabled {
		return silentLogger()
	}
	return slog.Default()
}

func runtimeLoggerFor(cfg *config, useUI bool) *slog.Logger {
	if cfg == nil {
		return runtimeLogger(false)
	}
	if cfg.mode == model.ExecutionModeExec {
		return runtimeLogger(cfg.verbose)
	}
	return runtimeLogger(!useUI)
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
