## status: pending

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
- **ALWAYS READ** @sdk2/MIGRATION_GUIDE.md before starting
- **ALWAYS READ** @sdk2/CODEGEN_COMPARISON.md for patterns
- **ALWAYS USE** @sdk2/agent/ and @sdk2/agentaction/ as reference implementations
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

- [ ] 1.1 Create sdk2/model/ directory structure
- [ ] 1.2 Create generate.go with go:generate directive
- [ ] 1.3 Run go generate to create options_generated.go
- [ ] 1.4 Create constructor.go with validation logic
- [ ] 1.5 Create constructor_test.go with comprehensive tests
- [ ] 1.6 Run linter and tests to verify implementation
- [ ] 1.7 Create README.md documenting new API

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

**To Create in sdk2/model/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated (DO NOT EDIT)
- `constructor.go` - Validation logic (~80 lines)
- `constructor_test.go` - Test suite (~300 lines)
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/model/` - keep for reference and backwards compatibility during transition

### Dependent Files

**None** - This is a foundation package with no SDK dependencies

## Deliverables

- [ ] `sdk2/model/` directory created
- [ ] `sdk2/model/generate.go` created
- [ ] `sdk2/model/options_generated.go` generated successfully
- [ ] `sdk2/model/constructor.go` with validation implemented
- [ ] `sdk2/model/constructor_test.go` with >90% coverage
- [ ] `sdk2/model/README.md` with usage examples
- [ ] All tests passing: `gotestsum -- -race -parallel=4 ./sdk2/model`
- [ ] Linter passes: `golangci-lint run --fix ./sdk2/model/...`

## Tests

Unit tests must cover:

- [ ] Minimal configuration (provider + model only)
- [ ] Full configuration with all options
- [ ] Context validation (nil context fails)
- [ ] Provider validation (empty fails, invalid enum fails)
- [ ] Model validation (empty fails)
- [ ] Parameter range validation:
  - [ ] Temperature: 0-2 range
  - [ ] TopP: 0-1 range
  - [ ] MaxTokens: positive integers
- [ ] Whitespace trimming for strings
- [ ] Deep copy verification
- [ ] Multiple error accumulation
- [ ] Provider enum validation (openai, anthropic, google, groq, deepseek, etc.)

## Success Criteria

- [ ] Code generation produces 7+ option functions
- [ ] Constructor validates provider against known list
- [ ] Parameter ranges are enforced
- [ ] All tests pass: `gotestsum --format pkgname -- -race -parallel=4 ./model`
- [ ] Linter passes: `golangci-lint run --fix --allow-parallel-runners ./model/...`
- [ ] Code reduction: ~257 LOC → ~80 LOC manual code (70% reduction)
- [ ] Zero manual maintenance when engine adds new parameters

## Reference

Use these as templates:
- Constructor pattern: `sdk2/agent/constructor.go`
- Test patterns: `sdk2/agent/constructor_test.go`
- Generate directive: `sdk2/agentaction/generate.go`
- README structure: `sdk2/agentaction/README.md`
