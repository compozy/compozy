package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// extractCLIFlags extracts command line flags from a cobra command into a map.
// It processes only flags that have been explicitly changed by the user.
func extractCLIFlags(cmd *cobra.Command, flags map[string]any) {
	// Generic helper to add any flag type
	addFlag := func(flagName, key string, getter func(string) (any, error)) {
		if cmd.Flags().Changed(flagName) {
			if value, err := getter(flagName); err == nil {
				flags[key] = value
			}
		}
	}

	// Define flag extractors with proper type conversion
	getString := func(name string) (any, error) { return cmd.Flags().GetString(name) }
	getInt := func(name string) (any, error) { return cmd.Flags().GetInt(name) }
	getBool := func(name string) (any, error) { return cmd.Flags().GetBool(name) }

	// Flag definitions with their types
	flagDefs := []struct {
		flagName string
		key      string
		getter   func(string) (any, error)
	}{
		// Server flags
		{"host", "host", getString},
		{"port", "port", getInt},
		{"cors", "cors", getBool},

		// Database flags
		{"db-host", "db-host", getString},
		{"db-port", "db-port", getString},
		{"db-user", "db-user", getString},
		{"db-password", "db-password", getString},
		{"db-name", "db-name", getString},
		{"db-ssl-mode", "db-ssl-mode", getString},
		{"db-conn-string", "db-conn-string", getString},

		// Temporal flags
		{"temporal-host", "temporal-host", getString},
		{"temporal-namespace", "temporal-namespace", getString},
		{"temporal-task-queue", "temporal-task-queue", getString},
	}

	// Process all flags
	for _, def := range flagDefs {
		addFlag(def.flagName, def.key, def.getter)
	}
}

// loadEnvFile loads environment variables from a file with security validation
func loadEnvFile(cmd *cobra.Command) (string, error) {
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		return "", fmt.Errorf("failed to get env-file flag: %w", err)
	}
	if envFile != "" {
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		if !filepath.IsAbs(envFile) {
			envFile = filepath.Join(pwd, envFile)
		}
		cleanPath := filepath.Clean(envFile)
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve env file path: %w", err)
		}
		if !isPathWithinDirectory(absPath, pwd) {
			return "", fmt.Errorf("env file path '%s' is outside the project directory", envFile)
		}
		fileInfo, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				return absPath, nil
			}
			return "", fmt.Errorf("failed to stat env file: %w", err)
		}
		if !fileInfo.Mode().IsRegular() {
			return "", fmt.Errorf("env file path '%s' is not a regular file", envFile)
		}
		if err := godotenv.Load(absPath); err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to load env file %s: %w", absPath, err)
			}
		}
		return absPath, nil
	}
	return envFile, nil
}

// isPathWithinDirectory checks if a given path is within the specified directory
func isPathWithinDirectory(path, dir string) bool {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false
	}
	if !strings.HasSuffix(absDir, string(filepath.Separator)) {
		absDir += string(filepath.Separator)
	}
	return strings.HasPrefix(absPath, absDir) || absPath == strings.TrimSuffix(absDir, string(filepath.Separator))
}
