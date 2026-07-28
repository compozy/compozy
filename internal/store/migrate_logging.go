package store

import (
	"context"
	"log/slog"
)

type migrationLoggerContextKey struct{}

// WithMigrationLogger overrides migration observability for operations using ctx.
func WithMigrationLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil || logger == nil {
		return ctx
	}
	return context.WithValue(ctx, migrationLoggerContextKey{}, logger)
}

func migrationLogger(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(migrationLoggerContextKey{}).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}
