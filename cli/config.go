package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/compozy/compozy/pkg/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ConfigCmd returns the config command
func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management and diagnostics",
	}

	cmd.AddCommand(
		configShowCmd(),
		configDiagnosticsCmd(),
		configValidateCmd(),
	)

	return cmd
}

// configShowCmd shows the current configuration with source information
func configShowCmd() *cobra.Command {
	var (
		format      string
		showSources bool
		configFile  string
		envFile     string
	)

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration values and their sources",
		Long: `Display the current configuration with optional source information.
This command shows which source (CLI, YAML, environment, or default) provided each value.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Load environment file if specified
			if envFile != "" {
				_, err := loadEnvFile(cmd)
				if err != nil {
					return err
				}
			}

			// Load configuration
			cfg, sources, err := loadConfigWithSources(ctx, cmd, configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Format output based on requested format
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
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (json, yaml, table)")
	cmd.Flags().BoolVarP(&showSources, "sources", "s", false, "Show configuration sources")
	cmd.Flags().StringVar(&configFile, "config", "compozy.yaml", "Path to configuration file")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to environment file")

	return cmd
}

// configDiagnosticsCmd provides detailed configuration diagnostics
func configDiagnosticsCmd() *cobra.Command {
	var (
		configFile string
		envFile    string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Run configuration diagnostics",
		Long: `Perform comprehensive configuration diagnostics including:
- Configuration loading and parsing
- Source precedence verification
- Validation errors
- Environment variable mapping
- File accessibility checks`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiagnostics(cmd, configFile, envFile, verbose)
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "compozy.yaml", "Path to configuration file")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to environment file")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed source information")

	return cmd
}

// runDiagnostics performs the actual diagnostics
func runDiagnostics(cmd *cobra.Command, configFile, envFile string, verbose bool) error {
	ctx := context.Background()

	fmt.Println("=== Configuration Diagnostics ===")

	// Check working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	fmt.Printf("Working Directory: %s\n\n", cwd)

	// Check files
	checkConfigFiles(cwd, configFile, envFile, cmd)

	// Load configuration with detailed error reporting
	fmt.Println("\n--- Configuration Loading ---")
	cfg, sources, err := loadConfigWithSources(ctx, cmd, configFile)
	if err != nil {
		fmt.Printf("❌ Failed to load configuration: %v\n", err)
		return nil // Don't exit, continue with diagnostics
	}
	fmt.Println("✅ Configuration loaded successfully")

	// Validate configuration
	fmt.Println("\n--- Configuration Validation ---")
	service := config.NewService()
	if err := service.Validate(cfg); err != nil {
		fmt.Printf("❌ Validation errors:\n%v\n", err)
	} else {
		fmt.Println("✅ Configuration is valid")
	}

	// Show source precedence
	fmt.Println("\n--- Source Precedence ---")
	fmt.Println("Configuration sources (highest to lowest precedence):")
	fmt.Println("1. CLI flags")
	fmt.Println("2. YAML configuration file")
	fmt.Println("3. Environment variables")
	fmt.Println("4. Default values")

	// Show configured values by source
	if verbose {
		fmt.Println("\n--- Configuration Sources ---")
		displaySourceBreakdown(sources)
	}

	// Environment variable mapping
	fmt.Println("\n--- Environment Variable Mapping ---")
	displayEnvMapping()

	return nil
}

// checkConfigFiles checks configuration and environment files
func checkConfigFiles(cwd, configFile, envFile string, cmd *cobra.Command) {
	// Check configuration file
	fmt.Println("--- Configuration File ---")
	configPath := configFile
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(cwd, configPath)
	}
	if _, err := os.Stat(configPath); err != nil {
		fmt.Printf("❌ Config file not found: %s\n", configPath)
	} else {
		fmt.Printf("✅ Config file found: %s\n", configPath)
	}

	// Check environment file
	fmt.Println("\n--- Environment File ---")
	envPath := envFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(cwd, envPath)
	}
	if _, err := os.Stat(envPath); err != nil {
		fmt.Printf("❌ Env file not found: %s\n", envPath)
	} else {
		fmt.Printf("✅ Env file found: %s\n", envPath)
		if _, err := loadEnvFile(cmd); err != nil {
			fmt.Printf("⚠️  Warning: Failed to load env file: %v\n", err)
		}
	}
}

// configValidateCmd validates configuration files
func configValidateCmd() *cobra.Command {
	var (
		configFile string
		envFile    string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  `Validate a configuration file for syntax errors and required fields.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Load environment file if specified
			if envFile != "" {
				if _, err := loadEnvFile(cmd); err != nil {
					return fmt.Errorf("failed to load env file: %w", err)
				}
			}

			// Load and validate configuration
			cfg, _, err := loadConfigWithSources(ctx, cmd, configFile)
			if err != nil {
				return fmt.Errorf("configuration loading failed: %w", err)
			}

			// Validate
			service := config.NewService()
			if err := service.Validate(cfg); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			fmt.Println("✅ Configuration is valid")
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "compozy.yaml", "Path to configuration file")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to environment file")

	return cmd
}

