package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/config/definition"
	"gopkg.in/yaml.v3"
)

// envProvider is a placeholder for backward compatibility.
// The actual environment loading is now handled by koanf's native env provider.
type envProvider struct{}

// NewEnvProvider creates a new environment variable configuration source.
// Note: This is kept for backward compatibility, but the actual loading
// is handled by koanf's native env provider in loader.go
func NewEnvProvider() Source {
	return &envProvider{}
}

// Load returns empty map as environment loading is handled natively by koanf.
func (e *envProvider) Load() (map[string]any, error) {
	return make(map[string]any), nil
}

// Watch is not implemented for environment variables as they don't change at runtime.
func (e *envProvider) Watch(_ context.Context, _ func()) error {
	return nil
}

// Type returns the source type identifier.
func (e *envProvider) Type() SourceType {
	return SourceEnv
}

// Close releases any resources held by the source.
func (e *envProvider) Close() error {
	return nil
}

// cliProvider implements Source interface for CLI flags.
type cliProvider struct {
	flags map[string]any
}

// NewCLIProvider creates a new CLI flags configuration source.
func NewCLIProvider(flags map[string]any) Source {
	return &cliProvider{
		flags: flags,
	}
}

// Load returns the CLI flags as configuration data.
func (c *cliProvider) Load() (map[string]any, error) {
	if c.flags == nil {
		return make(map[string]any), nil
	}
	registry := definition.CreateRegistry()
	flagToPath := registry.GetCLIFlagMapping()
	config := make(map[string]any)
	for key, value := range c.flags {
		if path, ok := flagToPath[key]; ok {
			if err := setNested(config, path, value); err != nil {
				return nil, fmt.Errorf("failed to set CLI flag %s: %w", key, err)
			}
		}
	}
	return config, nil
}

// Watch is not implemented for CLI flags as they don't change at runtime.
func (c *cliProvider) Watch(_ context.Context, _ func()) error {
	return nil
}

// Type returns the source type identifier.
func (c *cliProvider) Type() SourceType {
	return SourceCLI
}

// Close releases any resources held by the source.
func (c *cliProvider) Close() error {
	return nil
}

// setNested sets a value in a nested map structure using dot notation.
// It returns an error if a path conflict is encountered.
func setNested(m map[string]any, path string, value any) error {
	if path == "" {
		return nil // Don't set anything for empty path
	}
	parts := strings.Split(path, ".")
	current := m
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			return fmt.Errorf("configuration conflict: key %q is not a map", strings.Join(parts[:i+1], "."))
		}
		current = next
	}
	if len(parts) > 0 {
		current[parts[len(parts)-1]] = value
	}
	return nil
}

// yamlProvider implements Source interface for YAML files.
type yamlProvider struct {
	path       string
	watcher    *Watcher
	watcherMu  sync.Mutex
	isWatching bool
	watchOnce  sync.Once
	closeOnce  sync.Once
}

// NewYAMLProvider creates a new YAML file configuration source.
func NewYAMLProvider(path string) Source {
	return &yamlProvider{
		path: path,
	}
}

// Load reads configuration from a YAML file.
func (y *yamlProvider) Load() (map[string]any, error) {
	data, err := os.ReadFile(y.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}
	filtered := filterNilValues(config)
	return filtered, nil
}

// filterNilValues recursively removes nil values from a map
// This prevents koanf from overriding existing values with nil
func filterNilValues(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		if v == nil {
			continue
		}
		if nestedMap, ok := v.(map[string]any); ok {
			filtered := filterNilValues(nestedMap)
			if len(filtered) > 0 {
				result[k] = filtered
			}
		} else {
			result[k] = v
		}
	}
	return result
}

