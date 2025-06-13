# Auto-Load Configuration System - Architecture Design

## Executive Summary

This document outlines a pragmatic, simple architecture for implementing the Auto-Load Configuration System in Compozy. The design prioritizes simplicity, reusability of existing code, and correctness while avoiding over-engineering.

### Key Design Decisions

1. **Single-pass loading with lazy resource resolution** - Avoids ordering complexity
2. **Reuse existing `core.LoadConfig()` and `pkg/ref` infrastructure** - Minimal new code
3. **Fail-fast on conflicts with optional non-strict mode** - Clear error handling
4. **Simple file-based discovery with security sandboxing** - Safe and predictable

## Architecture Overview

### System Components

```
┌─────────────────────┐
│   compozy.yaml      │
│  (autoload config)  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐     ┌─────────────────────┐
│   AutoLoader        │────▶│  FileDiscoverer     │
│  (orchestrator)     │     │  (glob matching)    │
└──────────┬──────────┘     └─────────────────────┘
           │
           ▼
┌─────────────────────┐     ┌─────────────────────┐
│  core.LoadConfig()  │────▶│   pkg/ref           │
│  (existing loader)  │     │  (lazy resolution)  │
└──────────┬──────────┘     └─────────────────────┘
           │
           ▼
┌─────────────────────┐
│  Config Registry    │
│  (in-memory cache)  │
└─────────────────────┘
```

### Core Design Principles

1. **Leverage Existing Infrastructure**: Use `core.LoadConfig()` for all file loading
2. **Lazy Resolution**: Resource references are resolved on-demand, not during loading
3. **Simple Discovery**: Basic glob matching with security constraints
4. **Minimal Abstractions**: Only create interfaces where testing requires it

## Component Design

### 1. AutoLoader (Main Orchestrator)

```go
package autoload

import (
    "context"
    "path/filepath"
    "github.com/compozy/compozy/core"
    "github.com/compozy/compozy/pkg/logger"
)

type AutoLoader struct {
    projectRoot string
    config      AutoLoadConfig
    registry    *ConfigRegistry
    discoverer  FileDiscoverer
}

func New(projectRoot string, config AutoLoadConfig, registry *ConfigRegistry) *AutoLoader {
    if registry == nil {
        registry = NewConfigRegistry()
    }
    return &AutoLoader{
        projectRoot: projectRoot,
        config:      config,
        registry:    registry,
        discoverer:  &fsDiscoverer{root: projectRoot},
    }
}

func (al *AutoLoader) Load(ctx context.Context) error {
    // 1. Discover files
    files, err := al.discoverer.Discover(al.config.Include, al.config.Exclude)
    if err != nil {
        return core.NewError(err, "AUTOLOAD_DISCOVERY_FAILED", nil)
    }

    // 2. Load each file
    for _, file := range files {
        config, err := core.LoadConfig(file)
        if err != nil {
            if al.config.Strict {
                return core.NewError(err, "AUTOLOAD_FILE_FAILED", map[string]any{
                    "file": file,
                })
            }
            logger.Warn("Skipping invalid config file", "file", file, "error", err)
            continue
        }

        // 3. Register config with "autoload" source
        if err := al.registry.Register(config, "autoload"); err != nil {
            if al.config.Strict {
                return err
            }
            logger.Warn("Config registration failed", "file", file, "error", err)
        }
    }

    // 4. Install lazy resolver for resource:: scope
    al.installResourceResolver()

    return nil
}
```

### 2. File Discovery

