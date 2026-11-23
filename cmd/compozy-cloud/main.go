package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, logger)
	stop()
	if err != nil {
		logger.Error("Compozy Cloud command failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer, logger *slog.Logger) error {
	switch {
	case slices.Equal(args, []string{"serve"}):
		settings, err := loadServeConfig(getenv)
		if err != nil {
			return err
		}
		settings.Cloud.Logger = logger
		return serve(ctx, settings)
	case slices.Equal(args, []string{"migrate", "up"}):
		environment, err := cloudEnvironment(getenv)
		if err != nil {
			return err
		}
		direct, err := databaseURL(getenv, envDirectDSN, environment)
		if err != nil {
			return err
		}
		report, err := store.MigrateUp(ctx, direct)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Cloud schema version %d (%d migrations applied)\n", report.Version, report.Applied)
		return err
	default:
		return errors.New("usage: compozy-cloud serve | compozy-cloud migrate up")
	}
}

func serve(ctx context.Context, settings serveConfig) error {
	app, err := cloud.Open(ctx, settings.Cloud)
	if err != nil {
		return err
	}
	defer app.Close()
	var sockets net.ListenConfig
	listener, err := sockets.Listen(ctx, "tcp", settings.Address)
	if err != nil {
		return fmt.Errorf("listen for cloud HTTP: %w", err)
	}
	return app.Serve(ctx, listener)
}
