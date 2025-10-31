# Technical Specification: SDK Migration to Auto-Generated Functional Options

## Document Information

**Version:** 1.0
**Status:** Approved
**Last Updated:** 2025-10-27
**Authors:** Engineering Team
**Stakeholders:** SDK Users, Core Team, Documentation Team

---

## 1. Executive Summary

### 1.1 Objective

Migrate the entire Compozy SDK from manual builder pattern to auto-generated functional options pattern, reducing maintenance burden by 70-75% while maintaining API quality and type safety.

### 1.2 Scope

**In Scope:**
- 11 SDK packages (agent, model, tool, task, workflow, project, mcp, runtime, schedule, schema, knowledge, memory)
- 9 example files in `sdk/cmd/`
- Code generation infrastructure
- Comprehensive test coverage
- Documentation updates

**Out of Scope:**
- Engine package modifications (read-only reference)
- Backwards compatibility with old SDK (greenfield approach)
- Migration of existing user code (breaking change accepted)

### 1.3 Success Metrics

| Metric | Target | Rationale |
|--------|--------|-----------|
| Code reduction | 70-75% | Reduce ~4,000 LOC to ~800-1,200 LOC |
| Maintenance cost | 0 lines/field | `go generate` handles new fields automatically |
| Test coverage | ≥95% | Match or exceed current coverage |
| Build time | No regression | Generated code must not slow builds |
| API ergonomics | Improved | More idiomatic Go patterns |

---

## 2. System Architecture

### 2.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     SDK2 Package Structure                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌────────────┐ │
│  │   generate   │────▶│   options    │────▶│constructor │ │
│  │   (manual)   │     │ (generated)  │     │  (manual)  │ │
│  └──────────────┘     └──────────────┘     └────────────┘ │
│         │                     │                    │        │
│         │                     │                    │        │
│         ▼                     ▼                    ▼        │
│  //go:generate         Option funcs         Validation     │
│    directive           WithX() for         + Deep copy     │
│                       each field                           │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│                  Code Generation Pipeline                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Engine Struct ──▶ AST Parser ──▶ Generator ──▶ Options    │
│  (config.go)        (types.go)     (jennifer)   (generated) │
│                                                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│              Code Generation Infrastructure                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  sdk/internal/codegen/                                     │
│  ├── types.go         - Metadata structures                 │
│  ├── parser.go        - AST-based field discovery           │
│  ├── generator.go     - Jennifer code generation            │
│  └── cmd/optionsgen/                                        │
│      └── main.go      - CLI tool with flags                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

#### 2.2.1 Parser Component (`parser.go`)

**Purpose:** Automatically discover all exported fields from engine structs using Go AST

**Capabilities:**
- Parses engine struct files using `go/ast` package
- Discovers all exported fields (capitalized)
- Handles embedded structs (flattens hierarchy)
- Resolves complex types:
  - Simple: `string`, `int`, `bool`
  - Slices: `[]Type`, `[]*Type`
  - Pointers: `*Type`
  - Maps: `map[K]V`
  - Cross-package: `pkg.Type`
- Extracts documentation comments
- Skips unexported fields automatically

**Input:** Path to engine struct file + struct name
**Output:** `StructInfo` with complete field metadata

#### 2.2.2 Generator Component (`generator.go`)

**Purpose:** Generate type-safe functional options using Jennifer library

**Capabilities:**
- Generates `Option func(*Config)` type
- Creates `WithX()` function for each field
- Preserves field documentation
- Handles import management automatically
- Produces clean, readable, linter-compliant code
- Supports custom package names via `-package` flag

**Input:** `StructInfo` from parser + output path + package name
**Output:** `options_generated.go` file with all option functions

#### 2.2.3 Constructor Component (Manual)

**Purpose:** Centralized validation and configuration instantiation

**Responsibilities:**
- Accept `context.Context` as first parameter
- Initialize config with required defaults
- Apply all functional options sequentially
- Perform comprehensive validation
- Collect validation errors (no fail-fast)
- Deep copy config before returning
- Return `(*engine.Config, error)`