```go
type FileDiscoverer interface {
    Discover(includes, excludes []string) ([]string, error)
}

type fsDiscoverer struct {
    root string
}

// Default exclude patterns for common temporary/backup files
var defaultExcludes = []string{
    "**/.#*",     // Emacs lock files
    "**/*~",      // Backup files
    "**/*.bak",   // Backup files
    "**/*.swp",   // Vim swap files
    "**/*.tmp",   // Temporary files
    "**/._*",     // macOS resource forks
}

func (d *fsDiscoverer) Discover(includes, excludes []string) ([]string, error) {
    var files []string

    for _, pattern := range includes {
        // Security: Ensure pattern is relative and doesn't escape root
        cleanPattern := filepath.Clean(pattern)
        if filepath.IsAbs(cleanPattern) || strings.Contains(cleanPattern, "..") {
            return nil, core.NewError(nil, "INVALID_PATTERN", map[string]any{
                "pattern": pattern,
            })
        }

        // Note: filepath.Glob does not follow symbolic links, preventing infinite loops
        matches, err := filepath.Glob(filepath.Join(d.root, cleanPattern))
        if err != nil {
            return nil, err
        }

        files = append(files, matches...)
    }

    // Apply default excludes first, then user excludes
    allExcludes := append(defaultExcludes, excludes...)
    files = d.applyExcludes(files, allExcludes)

    // Security: Validate all paths are within project root
    for _, file := range files {
        if !strings.HasPrefix(file, d.root) {
            return nil, core.NewError(nil, "PATH_ESCAPE_ATTEMPT", map[string]any{
                "file": file,
            })
        }
    }

    return files, nil
}

// Expose Discover method for CLI usage
func (al *AutoLoader) Discover(ctx context.Context) ([]string, error) {
    return al.discoverer.Discover(al.config.Include, al.config.Exclude)
}
```

### 3. Config Registry

```go
type ConfigRegistry struct {
    mu       sync.RWMutex
    configs  map[string]map[string]*configEntry // type -> id -> entry
}

type configEntry struct {
    config any
    source string // "manual" or "autoload"
}

func NewConfigRegistry() *ConfigRegistry {
    return &ConfigRegistry{
        configs: make(map[string]map[string]*configEntry),
    }
}

func (r *ConfigRegistry) Register(config any, source string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Extract resource type and ID
    resourceType, id, err := extractResourceInfo(config)
    if err != nil {
        return err
    }

    // Check for duplicates
    if r.configs[resourceType] == nil {
        r.configs[resourceType] = make(map[string]*configEntry)
    }

    if existing, exists := r.configs[resourceType][id]; exists {
        return core.NewError(nil, "DUPLICATE_CONFIG", map[string]any{
            "type":            resourceType,
            "id":              id,
            "source":          source,
            "existing_source": existing.source,
        })
    }

    // Register
    r.configs[resourceType][id] = &configEntry{
        config: config,
        source: source,
    }

    return nil
}

func (r *ConfigRegistry) Get(resourceType, id string) (any, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if configs, ok := r.configs[resourceType]; ok {
        if entry, ok := configs[id]; ok {
            return entry.config, nil
        }
    }

    return nil, core.NewError(nil, "RESOURCE_NOT_FOUND", map[string]any{
        "type": resourceType,
        "id":   id,
    })
}
```

### 4. Lazy Resource Resolution with Cycle Detection

```go
// Extension to pkg/ref evaluator
func (al *AutoLoader) installResourceResolver() {
    ref.RegisterScope("resource", func(path string, ctx ...any) (any, error) {
        // Extract or create resolution context for cycle detection
        var resolutionCtx *resolutionContext
        if len(ctx) > 0 {
            if rc, ok := ctx[0].(*resolutionContext); ok {
                resolutionCtx = rc
            }
        }
        if resolutionCtx == nil {
            resolutionCtx = &resolutionContext{
                visited: make(map[string]bool),
                depth:   0,
            }
        }

        // Check recursion depth limit
        const maxDepth = 50
        if resolutionCtx.depth > maxDepth {
            return nil, fmt.Errorf("max recursion depth exceeded: %d", maxDepth)
        }

        // Parse resource path: resource::type::#(id=='task1').outputs
        parts := strings.SplitN(path, "::", 2)
        if len(parts) != 2 {
            return nil, fmt.Errorf("invalid resource path format: %s", path)
        }

        resourceType := parts[0]
        remainingPath := parts[1]

        // Extract ID and drill-down path
        id, drillDownPath, err := parseResourceSelector(remainingPath)
        if err != nil {
            return nil, err
        }

        // Check for cycles
        resourceKey := fmt.Sprintf("%s/%s", resourceType, id)
        if resolutionCtx.visited[resourceKey] {
            return nil, fmt.Errorf("cyclic dependency detected: %s", resourceKey)
        }

        // Mark as visited
        resolutionCtx.visited[resourceKey] = true
        resolutionCtx.depth++

        // Get from registry
        config, err := al.registry.Get(resourceType, id)
        if err != nil {
            return nil, err
        }

        // Apply drill-down path if present
        if drillDownPath != "" {
            return ref.GetValueFromPath(config, drillDownPath, resolutionCtx)
        }

        return config, nil
    })
}

type resolutionContext struct {
    visited map[string]bool
    depth   int
}

// Helper to parse selector like #(id=='task1').outputs
func parseResourceSelector(selector string) (id string, drillDownPath string, err error) {
    // Example: #(id=='task1').outputs
    // Returns: id="task1", drillDownPath=".outputs"

    // Find the ID within the selector
    if strings.HasPrefix(selector, "#(id==") {
        endIdx := strings.Index(selector, ")")
        if endIdx == -1 {
            return "", "", fmt.Errorf("invalid selector format: %s", selector)
        }

        // Extract ID value
        idPart := selector[6:endIdx] // Skip "#(id=="
        id = strings.Trim(idPart, "'\"")

        // Extract drill-down path if present
        if endIdx+1 < len(selector) {
            drillDownPath = selector[endIdx+1:]
        }

        return id, drillDownPath, nil
    }

    return "", "", fmt.Errorf("unsupported selector format: %s", selector)
}
```

