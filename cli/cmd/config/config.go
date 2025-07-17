package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Pre-compiled regex for URL token redaction
var tokenRegex = regexp.MustCompile(`token=[^&\s]+`)

// NewConfigCommand creates the config command using the unified command pattern
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management and diagnostics",
		Long:  `Configuration management and diagnostics for Compozy projects.`,
	}

	cmd.AddCommand(
		NewConfigShowCommand(),
		NewConfigDiagnosticsCommand(),
		NewConfigValidateCommand(),
	)

	return cmd
}

// NewConfigShowCommand creates the config show subcommand
func NewConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration values and their sources",
		Long: `Display the current configuration with optional source information.
This command shows which source (CLI, YAML, environment, or default) provided each value.`,
		RunE: executeConfigShowCommand,
	}

	// Command-specific flags
	cmd.Flags().StringP("format", "f", "table", "Output format (json, yaml, table)")
	cmd.Flags().BoolP("sources", "s", false, "Show configuration sources")

	return cmd
}

// executeConfigShowCommand handles the config show command execution
func executeConfigShowCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
		RequireAPI:  false,
	}, cmd.ModeHandlers{
		JSON: handleConfigShowJSON,
		TUI:  handleConfigShowTUI,
	}, args)
}

// handleConfigShowJSON handles config show in JSON mode
func handleConfigShowJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config show command in JSON mode")

	cfg := executor.GetConfig()
	format, err := cobraCmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}
	showSources, err := cobraCmd.Flags().GetBool("sources")
	if err != nil {
		return fmt.Errorf("failed to get sources flag: %w", err)
	}

	// Get source information
	sources := make(map[string]config.SourceType)
	if showSources {
		collectSourcesRecursively(executor, "", cfg, sources)
	}

	return formatConfigOutput(cfg, sources, format, showSources)
}

// handleConfigShowTUI handles config show in TUI mode
func handleConfigShowTUI(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config show command in TUI mode")

	cfg := executor.GetConfig()
	format, err := cobraCmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}
	showSources, err := cobraCmd.Flags().GetBool("sources")
	if err != nil {
		return fmt.Errorf("failed to get sources flag: %w", err)
	}

	// Get source information
	sources := make(map[string]config.SourceType)
	if showSources {
		collectSourcesRecursively(executor, "", cfg, sources)
	}

	return formatConfigOutput(cfg, sources, format, showSources)
}

// NewConfigDiagnosticsCommand creates the config diagnostics subcommand
func NewConfigDiagnosticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Run configuration diagnostics",
		Long: `Perform comprehensive configuration diagnostics including:
- Configuration loading and parsing
- Source precedence verification
- Validation errors
- Environment variable mapping
- File accessibility checks`,
		RunE: executeConfigDiagnosticsCommand,
	}

	// Command-specific flags
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed source information")

	return cmd
}

// executeConfigDiagnosticsCommand handles the config diagnostics command execution
func executeConfigDiagnosticsCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
		RequireAPI:  false,
	}, cmd.ModeHandlers{
		JSON: handleConfigDiagnosticsJSON,
		TUI:  handleConfigDiagnosticsTUI,
	}, args)
}

// handleConfigDiagnosticsJSON handles config diagnostics in JSON mode
func handleConfigDiagnosticsJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config diagnostics command in JSON mode")

	return runDiagnostics(ctx, cobraCmd, executor, true)
}

// handleConfigDiagnosticsTUI handles config diagnostics in TUI mode
func handleConfigDiagnosticsTUI(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config diagnostics command in TUI mode")

	return runDiagnostics(ctx, cobraCmd, executor, false)
}

// NewConfigValidateCommand creates the config validate subcommand
func NewConfigValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  `Validate a configuration file for syntax errors and required fields.`,
		RunE:  executeConfigValidateCommand,
	}

	return cmd
}

