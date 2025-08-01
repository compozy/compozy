# `config` – _Type-safe configuration management with hot-reload and multi-source support_

> **Provides a robust, production-ready configuration system for Compozy with support for YAML files, environment variables, CLI flags, and defaults with proper precedence handling.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `config` package provides a comprehensive configuration management system for Compozy. It supports:

- **Multiple Sources** - YAML files, environment variables, CLI flags, and defaults
- **Type Safety** - Strongly typed configuration with validation
- **Hot Reload** - File watching with automatic configuration updates
- **Source Tracking** - Know which source provided each configuration value
- **Sensitive Data** - Automatic redaction of passwords and API keys
- **Precedence Rules** - Clear ordering: CLI > YAML > Environment > Defaults

The package is built on top of [koanf](https://github.com/knadh/koanf) for configuration loading and includes custom providers for each source type.

---

## 💡 Motivation

- **Production Ready** - Robust configuration system for enterprise deployments
- **Developer Experience** - Clear diagnostics and validation for debugging
- **Security First** - Automatic handling of sensitive configuration data
- **Flexibility** - Support for multiple configuration sources and environments

---

## ⚡ Design Highlights

- **Atomic Updates** - Thread-safe configuration updates with hot-reload
- **Validation** - Comprehensive struct tag validation with custom business rules
- **Source Tracking** - Debug which source provided each configuration value
- **Sensitive Data Protection** - Automatic redaction of secrets in logs and output
- **File Watching** - Intelligent debouncing for configuration file changes
- **Environment Mapping** - Automatic generation of environment variable mappings

---

## 🚀 Getting Started

### Installation

The config package is included in the Compozy project:

```go
import "github.com/compozy/compozy/pkg/config"
```

### Basic Usage

```go
package main

import (
    "context"
    "github.com/compozy/compozy/pkg/config"
)

func main() {
    // Create configuration service
    service := config.NewService()

    // Load configuration from multiple sources
    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("config.yaml"),
    )
    if err != nil {
        panic(err)
    }

    // Use configuration
    fmt.Printf("Server running on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
}
```

---

## 📖 Usage

### Library Usage

#### Service Interface

The `Service` interface provides the main configuration functionality:

```go
type Service interface {
    Load(ctx context.Context, sources ...Source) (*Config, error)
    Watch(ctx context.Context, callback func(*Config)) error
    Validate(config *Config) error
    GetSource(key string) SourceType
}
```

#### Configuration Loading

```go
// Create service
service := config.NewService()

// Load from multiple sources (precedence order)
cfg, err := service.Load(ctx,
    config.NewDefaultProvider(),           // Lowest precedence
    config.NewEnvProvider(),
    config.NewYAMLProvider("config.yaml"),
    config.NewCLIProvider(flagMap),        // Highest precedence
)
```

#### Hot Reload with Manager

```go
// Create manager for hot-reload support
manager := config.NewManager(service)

// Register change callback
manager.OnChange(func(cfg *config.Config) {
    log.Info("Configuration updated", "server_port", cfg.Server.Port)
})

// Load with watching
cfg, err := manager.Load(ctx, sources...)

// Get current configuration atomically
current := manager.Get()
```

#### Configuration Validation

```go
// Validate configuration
if err := service.Validate(cfg); err != nil {
    log.Error("Configuration validation failed", "error", err)
    return err
}
```

---

## 🔧 Configuration

### Configuration Structure

```go
type Config struct {
    Server    ServerConfig    `koanf:"server"`
    Database  DatabaseConfig  `koanf:"database"`
    Temporal  TemporalConfig  `koanf:"temporal"`
    Runtime   RuntimeConfig   `koanf:"runtime"`
    Limits    LimitsConfig    `koanf:"limits"`
    RateLimit RateLimitConfig `koanf:"ratelimit"`
    OpenAI    OpenAIConfig    `koanf:"openai"`
    Memory    MemoryConfig    `koanf:"memory"`
    LLM       LLMConfig       `koanf:"llm"`
    CLI       CLIConfig       `koanf:"cli"`
}
```

### Security Configuration

The config package includes comprehensive security configuration support:

```go
type AuthConfig struct {
    Enabled             bool     `koanf:"enabled" env:"SERVER_AUTH_ENABLED"`
    WorkflowExceptions  []string `koanf:"workflow_exceptions" env:"SERVER_AUTH_WORKFLOW_EXCEPTIONS" validate:"dive,workflow_id"`
}

type ServerConfig struct {
    // ... other fields
    Auth AuthConfig `koanf:"auth"`
}
```

#### Authentication Configuration

- **`server.auth.enabled`**: Enable/disable authentication globally (default: `true`)
- **`server.auth.workflow_exceptions`**: Array of workflow IDs that bypass authentication
- **Custom Validation**: Workflow IDs are validated using the `workflow_id` validator

#### Environment Variables

```bash
# Enable/disable authentication
SERVER_AUTH_ENABLED=true

# Specify workflow exceptions (comma-separated)
SERVER_AUTH_WORKFLOW_EXCEPTIONS=workflow1,workflow2,test-workflow
```

#### YAML Configuration

```yaml
server:
  auth:
    enabled: true
    workflow_exceptions:
      - "public-workflow"
      - "health-check"
      - "metrics-collector"
```

### Environment Variables

The package automatically generates environment variable mappings:

```go
// Get environment mappings
mappings := config.GenerateEnvMappings()

// Check specific environment variable
envVar := config.GetEnvVarForConfigPath("server.port")  // Returns "SERVER_PORT"
```

### Sensitive Data Protection

```go
type DatabaseConfig struct {
    Host     string          `koanf:"host"`
    Password SensitiveString `koanf:"password" sensitive:"true"`
}
```

The `SensitiveString` type automatically redacts sensitive values in logs and output.

---

## 🎨 Examples

### Basic Configuration Loading

```go
func LoadConfig() (*config.Config, error) {
    service := config.NewService()

    return service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("compozy.yaml"),
    )
}
```

### Hot Reload Configuration

```go
func SetupHotReload() error {
    service := config.NewService()
    manager := config.NewManager(service)

    // Register change callback
    manager.OnChange(func(cfg *config.Config) {
        // Update application configuration
        updateServerConfig(cfg.Server)
        updateDatabaseConfig(cfg.Database)
    })

    // Load with watching
    _, err := manager.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("compozy.yaml"),
    )

    return err
}
```

### Configuration Diagnostics

```go
func DiagnoseConfig() error {
    service := config.NewService()

    // Load configuration
    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("compozy.yaml"),
    )
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Validate
    if err := service.Validate(cfg); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Check sources
    serverPortSource := service.GetSource("server.port")
    fmt.Printf("Server port from: %s\n", serverPortSource)

    // Check authentication configuration
    authEnabledSource := service.GetSource("server.auth.enabled")
    fmt.Printf("Auth enabled from: %s (value: %t)\n", authEnabledSource, cfg.Server.Auth.Enabled)

    return nil
}
```

### Security Configuration Examples

#### Basic Authentication Setup

```go
func SetupSecureWorkflow() error {
    service := config.NewService()

    // Load configuration with authentication enabled
    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("secure-config.yaml"),
    )
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Verify authentication is enabled
    if !cfg.Server.Auth.Enabled {
        return fmt.Errorf("authentication must be enabled for secure workflows")
    }

    // Log workflow exceptions
    if len(cfg.Server.Auth.WorkflowExceptions) > 0 {
        fmt.Printf("Workflow exceptions: %v\n", cfg.Server.Auth.WorkflowExceptions)
    }

    return nil
}
```

#### Development Environment Configuration

```go
func SetupDevelopmentAuth() error {
    service := config.NewService()

    // Create development configuration with selective authentication
    devConfig := map[string]any{
        "server": map[string]any{
            "auth": map[string]any{
                "enabled": true,
                "workflow_exceptions": []string{
                    "health-check",
                    "metrics",
                    "debug-workflow",
                },
            },
        },
    }

    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewRawProvider(devConfig),
    )
    if err != nil {
        return fmt.Errorf("failed to load development config: %w", err)
    }

    // Validation will automatically check workflow ID format
    if err := service.Validate(cfg); err != nil {
        return fmt.Errorf("development config validation failed: %w", err)
    }

    return nil
}
```

#### Production Security Configuration

```go
func SetupProductionAuth() error {
    service := config.NewService()

    // Load production configuration with strict authentication
    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
        config.NewYAMLProvider("production.yaml"),
    )
    if err != nil {
        return fmt.Errorf("failed to load production config: %w", err)
    }

    // Ensure authentication is enabled in production
    if !cfg.Server.Auth.Enabled {
        return fmt.Errorf("authentication must be enabled in production")
    }

    // Warn about workflow exceptions in production
    if len(cfg.Server.Auth.WorkflowExceptions) > 0 {
        fmt.Printf("WARNING: %d workflow exceptions in production: %v\n",
            len(cfg.Server.Auth.WorkflowExceptions),
            cfg.Server.Auth.WorkflowExceptions)
    }

    return nil
}
```

### Custom Provider

```go
type consulProvider struct {
    client *consul.Client
}

func (c *consulProvider) Load() (map[string]any, error) {
    // Load configuration from Consul
    data, err := c.client.Get("config/compozy")
    if err != nil {
        return nil, err
    }

    return data, nil
}

func (c *consulProvider) Watch(ctx context.Context, callback func()) error {
    // Watch Consul for changes
    return c.client.Watch(ctx, "config/compozy", callback)
}

func (c *consulProvider) Type() config.SourceType {
    return config.SourceType("consul")
}

func (c *consulProvider) Close() error {
    return c.client.Close()
}
```

### Environment Variable Mapping

```go
func PrintEnvMappings() {
    mappings := config.GenerateEnvMappings()

    for _, mapping := range mappings {
        fmt.Printf("%s -> %s\n", mapping.EnvVar, mapping.ConfigPath)
    }

    // Output:
    // SERVER_HOST -> server.host
    // SERVER_PORT -> server.port
    // DB_HOST -> database.host
    // DB_PASSWORD -> database.password
}
```

---

## 📚 API Reference

### Core Types

#### Config

```go
type Config struct {
    Server   ServerConfig   `koanf:"server"`
    Database DatabaseConfig `koanf:"database"`
    // ... other configuration sections
}
```

The main configuration struct with all application settings.

#### Service Interface

```go
type Service interface {
    Load(ctx context.Context, sources ...Source) (*Config, error)
    Watch(ctx context.Context, callback func(*Config)) error
    Validate(config *Config) error
    GetSource(key string) SourceType
}
```

#### Source Interface

```go
type Source interface {
    Load() (map[string]any, error)
    Watch(ctx context.Context, callback func()) error
    Type() SourceType
    Close() error
}
```

### Factory Functions

#### NewService

```go
func NewService() Service
```

Creates a new configuration service with validation support.

#### NewManager

```go
func NewManager(service Service) *Manager
```

Creates a configuration manager with hot-reload support.

#### Provider Factories

```go
func NewDefaultProvider() Source
func NewEnvProvider() Source
func NewYAMLProvider(path string) Source
func NewCLIProvider(flags map[string]any) Source
```

### Manager Methods

#### Load

```go
func (m *Manager) Load(ctx context.Context, sources ...Source) (*Config, error)
```

Load configuration from sources and start watching.

#### Get

```go
func (m *Manager) Get() *Config
```

Get current configuration atomically.

#### Reload

```go
func (m *Manager) Reload(ctx context.Context) error
```

Force configuration reload from all sources.

#### OnChange

```go
func (m *Manager) OnChange(callback func(*Config))
```

Register callback for configuration changes.

### Utility Functions

#### GenerateEnvMappings

```go
func GenerateEnvMappings() []EnvMapping
```

Generate environment variable to configuration path mappings.

#### GetEnvVarForConfigPath

```go
func GetEnvVarForConfigPath(configPath string) string
```

Get environment variable name for configuration path.

#### IsSensitiveConfigPath

```go
func IsSensitiveConfigPath(configPath string) bool
```

Check if configuration path contains sensitive data.

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./pkg/config/...

# Run with coverage
go test -coverprofile=coverage.out ./pkg/config/...
go tool cover -html=coverage.out

# Run specific test
go test -v ./pkg/config -run TestManager_Load
```

### Test Coverage

The config package includes comprehensive tests for:

- Configuration loading from all source types
- Hot reload functionality with file watching
- Validation with custom business rules
- Source tracking and precedence
- Sensitive data protection
- Environment variable mapping

### Integration Testing

```bash
# Test with real configuration files
go test -v ./pkg/config -run TestIntegration

# Test file watching
go test -v ./pkg/config -run TestWatcher
```

### Example Test

```go
func TestConfigLoad(t *testing.T) {
    service := config.NewService()

    // Create test environment
    os.Setenv("SERVER_PORT", "5001")
    defer os.Unsetenv("SERVER_PORT")

    // Load configuration
    cfg, err := service.Load(context.Background(),
        config.NewDefaultProvider(),
        config.NewEnvProvider(),
    )

    assert.NoError(t, err)
    assert.Equal(t, 5001, cfg.Server.Port)

    // Validate
    assert.NoError(t, service.Validate(cfg))
}
```

### Authentication Configuration Testing

```go
func TestAuthenticationConfig(t *testing.T) {
    service := config.NewService()

    t.Run("Should validate workflow exceptions", func(t *testing.T) {
        // Set valid workflow exceptions
        os.Setenv("SERVER_AUTH_WORKFLOW_EXCEPTIONS", "health-check,metrics,test-workflow")
        defer os.Unsetenv("SERVER_AUTH_WORKFLOW_EXCEPTIONS")

        cfg, err := service.Load(context.Background(),
            config.NewDefaultProvider(),
            config.NewEnvProvider(),
        )

        assert.NoError(t, err)
        assert.True(t, cfg.Server.Auth.Enabled)
        assert.Equal(t, []string{"health-check", "metrics", "test-workflow"}, cfg.Server.Auth.WorkflowExceptions)
        assert.NoError(t, service.Validate(cfg))
    })

    t.Run("Should reject invalid workflow IDs", func(t *testing.T) {
        // Set invalid workflow exceptions
        os.Setenv("SERVER_AUTH_WORKFLOW_EXCEPTIONS", "invalid-workflow-id-that-is-too-long-and-exceeds-limit")
        defer os.Unsetenv("SERVER_AUTH_WORKFLOW_EXCEPTIONS")

        cfg, err := service.Load(context.Background(),
            config.NewDefaultProvider(),
            config.NewEnvProvider(),
        )

        assert.NoError(t, err)
        // Validation should fail due to invalid workflow ID format
        assert.Error(t, service.Validate(cfg))
    })

    t.Run("Should handle authentication disabled", func(t *testing.T) {
        os.Setenv("SERVER_AUTH_ENABLED", "false")
        defer os.Unsetenv("SERVER_AUTH_ENABLED")

        cfg, err := service.Load(context.Background(),
            config.NewDefaultProvider(),
            config.NewEnvProvider(),
        )

        assert.NoError(t, err)
        assert.False(t, cfg.Server.Auth.Enabled)
        assert.NoError(t, service.Validate(cfg))
    })
}
```

### Custom Validator Testing

```go
func TestWorkflowIDValidator(t *testing.T) {
    service := config.NewService()

    testCases := []struct {
        name      string
        workflowID string
        valid     bool
    }{
        {"Valid simple ID", "test-workflow", true},
        {"Valid with underscores", "test_workflow_123", true},
        {"Valid with numbers", "workflow123", true},
        {"Invalid too short", "ab", false},
        {"Invalid empty", "", false},
        {"Invalid too long", strings.Repeat("a", 101), false},
        {"Invalid special chars", "test@workflow", false},
        {"Invalid starts with dash", "-test", false},
        {"Invalid ends with dash", "test-", false},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            cfg := &config.Config{
                Server: config.ServerConfig{
                    Auth: config.AuthConfig{
                        Enabled:            true,
                        WorkflowExceptions: []string{tc.workflowID},
                    },
                },
            }

            err := service.Validate(cfg)
            if tc.valid {
                assert.NoError(t, err)
            } else {
                assert.Error(t, err)
            }
        })
    }
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

### Development Setup

```bash
# Clone repository
git clone https://github.com/compozy/compozy
cd compozy

# Install dependencies
go mod download

# Run tests
make test

# Run linting
make lint
```

### Code Style

- Follow Go conventions and project coding standards
- Use structured logging with context
- Include comprehensive error handling
- Add unit tests for new functionality
- Document exported functions and types

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE) for details.
