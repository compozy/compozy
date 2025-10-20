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
	if os.Getenv("CI") != "" {
		return true
	}
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
		return models.ModeTUI, false // fall through to auto-detection with TUI preference
	default:
		return models.ModeTUI, false // fall through to auto-detection with TUI preference
	}
}

// isInteractiveEnvironment checks if we're in an interactive environment
func isInteractiveEnvironment(cfg *config.Config) bool {
	if cfg.CLI.Interactive {
		return true
	}
	if isRunningInCI() {
		return false
	}
	stdinIsTerminal := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	stdoutIsTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if !stdinIsTerminal || !stdoutIsTerminal {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	return true
}

// DetectMode intelligently detects the output mode based on configuration
func DetectMode(cmd *cobra.Command) models.Mode {
	configValue := cmd.Context().Value(ConfigKey)
	if configValue == nil {
		if !isRunningInCI() {
			return models.ModeTUI
		}
		return models.ModeJSON
	}
	cfg, ok := configValue.(*config.Config)
	if !ok {
		if !isRunningInCI() {
			return models.ModeTUI
		}
		return models.ModeJSON
	}
	if mode, found := checkExplicitFormat(cfg); found {
		return mode
	}
	if isInteractiveEnvironment(cfg) {
		return models.ModeTUI
	}
	return models.ModeJSON
}

// ShouldUseColor determines if colored output should be used
func ShouldUseColor(cmd *cobra.Command) bool {
	configValue := cmd.Context().Value(ConfigKey)
	if configValue == nil {
		return !isRunningInCI() && os.Getenv("NO_COLOR") == ""
	}
	cfg, ok := configValue.(*config.Config)
	if !ok {
		return !isRunningInCI() && os.Getenv("NO_COLOR") == ""
	}
	if cfg.CLI.NoColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return false
	}
	if isRunningInCI() {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	return true
}

// GetOutputMode is a convenience function that combines mode detection with color support
func GetOutputMode(cmd *cobra.Command) models.Mode {
	mode := DetectMode(cmd)
	if mode == models.ModeTUI && !ShouldUseColor(cmd) {
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return models.ModeJSON
		}
	}
	return mode
}
