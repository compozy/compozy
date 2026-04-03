package run

import (
	"io"
	"log/slog"
)

func runtimeLogger(quiet bool) *slog.Logger {
	if quiet {
		return silentLogger()
	}
	return slog.Default()
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
