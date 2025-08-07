// cmd/dry_run.go
package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/spf13/cobra"
)

func NewDryRunCmd(o *orchestrator.DryRunOrchestrator) *cobra.Command {
	var ciOutput bool
	cmd := &cobra.Command{
		Use:   "dry-run",
		Short: "Perform dry-run validations for release PR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := orchestrator.DryRunConfig{
				CIOutput: ciOutput,
				DryRun:   true,
			}
			return o.Execute(cmd.Context(), cfg)
		},
	}
	cmd.Flags().BoolVar(&ciOutput, "ci-output", false, "Output in CI-friendly format")
	return cmd
}
