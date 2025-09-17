package helpers

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagCategory represents a group of related flags
type FlagCategory struct {
	Name        string
	Description string
	Flags       []string
}

// GetFlagCategories returns all flag categories with their flags
func GetFlagCategories() []FlagCategory {
	return getFlagCategoryDefinitions()
}

func getFlagCategoryDefinitions() []FlagCategory {
	categories := []FlagCategory{}
	categories = append(categories, getCoreCategories()...)
	categories = append(categories, getServerCategories()...)
	categories = append(categories, getRuntimeCategories()...)
	categories = append(categories, getAttachmentsCategories()...)
	categories = append(categories, getInfrastructureCategories()...)
	return categories
}

func getCoreCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Core Configuration",
			Description: "Essential configuration flags",
			Flags: []string{
				"config", "env-file", "cwd", "help", "version",
			},
		},
		{
			Name:        "Output & Display",
			Description: "Control output formatting and display",
			Flags: []string{
				"format", "output", "color-mode", "no-color",
				"quiet", "debug", "log-level", "interactive",
			},
		},
		{
			Name:        "Authentication & Security",
			Description: "Authentication and API access settings",
			Flags: []string{
				"api-key", "auth-enabled", "auth-workflow-exceptions",
			},
		},
	}
}

func getServerCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Server Configuration",
			Description: "API server and network settings",
			Flags: []string{
				"host", "port", "base-url", "cors",
				"cors-allowed-origins", "cors-allow-credentials", "cors-max-age",
			},
		},
		{
			Name:        "Database Configuration",
			Description: "PostgreSQL connection settings",
			Flags: []string{
				"db-host", "db-port", "db-user", "db-password",
				"db-name", "db-ssl-mode", "db-conn-string", "db-auto-migrate",
				"db-migration-timeout",
			},
		},
	}
}

func getRuntimeCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Runtime & Performance",
			Description: "Execution runtime and performance tuning",
			Flags: []string{
				"runtime-type", "entrypoint-path", "bun-permissions",
				"tool-execution-timeout", "timeout", "page-size",
			},
		},
		{
			Name:        "LLM Configuration",
			Description: "LLM proxy and orchestration behavior",
			Flags: []string{
				"llm-proxy-url", "llm-mcp-readiness-timeout", "llm-mcp-readiness-poll-interval",
				"llm-mcp-header-template-strict", "llm-retry-attempts", "llm-retry-backoff-base",
				"llm-retry-backoff-max", "llm-retry-jitter", "llm-max-concurrent-tools",
				"llm-max-tool-iterations", "llm-max-sequential-tool-errors",
				"llm-allowed-mcp-names", "llm-fail-on-mcp-registration-error",
				"llm-mcp-client-timeout", "llm-retry-jitter-percent",
			},
		},
		{
			Name:        "Dispatcher & Workers",
			Description: "Task dispatcher and worker settings",
			Flags: []string{
				"dispatcher-heartbeat-interval", "dispatcher-heartbeat-ttl",
				"dispatcher-stale-threshold", "async-token-counter-workers",
				"async-token-counter-buffer-size",
			},
		},
		{
			Name:        "Resource Limits",
			Description: "System resource limits and constraints",
			Flags: []string{
				"max-nesting-depth", "max-string-length",
				"max-message-content-length", "max-total-content-size",
			},
		},
	}
}

func getAttachmentsCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Attachments & Media",
			Description: "Attachment limits, timeouts and redirects",
			Flags: []string{
				"attachments-max-download-size",
				"attachments-download-timeout",
				"attachments-max-redirects",
				"attachments-http-user-agent",
				"attachments-ssrf-strict",
			},
		},
	}
}

func getInfrastructureCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Temporal Configuration",
			Description: "Workflow orchestration settings",
			Flags: []string{
				"temporal-host", "temporal-namespace", "temporal-task-queue",
			},
		},
		{
			Name:        "MCP Proxy Configuration",
			Description: "Model Context Protocol proxy settings",
			Flags: []string{
				"mcp-host", "mcp-port", "mcp-base-url",
			},
		},
		{
			Name:        "Webhooks Configuration",
			Description: "Webhook processing and validation settings",
			Flags: []string{
				"webhook-default-method", "webhook-default-max-body",
				"webhook-default-dedupe-ttl", "webhook-stripe-skew",
			},
		},
	}
}

// SetupCategorizedHelp configures the command to use categorized help
func SetupCategorizedHelp(cmd *cobra.Command) {
	// Since Cobra doesn't support custom FuncMap in SetHelpTemplate,
	// we use SetHelpFunc which is the official way for complex help customization
	cmd.SetHelpFunc(categorizedHelpFunc)
}

// Define ANSI color codes
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorPrimary = "\033[36m" // Cyan
	colorSuccess = "\033[32m" // Green
	colorMuted   = "\033[90m" // Bright black (gray)
)

// createSeparator creates a styled separator line
func createSeparator(width int, noColor bool) string {
	if noColor {
		// Simple dashed line for no-color mode
		return strings.Repeat("─", width)
	}

	// Create a lipgloss style for the separator
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")). // Dark gray
		Width(width)

	// Use box-drawing character for a clean line
	return separatorStyle.Render(strings.Repeat("─", width))
}