// Watch monitors the YAML file for changes.
func (y *yamlProvider) Watch(ctx context.Context, callback func()) error {
	var watchErr error
	y.watchOnce.Do(func() {
		y.watcherMu.Lock()
		defer y.watcherMu.Unlock()

		watcher, err := NewWatcher()
		if err != nil {
			watchErr = fmt.Errorf("failed to create watcher: %w", err)
			return
		}
		y.watcher = watcher

		if err := y.watcher.Watch(ctx, y.path); err != nil {
			watchErr = fmt.Errorf("failed to watch YAML file: %w", err)
			return
		}
		y.isWatching = true
	})
	if watchErr != nil {
		return watchErr
	}
	y.watcherMu.Lock()
	defer y.watcherMu.Unlock()
	if y.watcher != nil {
		y.watcher.OnChange(callback)
	}
	return nil
}

// Type returns the source type identifier.
func (y *yamlProvider) Type() SourceType {
	return SourceYAML
}

// Close releases any resources held by the source.
func (y *yamlProvider) Close() error {
	var closeErr error
	y.closeOnce.Do(func() {
		y.watcherMu.Lock()
		defer y.watcherMu.Unlock()

		if y.watcher != nil {
			if err := y.watcher.Close(); err != nil {
				closeErr = fmt.Errorf("failed to close watcher: %w", err)
				return
			}
			y.watcher = nil
			y.isWatching = false
		}

		y.watchOnce = sync.Once{}
	})
	return closeErr
}

// defaultProvider implements Source interface for default configuration values.
type defaultProvider struct {
	defaults map[string]any
}

// NewDefaultProvider creates a new default configuration source.
func NewDefaultProvider() Source {
	return &defaultProvider{
		defaults: createDefaultMap(),
	}
}

// Load returns the default configuration values.
func (d *defaultProvider) Load() (map[string]any, error) {
	return d.defaults, nil
}

// Watch is not implemented for defaults as they don't change at runtime.
func (d *defaultProvider) Watch(_ context.Context, _ func()) error {
	return nil
}

// Type returns the source type identifier.
func (d *defaultProvider) Type() SourceType {
	return SourceDefault
}

// Close releases any resources held by the source.
func (d *defaultProvider) Close() error {
	return nil
}

// createDefaultMap creates a map representation of default values from the registry.
func createDefaultMap() map[string]any {
	defaultConfig := Default()
	result := make(map[string]any)
	addCoreDefaults(result, defaultConfig)
	addServiceDefaults(result, defaultConfig)
	addInfraDefaults(result, defaultConfig)
	return result
}

// addCoreDefaults adds core system configuration defaults
func addCoreDefaults(result map[string]any, defaultConfig *Config) {
	result["server"] = createServerDefaults(defaultConfig)
	result["database"] = createDatabaseDefaults(defaultConfig)
	result["temporal"] = createTemporalDefaults(defaultConfig)
	result["runtime"] = createRuntimeDefaults(defaultConfig)
	result["limits"] = createLimitsDefaults(defaultConfig)
	result["attachments"] = createAttachmentsDefaults(defaultConfig)
}

// addServiceDefaults adds service configuration defaults
func addServiceDefaults(result map[string]any, defaultConfig *Config) {
	result["memory"] = createMemoryDefaults(defaultConfig)
	result["llm"] = createLLMDefaults(defaultConfig)
	result["ratelimit"] = createRateLimitDefaults(defaultConfig)
	result["cli"] = createCLIDefaults(defaultConfig)
}

// addInfraDefaults adds infrastructure configuration defaults
func addInfraDefaults(result map[string]any, defaultConfig *Config) {
	result["redis"] = createRedisDefaults(defaultConfig)
	result["cache"] = createCacheDefaults(defaultConfig)
	result["worker"] = createWorkerDefaults(defaultConfig)
	result["mcp_proxy"] = createMCPProxyDefaults(defaultConfig)
	result["webhooks"] = createWebhooksDefaults(defaultConfig)
}