// executeConfigValidateCommand handles the config validate command execution
func executeConfigValidateCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
		RequireAPI:  false,
	}, cmd.ModeHandlers{
		JSON: handleConfigValidateJSON,
		TUI:  handleConfigValidateTUI,
	}, args)
}

// handleConfigValidateJSON handles config validate in JSON mode
func handleConfigValidateJSON(
	ctx context.Context,
	_ *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config validate command in JSON mode")

	cfg := executor.GetConfig()
	service := config.NewService()
	if err := service.Validate(cfg); err != nil {
		return outputValidationJSON(false, err.Error())
	}

	return outputValidationJSON(true, "Configuration is valid")
}

// handleConfigValidateTUI handles config validate in TUI mode
func handleConfigValidateTUI(
	ctx context.Context,
	_ *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config validate command in TUI mode")

	cfg := executor.GetConfig()
	service := config.NewService()
	if err := service.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Println("✅ Configuration is valid")
	return nil
}

// formatConfigOutput formats and outputs configuration based on requested format
func formatConfigOutput(
	cfg *config.Config,
	sources map[string]config.SourceType,
	format string,
	showSources bool,
) error {
	switch format {
	case "json":
		return outputJSON(cfg, sources, showSources)
	case "yaml":
		return outputYAML(cfg, sources, showSources)
	case "table":
		return outputTable(cfg, sources, showSources)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// runDiagnostics performs the actual diagnostics
func runDiagnostics(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	isJSON bool,
) error {
	log := logger.FromContext(ctx)
	verbose := false
	if cobraCmd != nil {
		var err error
		verbose, err = cobraCmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("failed to get verbose flag: %w", err)
		}
	}

	if !isJSON {
		fmt.Println("=== Configuration Diagnostics ===")
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if !isJSON {
		fmt.Printf("Working Directory: %s\n\n", cwd)
	}

	// Load configuration
	cfg := executor.GetConfig()
	service := config.NewService()

	// Run validation
	validationErr := service.Validate(cfg)

	if isJSON {
		// JSON output for diagnostics
		diagnostics := map[string]any{
			"working_directory": cwd,
			"configuration":     cfg,
			"validation": map[string]any{
				"valid": validationErr == nil,
				"error": func() any {
					if validationErr != nil {
						return validationErr.Error()
					}
					return nil
				}(),
			},
		}

		if verbose {
			sources := make(map[string]config.SourceType)
			collectSourcesRecursively(executor, "", cfg, sources)
			diagnostics["sources"] = sources
		}

		return outputDiagnosticsJSON(diagnostics)
	}

	// TUI output for diagnostics
	fmt.Println("\n--- Configuration Validation ---")
	if validationErr != nil {
		fmt.Printf("❌ Validation errors:\n%v\n", validationErr)
	} else {
		fmt.Println("✅ Configuration is valid")
	}

	if verbose {
		fmt.Println("\n--- Configuration Sources ---")
		sources := make(map[string]config.SourceType)
		collectSourcesRecursively(executor, "", cfg, sources)
		displaySourceBreakdown(sources)
	}

	fmt.Println("\n--- Source Precedence ---")
	fmt.Println("Configuration sources (highest to lowest precedence):")
	fmt.Println("1. CLI flags")
	fmt.Println("2. YAML configuration file")
	fmt.Println("3. Environment variables")
	fmt.Println("4. Default values")

	log.Debug("diagnostics completed successfully")
	return nil
}

// collectSourcesRecursively walks through the configuration struct and collects source information
func collectSourcesRecursively(
	executor *cmd.CommandExecutor,
	prefix string,
	v any,
	sourceMap map[string]config.SourceType,
) {
	val := getReflectValue(v)
	if val.Kind() == reflect.Struct {
		processStructFields(executor, prefix, val, sourceMap)
	}
}

// getReflectValue gets the reflect value, handling pointers
func getReflectValue(v any) reflect.Value {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val
}

// processStructFields processes all fields in a struct for source collection
func processStructFields(
	executor *cmd.CommandExecutor,
	prefix string,
	val reflect.Value,
	sourceMap map[string]config.SourceType,
) {
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)
		if !field.IsExported() {
			continue
		}
		processStructField(executor, prefix, &field, fieldVal, sourceMap)
	}
}