// getTerminalWidth attempts to get terminal width, with fallback
func getTerminalWidth() int {
	width := lipgloss.Width(strings.Repeat("x", 80)) // Start with default
	if lipgloss.Width(strings.Repeat("x", 100)) == 100 {
		// If we can display 100 chars, use 100 as width
		width = 100
	}
	return width
}

// categorizedHelpFunc is a custom help function that groups flags by category
func categorizedHelpFunc(cmd *cobra.Command, _ []string) {
	// Check if color should be disabled
	noColor := cmd.Flag("no-color") != nil && cmd.Flag("no-color").Value.String() == "true"

	// Print description
	if cmd.Long != "" {
		fmt.Println(cmd.Long)
	} else if cmd.Short != "" {
		fmt.Println(cmd.Short)
	}
	fmt.Println()

	// Print usage
	fmt.Printf("%sUsage:%s\n", applyColor(colorBold, noColor), applyColor(colorReset, noColor))
	if cmd.Runnable() {
		fmt.Printf("  %s\n", cmd.UseLine())
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Printf("  %s [command]\n", cmd.CommandPath())
	}
	fmt.Println()

	// Print available commands
	if cmd.HasAvailableSubCommands() {
		fmt.Printf("%sAvailable Commands:%s\n", applyColor(colorBold, noColor), applyColor(colorReset, noColor))
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() || c.Name() == "help" {
				fmt.Printf("  %s%-*s%s %s\n",
					applyColor(colorSuccess, noColor),
					cmd.NamePadding(),
					c.Name(),
					applyColor(colorReset, noColor),
					c.Short)
			}
		}
		fmt.Println()
	}

	// Check if this is a subcommand (not root)
	if cmd.Parent() != nil {
		// Print command-specific flags first
		printCommandSpecificFlags(cmd, noColor)
	}

	// Print categorized global flags
	printCategorizedFlags(cmd, noColor)

	// Print footer
	if cmd.HasAvailableSubCommands() {
		fmt.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	}
}

// applyColor returns the color code if colors are enabled, otherwise empty string
func applyColor(color string, noColor bool) string {
	if noColor {
		return ""
	}
	return color
}

// printCommandSpecificFlags prints only the flags specific to this command (not inherited)
func printCommandSpecificFlags(cmd *cobra.Command, noColor bool) {
	localFlags := cmd.LocalFlags()
	if !localFlags.HasAvailableFlags() {
		return
	}

	// Count visible local flags
	visibleCount := 0
	localFlags.VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden {
			visibleCount++
		}
	})

	if visibleCount == 0 {
		return
	}

	// Print section with separators
	width := getTerminalWidth()
	fmt.Println(createSeparator(width, noColor))
	fmt.Printf(
		"%sFlags%s\n",
		applyColor(colorPrimary+colorBold, noColor),
		applyColor(colorReset, noColor),
	)
	fmt.Println(createSeparator(width, noColor))
	fmt.Println() // Extra line after header

	// Print each local flag
	localFlags.VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden {
			printFlag(flag, noColor)
		}
	})
	fmt.Println()
}

// printCategorizedFlags prints flags organized by category
func printCategorizedFlags(cmd *cobra.Command, noColor bool) {
	categories := GetFlagCategories()
	flagMap := collectAllFlags(cmd)
	displayedFlags := make(map[string]bool)
	isSubcommand := cmd.Parent() != nil

	// If this is a subcommand, mark local flags as already displayed
	if isSubcommand {
		markLocalFlagsDisplayed(cmd, displayedFlags)
		printGlobalFlagsHeader(flagMap, displayedFlags, noColor)
	} else {
		// For root command, print main flags header
		width := getTerminalWidth()
		fmt.Println(createSeparator(width, noColor))
		fmt.Printf(
			"%sFlags%s\n",
			applyColor(colorPrimary+colorBold, noColor),
			applyColor(colorReset, noColor),
		)
		fmt.Println(createSeparator(width, noColor))
		fmt.Println() // Extra line after header
	}

	// Process each category
	printFlagCategories(categories, flagMap, displayedFlags, noColor)

	// Handle uncategorized flags
	printUncategorizedFlags(flagMap, displayedFlags, noColor)
}

// markLocalFlagsDisplayed marks local flags as already displayed
func markLocalFlagsDisplayed(cmd *cobra.Command, displayedFlags map[string]bool) {
	localFlags := cmd.LocalFlags()
	localFlags.VisitAll(func(flag *pflag.Flag) {
		displayedFlags[flag.Name] = true
	})
}

// printGlobalFlagsHeader prints the global flags header for subcommands
func printGlobalFlagsHeader(flagMap map[string]*pflag.Flag, displayedFlags map[string]bool, noColor bool) {
	hasGlobalFlags := false
	for name, flag := range flagMap {
		if !displayedFlags[name] && !flag.Hidden {
			hasGlobalFlags = true
			break
		}
	}
	if hasGlobalFlags {
		width := getTerminalWidth()
		fmt.Println(createSeparator(width, noColor))
		fmt.Printf(
			"%sGlobal Flags%s\n",
			applyColor(colorPrimary+colorBold, noColor),
			applyColor(colorReset, noColor),
		)
		fmt.Println(createSeparator(width, noColor))
		fmt.Println() // Extra line after header
	}
}