// createServerDefaults creates server configuration defaults
func createServerDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"host":         defaultConfig.Server.Host,
		"port":         defaultConfig.Server.Port,
		"cors_enabled": defaultConfig.Server.CORSEnabled,
		"timeout":      defaultConfig.Server.Timeout.String(),
		"cors": map[string]any{
			"allowed_origins":   defaultConfig.Server.CORS.AllowedOrigins,
			"allow_credentials": defaultConfig.Server.CORS.AllowCredentials,
			"max_age":           defaultConfig.Server.CORS.MaxAge,
		},
		"auth": map[string]any{
			"enabled":                           defaultConfig.Server.Auth.Enabled,
			"workflow_exceptions":               defaultConfig.Server.Auth.WorkflowExceptions,
			"api_key_last_used_max_concurrency": defaultConfig.Server.Auth.APIKeyLastUsedMaxConcurrency,
			"api_key_last_used_timeout":         defaultConfig.Server.Auth.APIKeyLastUsedTimeout.String(),
		},
		"timeouts": map[string]any{
			"monitoring_init":                defaultConfig.Server.Timeouts.MonitoringInit.String(),
			"monitoring_shutdown":            defaultConfig.Server.Timeouts.MonitoringShutdown.String(),
			"db_shutdown":                    defaultConfig.Server.Timeouts.DBShutdown.String(),
			"worker_shutdown":                defaultConfig.Server.Timeouts.WorkerShutdown.String(),
			"server_shutdown":                defaultConfig.Server.Timeouts.ServerShutdown.String(),
			"http_read":                      defaultConfig.Server.Timeouts.HTTPRead.String(),
			"http_write":                     defaultConfig.Server.Timeouts.HTTPWrite.String(),
			"http_idle":                      defaultConfig.Server.Timeouts.HTTPIdle.String(),
			"schedule_retry_max_duration":    defaultConfig.Server.Timeouts.ScheduleRetryMaxDuration.String(),
			"schedule_retry_base_delay":      defaultConfig.Server.Timeouts.ScheduleRetryBaseDelay.String(),
			"schedule_retry_max_delay":       defaultConfig.Server.Timeouts.ScheduleRetryMaxDelay.String(),
			"schedule_retry_max_attempts":    defaultConfig.Server.Timeouts.ScheduleRetryMaxAttempts,
			"schedule_retry_backoff_seconds": defaultConfig.Server.Timeouts.ScheduleRetryBackoffSeconds,
			"knowledge_ingest":               defaultConfig.Server.Timeouts.KnowledgeIngest.String(),
			"temporal_reachability":          defaultConfig.Server.Timeouts.TemporalReachability.String(),
			"start_probe_delay":              defaultConfig.Server.Timeouts.StartProbeDelay.String(),
		},
		"reconciler": map[string]any{
			"queue_capacity":    defaultConfig.Server.Reconciler.QueueCapacity,
			"debounce_wait":     defaultConfig.Server.Reconciler.DebounceWait.String(),
			"debounce_max_wait": defaultConfig.Server.Reconciler.DebounceMaxWait.String(),
		},
	}
}

// createDatabaseDefaults creates database configuration defaults
func createDatabaseDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"host":                 defaultConfig.Database.Host,
		"port":                 defaultConfig.Database.Port,
		"user":                 defaultConfig.Database.User,
		"password":             defaultConfig.Database.Password,
		"name":                 defaultConfig.Database.DBName,
		"ssl_mode":             defaultConfig.Database.SSLMode,
		"conn_string":          defaultConfig.Database.ConnString,
		"auto_migrate":         defaultConfig.Database.AutoMigrate,
		"migration_timeout":    defaultConfig.Database.MigrationTimeout.String(),
		"max_open_conns":       defaultConfig.Database.MaxOpenConns,
		"max_idle_conns":       defaultConfig.Database.MaxIdleConns,
		"conn_max_lifetime":    defaultConfig.Database.ConnMaxLifetime.String(),
		"conn_max_idle_time":   defaultConfig.Database.ConnMaxIdleTime.String(),
		"ping_timeout":         defaultConfig.Database.PingTimeout.String(),
		"health_check_timeout": defaultConfig.Database.HealthCheckTimeout.String(),
		"health_check_period":  defaultConfig.Database.HealthCheckPeriod.String(),
		"connect_timeout":      defaultConfig.Database.ConnectTimeout.String(),
	}
}

