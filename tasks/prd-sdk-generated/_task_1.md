## status: completed

<task_context>
<domain>sdk/model</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 1.0: Migrate model Package to Functional Options

## Overview

Migrate the `sdk/model` package from manual builder pattern to auto-generated functional options. The model package configures LLM provider settings (OpenAI, Anthropic, Google, etc.) with parameters like temperature, max_tokens, and top_p.

**Estimated Time:** 1-2 hours

<critical>
- **ALWAYS READ** @sdk/MIGRATION_GUIDE.md before starting
- **ALWAYS READ** @sdk/CODEGEN_COMPARISON.md for patterns
- **ALWAYS USE** @sdk/agent/ and @sdk/agentaction/ as reference implementations
- **GREENFIELD APPROACH:** Delete old builder files immediately, no backwards compatibility
</critical>

<requirements>
- Generate functional options from engine/core/provider_config.go
- Create constructor with centralized validation
- Validate provider enum (OpenAI, Anthropic, Google, Groq, etc.)
- Validate parameter ranges (temperature 0-2, top_p 0-1, etc.)
- Deep copy configuration before returning
- Comprehensive test coverage (>90%)
- Pass all linter checks
</requirements>

## Subtasks

- [x] 1.1 Create sdk/model/ directory structure
- [x] 1.2 Create generate.go with go:generate directive
- [x] 1.3 Run go generate to create options_generated.go
- [x] 1.4 Create constructor.go with validation logic
- [x] 1.5 Create constructor_test.go with comprehensive tests
- [x] 1.6 Run linter and tests to verify implementation
- [x] 1.7 Create README.md documenting new API

## Implementation Details

### Engine Source
```go
// From engine/core/provider_config.go
type ProviderConfig struct {
    Provider string                 // "openai", "anthropic", "google", "groq", etc.
    Model    string                 // Model identifier
    Params   map[string]interface{} // Provider-specific parameters
}
```

### Generated Options (7 fields expected)
- WithProvider(provider string) Option
- WithModel(model string) Option
- WithParams(params map[string]interface{}) Option
- WithTemperature(temperature float64) Option
- WithMaxTokens(maxTokens int) Option
- WithTopP(topP float64) Option
- WithTopK(topK int) Option

### Constructor Pattern
```go
func New(ctx context.Context, provider string, model string, opts ...Option) (*core.ProviderConfig, error) {
    if ctx == nil {
        return nil, fmt.Errorf("context is required")
    }

    cfg := &core.ProviderConfig{
        Provider: strings.TrimSpace(provider),
        Model:    strings.TrimSpace(model),
        Params:   make(map[string]interface{}),
    }

    // Apply options
    for _, opt := range opts {
        opt(cfg)
    }

    // Validate provider enum
    // Validate parameter ranges
    // Deep copy and return
}
```

### Relevant Files

**Reference (for understanding):**
- `sdk/model/builder.go` (~257 lines) - Old builder pattern to understand requirements
- `sdk/model/builder_test.go` - Old tests to understand test cases
- `engine/core/provider_config.go` - Source struct for generation

**To Create in sdk/model/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated (DO NOT EDIT)
- `constructor.go` - Validation logic (~80 lines)
- `constructor_test.go` - Test suite (~300 lines)
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/model/` - keep for reference and backwards compatibility during transition

### Dependent Files

**None** - This is a foundation package with no SDK dependencies

## Deliverables

- [x] `sdk/model/` directory created
- [x] `sdk/model/generate.go` created
- [x] `sdk/model/options_generated.go` generated successfully
- [x] `sdk/model/constructor.go` with validation implemented
- [x] `sdk/model/constructor_test.go` with >90% coverage
- [x] `sdk/model/README.md` with usage examples
- [x] All tests passing: `gotestsum -- -race -parallel=4 ./sdk/model`
- [x] Linter passes: `golangci-lint run --fix ./sdk/model/...`

## Tests

Unit tests must cover:

- [x] Minimal configuration (provider + model only)
- [x] Full configuration with all options
- [x] Context validation (nil context fails)
- [x] Provider validation (empty fails, invalid enum fails)
- [x] Model validation (empty fails)
- [x] Parameter range validation:
  - [x] Temperature: 0-2 range
  - [x] TopP: 0-1 range
  - [x] MaxTokens: positive integers
- [x] Whitespace trimming for strings
- [x] Deep copy verification
- [x] Multiple error accumulation
- [x] Provider enum validation (openai, anthropic, google, groq, deepseek, etc.)

## Success Criteria

- [x] Code generation produces 7+ option functions (10 functions generated)
- [x] Constructor validates provider against known list
- [x] Parameter ranges are enforced
- [x] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./model`
- [x] Linter passes: `golangci-lint run --fix --allow-parallel-runners ./model/...`
- [x] Code reduction: ~257 LOC → ~130 LOC manual code (50% reduction)
- [x] Zero manual maintenance when engine adds new parameters

## Reference

Use these as templates:
- Constructor pattern: `sdk/agent/constructor.go`
- Test patterns: `sdk/agent/constructor_test.go`
- Generate directive: `sdk/agentaction/generate.go`
- README structure: `sdk/agentaction/README.md`
