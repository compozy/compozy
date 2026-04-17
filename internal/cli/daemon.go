package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "daemon",
		Short:        "Manage the home-scoped daemon bootstrap lifecycle",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newDaemonStartCommand(),
		newDaemonStatusCommand(),
	)
	return cmd
}

func newDaemonStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "start",
		Short:        "Start the home-scoped daemon singleton in the foreground",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			result, err := daemon.Start(ctx, daemon.StartOptions{
				Version: version.String(),
			})
			if err != nil {
				return err
			}
			if result.Outcome == daemon.StartOutcomeAlreadyRunning {
				return nil
			}
			defer func() {
				_ = result.Host.Close(context.Background())
			}()

			<-ctx.Done()
			return nil
		},
	}
}

func newDaemonStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Short:        "Show the current daemon readiness state",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := daemon.QueryStatus(context.Background(), compozyconfig.HomePaths{}, daemon.ProbeOptions{})
			if err != nil {
				return err
			}

			if status.Info == nil {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), string(status.State))
				return err
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"state: %s\nhealthy: %t\npid: %d\nsocket: %s\n",
				status.State,
				status.Healthy,
				status.Info.PID,
				status.Info.SocketPath,
			)
			return err
		},
	}
}
