// Package main is the AGH CLI entrypoint.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/compozy/agh/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return cli.ExecuteContext(ctx, args, stdout, stderr)
}
