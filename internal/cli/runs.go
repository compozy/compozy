package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/spf13/cobra"
)

func newRunsCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	defaults = defaults.withFallbacks()

	cmd := &cobra.Command{
		Use:          "runs",
		Short:        "Inspect, attach, watch, and clean persisted daemon run artifacts",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newRunsAttachCommand(defaults),
		newRunsWatchCommand(),
		newRunsPurgeCommand(),
	)
	return cmd
}

func newRunsAttachCommand(defaults commandStateDefaults) *cobra.Command {
	return &cobra.Command{
		Use:          "attach <run-id>",
		Short:        "Attach the interactive TUI to one daemon-managed run",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := strings.TrimSpace(args[0])
			isInteractive := defaults.isInteractive
			if isInteractive == nil {
				isInteractive = isInteractiveTerminal
			}
			if !isInteractive() {
				return withExitCode(
					1,
					fmt.Errorf(
						"%s requires an interactive terminal for ui attach; rerun with `compozy runs watch %s`",
						cmd.CommandPath(),
						runID,
					),
				)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			if err := attachCLIRunUI(ctx, client, runID); err != nil {
				if errors.Is(err, errRunSettledBeforeUIAttach) {
					if err := watchCLIRun(ctx, cmd.OutOrStdout(), client, runID); err != nil {
						return mapDaemonCommandError(err)
					}
					return nil
				}
				return mapDaemonCommandError(err)
			}
			return nil
		},
	}
}

func newRunsWatchCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "watch <run-id>",
		Short:        "Stream textual observation for one daemon-managed run",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			if err := watchCLIRun(ctx, cmd.OutOrStdout(), client, strings.TrimSpace(args[0])); err != nil {
				return mapDaemonCommandError(err)
			}
			return nil
		},
	}
}

func newRunsPurgeCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "purge",
		Short:        "Delete terminal run artifacts according to configured retention",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			settings, _, err := daemon.LoadRunLifecycleSettings(ctx)
			if err != nil {
				return err
			}

			paths, err := compozyconfig.ResolveHomePaths()
			if err != nil {
				return err
			}
			if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
				return err
			}

			db, err := globaldb.Open(ctx, paths.GlobalDBPath)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			result, err := daemon.PurgeTerminalRuns(ctx, db, settings)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "purged %d run(s)\n", len(result.PurgedRunIDs))
			return err
		},
	}
}
