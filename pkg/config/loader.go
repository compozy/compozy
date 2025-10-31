package config

import (
	"context"
	"fmt"
	"maps"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const (
	maxTCPPort          = 65535
	temporalServiceSpan = 3 // Temporal reserves FrontendPort through FrontendPort+3
)

// loader implements the Service interface for configuration management.
type loader struct {
	koanf          *koanf.Koanf
	validator      *validator.Validate
	metadata       Metadata
	metadataMu     sync.RWMutex
	currentConfig  atomic.Value // stores *Config
	watchCallbacks []func(*Config)
	callbackMu     sync.RWMutex
}

// sensitiveStringDecodeHook is a mapstructure decode hook that converts strings to SensitiveString
func sensitiveStringDecodeHook(_ reflect.Type, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeOf(SensitiveString("")) {
		return data, nil
	}
	switch v := data.(type) {
	case string:
		return SensitiveString(v), nil
	case []byte:
		return SensitiveString(v), nil
	default:
		return data, nil
	}
}

// NewService creates a new configuration service with validation support.
func NewService() Service {
	v := validator.New()
	if err := RegisterCustomValidators(v); err != nil {
		panic(fmt.Sprintf("Failed to register custom validators: %v", err))
	}
	return &loader{
		koanf:     koanf.New("."),
		validator: v,
		metadata: Metadata{
			Sources: make(map[string]SourceType),
		},
		watchCallbacks: make([]func(*Config), 0),
	}
}

// Load loads configuration from the specified sources with precedence order.
// Precedence order (lowest to highest): defaults -> config file -> env -> CLI flags
func (l *loader) Load(_ context.Context, sources ...Source) (*Config, error) {
	l.reset()
	if err := l.loadDefaults(); err != nil {
		return nil, err
	}
	cliSource, otherSources := l.partitionSources(sources)
	if err := l.applyNonDefaultSources(otherSources, cliSource); err != nil {
		return nil, err
	}
	config, err := l.unmarshalAndValidate()
	if err != nil {
		return nil, err
	}
	l.currentConfig.Store(config)
	return config, nil
}

// partitionSources separates CLI, env, and other providers for precedence handling.
// Env sources are skipped because loadEnvironment already covers that tier.
func (l *loader) partitionSources(sources []Source) (Source, []Source) {
	var cliSource Source
	var otherSources []Source
	for _, source := range sources {
		if source == nil {
			continue
		}
		switch source.Type() {
		case SourceEnv:
			continue
		case SourceCLI:
			cliSource = source
		default:
			otherSources = append(otherSources, source)
		}
	}
	return cliSource, otherSources
}

// applyNonDefaultSources layers configuration beyond defaults in precedence order.
// It applies file or struct sources, then environment, and finally CLI overrides.
func (l *loader) applyNonDefaultSources(otherSources []Source, cliSource Source) error {
	if err := l.loadSources(otherSources); err != nil {
		return err
	}
	if err := l.loadEnvironment(); err != nil {
		return err
	}
	return l.loadCLISource(cliSource)
}

// loadCLISource applies CLI overrides when available.
// It keeps the core Load path small by isolating the nil guard.
func (l *loader) loadCLISource(cliSource Source) error {
	if cliSource == nil {
		return nil
	}
	return l.loadSource(cliSource)
}

// reset clears the configuration and metadata.
func (l *loader) reset() {
	l.koanf.Cut("")
	l.metadataMu.Lock()
	l.metadata.Sources = make(map[string]SourceType)
	l.metadata.LoadedAt = time.Now()
	l.metadataMu.Unlock()
}

// loadDefaults loads the default configuration.
func (l *loader) loadDefaults() error {
	defaultConfig := Default()
	if err := l.koanf.Load(structs.Provider(defaultConfig, "koanf"), nil); err != nil {
		return fmt.Errorf("failed to load defaults: %w", err)
	}
	for _, key := range l.koanf.Keys() {
		l.trackSource(key, SourceDefault)
	}
	return nil
}

// transformEnvKey converts environment variable names to koanf paths.
// For example: LIMITS_MAX_NESTING_DEPTH -> limits.max_nesting_depth
func transformEnvKey(s string) string {
	s = strings.ToLower(s)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_'
	})
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	result = result + "." + strings.Join(parts[1:], "_")
	return result
}

