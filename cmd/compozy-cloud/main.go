// SPDX-License-Identifier: FSL-1.1-MIT

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
		direct, err := databaseURL(getenv, envNeonDirect, environment)
		if err != nil {
			return err
		}
		report, err := store.MigrateUp(ctx, direct)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Cloud schema version %d (%d migrations applied)\n", report.Version, report.Applied)
		return err
	case len(args) >= 2 && slices.Equal(args[:2], []string{"broadcast", "send"}):
		return sendBroadcast(ctx, args[2:], getenv, out)
	case len(args) == 3 && slices.Equal(args[:2], []string{"keys", "generate"}):
		return generateSigningKey(args[2], out)
	case slices.Equal(args, []string{"keys", "jwk"}):
		return printSigningJWK(getenv, out)
	case slices.Equal(args, []string{"keys", "publish"}):
		return publishSigningKeys(ctx, "", getenv)
	case len(args) == 3 && slices.Equal(args[:2], []string{"keys", "retire"}):
		if args[2] == "" {
			return errors.New("keys retire requires a KID")
		}
		return publishSigningKeys(ctx, args[2], getenv)
	case len(args) == 5 && slices.Equal(args[:3], []string{"email", "privacy-reply", "--request-id"}):
		return sendPrivacyReply(ctx, args[3], args[4], getenv)
	case slices.Equal(args, []string{"backup", "security-events"}):
		return shipEvents(ctx, getenv, out)
	case len(args) == 4 && slices.Equal(args[:3], []string{"restore", "finalize", "--restore-point"}):
		return restoreFinalize(ctx, args[3], getenv, out, logger)
	default:
		return errors.New(
			"usage: compozy-cloud serve | migrate up | keys {generate|jwk|publish|retire} | " +
				"email privacy-reply --request-id <id> <file> | broadcast send | " +
				"backup security-events | restore finalize --restore-point <time>",
		)
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
