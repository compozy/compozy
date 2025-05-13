package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

// GetCommonFlags extracts common flags from command
func GetCommonFlags(cmd *cobra.Command) (string, string, string, error) {
	cwd, err := cmd.Flags().GetString("cwd")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get cwd flag: %w", err)
	}

	config, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get config flag: %w", err)
	}

	// Resolve paths
	if cwd == "" {
		cwd, err = filepath.Abs(".")
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get absolute path: %w", err)
		}
	}
	configPath := filepath.Join(cwd, config)

	return cwd, config, configPath, nil
}

// AddCommonFlags adds common flags to a command
func AddCommonFlags(cmd *cobra.Command) {
	cmd.Flags().String("cwd", "", "Working directory for the project")
	cmd.Flags().String("config", "./compozy.yaml", "Path to the project configuration file")
}
