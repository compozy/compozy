# `ref` – _Powerful directive system for YAML/JSON configuration processing_

> **A declarative directive system that enables references, transformations, and merging of configuration fragments, designed to reduce duplication and enable reusable, composable configurations.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Basic Reference Resolution](#basic-reference-resolution)
  - [Component Transformation](#component-transformation)
  - [Declarative Merging](#declarative-merging)
  - [Inline Merge](#inline-merge)
  - [Custom Directives](#custom-directives)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `ref` package provides a powerful directive system for YAML/JSON configurations that enables declarative references, transformations, and merging of configuration fragments. It's designed to reduce duplication and enable reusable, composable configurations while keeping files valid YAML.

The package supports three core directives: `$ref` for direct value injection, `$use` for component transformation, and `$merge` for declarative merging. It features automatic inline merging, cycle detection, caching, and extensibility through custom directives.

---

## 💡 Motivation

- **Reduce Duplication**: Eliminate repetitive configuration blocks through references
- **Enable Composition**: Build complex configurations from reusable components
- **Maintain Valid YAML**: Keep configuration files as valid YAML/JSON
- **Flexible Merging**: Declarative merging strategies for different use cases
- **Extensibility**: Custom directives for domain-specific transformations

---

## ⚡ Design Highlights

- **Three Core Directives**: `$ref`, `$use`, and `$merge` for different use cases
- **Inline Merge**: Automatic merging of directive results with sibling keys
- **Cycle Detection**: Prevents infinite recursion with automatic cycle detection
- **Thread-Safe**: Safe for concurrent use across multiple goroutines
- **Extensible**: Register custom directives for domain-specific transformations
- **High Performance**: Optimized merging algorithms and optional caching
- **GJSON Integration**: Powerful path queries using GJSON syntax

---

## 🚀 Getting Started

```go
package main

import (
    "fmt"
    "log"

    "github.com/compozy/compozy/pkg/ref"
)

func main() {
    // Define scopes with reusable values
    localScope := map[string]any{
        "database": map[string]any{
            "host": "localhost",
            "port": 5432,
        },
        "features": map[string]any{
            "auth": true,
            "logging": true,
        },
    }

    // YAML with directives
    yamlDoc := `
app:
  name: my-service
  db:
    $ref: "local::database"
  config:
    $ref: "local::features"
    debug: true
`

    // Process the document
    result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("%+v\n", result)
    // Output: map[app:map[config:map[auth:true debug:true logging:true] db:map[host:localhost port:5432] name:my-service]]
}
```

---

## 📖 Usage

### Basic Reference Resolution

```go
// Simple value reference
localScope := map[string]any{
    "server": map[string]any{
        "host": "localhost",
        "port": 5001,
    },
}

yamlDoc := `
config:
  $ref: "local::server"
`

result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
// Result: map[config:map[host:localhost port:5001]]
```

### Component Transformation

```go
// Transform references into component configurations
localScope := map[string]any{
    "worker": map[string]any{
        "type": "background",
        "replicas": 3,
    },
}

yamlDoc := `
deployment:
  $use: "agent(local::worker)"
`

result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
// Result: map[deployment:map[agent:map[replicas:3 type:background]]]
```

### Declarative Merging

```go
// Merge multiple configurations
localScope := map[string]any{
    "base": map[string]any{
        "host": "localhost",
        "port": 5001,
    },
    "overrides": map[string]any{
        "port": 9090,
        "ssl": true,
    },
}

yamlDoc := `
server:
  $merge:
    - $ref: "local::base"
    - $ref: "local::overrides"
`

result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
// Result: map[server:map[host:localhost port:9090 ssl:true]]
```

### Inline Merge

```go
// Automatic merging with sibling keys
localScope := map[string]any{
    "defaults": map[string]any{
        "host": "localhost",
        "port": 5001,
    },
}

yamlDoc := `
server:
  $ref: "local::defaults"
  port: 9090
  ssl: true
`

result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
// Result: map[server:map[host:localhost port:9090 ssl:true]]
```

### Custom Directives

```go
// Register custom directive
customDirective := ref.Directive{
    Name: "$env",
    Validator: func(node ref.Node) error {
        if _, ok := node.(string); !ok {
            return fmt.Errorf("$env expects a string")
        }
        return nil
    },
    Handler: func(ctx ref.EvaluatorContext, node ref.Node) (ref.Node, error) {
        envVar := node.(string)
        return os.Getenv(envVar), nil
    },
}

err := ref.Register(customDirective)
if err != nil {
    log.Fatal(err)
}

// Use in YAML
yamlDoc := `
database:
  host:
    $env: "DB_HOST"
  port: 5432
`
```

---

## 🔧 Configuration

### Evaluator Options

```go
type EvalConfigOption func(*Evaluator)

// Scope configuration
func WithLocalScope(scope map[string]any) EvalConfigOption
func WithGlobalScope(scope map[string]any) EvalConfigOption
func WithScopes(local, global map[string]any) EvalConfigOption

// Transformation customization
func WithTransformUse(transform TransformUseFunc) EvalConfigOption
func WithPreEval(hook PreEvalFunc) EvalConfigOption

// Caching
func WithCacheEnabled() EvalConfigOption
func WithCache(config CacheConfig) EvalConfigOption

// Resource resolution
func WithResourceResolver(resolver ResourceResolver) EvalConfigOption
```

### Cache Configuration

```go
type CacheConfig struct {
    MaxCost     int64 // Maximum cost of cache (approximately memory in bytes)
    NumCounters int64 // Number of counters for tracking frequency
    BufferItems int64 // Number of keys per Get buffer
}

// Default cache configuration
func DefaultCacheConfig() CacheConfig {
    return CacheConfig{
        MaxCost:     100 << 20, // 100 MB
        NumCounters: 1e7,       // 10 million
        BufferItems: 64,
    }
}
```

### Merge Options

```go
// Inline merge options syntax
"!merge:<strategy>,<key_conflict>"

// Strategies for objects
"deep"     // Deep merge nested objects
"shallow"  // Replace nested objects entirely
"replace"  // Replace entire value

// Strategies for arrays
"concat"   // Concatenate arrays
"prepend"  // Prepend to existing array
"unique"   // Remove duplicates
"append"   // Append to existing array
"union"    // Union of arrays

// Key conflict resolution
"replace"  // Sibling keys override directive result
"first"    // Directive result takes precedence
"error"    // Error on conflicts
```

---

## 🎨 Examples

### Complete Application Configuration

```go
package main

import (
    "fmt"
    "log"

    "github.com/compozy/compozy/pkg/ref"
)

func main() {
    // Define comprehensive scopes
    localScope := map[string]any{
        "defaults": map[string]any{
            "server": map[string]any{
                "host": "0.0.0.0",
                "port": 5001,
                "timeout": "30s",
            },
            "database": map[string]any{
                "host": "localhost",
                "port": 5432,
                "ssl": false,
            },
        },
        "agents": map[string]any{
            "worker": map[string]any{
                "type": "background",
                "replicas": 3,
                "resources": map[string]any{
                    "cpu": "100m",
                    "memory": "512Mi",
                },
            },
        },
    }

    globalScope := map[string]any{
        "env": map[string]any{
            "production": map[string]any{
                "database": map[string]any{
                    "ssl": true,
                    "maxConnections": 100,
                },
                "features": map[string]any{
                    "monitoring": true,
                    "debug": false,
                },
            },
        },
    }

    yamlDoc := `
app:
  name: my-service
  version: "1.0.0"

  # Server configuration with inline merge
  server:
    $ref: "local::defaults.server"
    port: 9090
    ssl: true

  # Database configuration with declarative merge
  database:
    $merge:
      - $ref: "local::defaults.database"
      - $ref: "global::env.production.database"
      - timeout: "10s"

  # Worker deployment with component transformation
  deployment:
    $use: "agent(local::agents.worker)"
    resources:
      cpu: "500m"
      memory: "1Gi"
    scaling:
      enabled: true
      min: 1
      max: 10

  # Feature flags with merge strategies
  features:
    $merge:
      strategy: deep
      key_conflict: replace
      sources:
        - $ref: "global::env.production.features"
        - analytics: true
        - logging: true
`

    // Process with caching for performance
    ev := ref.NewEvaluator(
        ref.WithLocalScope(localScope),
        ref.WithGlobalScope(globalScope),
        ref.WithCacheEnabled(),
    )

    result, err := ref.ProcessBytesWithEvaluator([]byte(yamlDoc), ev)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("%+v\n", result)
}
```

### Array Merging Strategies

```go
localScope := map[string]any{
    "base_tags": []string{"web", "api"},
    "env_tags": []string{"production", "api"},
    "additional_tags": []string{"monitoring"},
}

yamlDoc := `
# Concatenate arrays (default)
all_tags:
  $merge:
    - $ref: "local::base_tags"
    - $ref: "local::env_tags"
    - $ref: "local::additional_tags"

# Unique values only
unique_tags:
  $merge:
    strategy: unique
    sources:
      - $ref: "local::base_tags"
      - $ref: "local::env_tags"
      - $ref: "local::additional_tags"

# Prepend arrays
priority_tags:
  $merge:
    strategy: prepend
    sources:
      - ["critical"]
      - $ref: "local::base_tags"
`

result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
// Results:
// all_tags: [web api production api monitoring]
// unique_tags: [web api production monitoring]
// priority_tags: [critical web api]
```

### Performance Optimization with Caching

```go
// For processing multiple documents efficiently
ev := ref.NewEvaluator(
    ref.WithLocalScope(localScope),
    ref.WithGlobalScope(globalScope),
    ref.WithCacheEnabled(),
)

// Process multiple configuration files
configFiles := []string{"app.yaml", "services.yaml", "deployment.yaml"}
for _, file := range configFiles {
    result, err := ref.ProcessFileWithEvaluator(file, ev)
    if err != nil {
        log.Printf("Error processing %s: %v", file, err)
        continue
    }
    // Process result...
}
```

---

## 📚 API Reference

### Core Types

```go
type Node any
type Evaluator struct { ... }
type EvaluatorContext interface { ... }
type TransformUseFunc func(component string, config Node) (key string, value Node, err error)
type PreEvalFunc func(node Node) (Node, error)
type ResourceResolver interface { ... }
```

### Processing Functions

```go
// Process from various sources
func ProcessBytes(data []byte, options ...EvalConfigOption) (Node, error)
func ProcessReader(reader io.Reader, options ...EvalConfigOption) (Node, error)
func ProcessFile(filename string, options ...EvalConfigOption) (Node, error)

// Process with existing evaluator
func ProcessBytesWithEvaluator(data []byte, ev *Evaluator) (Node, error)
func ProcessFileWithEvaluator(filename string, ev *Evaluator) (Node, error)
```

### Evaluator Creation

```go
func NewEvaluator(options ...EvalConfigOption) *Evaluator
func (ev *Evaluator) Eval(node Node) (Node, error)
```

### Directive Registration

```go
type Directive struct {
    Name      string
    Validator func(node Node) error
    Handler   func(ctx EvaluatorContext, node Node) (Node, error)
}

func Register(directive Directive) error
```

### Configuration Options

```go
func WithLocalScope(scope map[string]any) EvalConfigOption
func WithGlobalScope(scope map[string]any) EvalConfigOption
func WithScopes(local, global map[string]any) EvalConfigOption
func WithTransformUse(transform TransformUseFunc) EvalConfigOption
func WithPreEval(hook PreEvalFunc) EvalConfigOption
func WithResourceResolver(resolver ResourceResolver) EvalConfigOption
func WithCacheEnabled() EvalConfigOption
func WithCache(config CacheConfig) EvalConfigOption
```

### Path Resolution

Uses [GJSON](https://github.com/tidwall/gjson) syntax for powerful path queries:

```go
// Basic paths
"config.server.port"     // Nested object access
"users.0.name"           // Array index access
"config.ports.#"         // Array length
"users.#.name"           // Map array to property values

// Advanced queries
"users.#(age>30).name"   // Conditional queries
"config.*.enabled"       // Wildcard matching
"data.@reverse"          // Modifiers
```

---

## 🧪 Testing

```go
func TestBasicReference(t *testing.T) {
    scope := map[string]any{
        "test": map[string]any{
            "value": "hello",
        },
    }

    yamlDoc := `
result:
  $ref: "local::test.value"
`

    result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(scope))
    require.NoError(t, err)

    expected := map[string]any{
        "result": "hello",
    }
    require.Equal(t, expected, result)
}

func TestInlineMerge(t *testing.T) {
    scope := map[string]any{
        "base": map[string]any{
            "host": "localhost",
            "port": 5001,
        },
    }

    yamlDoc := `
server:
  $ref: "local::base"
  port: 9090
  ssl: true
`

    result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(scope))
    require.NoError(t, err)

    server := result.(map[string]any)["server"].(map[string]any)
    require.Equal(t, "localhost", server["host"])
    require.Equal(t, 9090, server["port"]) // Overridden
    require.Equal(t, true, server["ssl"])   // Added
}
```

### Benchmark Tests

```go
func BenchmarkEvaluatorWithCache(b *testing.B) {
    scope := map[string]any{
        "config": map[string]any{
            "server": map[string]any{
                "host": "localhost",
                "port": 5001,
            },
        },
    }

    yamlDoc := `
app:
  server:
    $ref: "local::config.server"
`

    ev := ref.NewEvaluator(
        ref.WithLocalScope(scope),
        ref.WithCacheEnabled(),
    )

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := ref.ProcessBytesWithEvaluator([]byte(yamlDoc), ev)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
