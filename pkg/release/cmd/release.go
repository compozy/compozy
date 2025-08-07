package cmd

import (
	"github.com/compozy/compozy/pkg/release/internal/orchestrator"
	"github.com/spf13/cobra"
)

// NewReleaseCmd creates the release command
func NewReleaseCmd(orch *orchestrator.ReleaseOrchestrator) *cobra.Command {
	var (
		releaseVersion       string
		releaseSkipTag       bool
		releaseSkipNPM       bool
		releaseSkipChangelog bool
		releaseCIOutput      bool
		releaseDryRun        bool
	)
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Execute a production release",
		Long: `Execute a production release after a release PR is merged.

This command orchestrates the entire release workflow:
- Extracts or validates the version
- Creates a git tag
- Generates the final changelog
- Runs GoReleaser (if configured)
- Publishes NPM packages
- Updates the main changelog`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Execute release workflow
			cfg := orchestrator.ReleaseConfig{
				Version:       releaseVersion,
				SkipTag:       releaseSkipTag,
				SkipNPM:       releaseSkipNPM,
				SkipChangelog: releaseSkipChangelog,
				CIOutput:      releaseCIOutput,
				DryRun:        releaseDryRun,
			}
			return orch.Execute(cmd.Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&releaseVersion, "version", "", "Version to release (auto-detected if not provided)")
	cmd.Flags().BoolVar(&releaseSkipTag, "skip-tag", false, "Skip git tag creation")
	cmd.Flags().BoolVar(&releaseSkipNPM, "skip-npm", false, "Skip NPM package publishing")
	cmd.Flags().BoolVar(&releaseSkipChangelog, "skip-changelog", false, "Skip changelog generation")
	cmd.Flags().BoolVar(&releaseCIOutput, "ci-output", false, "Output in CI-friendly format")
	cmd.Flags().BoolVar(&releaseDryRun, "dry-run", false, "Run without making actual changes")
	return cmd
}
