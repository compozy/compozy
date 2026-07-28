package network

import (
	"io"
	"log/slog"
)

func discardManagerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
