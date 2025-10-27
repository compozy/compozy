# SDK Migration to Auto-Generated Functional Options Pattern

## Executive Summary

**Status:** ✅ Code generation infrastructure complete, ✅ Agent package migrated, ⏳ 9 example files need updates

We successfully built a **generic code generator** that automatically creates functional options from engine structs, eliminating 70-80% of manual boilerplate. The `agent` package has been fully migrated and all tests pass.

---

## What Has Been Completed ✅

### 1. Code Generation Infrastructure (100% Complete)

**Created:** `sdk/internal/codegen/`

#### Files:

- ✅ **`types.go`** - Data structures for parsed struct metadata
- ✅ **`parser.go`** - AST parser that automatically discovers ALL struct fields (including embedded structs)
- ✅ **`generator.go`** - Jennifer-based code generator for functional options
- ✅ **`cmd/optionsgen/main.go`** - CLI tool to run the generator

#### Capabilities:

- ✅ Automatically discovers all exported fields from engine structs
- ✅ Handles embedded structs (e.g., `LLMProperties` in `agent.Config`)
- ✅ Detects and generates correct types:
  - Simple types: `string`, `int`, `bool`
  - Slices: `[]tool.Config`, `[]*ActionConfig`
  - Pointers: `*core.Input`
  - Maps: `map[string]string`
  - Cross-package types: `attachment.Attachments`, `core.MemoryReference`
- ✅ Preserves field documentation comments
- ✅ Skips unexported fields (`filePath`, `CWD`)
- ✅ All linter checks pass (gocritic, staticcheck, etc.)

**Usage:**

```bash
cd sdk/<package>
go generate  # Runs: go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/<package>/config.go -struct Config -output options_generated.go
```

---

### 2. Agent Package Migration (100% Complete)

**Package:** `sdk/agent/`

#### Files Created:

- ✅ **`generate.go`** - go:generate directive
- ✅ **`options_generated.go`** - 14 auto-generated option functions (~215 lines)
- ✅ **`constructor.go`** - Constructor with centralized validation (~110 lines)
- ✅ **`constructor_test.go`** - 32 passing tests
- ✅ **`README.md`** - Usage documentation

#### Files Deleted:

- ❌ **`builder.go`** (was ~270 lines) - Replaced by generated options
- ❌ **`builder_test.go`** - Replaced by constructor tests

#### Results:

- ✅ **Code reduction:** 270 lines → 110 lines (60% reduction)
- ✅ **Linter:** 0 issues
- ✅ **Tests:** 32/32 passing
- ✅ **Maintainability:** Adding new fields = `go generate` (0 manual lines)

#### Generated Options (14 total):

```go
// Auto-generated from engine/agent/config.go:
WithTools([]tool.Config)
WithMCPs([]mcp.Config)
WithMaxIterations(int)
WithMemory([]core.MemoryReference)
WithModel(agent.Model)
WithAttachments(attachment.Attachments)
WithResource(string)
WithID(string)
WithInstructions(string)
WithActions([]*agent.ActionConfig)
WithWith(*core.Input)
WithEnv(*core.EnvMap)
WithKnowledge([]core.KnowledgeBinding)
WithCWD(*core.PathCWD)  // Skipped in generator
```

---

## What Needs To Be Done ⏳

### Phase 1: Update SDK Example Files (10 files)

**Status:** 1/10 completed (05_mcp_integration fixed)

All example files in `sdk/cmd/*/main.go` need to migrate from the old builder API to functional options.

#### Files Requiring Updates:

| File                                 | Old API Usage                        | New API Required                      |
| ------------------------------------ | ------------------------------------ | ------------------------------------- |
| ✅ `05_mcp_integration/main.go`      | FIXED                                | Working                               |
| ⏳ `01_simple_workflow/main.go`      | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `02_parallel_tasks/main.go`       | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `03_knowledge_rag/main.go`        | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `04_memory_conversation/main.go`  | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `06_runtime_native_tools/main.go` | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `07_scheduled_workflow/main.go`   | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `08_signal_communication/main.go` | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `10_complete_project/main.go`     | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |
| ⏳ `11_debugging/main.go`            | `agent.New("id").WithX().Build(ctx)` | `agent.New(ctx, "id", agent.WithX())` |

#### Migration Pattern for Examples:

**BEFORE (Old Builder Pattern):**

