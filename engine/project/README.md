# `engine/project` – _Declarative Project Configuration Management_

> **Loads, validates, and manages Compozy project settings including workflows, models, and runtime configuration for AI-powered applications.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Configuration Loading](#configuration-loading)
  - [Validation](#validation)
  - [Environment Integration](#environment-integration)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `engine/project` package provides comprehensive project configuration management for Compozy applications. It handles loading, validation, and management of project settings including workflows, LLM models, runtime configurations, and environment integration.

This package serves as the foundation for Compozy projects, enabling declarative configuration of AI workflows through YAML files with robust validation and environment variable integration.

---

## 💡 Motivation

- **Declarative Configuration**: Define AI workflows and settings through YAML configuration files
- **Schema Validation**: Ensure project configurations are valid and complete before runtime
- **Environment Integration**: Seamlessly integrate environment variables and secrets
- **Security-First Defaults**: Provide secure default configurations with principle of least privilege

---

## ⚡ Design Highlights

- **Schema-Driven Validation**: Comprehensive validation of project configurations with detailed error messages
- **Environment Variable Integration**: Template-based environment variable resolution with fallback support
- **Modular Architecture**: Extensible design supporting multiple workflow sources, model providers, and runtime types
- **Security-First Defaults**: Secure default configurations for runtime permissions and resource access
- **Hot-Reload Support**: Autoload configuration for development with file watching and validation
- **Flexible Runtime Support**: Support for multiple JavaScript/TypeScript runtime environments

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/project"
    "github.com/compozy/compozy/engine/core"
)

func main() {
    ctx := context.Background()

    // Set up working directory
    cwd, err := core.CWDFromPath("/path/to/project")
    if err != nil {
        log.Fatal(err)
    }

    // Load project configuration
    config, err := project.Load(
        ctx,
        cwd,
        "compozy.yaml",  // Project config file
        ".env",          // Environment file
    )
    if err != nil {
        log.Fatal(err)
    }

    // Access configuration
    log.Printf("Project: %s v%s", config.Name, config.Version)
    log.Printf("Description: %s", config.Description)
    log.Printf("Workflows: %d", len(config.Workflows))
    log.Printf("Models: %d", len(config.Models))

    // Validate configuration
    if err := config.Validate(); err != nil {
        log.Fatal("Configuration validation failed:", err)
    }

    // Access runtime configuration
    runtime := config.Runtime
    log.Printf("Runtime Type: %s", runtime.Type)
    log.Printf("Entrypoint: %s", runtime.Entrypoint)

    log.Println("Project configuration loaded successfully!")
}
```

---

## 📖 Usage

### Library

The project package provides a `Config` struct that represents the complete project configuration:

```go
// Load configuration from file
config, err := project.Load(ctx, cwd, "compozy.yaml", ".env")
if err != nil {
    return err
}

// Access basic project information
fmt.Printf("Project: %s v%s\n", config.Name, config.Version)
fmt.Printf("Author: %s\n", config.Author.Name)

// Access workflows
for _, workflow := range config.Workflows {
    fmt.Printf("Workflow: %s\n", workflow.Source)
}

// Access model configurations
for _, model := range config.Models {
    fmt.Printf("Model: %s/%s\n", model.Provider, model.Model)
}
```

### Configuration Loading

Load and prepare configuration with environment integration:

```go
// Load with custom environment file
config, err := project.Load(ctx, cwd, "compozy.yaml", ".env.production")
if err != nil {
    return err
}

// Clone configuration for modifications
clonedConfig, err := config.Clone()
if err != nil {
    return err
}

// Merge configurations
err = config.Merge(otherConfig)
if err != nil {
    return err
}

// Convert to/from map for serialization
configMap, err := config.AsMap()
if err != nil {
    return err
}

err = config.FromMap(configMap)
if err != nil {
    return err
}
```

### Validation

Comprehensive validation of project configuration:

```go
// Validate entire configuration
if err := config.Validate(); err != nil {
    log.Printf("Validation failed: %v", err)
    return err
}

// Validate specific components
if config.Runtime.Type != "" {
    if err := validateRuntimeType(config.Runtime.Type); err != nil {
        return err
    }
}

// Validate workflows
validator := project.NewWorkflowsValidator(config.Workflows)
if err := validator.Validate(); err != nil {
    return err
}
```

### Environment Integration

Environment variables and template processing:

```go
// Access environment variables
env := config.GetEnv()
apiKey := env["OPENAI_API_KEY"]

// Set environment variables
config.SetEnv(core.EnvMap{
    "OPENAI_API_KEY": "sk-...",
    "DATABASE_URL":   "postgres://...",
})

// Template resolution happens automatically during loading
// Environment variables are available as {{ .env.VARIABLE_NAME }}
```

---

## 🔧 Configuration

Project configuration is defined in `compozy.yaml`:

### Basic Configuration

```yaml
# compozy.yaml
name: my-ai-project
version: 1.0.0
description: "AI-powered customer support system"

author:
  name: "AI Team"
  email: "ai@company.com"
  organization: "ACME Corp"

workflows:
  - source: ./workflows/customer-support.yaml
  - source: ./workflows/data-analysis.yaml

models:
  - provider: openai
    model: gpt-4
    api_key: "{{ .env.OPENAI_API_KEY }}"
    temperature: 0.7

  - provider: anthropic
    model: claude-3-opus
    api_key: "{{ .env.ANTHROPIC_API_KEY }}"
```

### Advanced Configuration

```yaml
# Advanced project configuration
name: enterprise-ai-system
version: 2.1.0
description: "Multi-agent system for enterprise automation"

workflows:
  - source: ./workflows/customer-support.yaml
  - source: ./workflows/data-pipeline.yaml
  - source: ./workflows/content-generation.yaml

models:
  - provider: openai
    model: gpt-4
    api_key: "{{ .env.OPENAI_API_KEY }}"
    temperature: 0.7
    max_tokens: 4000

  - provider: anthropic
    model: claude-3-opus
    api_key: "{{ .env.ANTHROPIC_API_KEY }}"

# Runtime configuration
runtime:
  type: bun
  entrypoint: ./tools.ts
  permissions:
    - --allow-read=/data
    - --allow-net=api.company.com
    - --allow-env=API_KEY,DATABASE_URL

# Schemas for type safety
schemas:
  - id: user-input
    schema:
      type: object
      properties:
        name:
          type: string
          minLength: 1
        email:
          type: string
          format: email
      required: ["name", "email"]

# Performance options
config:
  max_string_length: 52428800 # 50MB
  max_nesting_depth: 20
  async_token_counter_workers: 20
  dispatcher_heartbeat_interval: 30
  dispatcher_heartbeat_ttl: 300

# Auto-loading for development
autoload:
  enabled: true
  strict: true
  include:
    - "agents/**/*.yaml"
    - "memory/**/*.yaml"
    - "workflows/**/*.yaml"
  exclude:
    - "**/*.tmp"
    - "**/*~"

# Caching configuration
cache:
  url: redis://localhost:6379/0
  pool_size: 10
  key_prefix: "compozy:"

# Monitoring and observability
monitoring:
  enabled: true
  metrics:
    provider: prometheus
    endpoint: /metrics
  logging:
    level: info
    format: json
```

### Environment Variables

```bash
# .env
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
REDIS_URL=redis://localhost:6379/0
DATABASE_URL=postgres://user:pass@localhost/db

# Runtime configuration
MAX_STRING_LENGTH=52428800
ASYNC_TOKEN_COUNTER_WORKERS=20
DISPATCHER_HEARTBEAT_INTERVAL=30
```

---

## 🎨 Examples

### Basic Project Setup

```go
func setupBasicProject() {
    ctx := context.Background()

    // Create working directory
    cwd, err := core.CWDFromPath("./my-project")
    if err != nil {
        panic(err)
    }

    // Load configuration
    config, err := project.Load(ctx, cwd, "compozy.yaml", ".env")
    if err != nil {
        panic(err)
    }

    // Validate configuration
    if err := config.Validate(); err != nil {
        panic(err)
    }

    // Access project settings
    fmt.Printf("Project: %s\n", config.Name)
    fmt.Printf("Version: %s\n", config.Version)
    fmt.Printf("Description: %s\n", config.Description)

    // Access workflows
    for _, workflow := range config.Workflows {
        fmt.Printf("Workflow: %s\n", workflow.Source)
    }

    // Access model configurations
    for _, model := range config.Models {
        fmt.Printf("Model: %s/%s\n", model.Provider, model.Model)
    }
}
```

### Runtime Configuration

```go
func setupRuntimeConfig() {
    ctx := context.Background()

    // Load configuration
    config, err := project.Load(ctx, cwd, "compozy.yaml", ".env")
    if err != nil {
        panic(err)
    }

    // Access runtime configuration
    runtime := config.Runtime
    fmt.Printf("Runtime Type: %s\n", runtime.Type)
    fmt.Printf("Entrypoint: %s\n", runtime.Entrypoint)

    // Runtime permissions
    for _, perm := range runtime.Permissions {
        fmt.Printf("Permission: %s\n", perm)
    }

    // Node.js specific options
    if runtime.Type == "node" {
        for _, option := range runtime.NodeOptions {
            fmt.Printf("Node Option: %s\n", option)
        }
    }
}
```

### Configuration Validation

```go
func validateConfiguration() {
    ctx := context.Background()

    // Load configuration
    config, err := project.Load(ctx, cwd, "compozy.yaml", ".env")
    if err != nil {
        panic(err)
    }

    // Validate workflows
    validator := project.NewWorkflowsValidator(config.Workflows)
    if err := validator.Validate(); err != nil {
        fmt.Printf("Workflow validation failed: %v\n", err)
        return
    }

    // Validate complete configuration
    if err := config.Validate(); err != nil {
        fmt.Printf("Configuration validation failed: %v\n", err)
        return
    }

    fmt.Println("Configuration is valid!")
}
```

### Environment Integration

```go
func environmentIntegration() {
    ctx := context.Background()

    // Load with custom environment file
    config, err := project.Load(ctx, cwd, "compozy.yaml", ".env.production")
    if err != nil {
        panic(err)
    }

    // Access environment variables
    env := config.GetEnv()
    fmt.Printf("API Key: %s\n", env["OPENAI_API_KEY"])

    // Add or override environment variables
    config.SetEnv(core.EnvMap{
        "CUSTOM_VAR": "custom_value",
        "API_URL":    "https://api.example.com",
    })

    // Environment variables are automatically resolved in templates
    // Example: api_key: "{{ .env.OPENAI_API_KEY }}"
}
```

### Configuration Cloning and Merging

```go
func configurationManagement() {
    ctx := context.Background()

    // Load base configuration
    config, err := project.Load(ctx, cwd, "compozy.yaml", ".env")
    if err != nil {
        panic(err)
    }

    // Clone for modifications
    clonedConfig, err := config.Clone()
    if err != nil {
        panic(err)
    }

    // Modify cloned config
    clonedConfig.Description = "Modified description"

    // Load override configuration
    overrideConfig, err := project.Load(ctx, cwd, "compozy.override.yaml", ".env")
    if err != nil {
        panic(err)
    }

    // Merge configurations
    err = config.Merge(overrideConfig)
    if err != nil {
        panic(err)
    }

    // Convert to map for serialization
    configMap, err := config.AsMap()
    if err != nil {
        panic(err)
    }

    // Serialize to JSON/YAML
    jsonData, _ := json.Marshal(configMap)
    fmt.Printf("Config as JSON: %s\n", string(jsonData))
}
```

---

## 📚 API Reference

### Config

```go
// Load loads project configuration from file
func Load(ctx context.Context, cwd *core.PathCWD, path string, envFilePath string) (*Config, error)

// Core configuration methods
func (c *Config) Validate() error
func (c *Config) Clone() (*Config, error)
func (c *Config) Merge(other *Config) error
func (c *Config) AsMap() (map[string]any, error)
func (c *Config) FromMap(data any) error

// Environment management
func (c *Config) GetEnv() core.EnvMap
func (c *Config) SetEnv(env core.EnvMap)

// Path and file management
func (c *Config) GetFilePath() string
func (c *Config) SetFilePath(path string)
func (c *Config) GetCWD() *core.PathCWD
func (c *Config) SetCWD(path string) error

// Component information
func (c *Config) Component() core.ConfigType
func (c *Config) LoadID() (string, error)
func (c *Config) HasSchema() bool
```

### Config Structure

```go
type Config struct {
    Name         string                    `json:"name"`
    Version      string                    `json:"version"`
    Description  string                    `json:"description"`
    Author       core.Author               `json:"author"`
    Workflows    []*WorkflowSourceConfig   `json:"workflows"`
    Models       []*core.ProviderConfig    `json:"models"`
    Schemas      []schema.Schema           `json:"schemas"`
    Runtime      RuntimeConfig             `json:"runtime"`
    Opts         Opts                      `json:"config"`
    CacheConfig  *cache.Config             `json:"cache,omitempty"`
    AutoLoad     *autoload.Config          `json:"autoload,omitempty"`
    MonitoringConfig *monitoring.Config    `json:"monitoring,omitempty"`
    CWD          *core.PathCWD             `json:"CWD,omitempty"`
}
```

### Workflow Configuration

```go
type WorkflowSourceConfig struct {
    Source string `json:"source"`
}

// NewWorkflowsValidator creates a workflow validator
func NewWorkflowsValidator(workflows []*WorkflowSourceConfig) *WorkflowsValidator

// Validate workflows
func (v *WorkflowsValidator) Validate() error
```

### Runtime Configuration

```go
type RuntimeConfig struct {
    Type        string   `json:"type"`
    Entrypoint  string   `json:"entrypoint"`
    Permissions []string `json:"permissions"`
    NodeOptions []string `json:"node_options"`
}
```

### Options

```go
type Opts struct {
    MaxStringLength                int `json:"max_string_length"`
    MaxNestingDepth               int `json:"max_nesting_depth"`
    AsyncTokenCounterWorkers      int `json:"async_token_counter_workers"`
    AsyncTokenCounterBufferSize   int `json:"async_token_counter_buffer_size"`
    DispatcherHeartbeatInterval   int `json:"dispatcher_heartbeat_interval"`
    DispatcherHeartbeatTTL        int `json:"dispatcher_heartbeat_ttl"`
    DispatcherStaleThreshold      int `json:"dispatcher_stale_threshold"`
    MaxMessageContentLength       int `json:"max_message_content_length"`
    MaxTotalContentSize           int `json:"max_total_content_size"`
}
```

---

## 🧪 Testing

Run the test suite:

```bash
# Run all project tests
go test ./engine/project/...

# Run specific test
go test -v ./engine/project -run TestConfig_Load

# Run tests with coverage
go test -cover ./engine/project/...

# Run validation tests
go test -v ./engine/project -run TestConfig_Validate
```

### Test Examples

```go
func TestProjectConfiguration(t *testing.T) {
    t.Run("Should load valid configuration", func(t *testing.T) {
        ctx := context.Background()
        cwd, err := core.CWDFromPath("./testdata")
        require.NoError(t, err)

        config, err := project.Load(ctx, cwd, "valid-config.yaml", ".env")
        require.NoError(t, err)

        assert.Equal(t, "test-project", config.Name)
        assert.Equal(t, "1.0.0", config.Version)
        assert.NotEmpty(t, config.Description)
    })

    t.Run("Should validate configuration", func(t *testing.T) {
        config := &project.Config{
            Name:    "test-project",
            Version: "1.0.0",
            Runtime: project.ProjectRuntimeConfig{
                Type:       "bun",
                Entrypoint: "./tools.ts",
            },
        }

        err := config.Validate()
        require.NoError(t, err)
    })
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE)
