// Package main is the CompozyOS CLI entrypoint.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/computers/sshsetup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if used, err := sshsetup.RunAskpass(ctx, args, stdout); used {
		if err != nil {
			return 1
		}
		return 0
	}
	return cli.ExecuteContext(ctx, args, stdout, stderr)
}
