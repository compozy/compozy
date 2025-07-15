package auth

import (
	"os"

	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// DetectMode intelligently detects the output mode based on environment
func DetectMode(cmd *cobra.Command) models.Mode {
	// Check if mode is explicitly set via flag
	if modeFlag, err := cmd.Flags().GetString("output"); err == nil && modeFlag != "" {
		if modeFlag == "json" {
			return models.ModeJSON
		}
		if modeFlag == "tui" {
			return models.ModeTUI
		}
	}

	// Check if JSON mode is explicitly requested via flag
	if jsonFlag, err := cmd.Flags().GetBool("json"); err == nil && jsonFlag {
		return models.ModeJSON
	}

	// Check if TUI mode is explicitly requested via flag
	if tuiFlag, err := cmd.Flags().GetBool("tui"); err == nil && tuiFlag {
		return models.ModeTUI
	}

	// Check CI environment variable
	if os.Getenv("CI") != "" {
		return models.ModeJSON
	}

	// Check if running in a non-interactive environment
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return models.ModeJSON
	}

	// Check if NO_COLOR environment variable is set (respecting user preferences)
	if os.Getenv("NO_COLOR") != "" {
		return models.ModeJSON
	}

	// Check for common CI/CD environment variables
	ciVars := []string{
		"JENKINS_HOME",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
		"DRONE",
		"TF_BUILD", // Azure DevOps
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return models.ModeJSON
		}
	}

	// Default to TUI mode for interactive terminals
	return models.ModeTUI
}

// AddModeFlags adds common mode flags to a command
func AddModeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output in JSON format (non-interactive)")
	cmd.Flags().Bool("tui", false, "Force TUI mode (interactive)")
	cmd.Flags().String("output", "", "Output format: json or tui")

	// Mark as mutually exclusive (conceptually - Cobra doesn't have built-in support for this)
	cmd.MarkFlagsMutuallyExclusive("json", "tui")
}
