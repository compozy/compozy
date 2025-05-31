# Reference Resolution Package (`pkg/ref`)

The `ref` package provides powerful reference resolution capabilities for YAML and JSON documents with support for property references, file references, global references, and advanced caching.

## Features

### 🎯 Reference Types
- **Property References**: Reference values within the same document
- **File References**: Reference values in external files (local or remote)
- **Global References**: Reference values in global configuration files

### 🚀 Performance
- **LRU Caching**: Configurable document and path caching for optimal performance
- **Parallel Processing**: Automatic parallel resolution of large data structures
- **Cycle Detection**: Robust circular reference detection across all resolution paths

### 🔄 Merge Modes
- **Merge**: Deep merge referenced and inline values (default)
- **Replace**: Replace inline values with referenced values
- **Append**: Append referenced arrays to inline arrays

## Basic Usage

### Simple Property Reference

```go
package main

import (
    "context"
    "fmt"
    "github.com/compozy/compozy/pkg/ref"
)

func main() {
    data := map[string]any{
        "database": map[string]any{
            "host": "localhost",
            "port": 5432,
        },
        "app": map[string]any{
            "db_config": map[string]any{
                "$ref": "database",
                "ssl": true, // This will be merged with the referenced database config
            },
        },
    }

    // Resolve all references in the document
    resolver := &ref.WithRef{}
    resolver.SetRefMetadata("/path/to/file.yaml", "/project/root")
    
    ctx := context.Background()
    resolved, err := resolver.ResolveMapReference(ctx, data, data)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Resolved config: %+v\n", resolved)
}
```

### File Reference

```yaml
# config.yaml
services:
  api:
    $ref: ./services/api.yaml::config
    port: 8080  # Merged with referenced config
```

```go
ref := &ref.Ref{
    Type: ref.TypeFile,
    File: "./services/api.yaml",
    Path: "config",
    Mode: ref.ModeMerge,
}

result, err := ref.Resolve(ctx, currentDoc, "/path/to/config.yaml", "/project/root")
```

### Struct Integration

```go
type Config struct {
    ref.WithRef
    DatabaseRef any    `json:"database_ref" yaml:"database_ref" is_ref:"true"`
    Name        string `json:"name" yaml:"name"`
    Port        int    `json:"port" yaml:"port"`
}

var config Config
config.SetRefMetadata("/path/to/config.yaml", "/project/root")

// Parse YAML/JSON into config...
yaml.Unmarshal(data, &config)

// Resolve all reference fields
ctx := context.Background()
err := config.ResolveReferences(ctx, &config, currentDoc)
```

## Configuration

### Environment Variables

The package supports configuration via environment variables:

```bash
# Document cache size (default: 256)
export COMPOZY_REF_CACHE_SIZE=512

# Path cache size (default: 512)  
export COMPOZY_REF_PATH_CACHE_SIZE=1024

# Disable path cache (default: false)
export COMPOZY_REF_DISABLE_PATH_CACHE=true
```

### Programmatic Configuration

```go
import "github.com/compozy/compozy/pkg/ref"

// Configure caches before first use
ref.SetCacheConfig(&ref.CacheConfig{
    DocCacheSize:    512,    // LRU cache size for documents
    PathCacheSize:   1024,   // LRU cache size for GJSON paths
    EnablePathCache: true,   // Enable/disable path caching
})
```

### Cache Benefits

The caching system provides significant performance improvements:

- **Document Cache**: Avoids re-parsing files and remote URLs
- **Path Cache**: Speeds up repeated GJSON path lookups
- **LRU Eviction**: Automatically manages memory usage
- **Thread Safety**: Safe for concurrent use

Typical performance improvements:
- **20-25% faster** resolution for large nested configurations
- **5-10% additional speedup** with path caching enabled
- **Reduced memory allocations** through caching

## Reference Syntax

### Property References

```yaml
# Simple property reference
config:
  $ref: database.host

# Array filter reference  
user_schema:
  $ref: schemas.#(id=="user")

# Nested property reference
api_config:
  $ref: services.api.config.production
```

### File References

```yaml
# Local file reference
external_config:
  $ref: ./config/database.yaml::production

# Remote file reference
remote_schema:
  $ref: https://api.example.com/schema.yaml::user_schema

# File reference with merge mode
service_config:
  $ref: ./services/api.yaml::config!merge
  custom_setting: true
```

