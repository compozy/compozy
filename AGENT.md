# Compozy Agent Instructions

## Build & Test Commands
- `make test` - Run tests (excludes slow integration/worker tests)  
- `make test-all` - Run all tests including integration
- `go test -run TestSpecificTest ./path/to/package` - Run single test
- `make lint` - Run linter with fixes
- `make build` - Build binary to bin/compozy
- `make swagger` - Generate API docs

## Code Standards
- Go 1.24.3, follow `.golangci.yml` linter config
- Max 80 lines/50 statements per function, 120 char line limit
- Error handling: `fmt.Errorf("failed to X: %w", err)` with context
- Interface-first design with constructor pattern
- Use `context.Context` as first param for external calls
- Tests: `t.Run("Should...")` with testify assertions
- Always use 10s at most for timeouts when running tests commands

## Style Rules (from .cursor/rules)
- No extra line breaks in small functions  
- Section comments: `// -----...-----` (77 chars)
- DON'T add function comments unless complex
- Use dependency injection through constructors
