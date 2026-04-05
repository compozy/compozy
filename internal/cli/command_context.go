package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func signalCommandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	baseCtx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		baseCtx = cmd.Context()
	}
	return signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
}