**Pattern:**
```go
func New(ctx context.Context, id string, opts ...Option) (*engine.Config, error) {
    // 1. Context validation
    // 2. Initialize config
    // 3. Apply options
    // 4. Validate (collect errors)
    // 5. Deep copy
    // 6. Return
}
```

---

## 3. Technical Design

### 3.1 Design Principles

| Principle | Implementation | Rationale |
|-----------|----------------|-----------|
| **Context-First** | `ctx` always first param | Go idiom for cancellation + metadata |
| **Fail-Fast Validation** | Validate in constructor | Catch errors immediately, not at Build() |
| **Deep Copy** | Clone before return | Prevent external mutation of config |
| **Centralized Validation** | All validation in constructor | Single source of truth, no duplication |
| **Type Safety** | Full type checking | Leverage Go compiler for correctness |
| **Idiomatic Go** | Follow stdlib patterns | Match `context.WithCancel`, `http.ServerOption` |

### 3.2 API Design

#### 3.2.1 Constructor Signature

**Standard Pattern:**
```go
func New(ctx context.Context, id string, opts ...Option) (*engine.Config, error)
```

**Parameters:**
- `ctx context.Context` - Required for logging + cancellation
- `id string` - Primary identifier (required, non-empty)
- `opts ...Option` - Variadic functional options

**Returns:**
- `*engine.Config` - Deep copied configuration
- `error` - Validation errors (may be `*sdkerrors.BuildError` with multiple errors)

#### 3.2.2 Option Function Signature

**Generated Pattern:**
```go
// Option is a functional option for configuring Config
type Option func(*engine.Config)

// WithX sets the X field
//
// [Field documentation from engine struct]
func WithX(x Type) Option {
    return func(cfg *engine.Config) {
        cfg.X = x
    }
}
```

#### 3.2.3 Usage Pattern

**Before (Builder Pattern):**
```go
cfg, err := agent.New("assistant").
    WithInstructions("You are helpful").
    WithModel("openai", "gpt-4").
    AddAction(action).
    AddTool("tool1").
    Build(ctx)
```

**After (Functional Options):**
```go
cfg, err := agent.New(ctx, "assistant",
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
```

**Key Differences:**
1. ✅ `ctx` moved to first parameter
2. ✅ No `.Build(ctx)` call needed
3. ✅ `WithModel()` takes struct, not separate strings
4. ✅ Collection methods use plural names and take slices
5. ✅ Options are passed as variadic arguments

### 3.3 Code Generation Process

#### 3.3.1 Generation Workflow

```
1. Developer adds //go:generate directive
   ↓
2. Developer runs `go generate`
   ↓
3. optionsgen CLI parses engine struct
   ↓
4. Parser extracts field metadata
   ↓
5. Generator creates WithX() functions
   ↓
6. Output written to options_generated.go
   ↓
7. Developer creates constructor.go (one-time)
   ↓
8. Tests verify correctness
```

#### 3.3.2 Generator Flags

| Flag | Purpose | Required | Example |
|------|---------|----------|---------|
| `-engine` | Path to engine struct file | Yes | `../../engine/agent/config.go` |
| `-struct` | Struct name to parse | Yes | `Config` |
| `-output` | Output file path | Yes | `options_generated.go` |
| `-package` | Custom package name | No | `agentaction` |

**Usage Example:**
```bash
go run ../internal/codegen/cmd/optionsgen/main.go \
  -engine ../../engine/agent/config.go \
  -struct Config \
  -output options_generated.go \
  -package agent
```

### 3.4 Validation Strategy

#### 3.4.1 Validation Layers

**Layer 1: Constructor Validation (Required)**
- ID validation (non-empty, valid format)
- Required field presence
- Field-level constraints (ranges, formats)
- Cross-field validation
- Error accumulation (collect all errors)

**Layer 2: Engine Validation (Existing)**
- Business logic validation
- Runtime constraints
- Cross-reference validation

**Layer 3: Project Validation (Existing)**
- Inter-component validation
- Dependency checking
- Configuration consistency

#### 3.4.2 Error Handling Pattern

