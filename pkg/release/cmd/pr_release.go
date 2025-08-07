package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/spf13/cobra"
)

// NewPRReleaseCmd creates the pr-release command
func NewPRReleaseCmd(orch *orchestrator.PRReleaseOrchestrator) *cobra.Command {
	var (
		prReleaseForce          bool
		prReleaseDryRun         bool
		prReleaseCIOutput       bool
		prReleaseSkipPR         bool
		prReleaseEnableRollback bool
		prReleaseRollback       bool
		prReleaseSessionID      string
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
- Creates or updates a pull request

With rollback support enabled (--enable-rollback), the workflow can be
automatically rolled back if any step fails, restoring the repository
to its previous state.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Execute PR release workflow
			cfg := orchestrator.PRReleaseConfig{
				ForceRelease:   prReleaseForce,
				DryRun:         prReleaseDryRun,
				CIOutput:       prReleaseCIOutput,
				SkipPR:         prReleaseSkipPR,
				EnableRollback: prReleaseEnableRollback,
				Rollback:       prReleaseRollback,
				SessionID:      prReleaseSessionID,
			}
			return orch.Execute(cmd.Context(), cfg)
		},
	}

	cmd.Flags().BoolVar(&prReleaseForce, "force", false, "Force release even if no changes detected")
	cmd.Flags().BoolVar(&prReleaseDryRun, "dry-run", false, "Run without making actual changes")
	cmd.Flags().BoolVar(&prReleaseCIOutput, "ci-output", false, "Output in CI-friendly format")
	cmd.Flags().BoolVar(&prReleaseSkipPR, "skip-pr", false, "Skip PR creation (for testing)")
	cmd.Flags().BoolVar(&prReleaseEnableRollback, "enable-rollback", false, "Enable automatic rollback on failure")
	cmd.Flags().BoolVar(&prReleaseRollback, "rollback", false, "Rollback a failed release session")
	cmd.Flags().
		StringVar(&prReleaseSessionID, "session-id", "", "Session ID to rollback (uses latest if not specified)")
	return cmd
}
