# Technical Specification: Runtime Refactoring - Deno to Bun with Entrypoint Architecture

## Executive Summary

This specification outlines the refactoring of Compozy's runtime system from Deno to Bun, introducing a TypeScript entrypoint file pattern that eliminates the need for deno.json and importMap configurations. This architecture enables future support for multiple JavaScript runtimes (Bun, Node.js) while simplifying tool configuration and execution.

**Development Strategy**: Following the greenfield approach per backwards-compatibility.mdc - no backwards compatibility required during development phase. We will make breaking changes to achieve the best architecture.

## Current Architecture Analysis

### Existing Implementation

- **Runtime**: Deno-based execution (to be completely replaced)
- **Tool Resolution**: ImportMap in `deno.json` maps tool IDs to TypeScript files
- **Tool Configuration**: Each tool has an `execute` property pointing to the implementation file
- **Worker Process**: `compozy_worker.ts` template uses dynamic imports based on tool ID
- **Communication**: JSON-based stdin/stdout protocol between Go and runtime

### Current Flow

1. Tool configuration specifies `execute: ./weather_tool.ts`
2. Project's `deno.json` maps tool ID to file path via importMap
3. Worker imports tool using `await import(tool_id)`
4. Tool exports `run` or `default` function

### Limitations

- Single runtime option without flexibility
- Dependency on importMap configuration
- Redundant configuration (both `execute` property and importMap entry)
- Complex configuration requirements

## Proposed Architecture

### Core Concept: Entrypoint Pattern

Replace individual tool imports with a single TypeScript entrypoint file that exports all tools as named exports matching their tool IDs.

### New Configuration Structure

```go
// engine/project/config.go
type RuntimeConfig struct {
    Type        string   `json:"type,omitempty"        yaml:"type,omitempty"`        // "bun" | "node"
    Entrypoint  string   `json:"entrypoint"            yaml:"entrypoint"`            // Required: path to entrypoint file
    Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}
```

### Entrypoint File Structure

```typescript
// Example: .compozy/tools.ts (auto-generated or user-defined)
// Export tools with names matching their IDs
export { run as weather_tool } from "../tools/weather_tool.ts";
export { run as save_data } from "../tools/save_tool.ts";
export { default as analyze_code } from "../tools/analyzer.ts";

// Or direct implementations
export async function simple_tool(input: any) {
    return { result: `Processed: ${input.value}` };
}
```

### Tool Configuration Simplification

```yaml
# Before
tools:
  - id: weather_tool
    description: Get weather data
    execute: ./weather_tool.ts  # This property will be removed

# After
tools:
  - id: weather_tool
    description: Get weather data
    # No execute property needed - resolved via entrypoint
```

## Implementation Plan

### Phase 1: Core Infrastructure Changes

#### 1.1 Update Runtime Configuration

```go
// engine/runtime/config.go
type Config struct {
    // Existing fields...
    RuntimeType          string        // "bun" or "node"
    EntrypointPath       string        // Path to entrypoint file
    BunPermissions       []string      // Bun-specific permissions
    NodeOptions          []string      // Node.js-specific options
}
```

#### 1.2 Make Manager Implement Runtime Interface

```go
// engine/runtime/interface.go
// Runtime interface that existing Manager will implement
type Runtime interface {
    ExecuteTool(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap) (*core.Output, error)
    ExecuteToolWithTimeout(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap, timeout time.Duration) (*core.Output, error)
    GetGlobalTimeout() time.Duration
}

// Ensure Manager implements Runtime
var _ Runtime = (*Manager)(nil)
```

#### 1.3 Create Runtime Factory Following Project Patterns

```go
// engine/runtime/factory.go
// Factory creates Runtime instances based on configuration
type Factory interface {
    // CreateRuntime creates a new Runtime for the given configuration
    CreateRuntime(ctx context.Context, config *Config) (Runtime, error)
}

// DefaultFactory is the default implementation of Factory
type DefaultFactory struct {
    projectRoot string
}

// NewDefaultFactory creates a new DefaultFactory
func NewDefaultFactory(projectRoot string) Factory {
    return &DefaultFactory{projectRoot: projectRoot}
}

// CreateRuntime creates appropriate runtime based on config
func (f *DefaultFactory) CreateRuntime(ctx context.Context, config *Config) (Runtime, error) {
    switch config.RuntimeType {
    case "bun", "":
        // Default to Bun
        return NewBunManager(ctx, f.projectRoot, WithConfig(config))
    case "node":
        return NewNodeManager(ctx, f.projectRoot, WithConfig(config))
    default:
        return nil, fmt.Errorf("unsupported runtime type: %s", config.RuntimeType)
    }
}
```

#### 1.3 Implement Bun Runtime

```go
// engine/runtime/bun/runtime.go
type BunRuntime struct {
    config       *runtime.Config
    projectRoot  string
    entrypoint   string
}

func (r *BunRuntime) ExecuteTool(ctx context.Context, toolID string, input *core.Input, env core.EnvMap) (*core.Output, error) {
    // Implementation using Bun
}
```