// createTemporalDefaults creates temporal configuration defaults
func createTemporalDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"host_port":  defaultConfig.Temporal.HostPort,
		"namespace":  defaultConfig.Temporal.Namespace,
		"task_queue": defaultConfig.Temporal.TaskQueue,
	}
}

// createRuntimeDefaults creates runtime configuration defaults
func createRuntimeDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"environment":                     defaultConfig.Runtime.Environment,
		"log_level":                       defaultConfig.Runtime.LogLevel,
		"dispatcher_heartbeat_interval":   defaultConfig.Runtime.DispatcherHeartbeatInterval.String(),
		"dispatcher_heartbeat_ttl":        defaultConfig.Runtime.DispatcherHeartbeatTTL.String(),
		"dispatcher_stale_threshold":      defaultConfig.Runtime.DispatcherStaleThreshold.String(),
		"async_token_counter_workers":     defaultConfig.Runtime.AsyncTokenCounterWorkers,
		"async_token_counter_buffer_size": defaultConfig.Runtime.AsyncTokenCounterBufferSize,
		"task_execution_timeout_default":  defaultConfig.Runtime.TaskExecutionTimeoutDefault.String(),
		"task_execution_timeout_max":      defaultConfig.Runtime.TaskExecutionTimeoutMax.String(),
		"tool_execution_timeout":          defaultConfig.Runtime.ToolExecutionTimeout.String(),
		"runtime_type":                    defaultConfig.Runtime.RuntimeType,
		"entrypoint_path":                 defaultConfig.Runtime.EntrypointPath,
		"bun_permissions":                 defaultConfig.Runtime.BunPermissions,
	}
}

// createLimitsDefaults creates limits configuration defaults
func createLimitsDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"max_nesting_depth":             defaultConfig.Limits.MaxNestingDepth,
		"max_config_file_nesting_depth": defaultConfig.Limits.MaxConfigFileNestingDepth,
		"max_string_length":             defaultConfig.Limits.MaxStringLength,
		"max_config_file_size":          defaultConfig.Limits.MaxConfigFileSize,
		"max_message_content":           defaultConfig.Limits.MaxMessageContent,
		"max_total_content_size":        defaultConfig.Limits.MaxTotalContentSize,
		"max_task_context_depth":        defaultConfig.Limits.MaxTaskContextDepth,
		"parent_update_batch_size":      defaultConfig.Limits.ParentUpdateBatchSize,
	}
}

// createAttachmentsDefaults creates attachments configuration defaults
func createAttachmentsDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"max_download_size_bytes": defaultConfig.Attachments.MaxDownloadSizeBytes,
		"download_timeout":        defaultConfig.Attachments.DownloadTimeout.String(),
		"max_redirects":           defaultConfig.Attachments.MaxRedirects,
		"allowed_mime_types": map[string]any{
			"image": defaultConfig.Attachments.AllowedMIMETypes.Image,
			"audio": defaultConfig.Attachments.AllowedMIMETypes.Audio,
			"video": defaultConfig.Attachments.AllowedMIMETypes.Video,
			"pdf":   defaultConfig.Attachments.AllowedMIMETypes.PDF,
		},
		"temp_dir_quota_bytes": defaultConfig.Attachments.TempDirQuotaBytes,
	}
}

// createMemoryDefaults creates memory configuration defaults
func createMemoryDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"prefix":      defaultConfig.Memory.Prefix,
		"ttl":         defaultConfig.Memory.TTL.String(),
		"max_entries": defaultConfig.Memory.MaxEntries,
	}
}

