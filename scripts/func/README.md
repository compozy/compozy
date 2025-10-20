# Function Length Checker

This script analyzes the entire Go codebase to identify functions that exceed the 50-line limit as defined in the project's coding standards.

## Usage

### Check entire codebase from project root:

```bash
go run scripts/func/check-function-length.go
```

### Check specific directory:

```bash
go run scripts/func/check-function-length.go ./engine/agent
```

## Features

- ✅ Parses Go source files using Go's standard `go/parser` and `go/ast` packages
- ✅ Counts function lines including both regular functions and methods
- ✅ Excludes test files (`*_test.go`)
- ✅ Skips vendor, node_modules, and build directories
- ✅ Reports functions sorted by line count (highest first)
- ✅ Shows file path, function name, line count, and how much it exceeds the limit

## Output Example

```
Found 5 functions with more than 50 lines:

📄 engine/agent/service.go:42
   Function: (*Service).Execute
   Lines: 78 (exceeds limit by 28)

📄 engine/workflow/executor.go:120
   Function: executeTask
   Lines: 65 (exceeds limit by 15)

Total violations: 5
```

## Exit Codes

- `0`: No violations found (all functions are within limit)
- `1`: Violations found or error occurred

## Integration with CI/CD

Add to your Makefile or CI pipeline:

```makefile
check-func-length:
	@go run scripts/func/check-function-length.go
```

## Technical Details

The script uses:

- `go/parser`: Parses Go source code into an Abstract Syntax Tree (AST)
- `go/ast`: Provides AST node inspection and traversal
- `token.FileSet`: Tracks position information for calculating line counts

Line counting methodology:

- Counts from function declaration start to function body end
- Includes function signature, body, comments within the function
- Excludes comments before the function declaration
