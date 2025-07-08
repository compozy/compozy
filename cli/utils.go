package cli

import (
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