```go
collected := make([]error, 0)

// Validate ID
if err := validate.ID(ctx, cfg.ID); err != nil {
    collected = append(collected, fmt.Errorf("id is invalid: %w", err))
}

// Validate required fields
cfg.Instructions = strings.TrimSpace(cfg.Instructions)
if err := validate.NonEmpty(ctx, "instructions", cfg.Instructions); err != nil {
    collected = append(collected, err)
}

// Return all errors at once
if len(collected) > 0 {
    return nil, &sdkerrors.BuildError{Errors: collected}
}
```

**Benefits:**
- User sees all validation errors at once
- No iterative fix-test-fix cycles
- Better developer experience

---

## 4. Data Models

### 4.1 Core Types

#### 4.1.1 StructInfo (Parser Output)

```go
type StructInfo struct {
    PackageName    string       // Package containing the struct
    StructName     string       // Name of the struct
    Fields         []FieldInfo  // All exported fields
    EngineFilePath string       // Path to engine file
}
```

#### 4.1.2 FieldInfo (Field Metadata)

```go
type FieldInfo struct {
    Name        string   // Field name (e.g., "Instructions")
    Type        string   // Go type (e.g., "string", "[]tool.Config")
    JSONTag     string   // JSON tag if present
    Doc         string   // Documentation comment
    ImportPath  string   // Import path for external types
    IsSlice     bool     // True if field is a slice
    IsPointer   bool     // True if field is a pointer
    IsEmbedded  bool     // True if from embedded struct
}
```

### 4.2 Generated Types

#### 4.2.1 Option Function Type

```go
// Generated in each package
type Option func(*engine.Config)
```

#### 4.2.2 WithX Functions

```go
// Example for string field
func WithInstructions(instructions string) Option {
    return func(cfg *agent.Config) {
        cfg.Instructions = instructions
    }
}

// Example for slice field
func WithTools(tools []tool.Config) Option {
    return func(cfg *agent.Config) {
        cfg.Tools = tools
    }
}

// Example for struct field
func WithModel(model Model) Option {
    return func(cfg *agent.Config) {
        cfg.Model = model
    }
}
```

---

## 5. Migration Strategy

### 5.1 Package Migration Pattern

#### 5.1.1 Standard Migration Steps

**For each package in sdk/:**

1. **Create `generate.go`** (1 min)
   ```go
   package <package>

   //go:generate go run ../internal/codegen/cmd/optionsgen/main.go -engine ../../engine/<package>/config.go -struct Config -output options_generated.go
   ```

2. **Run Generator** (1 min)
   ```bash
   cd sdk/<package>
   go generate
   ```

3. **Create `constructor.go`** (30-60 min)
   - Read old `sdk/<package>/builder.go` for validation logic
   - Implement `New(ctx, id, ...opts)` function
   - Port validation logic to centralized location
   - Add deep copy before return

4. **Create `constructor_test.go`** (30-60 min)
   - Test minimal configuration
   - Test full configuration with all options
   - Test validation errors
   - Test nil context
   - Test deep copy behavior
   - Test whitespace trimming

5. **Create `README.md`** (15 min)
   - Usage examples
   - Migration guide
   - API documentation

6. **Verify Quality** (5 min)
   ```bash
   golangci-lint run --fix --allow-parallel-runners ./sdk/<package>/...
   gotestsum --format pkgname -- -race -parallel=4 ./sdk/<package>
   ```

**Total Time Per Package:** 1-2 hours (simple) to 4-8 hours (complex)

#### 5.1.2 Special Case: Separate Packages

**When to Use:**
- Option type conflict between packages (e.g., agent.Option vs action.Option)
- Struct naming conflict (e.g., agent.Config vs action.Config)

**Pattern:**
```
sdk/
├── agent/
│   ├── generate.go
│   ├── options_generated.go
│   ├── constructor.go
│   └── constructor_test.go
└── agentaction/  ← Separate package
    ├── generate.go (with -package agentaction flag)
    ├── options_generated.go
    ├── constructor.go
    └── constructor_test.go
```

### 5.2 Phase Execution Plan

#### Phase 1: Foundations (5 packages, parallelizable)

**Packages:** model, schedule, mcp, runtime, memory
**Effort:** 5 hours sequential, 1 hour parallel (5 developers)
**Dependencies:** None (independent packages)
**Success Criteria:** All 5 packages pass lint + tests

#### Phase 2: Components (4 packages, sequential)

