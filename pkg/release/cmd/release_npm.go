package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/spf13/cobra"
)

// NewReleaseNPMCmd creates the release-npm command
func NewReleaseNPMCmd(orch *orchestrator.ReleaseNPMOrchestrator) *cobra.Command {
	var (
		releaseNPMCIOutput bool
		releaseNPMDryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "release-npm",
		Short: "Publish NPM packages after release",
		Long: `Publish NPM packages to the registry after a successful release.

This command handles:
- Validates NPM_TOKEN environment variable
- Publishes all NPM packages in the tools/ directory
- Provides retry logic for network failures`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Execute NPM release workflow
			cfg := orchestrator.ReleaseNPMConfig{
				CIOutput: releaseNPMCIOutput,
				DryRun:   releaseNPMDryRun,
			}
			return orch.Execute(cmd.Context(), cfg)
		},
	}
	cmd.Flags().BoolVar(&releaseNPMCIOutput, "ci-output", false, "Output in CI-friendly format")
	cmd.Flags().BoolVar(&releaseNPMDryRun, "dry-run", false, "Run without making actual changes")
	return cmd
}