// loadConfigWithSources loads configuration and tracks sources
func loadConfigWithSources(
	ctx context.Context,
	cmd *cobra.Command,
	configFile string,
) (*config.Config, map[string]config.SourceType, error) {
	service := config.NewService()

	// Create sources
	sources := []config.Source{
		config.NewDefaultProvider(),
		config.NewEnvProvider(),
	}

	// Add YAML config if specified
	if configFile != "" {
		sources = append(sources, config.NewYAMLProvider(configFile))
	}

	// Add CLI flags
	cliFlags := make(map[string]any)
	extractCLIFlags(cmd, cliFlags)
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}

	// Load configuration
	cfg, err := service.Load(ctx, sources...)
	if err != nil {
		return nil, nil, err
	}

	// Get source information
	sourceMap := make(map[string]config.SourceType)
	// Note: In a real implementation, the service would track sources
	// For now, we'll return an empty map
	// TODO: Implement source tracking in the config service

	return cfg, sourceMap, nil
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

	// Server config
	result["server.host"] = cfg.Server.Host
	result["server.port"] = fmt.Sprintf("%d", cfg.Server.Port)
	result["server.cors_enabled"] = fmt.Sprintf("%v", cfg.Server.CORSEnabled)
	result["server.timeout"] = cfg.Server.Timeout.String()

	// Database config
	if cfg.Database.ConnString != "" {
		// Redact connection string as it may contain passwords
		result["database.conn_string"] = redactURL(cfg.Database.ConnString)
	}
	result["database.host"] = cfg.Database.Host
	result["database.port"] = cfg.Database.Port
	result["database.user"] = cfg.Database.User
	result["database.password"] = cfg.Database.Password.String()
	result["database.name"] = cfg.Database.DBName
	result["database.ssl_mode"] = cfg.Database.SSLMode

	// Temporal config
	result["temporal.host_port"] = cfg.Temporal.HostPort
	result["temporal.namespace"] = cfg.Temporal.Namespace
	result["temporal.task_queue"] = cfg.Temporal.TaskQueue

	// Runtime config
	result["runtime.environment"] = cfg.Runtime.Environment
	result["runtime.log_level"] = cfg.Runtime.LogLevel
	result["runtime.dispatcher_heartbeat_interval"] = cfg.Runtime.DispatcherHeartbeatInterval.String()
	result["runtime.dispatcher_heartbeat_ttl"] = cfg.Runtime.DispatcherHeartbeatTTL.String()
	result["runtime.dispatcher_stale_threshold"] = cfg.Runtime.DispatcherStaleThreshold.String()
	result["runtime.async_token_counter_workers"] = fmt.Sprintf("%d", cfg.Runtime.AsyncTokenCounterWorkers)
	result["runtime.async_token_counter_buffer_size"] = fmt.Sprintf("%d", cfg.Runtime.AsyncTokenCounterBufferSize)

	// Limits config
	result["limits.max_nesting_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxNestingDepth)
	result["limits.max_string_length"] = fmt.Sprintf("%d", cfg.Limits.MaxStringLength)
	result["limits.max_message_content"] = fmt.Sprintf("%d", cfg.Limits.MaxMessageContent)
	result["limits.max_total_content_size"] = fmt.Sprintf("%d", cfg.Limits.MaxTotalContentSize)
	result["limits.max_task_context_depth"] = fmt.Sprintf("%d", cfg.Limits.MaxTaskContextDepth)
	result["limits.parent_update_batch_size"] = fmt.Sprintf("%d", cfg.Limits.ParentUpdateBatchSize)

	// OpenAI config (optional)
	if cfg.OpenAI.APIKey != "" {
		result["openai.api_key"] = cfg.OpenAI.APIKey.String()
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

	// Memory config (optional)
	if cfg.Memory.RedisURL != "" {
		// Redact Redis URL as it may contain authentication credentials
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

	// LLM config (optional)
	if cfg.LLM.ProxyURL != "" {
		result["llm.proxy_url"] = cfg.LLM.ProxyURL
	}
	if cfg.LLM.AdminToken != "" {
		// Use String() method which already returns "[REDACTED]" for sensitive values
		result["llm.admin_token"] = cfg.LLM.AdminToken.String()
	}

	return result
}

// displaySourceBreakdown shows which values come from which sources
func displaySourceBreakdown(sources map[string]config.SourceType) {
	if len(sources) == 0 {
		fmt.Println("Source tracking not available in current implementation")
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
		re := regexp.MustCompile(`token=[^&\s]+`)
		return re.ReplaceAllString(urlStr, "token=[REDACTED]")
	}

	return urlStr
}

// isSensitiveEnvVar checks if an environment variable contains sensitive data
func isSensitiveEnvVar(envName, value string) bool {
	// Check for common sensitive patterns in the environment variable name
	sensitivePatterns := []string{
		"PASSWORD",
		"TOKEN",
		"API_KEY",
		"SECRET",
		"PRIVATE",
		"CREDENTIALS",
		"AUTH",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(envName, pattern) {
			return true
		}
	}

	// Check for specific values that look like secrets
	// Redis URLs often contain passwords: redis://user:password@host:port
	if strings.Contains(value, "redis://") && strings.Contains(value, "@") {
		return true
	}

	// Database connection strings may contain passwords
	if strings.Contains(value, "postgres://") || strings.Contains(value, "mysql://") ||
		strings.Contains(value, "mongodb://") {
		return true
	}

	// URLs with authentication
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		if strings.Contains(value, "@") || strings.Contains(value, "token=") {
			return true
		}
	}

	return false
}

// displayEnvMapping shows environment variable mappings
func displayEnvMapping() {
	// Generate mappings from struct tags
	envMappings := config.GenerateEnvMappings()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "ENVIRONMENT VARIABLE\tCONFIG PATH\tCURRENT VALUE")
	fmt.Fprintln(w, "--------------------\t-----------\t-------------")

	for _, mapping := range envMappings {
		value := os.Getenv(mapping.EnvVar)
		if value == "" {
			value = "(not set)"
		} else if isSensitiveEnvVar(mapping.EnvVar, value) {
			value = "[REDACTED]"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", mapping.EnvVar, mapping.ConfigPath, value)
	}
}
