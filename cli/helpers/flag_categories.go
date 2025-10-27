package helpers

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
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
	categories = append(categories, getKnowledgeCategories()...)
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
				"tool-execution-timeout", "task-execution-timeout-default",
				"task-execution-timeout-max", "timeout", "page-size",
			},
		},
		{
			Name:        "LLM Configuration",
			Description: "LLM proxy and orchestration behavior",
			Flags: []string{
				"llm-proxy-url", "llm-mcp-readiness-timeout", "llm-mcp-readiness-poll-interval",
				"llm-mcp-header-template-strict", "llm-retry-attempts", "llm-retry-backoff-base",
				"llm-retry-backoff-max", "llm-provider-timeout", "llm-retry-jitter", "llm-max-concurrent-tools",
				"llm-max-tool-iterations", "llm-max-sequential-tool-errors",
				"llm-max-consecutive-successes", "llm-enable-progress-tracking",
				"llm-no-progress-threshold", "llm-enable-loop-restarts",
				"llm-restart-stall-threshold", "llm-max-loop-restarts",
				"llm-enable-context-compaction", "llm-context-compaction-threshold",
				"llm-context-compaction-cooldown", "llm-enable-dynamic-prompt-state",
				"llm-finalize-output-retries", "llm-structured-output-retries", "llm-allowed-mcp-names",
				"llm-fail-on-mcp-registration-error", "llm-mcp-client-timeout",
				"llm-retry-jitter-percent", "llm-context-warning-thresholds",
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

func getKnowledgeCategories() []FlagCategory {
	return []FlagCategory{
		{
			Name:        "Knowledge Configuration",
			Description: "Knowledge ingestion and retrieval defaults",
			Flags: []string{
				"knowledge-embedder-batch-size",
				"knowledge-chunk-size",
				"knowledge-chunk-overlap",
				"knowledge-retrieval-top-k",
				"knowledge-retrieval-min-score",
				"knowledge-max-markdown-file-size-bytes",
				"knowledge-vector-http-timeout",
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
				"temporal-mode", "temporal-host", "temporal-namespace", "temporal-task-queue",
				"temporal-standalone-database", "temporal-standalone-frontend-port",
				"temporal-standalone-ui-port",
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
	cmd.SetHelpFunc(categorizedHelpFunc)
}

// Define ANSI color codes
const (
	colorReset                 = "\033[0m"
	colorBold                  = "\033[1m"
	colorPrimary               = "\033[36m" // Cyan
	colorSuccess               = "\033[32m" // Green
	colorMuted                 = "\033[90m" // Bright black (gray)
	flagDescriptionColumnWidth = 50         // width reserved for flag descriptions
)

// createSeparator creates a styled separator line
func createSeparator(width int, noColor bool) string {
	if noColor {
		return strings.Repeat("─", width)
	}
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")). // Dark gray
		Width(width)
	return separatorStyle.Render(strings.Repeat("─", width))
}

// getTerminalWidth attempts to get terminal width, with fallback
func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

// categorizedHelpFunc is a custom help function that groups flags by category
func categorizedHelpFunc(cmd *cobra.Command, _ []string) {
	noColor := isNoColorEnabled(cmd)
	printCommandDescription(cmd)
	printUsageSection(cmd, noColor)
	printAvailableCommands(cmd, noColor)
	if cmd.Parent() != nil {
		printCommandSpecificFlags(cmd, noColor)
	}
	printCategorizedFlags(cmd, noColor)
	printHelpFooter(cmd)
}

func isNoColorEnabled(cmd *cobra.Command) bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if f := cmd.Flags().Lookup("color-mode"); f != nil && strings.EqualFold(f.Value.String(), "never") {
		return true
	}
	if f := cmd.Flags().Lookup("no-color"); f != nil && f.Value.String() == "true" {
		return true
	}
	return false
}

func printCommandDescription(cmd *cobra.Command) {
	if cmd.Long != "" {
		fmt.Println(cmd.Long)
	} else if cmd.Short != "" {
		fmt.Println(cmd.Short)
	}
	fmt.Println()
}

func printUsageSection(cmd *cobra.Command, noColor bool) {
	fmt.Printf("%sUsage:%s\n", applyColor(colorBold, noColor), applyColor(colorReset, noColor))
	if cmd.Runnable() {
		fmt.Printf("  %s\n", cmd.UseLine())
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Printf("  %s [command]\n", cmd.CommandPath())
	}
	fmt.Println()
}

func printAvailableCommands(cmd *cobra.Command, noColor bool) {
	if !cmd.HasAvailableSubCommands() {
		return
	}
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

func printHelpFooter(cmd *cobra.Command) {
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
	visibleCount := 0
	localFlags.VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden {
			visibleCount++
		}
	})
	if visibleCount == 0 {
		return
	}
	width := getTerminalWidth()
	fmt.Println(createSeparator(width, noColor))
	fmt.Printf(
		"%sFlags%s\n",
		applyColor(colorPrimary+colorBold, noColor),
		applyColor(colorReset, noColor),
	)
	fmt.Println(createSeparator(width, noColor))
	fmt.Println() // Extra line after header
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
	if isSubcommand {
		markLocalFlagsDisplayed(cmd, displayedFlags)
		printGlobalFlagsHeader(flagMap, displayedFlags, noColor)
	} else {
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
	printFlagCategories(categories, flagMap, displayedFlags, noColor)
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
		width := getTerminalWidth()
		fmt.Println(createSeparator(width, noColor))
		fmt.Printf(
			"%sOther Flags%s\n",
			applyColor(colorPrimary+colorBold, noColor),
			applyColor(colorReset, noColor),
		)
		fmt.Println(createSeparator(width, noColor))
		fmt.Println() // Extra line after header
		sort.Slice(uncategorizedFlags, func(i, j int) bool {
			return uncategorizedFlags[i].Name < uncategorizedFlags[j].Name
		})
		for _, flag := range uncategorizedFlags {
			printFlag(flag, noColor)
		}
		fmt.Println()
	}
}

// printFlag prints a single flag with formatting
func printFlag(flag *pflag.Flag, noColor bool) {
	line := buildFlagLine(flag, noColor)
	appendFlagUsage(line, flag)
	fmt.Println(line.String())
}

func buildFlagLine(flag *pflag.Flag, noColor bool) *strings.Builder {
	line := &strings.Builder{}
	line.WriteString("  ")
	if flag.Shorthand != "" {
		fmt.Fprintf(line, "-%s, ", flag.Shorthand)
	} else {
		line.WriteString("    ")
	}
	fmt.Fprintf(
		line,
		"%s--%s%s",
		applyColor(colorSuccess, noColor),
		flag.Name,
		applyColor(colorReset, noColor),
	)
	if hint := flagTypeHint(flag, noColor); hint != "" {
		line.WriteString(hint)
	}
	padFlagLine(line)
	return line
}

func flagTypeHint(flag *pflag.Flag, noColor bool) string {
	if flag.Value.Type() == "bool" {
		return ""
	}
	typeStr := flag.Value.Type()
	switch typeStr {
	case "stringSlice", "stringArray":
		typeStr = "strings"
	case "stringToString":
		typeStr = "map"
	case "duration":
		typeStr = "duration"
	case "int", "int64":
		typeStr = "int"
	case "intSlice":
		typeStr = "ints"
	case "float64", "float32":
		typeStr = "float"
	}
	return fmt.Sprintf(" %s%s%s", applyColor(colorMuted, noColor), typeStr, applyColor(colorReset, noColor))
}

func padFlagLine(line *strings.Builder) {
	visibleLength := calculateVisibleLength(line.String())
	if visibleLength < flagDescriptionColumnWidth {
		line.WriteString(strings.Repeat(" ", flagDescriptionColumnWidth-visibleLength))
	} else {
		line.WriteString(" ")
	}
}

func appendFlagUsage(line *strings.Builder, flag *pflag.Flag) {
	usage := flag.Usage
	if usage == "" {
		return
	}
	if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "[]" {
		if flag.Value.Type() == "string" {
			usage += fmt.Sprintf(" (default %q)", flag.DefValue)
		} else {
			usage += fmt.Sprintf(" (default %s)", flag.DefValue)
		}
	}
	line.WriteString(usage)
}

// calculateVisibleLength calculates the visible length of a string, excluding ANSI codes
func calculateVisibleLength(s string) int {
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
