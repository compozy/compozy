package cli

import (
	"os"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Output format constants
const (
	OutputFormatJSON = "json"
	OutputFormatTUI  = "tui"
)

// isRunningInCI checks if we're running in a CI/CD environment
func isRunningInCI() bool {
	// Check standard CI environment variable
	if os.Getenv("CI") != "" {
		return true
	}
	// Check for common CI/CD environment variables
	return hasAnyEnvVar(getCIEnvironmentVars())
}

// getCIEnvironmentVars returns list of CI environment variables
func getCIEnvironmentVars() []string {
	return []string{
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
}

// hasAnyEnvVar checks if any of the given environment variables are set
func hasAnyEnvVar(vars []string) bool {
	for _, v := range vars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// checkExplicitFlags checks for explicit flag settings
func checkExplicitFlags(cmd *cobra.Command) (models.Mode, bool) {
	// Check string flags
	if mode, found := checkStringFlag(cmd, "output"); found {
		return mode, true
	}
	if mode, found := checkStringFlag(cmd, "format"); found {
		return mode, true
	}

	// Check boolean flags
	if jsonFlag, err := cmd.Flags().GetBool("json"); err == nil && jsonFlag {
		return models.ModeJSON, true
	}
	if tuiFlag, err := cmd.Flags().GetBool("tui"); err == nil && tuiFlag {
		return models.ModeTUI, true
	}

	return models.ModeJSON, false
}

// checkStringFlag checks a string flag for output mode
func checkStringFlag(cmd *cobra.Command, flagName string) (models.Mode, bool) {
	flag, err := cmd.Flags().GetString(flagName)
	if err != nil || flag == "" {
		return models.ModeJSON, false
	}

	switch flag {
	case OutputFormatJSON:
		return models.ModeJSON, true
	case OutputFormatTUI:
		return models.ModeTUI, true
	default:
		return models.ModeJSON, false
	}
}

// isInteractiveEnvironment checks if we're in an interactive environment
func isInteractiveEnvironment() bool {
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

// DetectOutputMode intelligently detects the output mode based on environment
func DetectOutputMode(cmd *cobra.Command) models.Mode {
	// Check for explicit flags first
	if mode, found := checkExplicitFlags(cmd); found {
		return mode
	}

	// Check environment for interactivity
	if isInteractiveEnvironment() {
		return models.ModeTUI
	}

	// Default to JSON mode for non-interactive environments
	return models.ModeJSON
}

// AddOutputModeFlags adds common output mode flags to a command
func AddOutputModeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output in JSON format (non-interactive)")
	cmd.Flags().Bool("tui", false, "Force TUI mode (interactive)")
	cmd.Flags().String("format", "", "Output format: json or tui (deprecated, use --output)")
	cmd.Flags().String("output", "", "Output format: json or tui")

	// Mark as mutually exclusive - include all flag combinations
	cmd.MarkFlagsMutuallyExclusive("json", "tui")
	cmd.MarkFlagsMutuallyExclusive("output", "json")
	cmd.MarkFlagsMutuallyExclusive("output", "tui")
	cmd.MarkFlagsMutuallyExclusive("output", "format")
	cmd.MarkFlagsMutuallyExclusive("format", "json")
	cmd.MarkFlagsMutuallyExclusive("format", "tui")
}

// AddGlobalOutputFlags adds global output-related flags to a command
func AddGlobalOutputFlags(cmd *cobra.Command) {
	AddOutputModeFlags(cmd)
	cmd.Flags().Bool("no-color", false, "Disable colored output")
	cmd.Flags().Bool("quiet", false, "Suppress non-essential output")
	cmd.Flags().Bool("verbose", false, "Enable verbose output")
	cmd.Flags().Bool("debug", false, "Enable debug output")
}

// ShouldUseColor determines if colored output should be used
func ShouldUseColor(cmd *cobra.Command) bool {
	// Check explicit --no-color flag
	if noColor, err := cmd.Flags().GetBool("no-color"); err == nil && noColor {
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

// IsQuietMode checks if quiet mode is enabled
func IsQuietMode(cmd *cobra.Command) bool {
	if quiet, err := cmd.Flags().GetBool("quiet"); err == nil && quiet {
		return true
	}
	return false
}

// IsVerboseMode checks if verbose mode is enabled
func IsVerboseMode(cmd *cobra.Command) bool {
	if verbose, err := cmd.Flags().GetBool("verbose"); err == nil && verbose {
		return true
	}
	return false
}

// IsDebugMode checks if debug mode is enabled
func IsDebugMode(cmd *cobra.Command) bool {
	if debug, err := cmd.Flags().GetBool("debug"); err == nil && debug {
		return true
	}
	return false
}

// GetOutputMode is a convenience function that combines mode detection with color support
func GetOutputMode(cmd *cobra.Command) models.Mode {
	mode := DetectOutputMode(cmd)

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
