package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/spf13/cobra"
)

// NewPRReleaseCmd creates the pr-release command
func NewPRReleaseCmd(orch *orchestrator.PRReleaseOrchestrator) *cobra.Command {
	var (
		prReleaseForce    bool
		prReleaseDryRun   bool
		prReleaseCIOutput bool
		prReleaseSkipPR   bool
	)
	cmd := &cobra.Command{
		Use:   "pr-release",
		Short: "Create or update a release pull request",
		Long: `Create or update a release pull request with all necessary changes.

This command orchestrates the entire PR release workflow:
- Checks for changes since the last release
- Calculates the next version
- Creates a release branch
- Updates package versions
- Generates changelog
- Creates or updates a pull request`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Execute PR release workflow
			cfg := orchestrator.PRReleaseConfig{
				ForceRelease: prReleaseForce,
				DryRun:       prReleaseDryRun,
				CIOutput:     prReleaseCIOutput,
				SkipPR:       prReleaseSkipPR,
			}
			return orch.Execute(cmd.Context(), cfg)
		},
	}

	cmd.Flags().BoolVar(&prReleaseForce, "force", false, "Force release even if no changes detected")
	cmd.Flags().BoolVar(&prReleaseDryRun, "dry-run", false, "Run without making actual changes")
	cmd.Flags().BoolVar(&prReleaseCIOutput, "ci-output", false, "Output in CI-friendly format")
	cmd.Flags().BoolVar(&prReleaseSkipPR, "skip-pr", false, "Skip PR creation (for testing)")
	return cmd
}