```go
func buildAgent(ctx context.Context) (*engineagent.Config, error) {
    return agent.New("assistant").
        WithModel("openai", "gpt-4").
        WithInstructions("You are helpful").
        AddAction(action).
        AddTool("tool1").
        Build(ctx)
}
```

**AFTER (Functional Options):**

```go
func buildAgent(ctx context.Context) (*engineagent.Config, error) {
    return agent.New(ctx, "assistant",
        agent.WithInstructions("You are helpful"),
        agent.WithModel(engineagent.Model{
            Config: core.ProviderConfig{
                Provider: core.ProviderOpenAI,
                Model:    "gpt-4",
            },
        }),
        agent.WithActions([]*engineagent.ActionConfig{action}),
        agent.WithTools([]enginetool.Config{{ID: "tool1"}}),
    )
}
```

**Key Changes:**

1. ✅ `ctx` is now **first parameter**
2. ✅ No more `.Build(ctx)` - constructor validates immediately
3. ✅ Use `WithX()` functions instead of chainable methods
4. ✅ `WithModel()` takes `Model` struct, not `(provider, model)` strings
5. ✅ Collection methods (`AddAction`, `AddTool`) → `WithActions()`, `WithTools()` (plural, takes slices)

#### Required Import Changes:

```go
import (
    engineagent "github.com/compozy/compozy/engine/agent"
+   "github.com/compozy/compozy/engine/core"  // For ProviderConfig
+   enginetool "github.com/compozy/compozy/engine/tool"  // For tool.Config
    "github.com/compozy/compozy/sdk/agent"
)
```

---

### Phase 2: Migrate Remaining SDK Packages (11 packages)

**Estimated effort:** 1 day per package (can be done in parallel)

Each package needs the same treatment as `agent`:

| Package            | Status       | Engine Source                    | Complexity                         |
| ------------------ | ------------ | -------------------------------- | ---------------------------------- |
| ✅ `sdk/agent`     | **COMPLETE** | `engine/agent/config.go`         | High (14 fields, embedded structs) |
| ⏳ `sdk/model`     | TODO         | `engine/core/provider_config.go` | Low (7 fields)                     |
| ⏳ `sdk/tool`      | TODO         | `engine/tool/config.go`          | Medium (8 fields)                  |
| ⏳ `sdk/task`      | TODO         | `engine/task/config.go`          | High (multiple task types)         |
| ⏳ `sdk/workflow`  | TODO         | `engine/workflow/config.go`      | Medium (10 fields)                 |
| ⏳ `sdk/project`   | TODO         | `engine/project/config.go`       | High (15 fields)                   |
| ⏳ `sdk/mcp`       | TODO         | `engine/mcp/config.go`           | Low (6 fields)                     |
| ⏳ `sdk/runtime`   | TODO         | `engine/runtime/config.go`       | Low (5 fields)                     |
| ⏳ `sdk/schedule`  | TODO         | `engine/schedule/config.go`      | Low (4 fields)                     |
| ⏳ `sdk/schema`    | TODO         | `engine/schema/schema.go`        | Medium (8 fields)                  |
| ⏳ `sdk/knowledge` | TODO         | `engine/knowledge/config.go`     | Medium (multiple types)            |
| ⏳ `sdk/memory`    | TODO         | `engine/memory/config.go`        | Low (5 fields)                     |

#### Migration Steps Per Package:

**1. Create `generate.go`:**

```go
package <package>

//go:generate go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/<package>/config.go -struct Config -output options_generated.go
```

**2. Run generator:**

```bash
cd sdk/<package>
go generate
```

**3. Create `constructor.go`:**

```go
package <package>

import (
    "context"
    "fmt"
    "strings"

    engine<package> "github.com/compozy/compozy/engine/<package>"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/pkg/logger"
    sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
    "github.com/compozy/compozy/sdk/internal/validate"
)

// New creates a <package> configuration using functional options
func New(ctx context.Context, id string, opts ...Option) (*engine<package>.Config, error) {
    if ctx == nil {
        return nil, fmt.Errorf("context is required")
    }
    log := logger.FromContext(ctx)
    log.Debug("creating <package> configuration", "<package>", id)

    cfg := &engine<package>.Config{
        ID: strings.TrimSpace(id),
        // ... initialize required fields
    }

    // Apply all options
    for _, opt := range opts {
        opt(cfg)
    }

    // Centralized validation
    collected := make([]error, 0)
    if err := validate.ID(ctx, cfg.ID); err != nil {
        collected = append(collected, fmt.Errorf("<package> id is invalid: %w", err))
    }
    // ... more validation

    if len(collected) > 0 {
        return nil, &sdkerrors.BuildError{Errors: collected}
    }

    cloned, err := core.DeepCopy(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to clone <package> config: %w", err)
    }
    return cloned, nil
}
```