**Packages:** tool, schema, workflow, knowledge
**Effort:** 9 hours
**Dependencies:** Requires Phase 1 complete (model package)
**Success Criteria:** All 4 packages pass lint + tests

#### Phase 3: Complex Integration (2 packages, sequential)

**Packages:** task, project
**Effort:** 12 hours
**Dependencies:** Requires Phase 2 complete (workflow, tool)
**Success Criteria:** All packages pass lint + tests, all constructors validated

#### Phase 4: Examples (9 files)

**Files:** `sdk/cmd/*/main.go` (9 examples)
**Effort:** 2 hours
**Dependencies:** Requires Phase 1 complete (can run before Phase 2/3)
**Success Criteria:** All examples compile and run successfully

**Total Effort:** 28 hours single developer, ~10 hours with team

### 5.3 Quality Gates

**Gate 1: Code Generation** (Per package)
- [ ] `go generate` runs without errors
- [ ] `options_generated.go` contains all expected fields
- [ ] Generated code passes `golangci-lint`
- [ ] No manual edits to generated file

**Gate 2: Constructor Implementation** (Per package)
- [ ] Constructor signature matches pattern
- [ ] Context validation present
- [ ] All required fields validated
- [ ] Deep copy implemented
- [ ] Error accumulation (not fail-fast on validation)

**Gate 3: Testing** (Per package)
- [ ] Test coverage ≥95%
- [ ] All validation paths tested
- [ ] Deep copy verified
- [ ] Nil context handled
- [ ] All tests pass with `-race` flag

**Gate 4: Integration** (Per phase)
- [ ] `make lint` passes for entire phase
- [ ] `make test` passes for entire phase
- [ ] No regressions in unrelated packages
- [ ] Documentation updated

---

## 6. Special Cases

### 6.1 Task Package (7+ Variants)

**Challenge:** Multiple task types with different configurations

**Solution:** Separate constructors per type

```go
// sdk/task/constructors.go

func NewBasic(ctx context.Context, id string, agentID string, actionID string, opts ...BasicOption) (*task.BasicTaskConfig, error)

func NewParallel(ctx context.Context, id string, tasks []string, opts ...ParallelOption) (*task.ParallelTaskConfig, error)

func NewCollection(ctx context.Context, id string, items []any, taskTemplate string, opts ...CollectionOption) (*task.CollectionTaskConfig, error)

func NewWait(ctx context.Context, id string, duration string, opts ...WaitOption) (*task.WaitTaskConfig, error)

func NewSignal(ctx context.Context, id string, signalName string, opts ...SignalOption) (*task.SignalTaskConfig, error)

func NewHuman(ctx context.Context, id string, prompt string, opts ...HumanOption) (*task.HumanTaskConfig, error)

func NewConditional(ctx context.Context, id string, condition string, opts ...ConditionalOption) (*task.ConditionalTaskConfig, error)
```

**Generation Strategy:**
```
sdk/task/
├── generate_basic.go       //go:generate ... -struct BasicTaskConfig
├── generate_parallel.go    //go:generate ... -struct ParallelTaskConfig
├── generate_collection.go  //go:generate ... -struct CollectionTaskConfig
├── ... (7+ generate files)
├── basic_options_generated.go
├── parallel_options_generated.go
├── ... (7+ generated files)
├── constructors.go         // All 7+ constructors
└── constructors_test.go    // Tests for all types
```

### 6.2 Schema Package (Hybrid Approach)

**Challenge:** Dynamic schema construction requires builder pattern

**Solution:** Keep `PropertyBuilder` pattern, migrate top-level `Schema` config

**Rationale:**
- PropertyBuilder is used for dynamic schema construction (runtime-dependent)
- Schema configuration (metadata) can use functional options
- Best of both worlds: flexibility + consistency

**API:**
```go
// Keep PropertyBuilder for dynamic schemas (in sdk/schema/)
schema := schema.NewProperty("object").
    AddProperty("name", schema.NewProperty("string")).
    AddProperty("age", schema.NewProperty("integer")).
    Build()

// Use functional options for Schema wrapper (in sdk/schema/)
schemaConfig, err := schema.New(ctx, "user-schema",
    schema.WithTitle("User Schema"),
    schema.WithDescription("Validates user data"),
    schema.WithVersion("1.0.0"),
    schema.WithProperties(schema), // Built with PropertyBuilder
)
```

