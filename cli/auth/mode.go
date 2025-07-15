package auth

import (
	"os"

	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// isRunningInCI checks if we're running in a CI/CD environment
func isRunningInCI() bool {
	// Check standard CI environment variable
	if os.Getenv("CI") != "" {
		return true
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
			return true
		}
	}
	return false
}

// DetectMode intelligently detects the output mode based on environment
func DetectMode(cmd *cobra.Command) models.Mode {
	// Check if mode is explicitly set via flag
	if modeFlag, err := cmd.Flags().GetString("output"); err == nil && modeFlag != "" {
		switch modeFlag {
		case "json":
			return models.ModeJSON
		case "tui":
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

	// Check if running in CI/CD environment
	if isRunningInCI() {
		return models.ModeJSON
	}

	// Check if running in a non-interactive environment (check both stdin and stdout)
	stdinIsTerminal := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	stdoutIsTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if !stdinIsTerminal || !stdoutIsTerminal {
		return models.ModeJSON
	}

	// Check if NO_COLOR environment variable is set (respecting user preferences)
	if os.Getenv("NO_COLOR") != "" {
		return models.ModeJSON
	}

	// Default to TUI mode for interactive terminals
	return models.ModeTUI
}

// AddModeFlags adds common mode flags to a command
func AddModeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output in JSON format (non-interactive)")
	cmd.Flags().Bool("tui", false, "Force TUI mode (interactive)")
	cmd.Flags().String("output", "", "Output format: json or tui")

	// Mark as mutually exclusive - include output flag with json/tui for completeness
	cmd.MarkFlagsMutuallyExclusive("json", "tui")
	cmd.MarkFlagsMutuallyExclusive("output", "json")
	cmd.MarkFlagsMutuallyExclusive("output", "tui")
}
