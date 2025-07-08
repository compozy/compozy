package config

import (
	"context"
	"fmt"
	"reflect"
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
	return &loader{
		koanf:     koanf.New("."),
		validator: validator.New(),
		metadata: Metadata{
			Sources: make(map[string]SourceType),
		},
		watchCallbacks: make([]func(*Config), 0),
	}
}

// Load loads configuration from the specified sources with precedence order.
// Sources are applied in reverse order, so the last source has highest precedence.
func (l *loader) Load(_ context.Context, sources ...Source) (*Config, error) {
	// Clear and reset
	l.reset()

	// Load defaults
	if err := l.loadDefaults(); err != nil {
		return nil, err
	}

	// Apply additional sources
	if err := l.loadSources(sources); err != nil {
		return nil, err
	}

	// Load environment variables
	if err := l.loadEnvironment(); err != nil {
		return nil, err
	}

	// Unmarshal and validate
	config, err := l.unmarshalAndValidate()
	if err != nil {
		return nil, err
	}

	// Store current configuration atomically
	l.currentConfig.Store(config)

	return config, nil
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

	// Use structs provider to automatically convert the default config to a map
	// This eliminates the need for hardcoded key-value pairs and reduces duplication
	if err := l.koanf.Load(structs.Provider(defaultConfig, "koanf"), nil); err != nil {
		return fmt.Errorf("failed to load defaults: %w", err)
	}

	// Track all keys as coming from defaults
	for _, key := range l.koanf.Keys() {
		l.trackSource(key, SourceDefault)
	}

	return nil
}

// transformEnvKey converts environment variable names to koanf paths.
// For example: LIMITS_MAX_NESTING_DEPTH -> limits.max_nesting_depth
func transformEnvKey(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Split by underscore and filter out empty parts
	// This handles edge cases like "FOO__BAR", "_FOO", "FOO_"
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_'
	})

	// Handle empty or single part
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// For config like LIMITS_MAX_NESTING_DEPTH:
	// parts = ["limits", "max", "nesting", "depth"]
	// We want: "limits.max_nesting_depth"

	// First part is the top-level key (e.g., "limits")
	result := parts[0]

	// Join the remaining parts with underscores to preserve field names
	result = result + "." + strings.Join(parts[1:], "_")

	return result
}

// loadEnvironment loads configuration from environment variables.
func (l *loader) loadEnvironment() error {
	// Track keys before loading env vars
	keysBefore := make(map[string]any)
	for _, key := range l.koanf.Keys() {
		keysBefore[key] = l.koanf.Get(key)
	}

	// Get env to config path mappings from struct tags
	envMappings := GenerateEnvMappings()

	// Create a map for quick lookup
	envToPath := make(map[string]string)
	for _, mapping := range envMappings {
		envToPath[mapping.EnvVar] = mapping.ConfigPath
	}

	// Load environment variables using env/v2 provider with transformation support
	if err := l.koanf.Load(env.Provider(".", env.Opt{
		Prefix: "",
		TransformFunc: func(key string, value string) (string, any) {
			// Check if this env var has an explicit mapping
			if configPath, exists := envToPath[key]; exists {
				return configPath, value
			}
			// If no explicit mapping, use the transform function for backward compatibility
			return transformEnvKey(key), value
		},
	}), nil); err != nil {
		return fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Track keys that were overridden by environment
	for _, key := range l.koanf.Keys() {
		valBefore, existed := keysBefore[key]
		valAfter := l.koanf.Get(key)
		if !existed || valBefore != valAfter {
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

	// Skip loading if data is empty
	if len(data) == 0 {
		return nil
	}

	// Track keys before loading
	keysBefore := make(map[string]any)
	for _, key := range l.koanf.Keys() {
		keysBefore[key] = l.koanf.Get(key)
	}

	// For YAML sources, use a merge strategy that preserves existing values
	// when the new source doesn't contain those keys
	if source.Type() == SourceYAML {
		// Merge only the keys present in the YAML, preserving existing values
		flattened := flattenMap("", data)
		for key, value := range flattened {
			if err := l.koanf.Set(key, value); err != nil {
				return fmt.Errorf("failed to set key %s from source %s: %w", key, source.Type(), err)
			}
		}
	} else {
		// For non-YAML sources, use the normal load behavior
		if err := l.koanf.Load(rawMap(data), nil); err != nil {
			return fmt.Errorf("failed to apply source %s: %w", source.Type(), err)
		}
	}

	// Track which keys were added or changed by this source
	for _, key := range l.koanf.Keys() {
		valBefore, existed := keysBefore[key]
		valAfter := l.koanf.Get(key)
		if !existed || valBefore != valAfter {
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
			// Recursively flatten nested maps
			for fk, fv := range flattenMap(key, nestedMap) {
				result[fk] = fv
			}
		} else {
			result[key] = v
		}
	}
	return result
}

// unmarshalAndValidate unmarshals the configuration and validates it.
func (l *loader) unmarshalAndValidate() (*Config, error) {
	var config Config

	// Use custom unmarshal configuration with decoder hook for SensitiveString
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
	// This method just registers callbacks for when configuration changes
	return nil
}

// Validate checks if the configuration meets all validation requirements.
func (l *loader) Validate(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Validate using struct tags
	if err := l.validator.Struct(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Additional custom validation
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
	// Validate database configuration
	if config.Database.ConnString == "" {
		// If connection string is not provided, ensure individual components are
		if config.Database.Host == "" || config.Database.Port == "" ||
			config.Database.User == "" || config.Database.DBName == "" {
			return fmt.Errorf("database configuration incomplete: either conn_string or individual components required")
		}
	}

	// Validate Temporal configuration
	if config.Temporal.HostPort == "" {
		return fmt.Errorf("temporal host_port is required")
	}

	// Validate dispatcher timing constraints
	if config.Runtime.DispatcherHeartbeatTTL <= config.Runtime.DispatcherHeartbeatInterval {
		return fmt.Errorf("dispatcher heartbeat TTL must be greater than heartbeat interval")
	}

	if config.Runtime.DispatcherStaleThreshold <= config.Runtime.DispatcherHeartbeatTTL {
		return fmt.Errorf("dispatcher stale threshold must be greater than heartbeat TTL")
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
