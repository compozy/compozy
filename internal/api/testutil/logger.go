package testutil

import (
	"io"
	"log/slog"
)

func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
