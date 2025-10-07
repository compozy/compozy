package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

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
		Short: "Show current configuration values",
		Long: `Display the current configuration values in different formats.
Supports JSON, YAML, and table output formats.`,
		RunE: executeConfigShowCommand,
	}

	// Command-specific flags
	cmd.Flags().StringP("format", "f", "table", "Output format (json, yaml, table)")

	return cmd
}

// executeConfigShowCommand handles the config show command execution
func executeConfigShowCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	}, cmd.ModeHandlers{
		JSON: handleConfigShowJSON,
		TUI:  handleConfigShowTUI,
	}, args)
}

// handleConfigShowJSON handles config show in JSON mode
func handleConfigShowJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	_ *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config show command in JSON mode")

	cfg := config.FromContext(ctx)
	format, err := cobraCmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}

	return formatConfigOutput(cfg, nil, format, false)
}

// handleConfigShowTUI handles config show in TUI mode
func handleConfigShowTUI(
	ctx context.Context,
	cobraCmd *cobra.Command,
	_ *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config show command in TUI mode")

	cfg := config.FromContext(ctx)
	format, err := cobraCmd.Flags().GetString("format")
	if err != nil {
		return fmt.Errorf("failed to get format flag: %w", err)
	}

	return formatConfigOutput(cfg, nil, format, false)
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
	}, cmd.ModeHandlers{
		JSON: handleConfigValidateJSON,
		TUI:  handleConfigValidateTUI,
	}, args)
}

// handleConfigValidateJSON handles config validate in JSON mode
func handleConfigValidateJSON(
	ctx context.Context,
	_ *cobra.Command,
	_ *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config validate command in JSON mode")

	cfg := config.FromContext(ctx)
	service := config.ManagerFromContext(ctx).Service
	if err := service.Validate(cfg); err != nil {
		return outputValidationJSON(false, err.Error())
	}

	return outputValidationJSON(true, "Configuration is valid")
}

// handleConfigValidateTUI handles config validate in TUI mode
func handleConfigValidateTUI(
	ctx context.Context,
	_ *cobra.Command,
	_ *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	log.Debug("executing config validate command in TUI mode")

	cfg := config.FromContext(ctx)
	service := config.ManagerFromContext(ctx).Service
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
	_ *cmd.CommandExecutor,
	isJSON bool,
) error {
	verbose, err := getVerboseFlag(cobraCmd)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg := config.FromContext(ctx)
	service := config.ManagerFromContext(ctx).Service
	validationErr := service.Validate(cfg)

	if isJSON {
		return outputDiagnosticsResults(ctx, cwd, cfg, validationErr, verbose)
	}

	return outputDiagnosticsTUI(ctx, cwd, validationErr, verbose)
}

// getVerboseFlag safely extracts the verbose flag from cobra command
func getVerboseFlag(cobraCmd *cobra.Command) (bool, error) {
	if cobraCmd == nil {
		return false, nil
	}
	verbose, err := cobraCmd.Flags().GetBool("verbose")
	if err != nil {
		return false, fmt.Errorf("failed to get verbose flag: %w", err)
	}
	return verbose, nil
}

// outputDiagnosticsResults outputs diagnostics in JSON format
func outputDiagnosticsResults(
	ctx context.Context,
	cwd string,
	cfg *config.Config,
	validationErr error,
	verbose bool,
) error {
	diagnostics := map[string]any{
		"working_directory": cwd,
		// Use flattened, redacted view to avoid leaking secrets
		"configuration": flattenConfig(cfg),
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
		sources := make(map[string]string)
		if service, ok := config.ManagerFromContext(ctx).Service.(interface {
			GetSources() map[string]config.SourceType
		}); ok {
			serviceSources := service.GetSources()
			for key, sourceType := range serviceSources {
				sources[key] = string(sourceType)
			}
		}
		if len(sources) > 0 {
			diagnostics["sources"] = sources
		} else {
			diagnostics["sources"] = map[string]any{"note": "Source tracking not available"}
		}
	}

	return outputDiagnosticsJSON(diagnostics)
}