// loadEnvironment loads configuration from environment variables.
func (l *loader) loadEnvironment() error {
	keysBefore := make(map[string]any)
	for _, key := range l.koanf.Keys() {
		keysBefore[key] = l.koanf.Get(key)
	}
	envMappings := GenerateEnvMappings()
	envToPath := make(map[string]string)
	for _, mapping := range envMappings {
		envToPath[mapping.EnvVar] = mapping.ConfigPath
	}
	if err := l.koanf.Load(env.Provider(".", env.Opt{
		Prefix: "",
		TransformFunc: func(key string, value string) (string, any) {
			if configPath, exists := envToPath[key]; exists {
				return configPath, value
			}
			return transformEnvKey(key), value
		},
	}), nil); err != nil {
		return fmt.Errorf("failed to load environment variables: %w", err)
	}
	for _, key := range l.koanf.Keys() {
		valBefore, existed := keysBefore[key]
		valAfter := l.koanf.Get(key)
		if !existed || !reflect.DeepEqual(valBefore, valAfter) {
			l.trackSource(key, SourceEnv)
		}
	}
	return nil
}

// loadSources loads configuration from additional sources.
func (l *loader) loadSources(sources []Source) error {
	for _, source := range sources {
		if source == nil || source.Type() == SourceEnv {
			continue
		}

		if err := l.loadSource(source); err != nil {
			return err
		}
	}
	return nil
}

// loadSource loads configuration from a single source.
func (l *loader) loadSource(source Source) error {
	data, err := source.Load()
	if err != nil {
		return fmt.Errorf("failed to load from source %s: %w", source.Type(), err)
	}
	if len(data) == 0 {
		return nil
	}
	keysBefore := make(map[string]any)
	for _, key := range l.koanf.Keys() {
		keysBefore[key] = l.koanf.Get(key)
	}
	if source.Type() == SourceYAML {
		flattened := flattenMap("", data)
		for key, value := range flattened {
			if err := l.koanf.Set(key, value); err != nil {
				return fmt.Errorf("failed to set key %s from source %s: %w", key, source.Type(), err)
			}
		}
	} else {
		if err := l.koanf.Load(rawMap(data), nil); err != nil {
			return fmt.Errorf("failed to apply source %s: %w", source.Type(), err)
		}
	}
	for _, key := range l.koanf.Keys() {
		valBefore, existed := keysBefore[key]
		valAfter := l.koanf.Get(key)
		if !existed || !reflect.DeepEqual(valBefore, valAfter) {
			l.trackSource(key, source.Type())
		}
	}
	return nil
}

// flattenMap flattens a nested map into dot-notation keys
func flattenMap(prefix string, m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		if nestedMap, ok := v.(map[string]any); ok {
			maps.Copy(result, flattenMap(key, nestedMap))
		} else {
			result[key] = v
		}
	}
	return result
}

