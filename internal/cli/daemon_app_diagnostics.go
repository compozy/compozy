package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newDaemonAppDiagnosticBundleCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "app-diagnostic-bundle",
		Short:  "Internal desktop diagnostic bundle export",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonAppDiagnosticBundle(cmd, deps)
		},
	}
	return cmd
}

func runDaemonAppDiagnosticBundle(cmd *cobra.Command, deps commandDeps) error {
	if cmd == nil {
		return fmt.Errorf("cli: app diagnostic bundle command is required")
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return fmt.Errorf("app diagnose: resolve home: %w", err)
	}
	report, err := readLocalAppDiagnosticReport(homePaths)
	if err != nil {
		return err
	}
	now := deps.now()
	outputPath, err := resolveAppDiagnosticBundlePath(homePaths, "", now)
	if err != nil {
		return err
	}
	bundle, err := writeLocalAppDiagnosticBundle(homePaths, report, outputPath, now, true)
	if err != nil {
		return err
	}
	result := appDiagnosticBundleExportResult{BundlePath: bundle.Path, Bytes: bundle.Bytes}
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
		return fmt.Errorf("app diagnose: encode internal bundle result: %w", err)
	}
	return nil
}