// outputDiagnosticsTUI outputs diagnostics in TUI format
func outputDiagnosticsTUI(ctx context.Context, cwd string, validationErr error, verbose bool) error {
	log := logger.FromContext(ctx)

	fmt.Println("=== Configuration Diagnostics ===")
	fmt.Printf("Working Directory: %s\n\n", cwd)

	fmt.Println("\n--- Configuration Validation ---")
	if validationErr != nil {
		fmt.Printf("❌ Validation errors:\n%v\n", validationErr)
	} else {
		fmt.Println("✅ Configuration is valid")
	}

	if verbose {
		fmt.Println("\n--- Configuration Sources ---")
		fmt.Println("Note: Source tracking is not currently implemented")
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

// outputJSON outputs configuration as JSON
func outputJSON(cfg *config.Config, sources map[string]config.SourceType, showSources bool) error {
	output := make(map[string]any)
	// Use redacted, flattened representation
	output["config"] = flattenConfig(cfg)
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
	// Use redacted, flattened representation
	output["config"] = flattenConfig(cfg)
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
	flattenAttachmentsConfig(cfg, result)
	flattenMemoryConfig(cfg, result)
	flattenKnowledgeConfig(cfg, result)
	flattenLLMConfig(cfg, result)
	flattenCLIConfig(cfg, result)
	flattenRedisConfig(cfg, result)
	flattenCacheConfig(cfg, result)
	flattenRateLimitConfig(cfg, result)
	flattenWorkerConfig(cfg, result)
	flattenMCPProxyConfig(cfg, result)
	flattenWebhooksConfig(cfg, result)
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
	result["database.auto_migrate"] = fmt.Sprintf("%v", cfg.Database.AutoMigrate)
	result["database.migration_timeout"] = cfg.Database.MigrationTimeout.String()
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
	result["runtime.task_execution_timeout_default"] = cfg.Runtime.TaskExecutionTimeoutDefault.String()
	result["runtime.task_execution_timeout_max"] = cfg.Runtime.TaskExecutionTimeoutMax.String()
	result["runtime.tool_execution_timeout"] = cfg.Runtime.ToolExecutionTimeout.String()
}

// flattenLimitsConfig flattens limits configuration
func flattenLimitsConfig(cfg *config.Config, result map[string]string) {
	result["limits.max_nesting_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxNestingDepth)
	result["limits.max_config_file_nesting_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxConfigFileNestingDepth)
	result["limits.max_string_length"] = fmt.Sprintf("%d", cfg.Limits.MaxStringLength)
	result["limits.max_config_file_size"] = fmt.Sprintf("%d", cfg.Limits.MaxConfigFileSize)
	result["limits.max_message_content"] = fmt.Sprintf("%d", cfg.Limits.MaxMessageContent)
	result["limits.max_total_content_size"] = fmt.Sprintf("%d", cfg.Limits.MaxTotalContentSize)
	result["limits.max_task_context_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxTaskContextDepth)
	result["limits.parent_update_batch_size"] = fmt.Sprintf("%d", cfg.Limits.ParentUpdateBatchSize)
}

// flattenAttachmentsConfig flattens global attachments configuration
func flattenAttachmentsConfig(cfg *config.Config, result map[string]string) {
	result["attachments.max_download_size_bytes"] = fmt.Sprintf("%d", cfg.Attachments.MaxDownloadSizeBytes)
	result["attachments.download_timeout"] = cfg.Attachments.DownloadTimeout.String()
	result["attachments.max_redirects"] = fmt.Sprintf("%d", cfg.Attachments.MaxRedirects)
	if cfg.Attachments.TextPartMaxBytes > 0 {
		result["attachments.text_part_max_bytes"] = fmt.Sprintf("%d", cfg.Attachments.TextPartMaxBytes)
	}
	if cfg.Attachments.PDFExtractMaxChars > 0 {
		result["attachments.pdf_extract_max_chars"] = fmt.Sprintf("%d", cfg.Attachments.PDFExtractMaxChars)
	}
	if cfg.Attachments.MIMEHeadMaxBytes > 0 {
		result["attachments.mime_head_max_bytes"] = fmt.Sprintf("%d", cfg.Attachments.MIMEHeadMaxBytes)
	}
	if cfg.Attachments.HTTPUserAgent != "" {
		result["attachments.http_user_agent"] = cfg.Attachments.HTTPUserAgent
	}
	result["attachments.ssrf_strict"] = fmt.Sprintf("%t", cfg.Attachments.SSRFStrict)
	if len(cfg.Attachments.AllowedMIMETypes.Image) > 0 {
		v := append([]string(nil), cfg.Attachments.AllowedMIMETypes.Image...)
		sort.Strings(v)
		result["attachments.allowed_mime_types.image"] = strings.Join(v, ",")
	} else {
		result["attachments.allowed_mime_types.image"] = ""
	}
	if len(cfg.Attachments.AllowedMIMETypes.Audio) > 0 {
		v := append([]string(nil), cfg.Attachments.AllowedMIMETypes.Audio...)
		sort.Strings(v)
		result["attachments.allowed_mime_types.audio"] = strings.Join(v, ",")
	} else {
		result["attachments.allowed_mime_types.audio"] = ""
	}
	if len(cfg.Attachments.AllowedMIMETypes.Video) > 0 {
		v := append([]string(nil), cfg.Attachments.AllowedMIMETypes.Video...)
		sort.Strings(v)
		result["attachments.allowed_mime_types.video"] = strings.Join(v, ",")
	} else {
		result["attachments.allowed_mime_types.video"] = ""
	}
	if len(cfg.Attachments.AllowedMIMETypes.PDF) > 0 {
		v := append([]string(nil), cfg.Attachments.AllowedMIMETypes.PDF...)
		sort.Strings(v)
		result["attachments.allowed_mime_types.pdf"] = strings.Join(v, ",")
	} else {
		result["attachments.allowed_mime_types.pdf"] = ""
	}
	if cfg.Attachments.TempDirQuotaBytes > 0 {
		result["attachments.temp_dir_quota_bytes"] = fmt.Sprintf("%d", cfg.Attachments.TempDirQuotaBytes)
	}
}

// flattenMemoryConfig flattens memory configuration (optional)
func flattenMemoryConfig(cfg *config.Config, result map[string]string) {
	if cfg.Memory.Prefix != "" {
		result["memory.prefix"] = cfg.Memory.Prefix
	}
	if cfg.Memory.TTL > 0 {
		result["memory.ttl"] = cfg.Memory.TTL.String()
	}
	if cfg.Memory.MaxEntries > 0 {
		result["memory.max_entries"] = fmt.Sprintf("%d", cfg.Memory.MaxEntries)
	}
}

// flattenKnowledgeConfig flattens knowledge configuration defaults
func flattenKnowledgeConfig(cfg *config.Config, result map[string]string) {
	result["knowledge.embedder_batch_size"] = fmt.Sprintf("%d", cfg.Knowledge.EmbedderBatchSize)
	result["knowledge.chunk_size"] = fmt.Sprintf("%d", cfg.Knowledge.ChunkSize)
	result["knowledge.chunk_overlap"] = fmt.Sprintf("%d", cfg.Knowledge.ChunkOverlap)
	result["knowledge.retrieval_top_k"] = fmt.Sprintf("%d", cfg.Knowledge.RetrievalTopK)
	result["knowledge.retrieval_min_score"] = strconv.FormatFloat(cfg.Knowledge.RetrievalMinScore, 'f', -1, 64)
}

// flattenLLMConfig flattens LLM configuration (optional)
func flattenLLMConfig(cfg *config.Config, result map[string]string) {
	if cfg.LLM.ProxyURL != "" {
		result["llm.proxy_url"] = redactURL(cfg.LLM.ProxyURL)
	}
	if cfg.LLM.MCPReadinessTimeout > 0 {
		result["llm.mcp_readiness_timeout"] = cfg.LLM.MCPReadinessTimeout.String()
	}
	if cfg.LLM.MCPReadinessPollInterval > 0 {
		result["llm.mcp_readiness_poll_interval"] = cfg.LLM.MCPReadinessPollInterval.String()
	}
	result["llm.mcp_header_template_strict"] = fmt.Sprintf("%v", cfg.LLM.MCPHeaderTemplateStrict)
	if cfg.LLM.RetryAttempts > 0 {
		result["llm.retry_attempts"] = fmt.Sprintf("%d", cfg.LLM.RetryAttempts)
	}
	if cfg.LLM.RetryBackoffBase > 0 {
		result["llm.retry_backoff_base"] = cfg.LLM.RetryBackoffBase.String()
	}
	if cfg.LLM.RetryBackoffMax > 0 {
		result["llm.retry_backoff_max"] = cfg.LLM.RetryBackoffMax.String()
	}
	result["llm.retry_jitter"] = fmt.Sprintf("%v", cfg.LLM.RetryJitter)
	if cfg.LLM.MaxConcurrentTools > 0 {
		result["llm.max_concurrent_tools"] = fmt.Sprintf("%d", cfg.LLM.MaxConcurrentTools)
	}
	if cfg.LLM.MaxToolIterations > 0 {
		result["llm.max_tool_iterations"] = fmt.Sprintf("%d", cfg.LLM.MaxToolIterations)
	}
	if cfg.LLM.MaxSequentialToolErrors > 0 {
		result["llm.max_sequential_tool_errors"] = fmt.Sprintf("%d", cfg.LLM.MaxSequentialToolErrors)
	}
	// MCP-related fields
	if len(cfg.LLM.AllowedMCPNames) > 0 {
		result["llm.allowed_mcp_names"] = strings.Join(cfg.LLM.AllowedMCPNames, ",")
	} else {
		result["llm.allowed_mcp_names"] = ""
	}
	result["llm.fail_on_mcp_registration_error"] = fmt.Sprintf("%v", cfg.LLM.FailOnMCPRegistrationError)
	if cfg.LLM.MCPClientTimeout > 0 {
		result["llm.mcp_client_timeout"] = cfg.LLM.MCPClientTimeout.String()
	}
	if cfg.LLM.RetryJitterPercent > 0 {
		result["llm.retry_jitter_percent"] = fmt.Sprintf("%d", cfg.LLM.RetryJitterPercent)
	}
	// register_mcps is a complex structure; surface count for diagnostics
	if len(cfg.LLM.RegisterMCPs) > 0 {
		result["llm.register_mcps"] = fmt.Sprintf("%d", len(cfg.LLM.RegisterMCPs))
	}
}

// flattenCLIConfig flattens CLI configuration
func flattenCLIConfig(cfg *config.Config, result map[string]string) {
	result["cli.api_key"] = redactSensitive(cfg.CLI.APIKey.String())
	result["cli.base_url"] = redactURL(cfg.CLI.BaseURL)
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
	result["cli.port_release_timeout"] = cfg.CLI.PortReleaseTimeout.String()
	result["cli.port_release_poll_interval"] = cfg.CLI.PortReleasePollInterval.String()
}

// flattenRedisConfig flattens Redis configuration
func flattenRedisConfig(cfg *config.Config, result map[string]string) {
	if cfg.Redis.URL != "" {
		result["redis.url"] = redactURL(cfg.Redis.URL)
	}
	result["redis.host"] = cfg.Redis.Host
	result["redis.port"] = cfg.Redis.Port
	if cfg.Redis.Password != "" {
		result["redis.password"] = redactSensitive(cfg.Redis.Password)
	}
	result["redis.db"] = fmt.Sprintf("%d", cfg.Redis.DB)
	result["redis.max_retries"] = fmt.Sprintf("%d", cfg.Redis.MaxRetries)
	result["redis.pool_size"] = fmt.Sprintf("%d", cfg.Redis.PoolSize)
	result["redis.min_idle_conns"] = fmt.Sprintf("%d", cfg.Redis.MinIdleConns)
	result["redis.max_idle_conns"] = fmt.Sprintf("%d", cfg.Redis.MaxIdleConns)
	result["redis.dial_timeout"] = cfg.Redis.DialTimeout.String()
	result["redis.read_timeout"] = cfg.Redis.ReadTimeout.String()
	result["redis.write_timeout"] = cfg.Redis.WriteTimeout.String()
	result["redis.pool_timeout"] = cfg.Redis.PoolTimeout.String()
	result["redis.ping_timeout"] = cfg.Redis.PingTimeout.String()
	result["redis.min_retry_backoff"] = cfg.Redis.MinRetryBackoff.String()
	result["redis.max_retry_backoff"] = cfg.Redis.MaxRetryBackoff.String()
	result["redis.notification_buffer_size"] = fmt.Sprintf("%d", cfg.Redis.NotificationBufferSize)
	result["redis.tls_enabled"] = fmt.Sprintf("%v", cfg.Redis.TLSEnabled)
}

// flattenCacheConfig flattens cache configuration
func flattenCacheConfig(cfg *config.Config, result map[string]string) {
	result["cache.enabled"] = fmt.Sprintf("%v", cfg.Cache.Enabled)
	result["cache.ttl"] = cfg.Cache.TTL.String()
	result["cache.prefix"] = cfg.Cache.Prefix
	result["cache.max_item_size"] = fmt.Sprintf("%d", cfg.Cache.MaxItemSize)
	result["cache.compression_enabled"] = fmt.Sprintf("%v", cfg.Cache.CompressionEnabled)
	result["cache.compression_threshold"] = fmt.Sprintf("%d", cfg.Cache.CompressionThreshold)
	result["cache.eviction_policy"] = cfg.Cache.EvictionPolicy
	result["cache.stats_interval"] = cfg.Cache.StatsInterval.String()
	result["cache.key_scan_count"] = fmt.Sprintf("%d", cfg.Cache.KeyScanCount)
}

// flattenRateLimitConfig flattens rate limit configuration
func flattenRateLimitConfig(cfg *config.Config, result map[string]string) {
	result["ratelimit.global_rate.limit"] = fmt.Sprintf("%d", cfg.RateLimit.GlobalRate.Limit)
	result["ratelimit.global_rate.period"] = cfg.RateLimit.GlobalRate.Period.String()
	result["ratelimit.api_key_rate.limit"] = fmt.Sprintf("%d", cfg.RateLimit.APIKeyRate.Limit)
	result["ratelimit.api_key_rate.period"] = cfg.RateLimit.APIKeyRate.Period.String()
	result["ratelimit.prefix"] = cfg.RateLimit.Prefix
	result["ratelimit.max_retry"] = fmt.Sprintf("%d", cfg.RateLimit.MaxRetry)
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

// flattenWorkerConfig flattens worker configuration
func flattenWorkerConfig(cfg *config.Config, result map[string]string) {
	result["worker.config_store_ttl"] = cfg.Worker.ConfigStoreTTL.String()
	result["worker.heartbeat_cleanup_timeout"] = cfg.Worker.HeartbeatCleanupTimeout.String()
	result["worker.mcp_shutdown_timeout"] = cfg.Worker.MCPShutdownTimeout.String()
	result["worker.dispatcher_retry_delay"] = cfg.Worker.DispatcherRetryDelay.String()
	result["worker.dispatcher_max_retries"] = fmt.Sprintf("%d", cfg.Worker.DispatcherMaxRetries)
	result["worker.mcp_proxy_health_check_timeout"] = cfg.Worker.MCPProxyHealthCheckTimeout.String()
	result["worker.start_workflow_timeout"] = cfg.Worker.StartWorkflowTimeout.String()
	result["worker.dispatcher.heartbeat_ttl"] = cfg.Worker.Dispatcher.HeartbeatTTL.String()
	result["worker.dispatcher.stale_threshold"] = cfg.Worker.Dispatcher.StaleThreshold.String()
}

// flattenMCPProxyConfig flattens MCP proxy configuration
func flattenMCPProxyConfig(cfg *config.Config, result map[string]string) {
	result["mcp_proxy.mode"] = cfg.MCPProxy.Mode
	result["mcp_proxy.host"] = cfg.MCPProxy.Host
	result["mcp_proxy.port"] = fmt.Sprintf("%d", cfg.MCPProxy.Port)
	result["mcp_proxy.base_url"] = redactURL(cfg.MCPProxy.BaseURL)
	result["mcp_proxy.shutdown_timeout"] = cfg.MCPProxy.ShutdownTimeout.String()
}

// flattenWebhooksConfig flattens webhooks configuration
func flattenWebhooksConfig(cfg *config.Config, result map[string]string) {
	result["webhooks.default_method"] = cfg.Webhooks.DefaultMethod
	result["webhooks.default_max_body"] = fmt.Sprintf("%d", cfg.Webhooks.DefaultMaxBody)
	result["webhooks.default_dedupe_ttl"] = cfg.Webhooks.DefaultDedupeTTL.String()
	result["webhooks.stripe_skew"] = cfg.Webhooks.StripeSkew.String()
}