## Data Flow

### Loading Phase

1. Application reads `compozy.yaml` and extracts `autoload` configuration
2. AutoLoader discovers files matching include patterns
3. Each file is loaded via `core.LoadConfig()` (preserving CWD behavior)
4. Configs are registered in the in-memory registry
5. Resource resolver is installed in the ref evaluator

### Resolution Phase (Lazy)

1. When a `$ref: "resource::task::#(id=='mytask')"` is encountered
2. The ref evaluator calls the resource resolver
3. Resource resolver queries the registry
4. Config is returned and selector is applied
5. Result is cached by the ref evaluator

## Configuration Lifecycle

The auto-load system is stateless between runs. On each startup, the `AutoLoader` performs a full discovery based on the current state of the filesystem.

- **File Addition**: New files matching the patterns will be discovered and loaded on the next run
- **File Deletion**: Files that have been deleted will no longer be discovered and will not be part of the configuration on the next run
- **File Modification**: Changes to existing files will be picked up on the next run because `core.LoadConfig` re-reads the file from disk

This stateless, from-scratch loading model ensures predictable and deterministic behavior without the complexity of caching or state reconciliation between runs.

## Error Handling Strategy

### Strict Mode (Default for Production)

- **Discovery Errors**: Fatal - application exits
- **Load Errors**: Fatal - application exits with file path
- **Duplicate Configs**: Fatal - application exits with conflict details
- **Missing Resources**: Fatal when referenced

### Non-Strict Mode (Development)

- **Discovery Errors**: Fatal - cannot proceed without files
- **Load Errors**: Warning - skip invalid files
- **Duplicate Configs**: Warning - skip duplicates
- **Missing Resources**: Warning - return nil/empty

### Error Codes

```go
const (
    ErrAutoloadDiscoveryFailed = "AUTOLOAD_DISCOVERY_FAILED"
    ErrAutoloadFileFailed      = "AUTOLOAD_FILE_FAILED"
    ErrDuplicateConfig         = "DUPLICATE_CONFIG"
    ErrResourceNotFound        = "RESOURCE_NOT_FOUND"
    ErrInvalidPattern          = "INVALID_PATTERN"
    ErrPathEscapeAttempt       = "PATH_ESCAPE_ATTEMPT"
)
```

## Security Considerations

### Path Traversal Prevention

```go
func validatePattern(pattern, root string) error {
    // 1. Clean the pattern
    clean := filepath.Clean(pattern)

    // 2. Reject absolute paths
    if filepath.IsAbs(clean) {
        return fmt.Errorf("absolute paths not allowed: %s", pattern)
    }

    // 3. Reject parent directory references
    if strings.Contains(clean, "..") {
        return fmt.Errorf("parent directory references not allowed: %s", pattern)
    }

    // 4. Ensure resolved path is within root
    resolved := filepath.Join(root, clean)
    if !strings.HasPrefix(resolved, root) {
        return fmt.Errorf("pattern escapes project root: %s", pattern)
    }

    return nil
}
```

### Resource Limits

```go
const (
    MaxAutoloadFiles = 1000  // Configurable limit
    MaxFileSize      = 10_000_000  // 10MB per config file
)
```

## Testing Strategy

### Unit Tests

