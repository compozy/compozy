# `autoload` – _Automated Configuration Discovery and Loading_

> **Provides automatic discovery, loading, and registration of configuration files across the Compozy project workspace.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Configuration Format](#configuration-format)
  - [Registry Operations](#registry-operations)
  - [Error Handling](#error-handling)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `autoload` package provides automatic discovery, loading, and registration of configuration files throughout a Compozy project. It scans the project directory for YAML/JSON configuration files, validates them, and makes them available through a centralized registry.

This package handles:

- File discovery using glob patterns
- Configuration parsing and validation
- Centralized registry management
- Error categorization and reporting
- Publishes discovered resources to a ResourceStore for ID-based linking
- Security validation (path traversal protection)

---

## 💡 Motivation

- **Zero-Configuration Discovery**: Automatically find and load configurations without manual registration
- **Centralized Management**: Provide a single registry for all project configurations
- **Development Efficiency**: Eliminate the need to manually track and load configuration files
- **Error Resilience**: Gracefully handle invalid configurations with detailed error reporting
- **Security**: Prevent path traversal attacks and ensure configurations stay within project boundaries

---

## ⚡ Design Highlights

- **Glob-Based Discovery**: Flexible file discovery using configurable include/exclude patterns
- **Type-Safe Registry**: Centralized registry with type-safe configuration retrieval
- **Error Categorization**: Comprehensive error classification (parse, validation, security, duplicate)
- **Strict/Non-Strict Modes**: Choose between failing fast or collecting all errors
- **Resource Linking**: ID-based selectors compiled via ResourceStore
- **Thread-Safe Operations**: Concurrent-safe registry operations with proper locking
- **Path Security**: Built-in protection against path traversal vulnerabilities

---

## 🚀 Getting Started

```go
import (
    "context"
    "github.com/compozy/compozy/engine/autoload"
)

// Create autoload configuration
config := &autoload.Config{
    Enabled: true,
    Strict:  true,
    Include: []string{
        "agents/**/*.yaml",
        "tools/**/*.yaml",
        "workflows/**/*.yaml",
    },
    Exclude: []string{
        "**/*.test.yaml",
        "**/tmp/**",
    },
}

// Create autoloader
registry := autoload.NewConfigRegistry()
loader := autoload.New("/path/to/project", config, registry)

// Load all configurations
ctx := context.Background()
if err := loader.Load(ctx); err != nil {
    log.Fatal(err)
}

// Access loaded configurations
agentConfig, err := registry.Get("agent", "my-agent")
if err != nil {
    log.Fatal(err)
}
```

---

## 📖 Usage

### Library

#### Basic Autoloading

```go
// Create a new autoloader
config := autoload.NewConfig()
config.Enabled = true
config.Include = []string{"**/*.yaml"}
config.Strict = false

registry := autoload.NewConfigRegistry()
loader := autoload.New(projectRoot, config, registry)

// Load configurations
ctx := context.Background()
result, err := loader.LoadWithResult(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Loaded %d configurations from %d files\n",
    result.ConfigsLoaded, result.FilesProcessed)
```

#### Discovery Without Loading

```go
// Discover files without loading them
files, err := loader.Discover(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d configuration files\n", len(files))
for _, file := range files {
    fmt.Printf("- %s\n", file)
}
```

### Configuration Format

#### Project Configuration (compozy.yaml)

```yaml
# Project configuration
name: "my-project"
version: "1.0.0"

# Autoload configuration
autoload:
  enabled: true
  strict: true
  include:
    - "agents/**/*.yaml"
    - "tools/**/*.yaml"
    - "workflows/**/*.yaml"
    - "mcps/**/*.yaml"
  exclude:
    - "**/*.test.yaml"
    - "**/*.bak"
    - "**/tmp/**"
    - "**/.git/**"
  watch_enabled: true
```

#### Resource Configuration Format

All discoverable configurations must include `resource` and `id` fields:

```yaml
# Agent configuration
resource: "agent"
id: "code-assistant"
config:
  provider: "openai"
  model: "gpt-4"
instructions: "You are a helpful coding assistant..."

---
# Tool configuration
resource: "tool"
id: "file-reader"
runtime: "bun"
code: |
  // Tool implementation
```

### Registry Operations

#### Retrieving Configurations

```go
// Get specific configuration
config, err := registry.Get("agent", "code-assistant")
if err != nil {
    log.Fatal(err)
}

// Get all configurations of a type
agents := registry.GetAll("agent")
fmt.Printf("Found %d agents\n", len(agents))

// Count configurations
totalConfigs := registry.Count()
agentCount := registry.CountByType("agent")
```

#### Manual Registration

```go
// Register configuration manually
agentConfig := &agent.Config{
    Resource: "agent",
    ID:       "manual-agent",
    // ... other fields
}

err := registry.Register(agentConfig, "manual")
if err != nil {
    log.Fatal(err)
}
```

### Error Handling

#### Detailed Error Reporting

```go
result, err := loader.LoadWithResult(ctx)
if err != nil {
    log.Fatal(err)
}

// Check for loading errors
if len(result.Errors) > 0 {
    fmt.Printf("Encountered %d errors:\n", len(result.Errors))

    for _, loadErr := range result.Errors {
        fmt.Printf("File: %s\n", loadErr.File)
        fmt.Printf("Error: %s\n", loadErr.Error)
    }

    // Print error summary
    summary := result.ErrorSummary
    fmt.Printf("Error Summary:\n")
    fmt.Printf("- Parse errors: %d\n", summary.ParseErrors)
    fmt.Printf("- Validation errors: %d\n", summary.ValidationErrors)
    fmt.Printf("- Duplicate errors: %d\n", summary.DuplicateErrors)
    fmt.Printf("- Security errors: %d\n", summary.SecurityErrors)
}
```

#### Validation Mode

```go
// Validate all configurations without loading
result, err := loader.Validate(ctx)
if err != nil {
    log.Fatal(err)
}

if len(result.Errors) > 0 {
    fmt.Printf("Validation failed with %d errors\n", len(result.Errors))
    // Handle validation errors
}
```

---

## 🔧 Configuration

### Autoload Configuration Fields

| Field           | Type     | Required | Description                                               |
| --------------- | -------- | -------- | --------------------------------------------------------- |
| `enabled`       | bool     | No       | Enable/disable autoloading (default: false)               |
| `strict`        | bool     | No       | Fail on first error vs collect all errors (default: true) |
| `include`       | []string | Yes\*    | Glob patterns for files to include                        |
| `exclude`       | []string | No       | Glob patterns for files to exclude                        |
| `watch_enabled` | bool     | No       | Enable file watching for changes                          |

\*Required when `enabled` is true

### Default Exclude Patterns

The autoloader automatically excludes common temporary/backup files:

```go
var DefaultExcludes = []string{
    "**/.#*",   // Emacs lock files
    "**/*~",    // Backup files
    "**/*.bak", // Backup files
    "**/*.swp", // Vim swap files
    "**/*.tmp", // Temporary files
    "**/._*",   // macOS resource forks
}
```

### Registry Configuration

The `ConfigRegistry` automatically handles:

- Case-insensitive resource types and IDs
- Duplicate detection and prevention
- Thread-safe concurrent access
- Type-based configuration retrieval

---

## 🎨 Examples

### Basic Project Setup

```go
package main

import (
    "context"
    "log"
    "github.com/compozy/compozy/engine/autoload"
)

func main() {
    // Configure autoloading
    config := &autoload.Config{
        Enabled: true,
        Strict:  false, // Continue on errors
        Include: []string{
            "agents/**/*.yaml",
            "tools/**/*.yaml",
            "workflows/**/*.yaml",
        },
        Exclude: []string{
            "**/*.test.yaml",
            "**/examples/**",
        },
    }

    // Create autoloader
    registry := autoload.NewConfigRegistry()
    loader := autoload.New("/path/to/project", config, registry)

    // Load all configurations
    ctx := context.Background()
    if err := loader.Load(ctx); err != nil {
        log.Fatal(err)
    }

    // Print statistics
    stats := loader.Stats()
    log.Printf("Loaded configurations: %+v", stats)

    // Access specific configuration
    agent, err := registry.Get("agent", "code-assistant")
    if err != nil {
        log.Printf("Agent not found: %v", err)
    } else {
        log.Printf("Found agent: %+v", agent)
    }
}
```

### Publishing to a Resource Store

During application startup, publish autoloaded items to a ResourceStore so workflows can resolve IDs during compile/link:

```go
import (
    "context"
    "github.com/compozy/compozy/engine/resources"
)

ctx := context.Background()
store := resources.NewMemoryResourceStore()

// projectName should match your loaded project config
if err := registry.SyncToResourceStore(ctx, projectName, store); err != nil {
    log.Fatalf("failed to publish resources: %v", err)
}
```

### Resource Linking

```go
// Autoload + ResourceStore: use ID-based selectors at compile time
// Example task selector (YAML):
// tasks:
//   - id: analyze
//     type: basic
//     agent: research-agent   # resolved by ResourceStore during compile
```

### Advanced Error Handling

```go
// Load with comprehensive error reporting
result, err := loader.LoadWithResult(ctx)
if err != nil {
    log.Fatalf("Critical error: %v", err)
}

// Process results
log.Printf("Processing completed:")
log.Printf("- Files processed: %d", result.FilesProcessed)
log.Printf("- Configurations loaded: %d", result.ConfigsLoaded)
log.Printf("- Errors encountered: %d", len(result.Errors))

// Handle different error types
if len(result.Errors) > 0 {
    summary := result.ErrorSummary

    if summary.SecurityErrors > 0 {
        log.Printf("⚠️  Security errors detected: %d", summary.SecurityErrors)
    }

    if summary.ParseErrors > 0 {
        log.Printf("📝 Parse errors: %d", summary.ParseErrors)
    }

    if summary.ValidationErrors > 0 {
        log.Printf("✅ Validation errors: %d", summary.ValidationErrors)
    }

    if summary.DuplicateErrors > 0 {
        log.Printf("🔄 Duplicate configuration errors: %d", summary.DuplicateErrors)
    }

    // Print detailed error information
    for _, loadErr := range result.Errors {
        log.Printf("Error in %s: %v", loadErr.File, loadErr.Error)
    }
}
```

### Custom Configuration Discovery

```go
// Custom discovery patterns
config := &autoload.Config{
    Enabled: true,
    Strict:  true,
    Include: []string{
        "configs/**/*.yaml",
        "definitions/**/*.json",
        "*.config.yaml",
    },
    Exclude: []string{
        "**/node_modules/**",
        "**/.git/**",
        "**/*.test.*",
        "**/.env*",
    },
}

// Discovery with custom patterns
loader := autoload.New(projectRoot, config, registry)
files, err := loader.Discover(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Discovered %d configuration files:", len(files))
for _, file := range files {
    log.Printf("  - %s", file)
}
```

---

## 📚 API Reference

### Core Types

#### `AutoLoader`

Main autoloader orchestrator.

```go
type AutoLoader struct {
    // private fields
}

func New(projectRoot string, config *Config, registry *ConfigRegistry) *AutoLoader
```

#### `Config`

Autoload configuration.

```go
type Config struct {
    Enabled      bool     `json:"enabled"`
    Strict       bool     `json:"strict"`
    Include      []string `json:"include"`
    Exclude      []string `json:"exclude"`
    WatchEnabled bool     `json:"watch_enabled,omitempty"`
}
```

#### `ConfigRegistry`

Thread-safe configuration registry.

```go
type ConfigRegistry struct {
    // private fields
}

func NewConfigRegistry() *ConfigRegistry
```

#### `LoadResult`

Detailed loading results.

```go
type LoadResult struct {
    FilesProcessed int
    ConfigsLoaded  int
    Errors         []LoadError
    ErrorSummary   *ErrorSummary
}
```

### Key Methods

#### AutoLoader Methods

```go
func (al *AutoLoader) Load(ctx context.Context) error
func (al *AutoLoader) LoadWithResult(ctx context.Context) (*LoadResult, error)
func (al *AutoLoader) Discover(ctx context.Context) ([]string, error)
func (al *AutoLoader) Validate(ctx context.Context) (*LoadResult, error)
func (al *AutoLoader) Stats() map[string]int
```

#### Registry Methods

```go
func (r *ConfigRegistry) Register(config any, source string) error
func (r *ConfigRegistry) Get(resourceType, id string) (any, error)
func (r *ConfigRegistry) GetAll(resourceType string) []any
func (r *ConfigRegistry) Count() int
func (r *ConfigRegistry) CountByType(resourceType string) int
func (r *ConfigRegistry) Clear()
```

#### Configuration Methods

```go
func (c *Config) Validate() error
func (c *Config) SetDefaults()
func (c *Config) GetAllExcludes() []string
```

---

## 🧪 Testing

### Running Tests

```bash
# Run all autoload package tests
go test -v ./engine/autoload

# Run specific test
go test -v ./engine/autoload -run TestAutoLoader_Load

# Run tests with coverage
go test -v ./engine/autoload -cover

# Run integration tests
go test -v ./engine/autoload -tags=integration
```

### Test Structure

The package includes comprehensive tests for:

- Configuration discovery and loading
- Registry operations and thread safety
- Error handling and categorization
- Path security validation
- Resource resolution integration

### Example Test

```go
func TestAutoLoader_Load(t *testing.T) {
    // Create test configuration
    config := &autoload.Config{
        Enabled: true,
        Strict:  true,
        Include: []string{"**/*.yaml"},
    }

    // Create autoloader
    registry := autoload.NewConfigRegistry()
    loader := autoload.New(testProjectRoot, config, registry)

    // Test loading
    ctx := context.Background()
    err := loader.Load(ctx)
    assert.NoError(t, err)

    // Verify configurations were loaded
    assert.Greater(t, registry.Count(), 0)
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