### Global References

```yaml
# Reference to global compozy.yaml
provider:
  $ref: $global::providers.#(id=="openai")

# Global reference with merge
llm_config:
  $ref: $global::providers.#(id=="openai")!merge
  temperature: 0.8
```

### Merge Modes

```yaml
# Replace mode - ignore inline values
config:
  $ref: base_config!replace
  ignored: true

# Merge mode (default) - deep merge
config:
  $ref: base_config!merge
  additional: true

# Append mode - for arrays
items:
  $ref: base_items!append
```

## Advanced Features

### Parallel Processing

The package automatically uses parallel processing for large data structures:

```go
// Automatically parallel for maps/arrays with 4+ elements on multi-core systems
data := map[string]any{
    "item1": map[string]any{"$ref": "template"},
    "item2": map[string]any{"$ref": "template"}, 
    "item3": map[string]any{"$ref": "template"},
    "item4": map[string]any{"$ref": "template"},
    // ... more items
}
```

### Cycle Detection

Robust circular reference detection across all resolution paths:

```yaml
# This will be detected and return an error
circular_a:
  $ref: circular_b

circular_b:  
  $ref: circular_a
```

### Security

Built-in security features:

- **Path Validation**: Prevents directory traversal attacks
- **URL Validation**: Blocks private networks and localhost
- **Size Limits**: Configurable limits on file and response sizes
- **Timeout Protection**: Automatic timeouts for remote requests

## Error Handling

The package provides detailed error messages with context:

```go
_, err := ref.Resolve(ctx, data, filePath, projectRoot)
if err != nil {
    if refErr, ok := err.(*ref.Error); ok {
        fmt.Printf("Reference error at %s:%d:%d: %s\n", 
            refErr.FilePath, refErr.Line, refErr.Column, refErr.Message)
    }
}
```

## Performance Tips

1. **Enable Caching**: Use appropriate cache sizes for your workload
2. **Batch Operations**: Resolve multiple references in single operations when possible
3. **Monitor Memory**: Tune cache sizes based on your application's memory constraints
4. **Use Benchmarks**: Run `go test -bench=.` to measure performance

## Benchmarks

Run performance benchmarks:

```bash
# Run all benchmarks
go test -bench=. ./pkg/ref

# Run specific benchmarks
go test -bench=BenchmarkResolve_LargeMap ./pkg/ref
go test -bench=BenchmarkCache_DocumentLoad ./pkg/ref

# Include memory allocation stats
go test -bench=. -benchmem ./pkg/ref
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/ref

# Run with race detection
go test -race ./pkg/ref

# Run specific test suites
go test -run TestCache ./pkg/ref
go test -run TestConcurrency ./pkg/ref
```

## API Reference

### Core Types

```go
type Ref struct {
    Type Type   // TypeProperty, TypeFile, TypeGlobal
    Path string // GJSON path for resolution
    Mode Mode   // ModeMerge, ModeReplace, ModeAppend  
    File string // File path for TypeFile references
}

type WithRef struct {
    // Embed in structs to add reference resolution capabilities
}

type CacheConfig struct {
    DocCacheSize    int  // Document cache size
    PathCacheSize   int  // Path cache size  
    EnablePathCache bool // Enable path caching
}
```

### Key Functions

```go
// Resolve a single reference
func (r *Ref) Resolve(ctx context.Context, currentDoc any, filePath, projectRoot string) (any, error)

// Configure caching
func SetCacheConfig(config *CacheConfig)

// Resolve references in structs
func (w *WithRef) ResolveReferences(ctx context.Context, target any, currentDoc any) error

// Resolve references in maps
func (w *WithRef) ResolveMapReference(ctx context.Context, data map[string]any, currentDoc any) (map[string]any, error)
```

## Migration Guide

### From Previous Versions

The caching improvements are backward compatible. Existing code will continue to work without changes, but you can now optimize performance by configuring caches:

```go
// Old code (still works)
ref := &ref.Ref{Type: ref.TypeProperty, Path: "config.database"}
result, err := ref.Resolve(ctx, doc, filePath, projectRoot)

// New code (with caching optimization)  
ref.SetCacheConfig(&ref.CacheConfig{DocCacheSize: 512})
result, err := ref.Resolve(ctx, doc, filePath, projectRoot) // Now uses cache
``` 