### 6.3 Knowledge Package (4 Types)

**Challenge:** Multiple configuration types for knowledge system

**Solution:** Separate constructors per type

```go
// sdk/knowledge/constructors.go

func NewBase(ctx context.Context, id string, opts ...BaseOption) (*knowledge.BaseConfig, error)

func NewBinding(ctx context.Context, id string, knowledgeID string, opts ...BindingOption) (*knowledge.BindingConfig, error)

func NewEmbedder(ctx context.Context, id string, provider string, opts ...EmbedderOption) (*knowledge.EmbedderConfig, error)

func NewVectorDB(ctx context.Context, id string, provider string, opts ...VectorDBOption) (*knowledge.VectorDBConfig, error)
```

---

## 7. Testing Strategy

### 7.1 Unit Testing

#### 7.1.1 Constructor Tests (Per Package)

**Test Categories:**

1. **Minimal Configuration**
   ```go
   func TestNew_MinimalConfig(t *testing.T) {
       cfg, err := New(t.Context(), "test-id")
       require.NoError(t, err)
       assert.Equal(t, "test-id", cfg.ID)
   }
   ```

2. **Full Configuration**
   ```go
   func TestNew_FullConfig(t *testing.T) {
       cfg, err := New(t.Context(), "test-id",
           WithField1(value1),
           WithField2(value2),
           // ... all options
       )
       require.NoError(t, err)
       assert.Equal(t, value1, cfg.Field1)
       assert.Equal(t, value2, cfg.Field2)
   }
   ```

3. **Validation Errors**
   ```go
   func TestNew_ValidationErrors(t *testing.T) {
       tests := []struct {
           name    string
           id      string
           opts    []Option
           wantErr string
       }{
           {
               name:    "empty id",
               id:      "",
               wantErr: "id is invalid",
           },
           // ... more cases
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               _, err := New(t.Context(), tt.id, tt.opts...)
               require.Error(t, err)
               assert.Contains(t, err.Error(), tt.wantErr)
           })
       }
   }
   ```

4. **Nil Context**
   ```go
   func TestNew_NilContext(t *testing.T) {
       _, err := New(nil, "test-id")
       require.Error(t, err)
       assert.Contains(t, err.Error(), "context is required")
   }
   ```

5. **Deep Copy**
   ```go
   func TestNew_DeepCopy(t *testing.T) {
       original, err := New(t.Context(), "test-id",
           WithField([]string{"value"}),
       )
       require.NoError(t, err)

       // Modify returned config
       original.Field[0] = "modified"

       // Create new config with same options
       copy, err := New(t.Context(), "test-id",
           WithField([]string{"value"}),
       )
       require.NoError(t, err)

       // Verify independence
       assert.NotEqual(t, original.Field[0], copy.Field[0])
   }
   ```

#### 7.1.2 Code Generation Tests

**Test Categories:**

1. **Parser Tests** (`parser_test.go`)
   - Field discovery accuracy
   - Embedded struct handling
   - Type resolution correctness
   - Comment extraction
   - Unexported field filtering

2. **Generator Tests** (`generator_test.go`)
   - Option function generation
   - Import management
   - Documentation preservation
   - Custom package names
   - Linter compliance

### 7.2 Integration Testing

#### 7.2.1 Example File Validation

**Test each example:**
```bash
cd sdk/cmd/01_simple_workflow
go build -o /dev/null main.go  # Verify compilation
go run main.go  # Verify execution
```

#### 7.2.2 Cross-Package Integration

**Test package combinations:**
```go
func TestIntegration_AgentWithTools(t *testing.T) {
    // Create tool config
    toolCfg, err := tool.New(t.Context(), "test-tool")
    require.NoError(t, err)

    // Use tool in agent
    agentCfg, err := agent.New(t.Context(), "test-agent",
        agent.WithTools([]enginetool.Config{*toolCfg}),
    )
    require.NoError(t, err)
    assert.Len(t, agentCfg.Tools, 1)
}
```