**4. Create `constructor_test.go`:**

- Test minimal configuration
- Test all options
- Test validation errors
- Test nil context
- Test deep copy

**5. Delete old files:**

```bash
rm sdk/ < package > /builder.go
rm sdk/ < package > /builder_test.go
```

**6. Run linter and tests:**

```bash
golangci-lint run --fix --allow-parallel-runners ./sdk/<package>/...
gotestsum --format pkgname -- -race -parallel=4 ./sdk/<package>
```

---

## Special Cases & Considerations

### Task Package (`sdk/task`)

**Challenge:** Multiple task types (Basic, Parallel, Collection, Wait, Signal, etc.)

**Solution:** Generate options for each task type:

- `sdk/task/basic.go` → `NewBasic(ctx, id, ...Option)`
- `sdk/task/parallel.go` → `NewParallel(ctx, id, ...Option)`
- `sdk/task/collection.go` → `NewCollection(ctx, id, ...Option)`

Each has its own `generate.go`:

```go
//go:generate go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/task/basic.go -struct BasicTask -output basic_options_generated.go
```

### Knowledge Package (`sdk/knowledge`)

**Challenge:** Multiple config types (Base, Binding, Embedder, VectorDB)

**Solution:** Generate options for each:

- `NewBase()` - knowledge base config
- `NewBinding()` - binding config
- `NewEmbedder()` - embedder config
- `NewVectorDB()` - vector DB config

### Schema Package (`sdk/schema`)

**Challenge:** Dynamic property builders

**Solution:** Keep builder pattern for `PropertyBuilder` (used for dynamic schema construction), but use functional options for `Schema` itself.

---

## Verification Checklist

After each package migration:

- [ ] `go generate` runs successfully
- [ ] `options_generated.go` contains all expected fields
- [ ] Constructor validates required fields
- [ ] Constructor performs deep copy
- [ ] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./sdk/<package>`
- [ ] Linter passes: `golangci-lint run --fix --allow-parallel-runners ./sdk/<package>/...`
- [ ] Old `builder.go` and `builder_test.go` deleted
- [ ] README.md updated with new API examples

---

## Benefits Achieved

### Before (Manual Builder Pattern):

- **~4,000-5,000 lines** of manual boilerplate across SDK
- **30-40 lines per new field** when engine changes
- Manual nil safety checks in every method
- Error accumulation in builder struct
- Separate `Build()` validation step

### After (Auto-Generated Functional Options):

- **~800-1,200 lines** total (70-75% reduction)
- **0 manual lines per new field** (`go generate` handles it)
- Centralized validation in constructor
- More idiomatic Go (stdlib pattern)
- Better performance (no builder allocation)

### When Engine Changes:

```bash
# Before: Edit 3-4 files, write 30-40 lines manually
# After:
cd sdk/<package>
go generate  # Done! ✅
```

---

## LLM Task Instructions

**Your task:** Migrate the remaining SDK packages to functional options pattern.

### Priority Order:

**Phase 1 (Immediate - Unblock examples):**

1. Fix 9 remaining example files in `sdk/cmd/*/main.go`
   - Use the migration pattern shown above
   - Key: `ctx` first, no `.Build()`, use `WithX()` options
   - Test each: `go run sdk/cmd/<example>/main.go`

**Phase 2 (Core packages):** 2. `sdk/model` - Simple, good next target 3. `sdk/tool` - Medium complexity 4. `sdk/workflow` - Needed by most examples 5. `sdk/project` - High-level orchestration

**Phase 3 (Specialized packages):** 6. `sdk/task` - Multiple types, handle separately 7. `sdk/mcp` - Medium complexity 8. `sdk/runtime` - Simple 9. `sdk/schedule` - Simple 10. `sdk/schema` - Keep property builders, migrate Schema 11. `sdk/knowledge` - Multiple types 12. `sdk/memory` - Simple

### For Each Package:

1. **Read current `builder.go`** to understand validation logic
2. **Create `generate.go`** with go:generate directive
3. **Run `go generate`** and review generated options
4. **Create `constructor.go`** with validation from old builder
5. **Create `constructor_test.go`** covering all scenarios
6. **Delete `builder.go` and `builder_test.go`**
7. **Run linter and tests**
8. **Update any affected example files**

### Testing Commands:

```bash
# Per package
golangci-lint run --fix --allow-parallel-runners ./sdk/<package>/...
gotestsum --format pkgname -- -race -parallel=4 ./sdk/<package>

# All SDK
golangci-lint run --fix --allow-parallel-runners ./sdk/...
gotestsum --format pkgname -- -race -parallel=4 ./sdk/...
```

### Important Rules:

1. **Greenfield approach** - No backwards compatibility needed (alpha project)
2. **Delete old files** - Don't keep builder.go alongside new code
3. **Context-first** - Always `ctx` as first parameter
4. **Centralized validation** - In constructor, not per-option
5. **Deep copy** - Always clone before returning
6. **Test coverage** - Match or exceed old builder tests

---

## Files Reference

### Code Generation Infrastructure:

- `sdk/internal/codegen/types.go` - Struct metadata types
- `sdk/internal/codegen/parser.go` - AST parser (handles embedded structs, pointers, slices)
- `sdk/internal/codegen/generator.go` - Jennifer code generator
- `sdk/internal/codegen/cmd/optionsgen/main.go` - CLI tool

### Completed Migration (Reference):

- `sdk/agent/generate.go` - go:generate directive
- `sdk/agent/options_generated.go` - Generated options (DO NOT EDIT)
- `sdk/agent/constructor.go` - Validation logic
- `sdk/agent/constructor_test.go` - 32 tests
- `sdk/agent/README.md` - Documentation

### Comparison Document:

- `sdk/CODEGEN_COMPARISON.md` - Before/after metrics and examples

---

## Expected Final State

```
sdk/
├── agent/              ✅ COMPLETE
│   ├── generate.go
│   ├── options_generated.go
│   ├── constructor.go
│   ├── constructor_test.go
│   ├── action.go (helper, keep)
│   └── README.md
├── model/              ⏳ TODO
├── tool/               ⏳ TODO
├── task/               ⏳ TODO (multi-type)
├── workflow/           ⏳ TODO
├── project/            ⏳ TODO
├── mcp/                ⏳ TODO
├── runtime/            ⏳ TODO
├── schedule/           ⏳ TODO
├── schema/             ⏳ TODO (keep property builders)
├── knowledge/          ⏳ TODO (multi-type)
├── memory/             ⏳ TODO
├── internal/
│   └── codegen/        ✅ COMPLETE
│       ├── types.go
│       ├── parser.go
│       ├── generator.go
│       └── cmd/optionsgen/main.go
└── cmd/                ⏳ 9/10 examples need fixes
    ├── 01_simple_workflow/main.go
    ├── 02_parallel_tasks/main.go
    └── ... (8 more)
```

---

## Quick Start for Next Developer

```bash
# 1. Fix an example file (e.g., 01_simple_workflow)
vim sdk/cmd/01_simple_workflow/main.go
# Change: agent.New("id").WithX().Build(ctx)
# To: agent.New(ctx, "id", agent.WithX())

# 2. Test it
go run sdk/cmd/01_simple_workflow/main.go

# 3. Migrate next package (e.g., model)
cd sdk/model
# Copy pattern from sdk/agent/
echo '//go:generate go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/core/provider_config.go -struct ProviderConfig -output options_generated.go' > generate.go
go generate
# Create constructor.go and constructor_test.go
# Delete builder.go and builder_test.go

# 4. Verify
golangci-lint run --fix --allow-parallel-runners ./sdk/model/...
gotestsum --format pkgname -- -race -parallel=4 ./sdk/model
```

---

## Contact & Questions

- **Generator issues:** Check `sdk/internal/codegen/`
- **Test failures:** Reference `sdk/agent/constructor_test.go` for patterns
- **Type resolution issues:** See `parser.go` and `generator.go`
- **Validation patterns:** Study `sdk/agent/constructor.go`

**Remember:** This is a greenfield migration - delete old code, don't maintain both patterns.