```go
func TestAutoLoader_Load(t *testing.T) {
    t.Run("Should load basic workflow pattern", func(t *testing.T) {
        // Setup
        mockDiscoverer := &mockFileDiscoverer{
            files: []string{"workflows/test.yaml"},
        }
        loader := &AutoLoader{
            discoverer: mockDiscoverer,
            registry:   NewConfigRegistry(),
        }

        // Execute
        err := loader.Load(context.Background())

        // Assert
        assert.NoError(t, err)
        assert.Equal(t, 1, loader.registry.Count())
    })

    t.Run("Should detect duplicate configs in strict mode", func(t *testing.T) {
        // Test duplicate detection
    })

    t.Run("Should skip invalid files in non-strict mode", func(t *testing.T) {
        // Test error handling
    })
}
```

### Integration Tests

```go
func TestAutoLoader_Integration(t *testing.T) {
    t.Run("Should resolve resource references", func(t *testing.T) {
        // Load configs with cross-references
        // Verify lazy resolution works
    })

    t.Run("Should maintain CWD for tool execution", func(t *testing.T) {
        // Load tool config from subdirectory
        // Verify execute path resolves correctly
    })
}
```

## Performance Considerations

### Benchmarks

```go
func BenchmarkAutoLoader_Load(b *testing.B) {
    // Target: <500ms for 200 files
    configs := generateTestConfigs(200)
    loader := New(".", defaultConfig)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = loader.Load(context.Background())
    }
}
```

### Optimization Strategies

1. **Parallel Loading**: Load files concurrently with worker pool
2. **Caching**: Cache parsed configs to avoid re-parsing
3. **Selective Loading**: Only load changed files during hot reload

## Integration Points

### With Existing Core Systems

```go
// In main application startup
func initializeCompozy(projectConfig *core.ProjectConfig) error {
    // 1. Create a single, shared registry
    configRegistry := autoload.NewConfigRegistry()

    // 2. Load and register manual configs first
    manualConfigs := loadManualConfigs(projectConfig.Configs)
    for _, config := range manualConfigs {
        if err := configRegistry.Register(config, "manual"); err != nil {
            return err // Fail fast on manual config duplicates
        }
    }

    // 3. Auto-load if enabled, using the same registry
    if projectConfig.AutoLoad.Enabled {
        autoLoader := autoload.New(projectConfig.Root, projectConfig.AutoLoad, configRegistry)
        if err := autoLoader.Load(ctx); err != nil {
            return err
        }
    }

    // 4. Continue with normal initialization
    return initializeWorkflows(configRegistry)
}
```

### CLI Integration

```go
// New command: compozy config discover
func cmdConfigDiscover(cmd *cobra.Command, args []string) error {
    config := loadProjectConfig()
    loader := autoload.New(".", config.AutoLoad, nil)

    files, err := loader.Discover(context.Background())
    if err != nil {
        return err
    }

    // Print discovered files if --print flag is set
    if printFlag, _ := cmd.Flags().GetBool("print"); printFlag {
        fmt.Printf("Discovered %d configuration files:\n", len(files))
        for _, file := range files {
            fmt.Printf("  ✓ %s\n", file)
        }
    } else {
        fmt.Printf("Would discover %d configuration files. Use --print to see details.\n", len(files))
    }

    return nil
}
```

## Migration Guide

### For Existing Projects

1. **Add autoload config to compozy.yaml**:

```yaml
autoload:
    enabled: true
    include:
        - "workflows/**/*.yaml"
        - "tasks/**/*.yaml"
    exclude:
        - "**/test/**"
```

2. **Run discovery dry-run**:

```bash
compozy config discover --dry-run
```

3. **Remove manual entries from configs/files**:

```yaml
# Before
configs:
  - workflows/user.yaml
  - workflows/admin.yaml
  - tasks/email.yaml

# After (empty or removed)
configs: []
```

## Future Enhancements (Post-MVP)

1. **File Watching**: Add fsnotify-based hot reload
2. **Performance**: Implement parallel loading for large projects
3. **Validation**: Enhanced schema validation during load
4. **Tooling**: IDE support for resource:: completion

## Conclusion

This architecture provides a simple, secure, and efficient solution for auto-loading configurations in Compozy. By leveraging existing infrastructure and using lazy evaluation for resource resolution, we avoid complexity while delivering a robust feature.

The design prioritizes:

- **Simplicity**: Minimal new code and concepts
- **Security**: Sandboxed file discovery
- **Performance**: <500ms for 200 files
- **Compatibility**: Zero breaking changes
- **Testability**: Clear interfaces for mocking

This approach delivers the MVP requirements while providing a solid foundation for future enhancements.