### 7.3 Regression Testing

**Before/After Comparison:**
- [ ] All old SDK tests adapted to new API
- [ ] No functionality loss
- [ ] Performance comparison (build time, runtime)
- [ ] Memory usage comparison

### 7.4 Performance Testing

**Benchmarks:**
```go
func BenchmarkNew_MinimalConfig(b *testing.B) {
    ctx := context.Background()
    for i := 0; i < b.N; i++ {
        _, _ = New(ctx, "test-id")
    }
}

func BenchmarkNew_FullConfig(b *testing.B) {
    ctx := context.Background()
    for i := 0; i < b.N; i++ {
        _, _ = New(ctx, "test-id",
            WithField1(value1),
            WithField2(value2),
            // ... all options
        )
    }
}
```

**Targets:**
- Constructor performance ≥ builder Build()
- Memory allocation ≤ builder pattern
- No regression in integration tests

---

## 8. Documentation Requirements

### 8.1 Per-Package Documentation

**README.md Structure:**

```markdown
# Package <name>

## Overview
[Brief description of package purpose]

## Installation
[Import statement]

## Usage

### Basic Example
[Minimal usage example]

### Full Configuration
[Example with all options]

## API Reference

### Constructor
[Constructor signature + description]

### Options
[List of all WithX() functions with descriptions]

## Migration Guide

### Before (Old SDK)
[Old builder pattern example]

### After (New SDK)
[Functional options example]

### Key Changes
[List of breaking changes]

## Examples
[Links to example files]

## Testing
[How to test configurations]
```

### 8.2 Migration Guide Updates

**Add to `MIGRATION_GUIDE.md`:**
- [ ] Per-package migration instructions
- [ ] Common pitfalls and solutions
- [ ] Before/after code examples
- [ ] Breaking changes summary
- [ ] FAQ section

### 8.3 API Documentation

**godoc Requirements:**
- [ ] Package-level documentation
- [ ] Constructor documentation
- [ ] All WithX() functions documented
- [ ] Examples in godoc
- [ ] Code examples that compile

---

## 9. Quality Standards

### 9.1 Code Quality

**Linter Requirements:**
```bash
# Must pass with zero warnings
golangci-lint run --fix --allow-parallel-runners ./sdk/<package>/...
```

**Enabled Linters:**
- govet
- errcheck
- staticcheck
- unused
- gosimple
- structcheck
- varcheck
- ineffassign
- deadcode
- typecheck
- gocritic

### 9.2 Test Quality

**Coverage Requirements:**
- [ ] Overall coverage ≥95%
- [ ] Constructor coverage 100%
- [ ] All validation paths tested
- [ ] All error conditions tested

**Test Standards:**
- [ ] Use `t.Context()` instead of `context.Background()`
- [ ] Use `require` for fatal errors
- [ ] Use `assert` for non-fatal assertions
- [ ] Table-driven tests for validation
- [ ] Race detector enabled (`-race`)

### 9.3 Performance Standards

**No Regressions:**
- [ ] Constructor ≤ Builder Build() time
- [ ] Memory allocation ≤ builder pattern
- [ ] Build time no increase

**Targets:**
- Constructor execution: < 1µs for minimal config
- Memory allocation: < 5KB per constructor call

### 9.4 Documentation Standards

**Requirements:**
- [ ] All public APIs documented
- [ ] Examples that compile
- [ ] Migration guide complete
- [ ] godoc examples provided

---

## 10. Deployment Plan

### 10.1 Rollout Strategy

**Phase 1: Internal Testing**
1. Complete all 11 package migrations
2. Update all 9 examples
3. Run full test suite
4. Performance benchmarks
5. Internal team review

**Phase 2: Alpha Release**
1. Tag as `v2.0.0-alpha.1`
2. Release notes with migration guide
3. Request community feedback
4. Monitor issue tracker

**Phase 3: Beta Release**
1. Address alpha feedback
2. Fix identified issues
3. Tag as `v2.0.0-beta.1`
4. Extended testing period

**Phase 4: Stable Release**
1. Final review
2. Complete documentation
3. Tag as `v2.0.0`
4. Deprecate SDK v1

### 10.2 Communication Plan

