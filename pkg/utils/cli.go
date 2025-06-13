package utils

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func GetConfigCWD(cmd *cobra.Command) (string, string, error) {
	CWD, err := cmd.Flags().GetString("CWD")
	if err != nil {
		return "", "", fmt.Errorf("failed to get CWD flag: %w", err)
	}
	config, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", "", fmt.Errorf("failed to get config flag: %w", err)
	}

	// Resolve paths
	if CWD == "" {
		CWD, err = filepath.Abs(".")
		if err != nil {
			return "", "", fmt.Errorf("failed to get absolute path: %w", err)
		}
	}
	return CWD, config, nil
}
