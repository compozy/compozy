package cli

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/spf13/cobra"
)

func newRunsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "runs",
		Short:        "Inspect and clean persisted run artifacts",
		SilenceUsage: true,
	}

	cmd.AddCommand(newRunsPurgeCommand())
	return cmd
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