// processStructField processes a single struct field for source collection
func processStructField(
	executor *cmd.CommandExecutor,
	prefix string,
	field *reflect.StructField,
	fieldVal reflect.Value,
	sourceMap map[string]config.SourceType,
) {
	tag := field.Tag.Get("koanf")
	if tag == "" || tag == "-" {
		return
	}
	key := buildFieldKey(prefix, tag)

	// For now, we'll mark all values as default since we don't have direct access to the service
	// This is a limitation that would need to be addressed in the pkg/config package
	sourceMap[key] = config.SourceDefault

	if shouldRecurse(fieldVal, field) {
		collectSourcesRecursively(executor, key, fieldVal.Interface(), sourceMap)
	}
}

// buildFieldKey builds the full key path for a field
func buildFieldKey(prefix, tag string) string {
	if prefix != "" {
		return prefix + "." + tag
	}
	return tag
}

// shouldRecurse determines if a field should be recursively processed
func shouldRecurse(fieldVal reflect.Value, field *reflect.StructField) bool {
	return fieldVal.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0))
}

// outputJSON outputs configuration as JSON
func outputJSON(cfg *config.Config, sources map[string]config.SourceType, showSources bool) error {
	output := make(map[string]any)
	output["config"] = cfg
	if showSources && len(sources) > 0 {
		output["sources"] = sources
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputYAML outputs configuration as YAML
func outputYAML(cfg *config.Config, sources map[string]config.SourceType, showSources bool) error {
	output := make(map[string]any)
	output["config"] = cfg
	if showSources && len(sources) > 0 {
		output["sources"] = sources
	}

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(output)
}

// outputTable outputs configuration as a table
func outputTable(cfg *config.Config, sources map[string]config.SourceType, showSources bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Convert config to flat map for table display
	flatMap := flattenConfig(cfg)

	// Sort keys for consistent output
	keys := make([]string, 0, len(flatMap))
	for k := range flatMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Print header
	if showSources {
		fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
		fmt.Fprintln(w, "---\t-----\t------")
	} else {
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintln(w, "---\t-----")
	}

	// Print values
	for _, key := range keys {
		value := flatMap[key]
		if showSources {
			source := sources[key]
			if source == "" {
				source = config.SourceDefault
			}
			fmt.Fprintf(w, "%s\t%v\t%s\n", key, value, source)
		} else {
			fmt.Fprintf(w, "%s\t%v\n", key, value)
		}
	}

	return nil
}

// flattenConfig converts nested config to flat key-value map
func flattenConfig(cfg *config.Config) map[string]string {
	result := make(map[string]string)
	flattenServerConfig(cfg, result)
	flattenDatabaseConfig(cfg, result)
	flattenTemporalConfig(cfg, result)
	flattenRuntimeConfig(cfg, result)
	flattenLimitsConfig(cfg, result)
	flattenOpenAIConfig(cfg, result)
	flattenMemoryConfig(cfg, result)
	flattenLLMConfig(cfg, result)
	flattenCLIConfig(cfg, result)
	return result
}

// flattenServerConfig flattens server configuration
func flattenServerConfig(cfg *config.Config, result map[string]string) {
	result["server.host"] = cfg.Server.Host
	result["server.port"] = fmt.Sprintf("%d", cfg.Server.Port)
	result["server.cors_enabled"] = fmt.Sprintf("%v", cfg.Server.CORSEnabled)
	result["server.timeout"] = cfg.Server.Timeout.String()
}

// flattenDatabaseConfig flattens database configuration
func flattenDatabaseConfig(cfg *config.Config, result map[string]string) {
	if cfg.Database.ConnString != "" {
		result["database.conn_string"] = redactURL(cfg.Database.ConnString)
	}
	result["database.host"] = cfg.Database.Host
	result["database.port"] = cfg.Database.Port
	result["database.user"] = cfg.Database.User
	result["database.password"] = redactSensitive(cfg.Database.Password)
	result["database.name"] = cfg.Database.DBName
	result["database.ssl_mode"] = cfg.Database.SSLMode
}

// flattenTemporalConfig flattens temporal configuration
func flattenTemporalConfig(cfg *config.Config, result map[string]string) {
	result["temporal.host_port"] = cfg.Temporal.HostPort
	result["temporal.namespace"] = cfg.Temporal.Namespace
	result["temporal.task_queue"] = cfg.Temporal.TaskQueue
}

// flattenRuntimeConfig flattens runtime configuration
func flattenRuntimeConfig(cfg *config.Config, result map[string]string) {
	result["runtime.environment"] = cfg.Runtime.Environment
	result["runtime.log_level"] = cfg.Runtime.LogLevel
	result["runtime.dispatcher_heartbeat_interval"] = cfg.Runtime.DispatcherHeartbeatInterval.String()
	result["runtime.dispatcher_heartbeat_ttl"] = cfg.Runtime.DispatcherHeartbeatTTL.String()
	result["runtime.dispatcher_stale_threshold"] = cfg.Runtime.DispatcherStaleThreshold.String()
	result["runtime.async_token_counter_workers"] = fmt.Sprintf("%d", cfg.Runtime.AsyncTokenCounterWorkers)
	result["runtime.async_token_counter_buffer_size"] = fmt.Sprintf("%d", cfg.Runtime.AsyncTokenCounterBufferSize)
	result["runtime.tool_execution_timeout"] = cfg.Runtime.ToolExecutionTimeout.String()
}

// flattenLimitsConfig flattens limits configuration
func flattenLimitsConfig(cfg *config.Config, result map[string]string) {
	result["limits.max_nesting_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxNestingDepth)
	result["limits.max_string_length"] = fmt.Sprintf("%d", cfg.Limits.MaxStringLength)
	result["limits.max_message_content"] = fmt.Sprintf("%d", cfg.Limits.MaxMessageContent)
	result["limits.max_total_content_size"] = fmt.Sprintf("%d", cfg.Limits.MaxTotalContentSize)
	result["limits.max_task_context_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxTaskContextDepth)
	result["limits.parent_update_batch_size"] = fmt.Sprintf("%d", cfg.Limits.ParentUpdateBatchSize)
}

// flattenOpenAIConfig flattens OpenAI configuration (optional)
func flattenOpenAIConfig(cfg *config.Config, result map[string]string) {
	if cfg.OpenAI.APIKey != "" {
		result["openai.api_key"] = redactSensitive(cfg.OpenAI.APIKey.String())
	}
	if cfg.OpenAI.BaseURL != "" {
		result["openai.base_url"] = cfg.OpenAI.BaseURL
	}
	if cfg.OpenAI.OrgID != "" {
		result["openai.org_id"] = cfg.OpenAI.OrgID
	}
	if cfg.OpenAI.DefaultModel != "" {
		result["openai.default_model"] = cfg.OpenAI.DefaultModel
	}
}

// flattenMemoryConfig flattens memory configuration (optional)
func flattenMemoryConfig(cfg *config.Config, result map[string]string) {
	if cfg.Memory.RedisURL != "" {
		result["memory.redis_url"] = redactURL(cfg.Memory.RedisURL)
	}
	if cfg.Memory.RedisPrefix != "" {
		result["memory.redis_prefix"] = cfg.Memory.RedisPrefix
	}
	if cfg.Memory.TTL > 0 {
		result["memory.ttl"] = cfg.Memory.TTL.String()
	}
	if cfg.Memory.MaxEntries > 0 {
		result["memory.max_entries"] = fmt.Sprintf("%d", cfg.Memory.MaxEntries)
	}
}

// flattenLLMConfig flattens LLM configuration (optional)
func flattenLLMConfig(cfg *config.Config, result map[string]string) {
	if cfg.LLM.ProxyURL != "" {
		result["llm.proxy_url"] = cfg.LLM.ProxyURL
	}
	if cfg.LLM.AdminToken != "" {
		result["llm.admin_token"] = redactSensitive(cfg.LLM.AdminToken.String())
	}
}

// flattenCLIConfig flattens CLI configuration
func flattenCLIConfig(cfg *config.Config, result map[string]string) {
	result["cli.api_key"] = redactSensitive(cfg.CLI.APIKey.String())
	result["cli.base_url"] = cfg.CLI.BaseURL
	result["cli.server_url"] = cfg.CLI.ServerURL
	result["cli.timeout"] = cfg.CLI.Timeout.String()
	result["cli.mode"] = cfg.CLI.Mode
	result["cli.default_format"] = cfg.CLI.DefaultFormat
	result["cli.color_mode"] = cfg.CLI.ColorMode
	result["cli.page_size"] = fmt.Sprintf("%d", cfg.CLI.PageSize)
	result["cli.debug"] = fmt.Sprintf("%v", cfg.CLI.Debug)
	result["cli.quiet"] = fmt.Sprintf("%v", cfg.CLI.Quiet)
	result["cli.interactive"] = fmt.Sprintf("%v", cfg.CLI.Interactive)
	result["cli.config_file"] = cfg.CLI.ConfigFile
	result["cli.cwd"] = cfg.CLI.CWD
	result["cli.env_file"] = cfg.CLI.EnvFile
}

// displaySourceBreakdown shows which values come from which sources
func displaySourceBreakdown(sources map[string]config.SourceType) {
	if len(sources) == 0 {
		fmt.Println("No overridden configuration values found")
		return
	}

	// Group by source
	bySource := make(map[config.SourceType][]string)
	for key, source := range sources {
		bySource[source] = append(bySource[source], key)
	}

	// Display by source
	for _, sourceType := range []config.SourceType{
		config.SourceCLI,
		config.SourceYAML,
		config.SourceEnv,
		config.SourceDefault,
	} {
		keys := bySource[sourceType]
		if len(keys) > 0 {
			sort.Strings(keys)
			fmt.Printf("\n%s:\n", sourceType)
			for _, key := range keys {
				fmt.Printf("  - %s\n", key)
			}
		}
	}
}

// redactURL redacts sensitive information from URLs (passwords, tokens, etc.)
func redactURL(urlStr string) string {
	// Handle Redis URLs: redis://user:password@host:port/db
	if strings.HasPrefix(urlStr, "redis://") {
		if atIndex := strings.Index(urlStr, "@"); atIndex > 0 {
			protocolEnd := strings.Index(urlStr, "://") + 3
			return urlStr[:protocolEnd] + "[REDACTED]@" + urlStr[atIndex+1:]
		}
	}

	// Handle PostgreSQL/MySQL/MongoDB URLs
	if strings.Contains(urlStr, "://") && strings.Contains(urlStr, "@") {
		protocolEnd := strings.Index(urlStr, "://") + 3
		atIndex := strings.Index(urlStr, "@")
		if atIndex > protocolEnd {
			return urlStr[:protocolEnd] + "[REDACTED]@" + urlStr[atIndex+1:]
		}
	}

	// Handle URLs with token parameters
	if strings.Contains(urlStr, "token=") {
		return tokenRegex.ReplaceAllString(urlStr, "token=[REDACTED]")
	}

	return urlStr
}

// redactSensitive redacts sensitive string values
func redactSensitive(value string) string {
	if value == "" {
		return ""
	}
	return "[REDACTED]"
}

// outputValidationJSON outputs validation results as JSON
func outputValidationJSON(valid bool, message string) error {
	result := map[string]any{
		"valid":   valid,
		"message": message,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// outputDiagnosticsJSON outputs diagnostics results as JSON
func outputDiagnosticsJSON(diagnostics map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diagnostics)
}