// unmarshalAndValidate unmarshals the configuration and validates it.
func (l *loader) unmarshalAndValidate() (*Config, error) {
	var config Config
	// NOTE: Decode SensitiveString fields explicitly so secrets aren't logged as plain text.
	if err := l.koanf.UnmarshalWithConf("", &config, koanf.UnmarshalConf{
		Tag: "koanf",
		DecoderConfig: &mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &config,
			TagName:          "koanf",
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
				sensitiveStringDecodeHook,
			),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}
	if err := l.Validate(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return &config, nil
}

// Watch monitors configuration changes and invokes callbacks on updates.
func (l *loader) Watch(_ context.Context, callback func(*Config)) error {
	if callback == nil {
		return fmt.Errorf("callback cannot be nil")
	}
	l.callbackMu.Lock()
	l.watchCallbacks = append(l.watchCallbacks, callback)
	l.callbackMu.Unlock()
	// Note: The actual file watching is handled by the Manager and Source providers
	return nil
}

// Validate checks if the configuration meets all validation requirements.
func (l *loader) Validate(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if err := l.validator.Struct(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if err := l.validateCustom(config); err != nil {
		return fmt.Errorf("custom validation failed: %w", err)
	}
	return nil
}

// GetSource returns the source type for a specific configuration key.
func (l *loader) GetSource(key string) SourceType {
	l.metadataMu.RLock()
	defer l.metadataMu.RUnlock()
	if source, ok := l.metadata.Sources[key]; ok {
		return source
	}
	return SourceDefault
}

// trackSource records which source provided a specific configuration key.
func (l *loader) trackSource(key string, source SourceType) {
	l.metadataMu.Lock()
	defer l.metadataMu.Unlock()
	l.metadata.Sources[key] = source
}

// validateCustom performs custom validation beyond struct tags.
func (l *loader) validateCustom(config *Config) error {
	if err := validateGlobalMode(config); err != nil {
		return err
	}
	if err := validateDatabase(config); err != nil {
		return err
	}
	if err := validateTemporal(config); err != nil {
		return err
	}
	if err := validateRedis(config); err != nil {
		return err
	}
	if err := validateDispatcherTiming(config); err != nil {
		return err
	}
	if err := validatePorts(config); err != nil {
		return err
	}
	if err := validateAuth(config); err != nil {
		return err
	}
	if err := validateMCPProxy(config); err != nil {
		return err
	}
	if err := validateCache(config); err != nil {
		return err
	}
	if err := validateTaskExecutionTimeouts(config); err != nil {
		return err
	}
	if err := validateNativeToolTimeouts(config); err != nil {
		return err
	}
	return nil
}

const deprecatedModeStandalone = "standalone"

func validateGlobalMode(cfg *Config) error {
	switch mode := strings.TrimSpace(cfg.Mode); mode {
	case "", ModeMemory, ModePersistent, ModeDistributed:
		return nil
	case deprecatedModeStandalone:
		return fmt.Errorf(
			"mode %q has been replaced by %q (ephemeral) and %q (persistent); choose the mode that fits your workflow",
			deprecatedModeStandalone,
			ModeMemory,
			ModePersistent,
		)
	default:
		return fmt.Errorf(
			"mode must be one of [%s %s %s], got %q",
			ModeMemory,
			ModePersistent,
			ModeDistributed,
			mode,
		)
	}
}

func validateDatabase(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required for database validation")
	}
	trimmed := strings.TrimSpace(cfg.Database.Driver)
	if trimmed == "" {
		cfg.Database.Driver = cfg.EffectiveDatabaseDriver()
	} else {
		cfg.Database.Driver = trimmed
	}
	if err := cfg.Database.Validate(); err != nil {
		return err
	}
	if cfg.Database.Driver == databaseDriverPostgres && cfg.Database.MigrationTimeout < 45*time.Second {
		// NOTE: Keep migration timeout above advisory-lock window so schema changes don't deadlock.
		return fmt.Errorf("database.migration_timeout must be >= 45s, got: %s", cfg.Database.MigrationTimeout)
	}
	return nil
}

func validateTemporal(cfg *Config) error {
	mode := strings.TrimSpace(cfg.Temporal.Mode)
	if mode == "" {
		resolved := cfg.EffectiveTemporalMode()
		if strings.TrimSpace(resolved) == "" {
			return fmt.Errorf("temporal.mode is required")
		}
		cfg.Temporal.Mode = resolved
		mode = resolved
	} else {
		cfg.Temporal.Mode = mode
	}
	switch mode {
	case ModeRemoteTemporal:
		if cfg.Temporal.HostPort == "" {
			return fmt.Errorf("temporal.host_port is required in remote mode")
		}
		return nil
	case ModeMemory, ModePersistent:
		return validateEmbeddedTemporalConfig(cfg)
	case deprecatedModeStandalone:
		return fmt.Errorf(
			"temporal.mode %q has been removed; use %q for in-memory Temporal or %q for persistent embedded Temporal",
			deprecatedModeStandalone,
			ModeMemory,
			ModePersistent,
		)
	default:
		return fmt.Errorf(
			"temporal.mode must be one of [%s %s %s], got %q",
			ModeMemory,
			ModePersistent,
			ModeRemoteTemporal,
			mode,
		)
	}
}

func validateEmbeddedTemporalConfig(cfg *Config) error {
	embedded := &cfg.Temporal.Standalone
	if err := validateEmbeddedTemporalDatabase(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalPorts(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalNetwork(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalMetadata(embedded); err != nil {
		return err
	}
	if err := validateEmbeddedTemporalLogLevel(embedded); err != nil {
		return err
	}
	return validateEmbeddedTemporalStartTimeout(embedded)
}

func validateEmbeddedTemporalDatabase(embedded *EmbeddedTemporalConfig) error {
	if embedded.DatabaseFile == "" {
		return fmt.Errorf("temporal.standalone.database_file is required when using embedded Temporal")
	}
	return nil
}

func validateEmbeddedTemporalPorts(embedded *EmbeddedTemporalConfig) error {
	if embedded.FrontendPort < 1 || embedded.FrontendPort > maxTCPPort {
		return fmt.Errorf("temporal.standalone.frontend_port must be between 1 and %d", maxTCPPort)
	}
	if embedded.FrontendPort+temporalServiceSpan > maxTCPPort {
		return fmt.Errorf("temporal.standalone.frontend_port reserves out-of-range service port")
	}
	if embedded.EnableUI {
		if embedded.UIPort < 1 || embedded.UIPort > maxTCPPort {
			return fmt.Errorf("temporal.standalone.ui_port must be between 1 and %d when enable_ui is true", maxTCPPort)
		}
		start := embedded.FrontendPort
		end := embedded.FrontendPort + temporalServiceSpan
		if embedded.UIPort >= start && embedded.UIPort <= end {
			return fmt.Errorf("temporal.standalone.ui_port must not collide with service ports [%d-%d]", start, end)
		}
	} else if embedded.UIPort != 0 && (embedded.UIPort < 1 || embedded.UIPort > maxTCPPort) {
		return fmt.Errorf("temporal.standalone.ui_port must be between 1 and %d when set", maxTCPPort)
	}
	return nil
}

func validateEmbeddedTemporalNetwork(embedded *EmbeddedTemporalConfig) error {
	if embedded.BindIP == "" {
		return fmt.Errorf("temporal.standalone.bind_ip is required when using embedded Temporal")
	}
	if net.ParseIP(embedded.BindIP) == nil {
		return fmt.Errorf("temporal.standalone.bind_ip must be a valid IP address")
	}
	return nil
}

func validateEmbeddedTemporalMetadata(embedded *EmbeddedTemporalConfig) error {
	if embedded.Namespace == "" {
		return fmt.Errorf("temporal.standalone.namespace is required when using embedded Temporal")
	}
	if embedded.ClusterName == "" {
		return fmt.Errorf("temporal.standalone.cluster_name is required when using embedded Temporal")
	}
	return nil
}

func validateEmbeddedTemporalLogLevel(embedded *EmbeddedTemporalConfig) error {
	switch embedded.LogLevel {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf(
			"temporal.standalone.log_level must be one of [debug info warn error], got %q",
			embedded.LogLevel,
		)
	}
}

func validateEmbeddedTemporalStartTimeout(embedded *EmbeddedTemporalConfig) error {
	if embedded.StartTimeout <= 0 {
		return fmt.Errorf("temporal.standalone.start_timeout must be positive")
	}
	return nil
}

// validateRedis performs validation for Redis configuration including
// deployment mode requirements and embedded persistence settings.
func validateRedis(cfg *Config) error {
	// Validate component mode values via struct tags; add friendly errors for clarity.
	switch mode := strings.TrimSpace(cfg.Redis.Mode); mode {
	case "", ModeMemory, ModePersistent, ModeDistributed:
		// ok
	case deprecatedModeStandalone:
		return fmt.Errorf(
			"redis.mode %q is no longer supported; switch to %q (ephemeral) or %q (persistent) to run the embedded Redis",
			deprecatedModeStandalone,
			ModeMemory,
			ModePersistent,
		)
	default:
		return fmt.Errorf(
			"redis.mode must be one of [%s %s %s] or empty for inheritance, got %q",
			ModeMemory,
			ModePersistent,
			ModeDistributed,
			mode,
		)
	}

	// Validate requirements based on effective mode
	if isEmbeddedMode(cfg.EffectiveRedisMode()) {
		// When using embedded redis, validate optional persistence settings when enabled.
		p := cfg.Redis.Standalone.Persistence
		if p.Enabled {
			if strings.TrimSpace(p.DataDir) == "" {
				return fmt.Errorf("redis.standalone.persistence.data_dir is required when persistence.enabled is true")
			}
			if p.SnapshotInterval <= 0 {
				return fmt.Errorf(
					"redis.standalone.persistence.snapshot_interval must be a positive duration " +
						"when persistence.enabled is true",
				)
			}
		}
	}
	return nil
}

func validateDispatcherTiming(cfg *Config) error {
	if cfg.Runtime.DispatcherHeartbeatTTL <= cfg.Runtime.DispatcherHeartbeatInterval {
		return fmt.Errorf("dispatcher heartbeat TTL must be greater than heartbeat interval")
	}
	if cfg.Runtime.DispatcherStaleThreshold <= cfg.Runtime.DispatcherHeartbeatTTL {
		return fmt.Errorf("dispatcher stale threshold must be greater than heartbeat TTL")
	}
	return nil
}

func validatePorts(cfg *Config) error {
	if cfg.Redis.Port != "" {
		if err := validateTCPPort(cfg.Redis.Port, "Redis port"); err != nil {
			return err
		}
	}
	if cfg.Database.Port != "" {
		if err := validateTCPPort(cfg.Database.Port, "Database port"); err != nil {
			return err
		}
	}
	return nil
}

func validateTaskExecutionTimeouts(cfg *Config) error {
	defaultTimeout := cfg.Runtime.TaskExecutionTimeoutDefault
	maxTimeout := cfg.Runtime.TaskExecutionTimeoutMax
	if defaultTimeout <= 0 {
		return fmt.Errorf("runtime.task_execution_timeout_default must be greater than 0, got: %s", defaultTimeout)
	}
	if maxTimeout <= 0 {
		return fmt.Errorf("runtime.task_execution_timeout_max must be greater than 0, got: %s", maxTimeout)
	}
	if defaultTimeout > maxTimeout {
		return fmt.Errorf(
			"runtime.task_execution_timeout_default (%s) must not exceed runtime.task_execution_timeout_max (%s)",
			defaultTimeout,
			maxTimeout,
		)
	}
	return nil
}

func validateAuth(cfg *Config) error {
	if cfg.Server.Auth.Enabled && cfg.Server.Auth.AdminKey == "" {
		return fmt.Errorf("server.auth.admin_key is required when authentication is enabled")
	}
	return nil
}

func validateMCPProxy(cfg *Config) error {
	if err := validateMCPProxyMode(cfg); err != nil {
		return err
	}
	mode := cfg.EffectiveMCPProxyMode()
	if isEmbeddedMode(mode) && cfg.MCPProxy.Port == 0 {
		return fmt.Errorf(
			"mcp_proxy.port must be non-zero when mode is %q or %q",
			ModeMemory,
			ModePersistent,
		)
	}
	return nil
}

func validateMCPProxyMode(cfg *Config) error {
	switch mode := strings.TrimSpace(cfg.MCPProxy.Mode); mode {
	case "", ModeMemory, ModePersistent, ModeDistributed:
		return nil
	case deprecatedModeStandalone:
		return fmt.Errorf(
			"mcp_proxy.mode %q is no longer supported; switch to %q (ephemeral), %q (persistent), or %q (external)",
			deprecatedModeStandalone,
			ModeMemory,
			ModePersistent,
			ModeDistributed,
		)
	default:
		return fmt.Errorf(
			"mcp_proxy.mode must be one of [%s %s %s] or empty for inheritance, got %q",
			ModeMemory,
			ModePersistent,
			ModeDistributed,
			mode,
		)
	}
}

func validateCache(cfg *Config) error {
	if cfg.Cache.KeyScanCount <= 0 {
		return fmt.Errorf("cache.key_scan_count must be > 0")
	}
	return nil
}

func validateNativeToolTimeouts(cfg *Config) error {
	tools := cfg.Runtime.NativeTools
	if tools.CallAgent.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_agent.default_timeout must be >= 0, got: %s",
			tools.CallAgent.DefaultTimeout,
		)
	}
	if tools.CallAgents.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_agents.default_timeout must be >= 0, got: %s",
			tools.CallAgents.DefaultTimeout,
		)
	}
	if tools.CallTask.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_task.default_timeout must be >= 0, got: %s",
			tools.CallTask.DefaultTimeout,
		)
	}
	if tools.CallTasks.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_tasks.default_timeout must be >= 0, got: %s",
			tools.CallTasks.DefaultTimeout,
		)
	}
	if tools.CallWorkflow.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_workflow.default_timeout must be >= 0, got: %s",
			tools.CallWorkflow.DefaultTimeout,
		)
	}
	if tools.CallWorkflows.DefaultTimeout < 0 {
		return fmt.Errorf(
			"runtime.native_tools.call_workflows.default_timeout must be >= 0, got: %s",
			tools.CallWorkflows.DefaultTimeout,
		)
	}
	return nil
}

// validateTCPPort validates that a string represents a valid TCP port number (1-maxTCPPort)
func validateTCPPort(portStr, fieldName string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("%s must be a valid integer, got: %s", fieldName, portStr)
	}
	if port < 1 || port > maxTCPPort {
		return fmt.Errorf("%s must be between 1 and %d, got: %d", fieldName, maxTCPPort, port)
	}
	return nil
}

// rawMap is a koanf.Provider adapter for map[string]any data.
// It's used to adapt custom source providers to koanf's loading mechanism.
type rawMap map[string]any

func (r rawMap) Read() (map[string]any, error) {
	return r, nil
}

func (r rawMap) ReadBytes() ([]byte, error) {
	return nil, fmt.Errorf("ReadBytes not implemented")
}