// printFlagCategories prints all flag categories
func printFlagCategories(categories []FlagCategory, flagMap map[string]*pflag.Flag,
	displayedFlags map[string]bool, noColor bool) {
	for _, category := range categories {
		categoryFlags := collectCategoryFlags(category, flagMap, displayedFlags)
		if len(categoryFlags) == 0 {
			continue
		}
		printCategory(category.Name, categoryFlags, noColor)
	}
}

// collectCategoryFlags collects flags for a specific category
func collectCategoryFlags(category FlagCategory, flagMap map[string]*pflag.Flag,
	displayedFlags map[string]bool) []*pflag.Flag {
	var categoryFlags []*pflag.Flag
	for _, flagName := range category.Flags {
		if flag, exists := flagMap[flagName]; exists && !flag.Hidden && !displayedFlags[flagName] {
			categoryFlags = append(categoryFlags, flag)
			displayedFlags[flagName] = true
		}
	}
	return categoryFlags
}

// printCategory prints a category with its flags
func printCategory(name string, flags []*pflag.Flag, noColor bool) {
	fmt.Printf(
		"%s%s:%s\n",
		applyColor(colorPrimary+colorBold, noColor),
		name,
		applyColor(colorReset, noColor),
	)
	for _, flag := range flags {
		printFlag(flag, noColor)
	}
	fmt.Println()
}

// printUncategorizedFlags prints flags that don't belong to any category
func printUncategorizedFlags(flagMap map[string]*pflag.Flag, displayedFlags map[string]bool, noColor bool) {
	var uncategorizedFlags []*pflag.Flag
	for name, flag := range flagMap {
		if !displayedFlags[name] && !flag.Hidden {
			uncategorizedFlags = append(uncategorizedFlags, flag)
		}
	}
	if len(uncategorizedFlags) > 0 {
		// Print section header with separators
		width := getTerminalWidth()
		fmt.Println(createSeparator(width, noColor))
		fmt.Printf(
			"%sOther Flags%s\n",
			applyColor(colorPrimary+colorBold, noColor),
			applyColor(colorReset, noColor),
		)
		fmt.Println(createSeparator(width, noColor))
		fmt.Println() // Extra line after header

		// Print the flags
		for _, flag := range uncategorizedFlags {
			printFlag(flag, noColor)
		}
		fmt.Println()
	}
}

// printFlag prints a single flag with formatting
func printFlag(flag *pflag.Flag, noColor bool) {
	// Build the flag line
	var line strings.Builder
	line.WriteString("  ")

	// Add shorthand if available
	if flag.Shorthand != "" {
		line.WriteString(fmt.Sprintf("-%s, ", flag.Shorthand))
	} else {
		line.WriteString("    ")
	}

	// Add the flag name with color
	line.WriteString(
		fmt.Sprintf("%s--%s%s", applyColor(colorSuccess, noColor), flag.Name, applyColor(colorReset, noColor)),
	)

	// Add type indicator for non-boolean flags
	if flag.Value.Type() != "bool" {
		typeStr := flag.Value.Type()
		switch typeStr {
		case "stringSlice":
			typeStr = "strings"
		case "duration":
			typeStr = "duration"
		case "int", "int64":
			typeStr = "int"
		}
		line.WriteString(
			fmt.Sprintf(" %s%s%s", applyColor(colorMuted, noColor), typeStr, applyColor(colorReset, noColor)),
		)
	}

	// Calculate padding for alignment
	// Account for ANSI codes when calculating length
	visibleLength := calculateVisibleLength(line.String())
	const columnWidth = 50
	if visibleLength < columnWidth {
		line.WriteString(strings.Repeat(" ", columnWidth-visibleLength))
	} else {
		line.WriteString(" ")
	}

	// Add the usage/help text
	usage := flag.Usage
	if usage != "" {
		// Check if there's a default value
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "[]" {
			usage += fmt.Sprintf(" (default %s)", flag.DefValue)
		}
		line.WriteString(usage)
	}

	fmt.Println(line.String())
}

// calculateVisibleLength calculates the visible length of a string, excluding ANSI codes
func calculateVisibleLength(s string) int {
	// Simple implementation: count characters that are not part of ANSI sequences
	inANSI := false
	length := 0
	for _, r := range s {
		switch {
		case r == '\033':
			inANSI = true
		case inANSI && r == 'm':
			inANSI = false
		case !inANSI:
			length++
		}
	}
	return length
}

// collectAllFlags collects all flags from the command and its parents
func collectAllFlags(cmd *cobra.Command) map[string]*pflag.Flag {
	flagMap := make(map[string]*pflag.Flag)
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flagMap[flag.Name] = flag
	})
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if _, exists := flagMap[flag.Name]; !exists {
			flagMap[flag.Name] = flag
		}
	})
	return flagMap
}