**Announcements:**
- [ ] GitHub release notes
- [ ] Blog post with migration guide
- [ ] Discord/Slack announcement
- [ ] Email to SDK users (if mailing list exists)

**Support:**
- [ ] Migration guide published
- [ ] FAQ document created
- [ ] Example code updated
- [ ] Community support channels prepared

---

## 11. Risk Management

### 11.1 Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Generated code quality issues | High | Low | Comprehensive linter integration, peer review |
| Performance regression | Medium | Medium | Benchmark suite, performance testing |
| Type resolution failures | High | Low | Extensive parser tests, AST validation |
| Deep copy overhead | Low | Low | Use efficient `core.DeepCopy()` |

### 11.2 Schedule Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Complex packages take longer | Medium | High | Phase-based approach, adjust estimates |
| Validation logic missing | High | Medium | Reference old builder.go files thoroughly |
| Test coverage gaps | Medium | Low | Strict coverage requirements per package |

### 11.3 Adoption Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Users resist migration | Medium | Medium | Clear migration guide, backward compat period |
| Breaking changes too disruptive | High | Low | Comprehensive examples, gradual rollout |
| Documentation insufficient | Medium | Low | Multi-format docs (godoc, README, examples) |

---

## 12. Success Criteria

### 12.1 Functional Criteria

- [x] Code generation infrastructure complete
- [x] Agent + AgentAction packages migrated (reference implementations)
- [ ] All 11 packages migrated to sdk/
- [ ] All 9 examples updated and working
- [ ] Zero regressions in functionality
- [ ] All tests pass (`make test`)
- [ ] All linters pass (`make lint`)

### 12.2 Quality Criteria

- [ ] Code reduction: 70-75% achieved
- [ ] Test coverage: ≥95%
- [ ] Linter warnings: 0
- [ ] Performance: No regression
- [ ] Documentation: 100% coverage

### 12.3 Adoption Criteria

- [ ] Migration guide published
- [ ] Examples demonstrate all patterns
- [ ] Community feedback positive
- [ ] Zero critical bugs in first month

---

## 13. Appendix

### 13.1 Package Complexity Matrix

| Package | Fields | Complexity | Estimated Time |
|---------|--------|------------|----------------|
| agent | 14 | High | 4-6 hours |
| agentaction | 12 | Medium | 2-3 hours |
| model | 7 | Low | 1 hour |
| schedule | 4 | Low | 1 hour |
| mcp | 6 | Low | 1 hour |
| runtime | 5 | Low | 1 hour |
| memory | 5 | Low | 1 hour |
| tool | 8 | Medium | 2-3 hours |
| schema | 8 | Medium | 2-3 hours (hybrid) |
| workflow | 10 | Medium | 2-3 hours |
| knowledge | ~20 | Medium | 3-4 hours (4 types) |
| task | ~40 | High | 4-6 hours (7+ types) |
| project | 15+ | High | 6-8 hours |

### 13.2 Technology Stack

**Languages:**
- Go 1.25.2

**Libraries:**
- `go/ast` - AST parsing
- `go/parser` - Go source parsing
- `go/types` - Type checking
- `dave.cheney.net/jennifer` - Code generation
- `github.com/stretchr/testify` - Testing

**Tools:**
- `golangci-lint` - Linting
- `gotestsum` - Test execution
- `go generate` - Code generation trigger

### 13.3 References

**Internal Documents:**
- `sdk/MIGRATION_GUIDE.md` - Migration patterns and guidelines
- `sdk/CODEGEN_COMPARISON.md` - Before/after comparison
- `tasks/prd-sdk-generated/_tasks.md` - Task breakdown

**External Resources:**
- [Go Blog: Self-referential functions and the design of options](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html)
- [Dave Cheney: Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- [Go AST Documentation](https://pkg.go.dev/go/ast)
- [Jennifer Library](https://github.com/dave/jennifer)

---

## Document Change Log

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-10-27 | Engineering Team | Initial technical specification |

---

**Approval Signatures:**

- [ ] Technical Lead: _________________ Date: _______
- [ ] SDK Team Lead: _________________ Date: _______
- [ ] QA Lead: _______________________ Date: _______
- [ ] Documentation Lead: _____________ Date: _______