### Phase 2: Worker Template Migration

#### 2.1 Bun Worker Template

```typescript
// engine/runtime/bun/worker.tpl.ts
#!/usr/bin/env bun

// Import all tools from entrypoint
import * as tools from "{{.EntrypointPath}}";

interface Request {
    tool_id: string;
    tool_exec_id: string;
    input: any;
    env: Record<string, string>;
    timeout_ms: number;
}

async function main() {
    const input = await Bun.stdin.text();
    const req: Request = JSON.parse(input);

    try {
        // Get tool function from entrypoint exports
        const toolFn = tools[req.tool_id];
        if (typeof toolFn !== "function") {
            throw new Error(`Tool ${req.tool_id} not found in entrypoint exports`);
        }

        // Execute with timeout
        const result = await Promise.race([
            toolFn(req.input),
            new Promise((_, reject) =>
                setTimeout(() => reject(new Error("Timeout")), req.timeout_ms)
            )
        ]);

        // Send response
        console.log(JSON.stringify({
            result,
            error: null,
            metadata: { tool_id: req.tool_id, tool_exec_id: req.tool_exec_id }
        }));
    } catch (error) {
        console.log(JSON.stringify({
            result: null,
            error: {
                message: error.message,
                stack: error.stack,
                name: error.name,
                tool_id: req.tool_id,
                tool_exec_id: req.tool_exec_id,
                timestamp: new Date().toISOString()
            }
        }));
    }
}

main().catch(console.error);
```

### Phase 3: Entrypoint Generation

#### 3.1 Entrypoint Generator

```go
// engine/runtime/generator/entrypoint.go
type EntrypointGenerator struct {
    projectRoot string
    tools       []*tool.Config
}

func (g *EntrypointGenerator) Generate() (string, error) {
    var imports []string

    for _, t := range g.tools {
        if t.Execute != "" {
            // Generate import based on execute path
            importPath := filepath.Join(g.projectRoot, t.Execute)
            imports = append(imports, fmt.Sprintf(
                `export { run as %s } from "%s";`,
                t.ID,
                importPath,
            ))
        }
    }

    return strings.Join(imports, "\n"), nil
}
```

### Phase 4: Configuration Updates

#### 4.1 Project Configuration Example

```yaml
# compozy.yaml
runtime:
    type: bun # or "node"
    entrypoint: ./.compozy/tools.ts
    permissions:
        - "--allow-net"
        - "--allow-read"
        - "--allow-env"
```

#### 4.2 Tool Configuration (Simplified)

```yaml
tools:
    - id: weather_tool
      description: Get weather data
      input:
          type: object
          properties:
              city:
                  type: string
      # No execute property needed
```

## Implementation Strategy (Greenfield)

### Direct Replacement

Since we're following the greenfield approach with no backwards compatibility requirements:

1. **Complete Removal**: Remove all Deno-related code and dependencies
2. **Direct Implementation**: Implement Bun runtime as the primary runtime
3. **Clean Architecture**: No compatibility layers or migration paths needed
4. **Breaking Changes**: Remove `execute` property immediately
5. **Fresh Start**: All projects must use the new entrypoint pattern

## Benefits

1. **Runtime Flexibility**: Easy to add Node.js or other JavaScript runtime support
2. **Simplified Configuration**: Remove redundant tool path specifications
3. **Better Performance**: Bun's faster startup and execution times
4. **Type Safety**: Single entrypoint file enables better TypeScript integration
5. **Easier Testing**: Mock entire tool suite by replacing entrypoint
6. **Future-Proof**: Architecture supports WebAssembly, native modules, etc.

## Risks and Mitigations

### Risk 1: Bun Stability

- **Mitigation**: Runtime interface allows easy switch to Node.js if needed
- **Mitigation**: Comprehensive testing before release

### Risk 2: Developer Learning Curve

- **Mitigation**: Clear documentation and examples
- **Mitigation**: Simple entrypoint pattern is easier than importMap

## Implementation Timeline

1. **Week 1-2**: Core infrastructure (runtime interface, Bun implementation)
2. **Week 3**: Worker template and process communication
3. **Week 4**: Backward compatibility and migration tools
4. **Week 5**: Testing and documentation
5. **Week 6**: Release as experimental feature

## Success Criteria

1. All existing examples work with new runtime
2. Performance improvement of at least 20% in tool execution
3. Successful migration of 3 real projects without issues
4. Support for both Bun and Node.js runtimes
5. Simplified tool configuration process

## Future Enhancements

1. **Hot Reload**: Detect entrypoint changes and reload runtime
2. **Plugin System**: Allow runtime extensions via entrypoint
3. **Native Modules**: Support for Go/Rust tools via entrypoint
4. **Remote Tools**: Import tools from URLs or registries
5. **Tool Versioning**: Support multiple versions via entrypoint
