## status: pending

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/runtime</dependencies>
</task_context>

# Task 49.0: Example: Runtime + Native Tools (S)

## Overview

Create example demonstrating runtime configuration (Bun/Node/Deno) with native tools integration (call_agents, call_workflows).

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 6: Runtime + Native Tools)
- **MUST** demonstrate all 3 runtime types
- **MUST** show native tools configuration
</critical>

<requirements>
- Runnable example: sdk/examples/06_runtime_native_tools.go
- Demonstrates: Bun, Node, Deno runtime configurations
- Shows: Native tools (call_agents, call_workflows)
- Permissions and options per runtime type
- Memory limits
- Clear comments on runtime selection
</requirements>

## Subtasks

- [ ] 49.1 Create sdk/examples/06_runtime_native_tools.go
- [ ] 49.2 Build Bun runtime with:
  - [ ] Entrypoint
  - [ ] Permissions
  - [ ] Native tools enabled
  - [ ] Memory limit
- [ ] 49.3 Build Node runtime with:
  - [ ] Entrypoint
  - [ ] Node options
  - [ ] Memory limit
- [ ] 49.4 Build Deno runtime with:
  - [ ] Entrypoint
  - [ ] Permissions
  - [ ] Memory limit
- [ ] 49.5 Build native tools config (call_agents + call_workflows)
- [ ] 49.6 Build project with runtime configuration
- [ ] 49.7 Add comments explaining runtime choices
- [ ] 49.8 Update README.md with runtime example
- [ ] 49.9 Test example runs successfully

## Implementation Details

Per 05-examples.md section 6:

**Bun runtime with native tools:**
```go
bunRuntime, err := runtime.NewBun().
    WithEntrypoint("./tools/main.ts").
    WithBunPermissions("--allow-read", "--allow-env", "--allow-net").
    WithNativeTools(
        runtime.NewNativeTools().
            WithCallAgents().
            WithCallWorkflows().
            Build(ctx),
    ).
    WithMaxMemoryMB(512).
    Build(ctx)
```

**Node runtime:**
```go
nodeRuntime, err := runtime.NewNode().
    WithEntrypoint("./tools/index.js").
    WithNodeOptions("--experimental-modules", "--enable-source-maps").
    WithMaxMemoryMB(1024).
    Build(ctx)
```

**Deno runtime:**
```go
denoRuntime, err := runtime.NewDeno().
    WithEntrypoint("./tools/mod.ts").
    WithDenoPermissions("--allow-read", "--allow-write", "--allow-net").
    WithMaxMemoryMB(512).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/06_runtime_native_tools.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/runtime/builder.go` - Runtime builder
- `sdk/runtime/native_tools.go` - NativeToolsBuilder
- `sdk/project/builder.go` - Project with runtime

## Deliverables

- [ ] sdk/examples/06_runtime_native_tools.go (runnable)
- [ ] Updated README.md with runtime example section
- [ ] Comments explaining:
  - When to use Bun vs Node vs Deno
  - Native tools capabilities (call_agents, call_workflows)
  - Permission models per runtime
  - Memory limit configuration
- [ ] All 3 runtime types demonstrated
- [ ] Native tools enabled on Bun example
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Bun runtime config with permissions
  - [ ] Node runtime config with options
  - [ ] Deno runtime config with permissions
  - [ ] Native tools config validated
  - [ ] Memory limits validated (positive MB)
  - [ ] Entrypoint paths validated
  - [ ] Project uses runtime config correctly

## Success Criteria

- Example demonstrates all 3 runtime types
- Native tools configuration shown
- Comments explain runtime selection criteria
- README updated with runtime requirements
- Example runs end-to-end successfully
- Code passes `make lint`