// createLLMDefaults creates LLM configuration defaults
func createLLMDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"proxy_url":                    defaultConfig.LLM.ProxyURL,
		"retry_attempts":               defaultConfig.LLM.RetryAttempts,
		"retry_backoff_base":           defaultConfig.LLM.RetryBackoffBase.String(),
		"retry_backoff_max":            defaultConfig.LLM.RetryBackoffMax.String(),
		"provider_timeout":             defaultConfig.LLM.ProviderTimeout.String(),
		"retry_jitter":                 defaultConfig.LLM.RetryJitter,
		"max_concurrent_tools":         defaultConfig.LLM.MaxConcurrentTools,
		"max_tool_iterations":          defaultConfig.LLM.MaxToolIterations,
		"max_sequential_tool_errors":   defaultConfig.LLM.MaxSequentialToolErrors,
		"max_consecutive_successes":    defaultConfig.LLM.MaxConsecutiveSuccesses,
		"enable_progress_tracking":     defaultConfig.LLM.EnableProgressTracking,
		"no_progress_threshold":        defaultConfig.LLM.NoProgressThreshold,
		"enable_loop_restarts":         defaultConfig.LLM.EnableLoopRestarts,
		"restart_stall_threshold":      defaultConfig.LLM.RestartStallThreshold,
		"max_loop_restarts":            defaultConfig.LLM.MaxLoopRestarts,
		"enable_context_compaction":    defaultConfig.LLM.EnableContextCompaction,
		"context_compaction_threshold": defaultConfig.LLM.ContextCompactionThreshold,
		"context_compaction_cooldown":  defaultConfig.LLM.ContextCompactionCooldown,
		"enable_dynamic_prompt_state":  defaultConfig.LLM.EnableDynamicPromptState,
		"tool_call_caps": map[string]any{
			"default":   defaultConfig.LLM.ToolCallCaps.Default,
			"overrides": defaultConfig.LLM.ToolCallCaps.Overrides,
		},
		"structured_output_retries":      defaultConfig.LLM.StructuredOutputRetryAttempts,
		"finalize_output_retries":        defaultConfig.LLM.FinalizeOutputRetryAttempts,
		"register_mcps":                  defaultConfig.LLM.RegisterMCPs,
		"allowed_mcp_names":              defaultConfig.LLM.AllowedMCPNames,
		"denied_mcp_names":               defaultConfig.LLM.DeniedMCPNames,
		"fail_on_mcp_registration_error": defaultConfig.LLM.FailOnMCPRegistrationError,
		"mcp_client_timeout":             defaultConfig.LLM.MCPClientTimeout.String(),
		"retry_jitter_percent":           defaultConfig.LLM.RetryJitterPercent,
		"context_warning_thresholds":     defaultConfig.LLM.ContextWarningThresholds,
		"usage_metrics": map[string]any{
			"persist_buckets": defaultConfig.LLM.UsageMetrics.PersistBuckets,
		},
	}
}

// createRateLimitDefaults creates rate limit configuration defaults
func createRateLimitDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"global_rate": map[string]any{
			"limit":  defaultConfig.RateLimit.GlobalRate.Limit,
			"period": defaultConfig.RateLimit.GlobalRate.Period.String(),
		},
		"api_key_rate": map[string]any{
			"limit":  defaultConfig.RateLimit.APIKeyRate.Limit,
			"period": defaultConfig.RateLimit.APIKeyRate.Period.String(),
		},
		"prefix":    defaultConfig.RateLimit.Prefix,
		"max_retry": defaultConfig.RateLimit.MaxRetry,
	}
}

// createCLIDefaults creates CLI configuration defaults
func createCLIDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"api_key":             string(defaultConfig.CLI.APIKey),
		"base_url":            defaultConfig.CLI.BaseURL,
		"server_url":          defaultConfig.CLI.ServerURL,
		"timeout":             defaultConfig.CLI.Timeout.String(),
		"mode":                defaultConfig.CLI.Mode,
		"default_format":      defaultConfig.CLI.DefaultFormat,
		"color_mode":          defaultConfig.CLI.ColorMode,
		"page_size":           defaultConfig.CLI.PageSize,
		"output_format_alias": defaultConfig.CLI.OutputFormatAlias,
		"no_color":            defaultConfig.CLI.NoColor,
		"debug":               defaultConfig.CLI.Debug,
		"quiet":               defaultConfig.CLI.Quiet,
		"interactive":         defaultConfig.CLI.Interactive,
		"config_file":         defaultConfig.CLI.ConfigFile,
		"cwd":                 defaultConfig.CLI.CWD,
		"env_file":            defaultConfig.CLI.EnvFile,
		"max_retries":         defaultConfig.CLI.MaxRetries,
		"dev": map[string]any{
			"watcher_debounce":      defaultConfig.CLI.Dev.WatcherDebounce.String(),
			"watcher_retry_initial": defaultConfig.CLI.Dev.WatcherRetryInitial.String(),
			"watcher_retry_max":     defaultConfig.CLI.Dev.WatcherRetryMax.String(),
		},
	}
}

