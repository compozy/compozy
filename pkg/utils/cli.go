package utils

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func GetConfigCWD(cmd *cobra.Command) (string, string, error) {
	cwd, err := cmd.Flags().GetString("cwd")
	if err != nil {
		return "", "", fmt.Errorf("failed to get cwd flag: %w", err)
	}
	config, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", "", fmt.Errorf("failed to get config flag: %w", err)
	}

	// Resolve paths
	if cwd == "" {
		cwd, err = filepath.Abs(".")
		if err != nil {
			return "", "", fmt.Errorf("failed to get absolute path: %w", err)
		}
	}
	return cwd, config, nil
}
