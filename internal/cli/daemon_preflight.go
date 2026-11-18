package cli

import (
	"encoding/json"
	"fmt"

	compozydaemon "github.com/compozy/compozy/internal/daemon"

	"github.com/spf13/cobra"
)

// newDaemonPreflightCommand reports whether this binary can boot the home. A running daemon runs
// it on the replacement binary before signaling itself, and reads the single JSON line it prints.
func newDaemonPreflightCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:    compozydaemon.RestartPreflightCommand,
		Short:  "Internal replacement daemon pre-flight",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonPreflight(cmd, deps)
		},
	}
}

// runDaemonPreflight never takes the runtime context: its gateway profile recovery mutates the home.
func runDaemonPreflight(cmd *cobra.Command, deps commandDeps) error {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	report := compozydaemon.RunRestartPreflight(cmd.Context(), &compozydaemon.RestartPreflightConfig{
		HomePaths: homePaths,
		Getenv:    deps.getenv,
		LoadLaunchConfig: func() error {
			_, loadErr := deps.loadConfig()
			return loadErr
		},
	})
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("cli: encode daemon pre-flight report: %w", err)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", payload); err != nil {
		return fmt.Errorf("cli: write daemon pre-flight report: %w", err)
	}
	if !report.OK {
		return fmt.Errorf("cli: daemon pre-flight failed: %s", report.FailureReason())
	}
	return nil
}