// createRedisDefaults creates Redis configuration defaults
func createRedisDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"url":            defaultConfig.Redis.URL,
		"host":           defaultConfig.Redis.Host,
		"port":           defaultConfig.Redis.Port,
		"password":       defaultConfig.Redis.Password,
		"db":             defaultConfig.Redis.DB,
		"max_retries":    defaultConfig.Redis.MaxRetries,
		"pool_size":      defaultConfig.Redis.PoolSize,
		"min_idle_conns": defaultConfig.Redis.MinIdleConns,
	}
}

// createCacheDefaults creates cache configuration defaults
func createCacheDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"enabled":               defaultConfig.Cache.Enabled,
		"ttl":                   defaultConfig.Cache.TTL.String(),
		"prefix":                defaultConfig.Cache.Prefix,
		"max_item_size":         defaultConfig.Cache.MaxItemSize,
		"compression_enabled":   defaultConfig.Cache.CompressionEnabled,
		"compression_threshold": defaultConfig.Cache.CompressionThreshold,
		"eviction_policy":       defaultConfig.Cache.EvictionPolicy,
		"stats_interval":        defaultConfig.Cache.StatsInterval.String(),
	}
}

// createWorkerDefaults creates worker configuration defaults
func createWorkerDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"config_store_ttl":               defaultConfig.Worker.ConfigStoreTTL.String(),
		"heartbeat_cleanup_timeout":      defaultConfig.Worker.HeartbeatCleanupTimeout.String(),
		"mcp_shutdown_timeout":           defaultConfig.Worker.MCPShutdownTimeout.String(),
		"dispatcher_retry_delay":         defaultConfig.Worker.DispatcherRetryDelay.String(),
		"dispatcher_max_retries":         defaultConfig.Worker.DispatcherMaxRetries,
		"mcp_proxy_health_check_timeout": defaultConfig.Worker.MCPProxyHealthCheckTimeout.String(),
		"dispatcher": map[string]any{
			"heartbeat_ttl":   defaultConfig.Worker.Dispatcher.HeartbeatTTL.String(),
			"stale_threshold": defaultConfig.Worker.Dispatcher.StaleThreshold.String(),
		},
	}
}

// createMCPProxyDefaults creates MCP proxy configuration defaults
func createMCPProxyDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"mode":                    defaultConfig.MCPProxy.Mode,
		"host":                    defaultConfig.MCPProxy.Host,
		"port":                    defaultConfig.MCPProxy.Port,
		"base_url":                defaultConfig.MCPProxy.BaseURL,
		"shutdown_timeout":        defaultConfig.MCPProxy.ShutdownTimeout.String(),
		"max_idle_conns":          defaultConfig.MCPProxy.MaxIdleConns,
		"max_idle_conns_per_host": defaultConfig.MCPProxy.MaxIdleConnsPerHost,
		"max_conns_per_host":      defaultConfig.MCPProxy.MaxConnsPerHost,
		"idle_conn_timeout":       defaultConfig.MCPProxy.IdleConnTimeout.String(),
	}
}

// createWebhooksDefaults creates webhooks configuration defaults
func createWebhooksDefaults(defaultConfig *Config) map[string]any {
	return map[string]any{
		"default_method":     defaultConfig.Webhooks.DefaultMethod,
		"default_max_body":   defaultConfig.Webhooks.DefaultMaxBody,
		"default_dedupe_ttl": defaultConfig.Webhooks.DefaultDedupeTTL.String(),
		"stripe_skew":        defaultConfig.Webhooks.StripeSkew.String(),
	}
}
