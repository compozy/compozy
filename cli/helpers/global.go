package helpers

import (
	"fmt"
	"reflect"
	"time"

	"github.com/compozy/compozy/pkg/config/definition"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AddGlobalFlags adds all global flags to the given command
func AddGlobalFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	addRegistryFlags(flags)
}

// addRegistryFlags adds all flags defined in the registry
func addRegistryFlags(flags *pflag.FlagSet) {
	registry := definition.CreateRegistry()
	for _, field := range registry.GetAllFields() {
		if field.CLIFlag != "" {
			addFlagByType(flags, &field)
		}
	}
}

// addFlagByType adds a flag to the flag set based on its type
func addFlagByType(flags *pflag.FlagSet, field *definition.FieldDef) {
	switch field.Type {
	case reflect.TypeOf(""):
		addStringFlag(flags, field)
	case reflect.TypeOf(0):
		addIntFlag(flags, field)
	case reflect.TypeOf(true):
		addBoolFlag(flags, field)
	case reflect.TypeOf(time.Second):
		addDurationFlag(flags, field)
	case reflect.TypeOf([]string{}):
		addStringSliceFlag(flags, field)
	case reflect.TypeOf(int64(0)):
		addInt64Flag(flags, field)
	case reflect.TypeOf(float64(0)):
		addFloat64Flag(flags, field)
	}
	addShorthandFlags(flags, field)
}

func addStringFlag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := ""
	if field.Default != nil {
		if val, ok := field.Default.(string); ok {
			defaultVal = val
		}
	}
	flags.String(field.CLIFlag, defaultVal, field.Help)
}

func addIntFlag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := 0
	if field.Default != nil {
		if val, ok := field.Default.(int); ok {
			defaultVal = val
		}
	}
	flags.Int(field.CLIFlag, defaultVal, field.Help)
}

func addBoolFlag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := false
	if field.Default != nil {
		if val, ok := field.Default.(bool); ok {
			defaultVal = val
		}
	}
	flags.Bool(field.CLIFlag, defaultVal, field.Help)
}

func addDurationFlag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := time.Duration(0)
	if field.Default != nil {
		if val, ok := field.Default.(time.Duration); ok {
			defaultVal = val
		}
	}
	flags.Duration(field.CLIFlag, defaultVal, field.Help)
}

func addStringSliceFlag(flags *pflag.FlagSet, field *definition.FieldDef) {
	var defaultVal []string
	if field.Default != nil {
		if val, ok := field.Default.([]string); ok {
			defaultVal = val
		}
	}
	flags.StringSlice(field.CLIFlag, defaultVal, field.Help)
}

func addInt64Flag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := int64(0)
	if field.Default != nil {
		if val, ok := field.Default.(int64); ok {
			defaultVal = val
		}
	}
	flags.Int64(field.CLIFlag, defaultVal, field.Help)
}

func addFloat64Flag(flags *pflag.FlagSet, field *definition.FieldDef) {
	defaultVal := 0.0
	if field.Default != nil {
		if val, ok := field.Default.(float64); ok {
			defaultVal = val
		}
	}
	flags.Float64(field.CLIFlag, defaultVal, field.Help)
}

func addShorthandFlags(flags *pflag.FlagSet, field *definition.FieldDef) {
	if field.Shorthand != "" {
		if flag := flags.Lookup(field.CLIFlag); flag != nil {
			flag.Shorthand = field.Shorthand
		}
	}
}

// ExtractCLIFlags extracts CLI flag values for configuration override
func ExtractCLIFlags(cmd *cobra.Command) (map[string]any, error) {
	flags := make(map[string]any)
	if err := extractRegistryFlags(cmd, flags); err != nil {
		return nil, err
	}
	if err := postProcessFlags(cmd, flags); err != nil {
		return nil, err
	}
	return flags, nil
}

// extractRegistryFlags extracts all flags defined in the registry
func extractRegistryFlags(cmd *cobra.Command, flags map[string]any) error {
	registry := definition.CreateRegistry()
	for _, field := range registry.GetAllFields() {
		if field.CLIFlag != "" {
			if err := extractFlagByType(cmd, flags, &field); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractFlagByType extracts a flag value based on its type
func extractFlagByType(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	switch field.Type {
	case reflect.TypeOf(""):
		return extractStringFlag(cmd, flags, field)
	case reflect.TypeOf(0):
		return extractIntFlag(cmd, flags, field)
	case reflect.TypeOf(true):
		return extractBoolFlag(cmd, flags, field)
	case reflect.TypeOf(time.Second):
		return extractDurationFlag(cmd, flags, field)
	case reflect.TypeOf([]string{}):
		return extractStringSliceFlag(cmd, flags, field)
	case reflect.TypeOf(int64(0)):
		return extractInt64Flag(cmd, flags, field)
	case reflect.TypeOf(float64(0)):
		return extractFloat64Flag(cmd, flags, field)
	}
	return nil
}

func extractStringFlag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetString(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractIntFlag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetInt(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractBoolFlag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetBool(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractDurationFlag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetDuration(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractFloat64Flag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetFloat64(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractStringSliceFlag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetStringSlice(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

func extractInt64Flag(cmd *cobra.Command, flags map[string]any, field *definition.FieldDef) error {
	if cmd.Flags().Changed(field.CLIFlag) {
		val, err := cmd.Flags().GetInt64(field.CLIFlag)
		if err != nil {
			return fmt.Errorf("failed to get %s flag: %w", field.CLIFlag, err)
		}
		flags[field.CLIFlag] = val
	}
	return nil
}

// postProcessFlags handles special flag logic like --output alias and --no-color conversion
func postProcessFlags(cmd *cobra.Command, flags map[string]any) error {
	// Handle --output alias for --format
	if cmd.Flags().Changed("output") && cmd.Flags().Changed("format") {
		return fmt.Errorf("cannot specify both --format and --output flags")
	}
	if cmd.Flags().Changed("output") && !cmd.Flags().Changed("format") {
		output, err := cmd.Flags().GetString("output")
		if err != nil {
			return fmt.Errorf("failed to get output flag: %w", err)
		}
		flags["format"] = output
	}
	// Handle --no-color flag by setting color-mode to "off"
	if cmd.Flags().Changed("no-color") {
		noColor, err := cmd.Flags().GetBool("no-color")
		if err != nil {
			return fmt.Errorf("failed to get no-color flag: %w", err)
		}
		if noColor {
			flags["color-mode"] = "off"
		}
	}
	return nil
}
