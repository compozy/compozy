package helpers

import (
	"os"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
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
		"TF_BUILD",               // Azure DevOps
		"APPVEYOR",               // AppVeyor
		"BAMBOO_BUILD",           // Atlassian Bamboo
		"BITBUCKET_COMMIT",       // Bitbucket Pipelines
		"CODEBUILD_BUILD_ID",     // AWS CodeBuild
		"HEROKU_TEST_RUN_ID",     // Heroku CI
		"TEAMCITY_VERSION",       // TeamCity
		"JENKINS_URL",            // Jenkins (alternative)
		"BUILD_NUMBER",           // Generic build number
		"CONTINUOUS_INTEGRATION", // Generic CI flag
	}

	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// checkExplicitFormat checks for explicit format from configuration
func checkExplicitFormat(cfg *config.Config) (models.Mode, bool) {
	// Check format from configuration (includes CLI flags with proper precedence)
	format := cfg.CLI.DefaultFormat
	if cfg.CLI.OutputFormatAlias != "" {
		format = cfg.CLI.OutputFormatAlias
	}

	switch format {
	case string(OutputFormatJSON):
		return models.ModeJSON, true
	case string(OutputFormatTUI):
		return models.ModeTUI, true
	case "auto":
		return models.ModeJSON, false // fall through to auto-detection
	default:
		return models.ModeJSON, false // fall through to auto-detection
	}
}

// isInteractiveEnvironment checks if we're in an interactive environment
func isInteractiveEnvironment(cfg *config.Config) bool {
	// Allow user to override auto-detection
	if cfg.CLI.Interactive {
		return true
	}

	// Check if running in CI/CD environment
	if isRunningInCI() {
		return false
	}

	// Check if running in a non-interactive environment (check both stdin and stdout)
	stdinIsTerminal := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	stdoutIsTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if !stdinIsTerminal || !stdoutIsTerminal {
		return false
	}

	// Check if NO_COLOR environment variable is set (respecting user preferences)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if TERM environment variable suggests non-interactive terminal
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}

	return true
}

// DetectMode intelligently detects the output mode based on configuration
func DetectMode(cmd *cobra.Command) models.Mode {
	// Get configuration from context
	configValue := cmd.Context().Value(ConfigKey)
	if configValue == nil {
		// Fallback to JSON mode if no config available
		return models.ModeJSON
	}
	cfg, ok := configValue.(*config.Config)
	if !ok {
		// Fallback to JSON mode if config type assertion fails
		return models.ModeJSON
	}

	// Check for explicit format from configuration first
	if mode, found := checkExplicitFormat(cfg); found {
		return mode
	}

	// Check environment for interactivity
	if isInteractiveEnvironment(cfg) {
		return models.ModeTUI
	}

	// Default to JSON mode for non-interactive environments
	return models.ModeJSON
}

// ShouldUseColor determines if colored output should be used
func ShouldUseColor(cmd *cobra.Command) bool {
	// Get configuration from context
	configValue := cmd.Context().Value(ConfigKey)
	if configValue == nil {
		// Fallback to basic environment check if no config available
		return !isRunningInCI() && os.Getenv("NO_COLOR") == ""
	}
	cfg, ok := configValue.(*config.Config)
	if !ok {
		// Fallback to basic environment check if config type assertion fails
		return !isRunningInCI() && os.Getenv("NO_COLOR") == ""
	}

	// Check configuration for no-color setting
	if cfg.CLI.NoColor {
		return false
	}

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is a terminal
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return false
	}

	// Check if running in CI (most CI environments don't handle colors well)
	if isRunningInCI() {
		return false
	}

	// Check TERM environment variable
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}

	return true
}

// GetOutputMode is a convenience function that combines mode detection with color support
func GetOutputMode(cmd *cobra.Command) models.Mode {
	mode := DetectMode(cmd)

	// Force JSON mode if colors are disabled and we're in TUI mode
	// This provides better automation experience
	if mode == models.ModeTUI && !ShouldUseColor(cmd) {
		// Only force JSON if we're clearly in a non-interactive environment
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return models.ModeJSON
		}
	}

	return mode
}
