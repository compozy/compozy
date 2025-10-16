# Code Reviewer Example

An advanced AI-powered code review system for Go projects using Compozy agents. This example demonstrates how to build specialized code analysis workflows that follow your project's coding standards and best practices.

## Features

### Multi-Type Code Analysis

The code-reviewer system supports eight specialized review types:

1. **Performance Review** (`performance`)
   - Memory allocation analysis and GC pressure
   - Algorithm efficiency and complexity
   - Map operations (proper use of `core.CopyMap`, `core.CloneMap`, etc.)
   - Goroutine management and concurrency patterns
   - Context propagation patterns
   - Database query optimization
   - Caching opportunities
   - Hot path optimizations

2. **Security Review** (`security`)
   - Input validation and sanitization
   - SQL injection risks
   - Authentication and authorization checks
   - Sensitive data exposure
   - Error information leakage
   - Resource exhaustion vulnerabilities
   - Insecure dependencies
   - Proper error handling at boundaries

3. **Monitoring Review** (`monitoring`)
   - Logging patterns (`logger.FromContext` usage)
   - Metrics and instrumentation opportunities
   - Tracing context propagation
   - Error tracking and reporting
   - Performance monitoring points
   - Debug information availability
   - Alert-worthy conditions
   - SLI/SLO measurement points

4. **Architecture Review** (`architecture`)
   - SOLID principles adherence (SRP, OCP, LSP, ISP, DIP)
   - Clean Architecture layer separation
   - Domain boundaries and package organization
   - Dependency injection through constructors
   - Interface design and segregation
   - Coupling and cohesion analysis
   - Proper use of abstractions vs concretions
   - Layer dependency flow validation
   - God objects and tight coupling anti-patterns

5. **Testing Review** (`testing`)
   - Test naming patterns (`t.Run("Should...")`)
   - Test coverage gaps and missing edge cases
   - Anti-patterns (suite patterns, redundant tests, mock-heavy tests)
   - Low-value tests detection
   - Proper use of testify assertions
   - Integration vs unit test appropriateness
   - Test maintainability and readability
   - Weak assertions identification
   - Business logic coverage (>80% target)

6. **Error Handling Review** (`error_handling`)
   - Unified error handling strategy compliance
   - Proper error propagation and wrapping
   - Early returns vs deep nesting
   - Error information leakage
   - Ignored errors detection
   - Naked returns in longer functions
   - Error context and helpful messages
   - Proper use of `fmt.Errorf` with `%w` wrapping
   - Domain boundary error handling

7. **Documentation Comment Review** (`doc_comment`)
   - Missing doc comments on exported functions
   - Missing doc comments on exported types (structs, interfaces)
   - Missing doc comments on exported constants and variables
   - Missing doc comments on exported methods
   - Doc comment quality (clear, concise, 2-4 lines max)
   - Proper doc comment format (starts with element name)
   - Package-level documentation
   - Complex internal functions needing documentation
   - Inappropriate change explanations in comments
   - Technical debt and TODO comments review

8. **README Documentation Review** (`readme`)
   - README accuracy vs actual code structure
   - Missing documentation for files/components
   - Outdated API references and examples
   - Incorrect usage patterns
   - Broken links and references
   - Missing setup instructions
   - Incomplete or outdated examples
   - Documentation structure and completeness
   - Cross-reference verification (docs vs code)
   - Generates `README_REVIEW.md` in `_reviews/` directory

### Rule-Based Analysis

The agent is configured with references to your project's coding standards:

- **Go Coding Standards** (`.cursor/rules/go-coding-standards.mdc`)
  - Function length: 50 lines max for business logic
  - Line length: 120 characters max
  - Cyclomatic complexity: <10
  - Error handling patterns
  - Map operations guidelines
  - Context propagation requirements

- **Architecture Patterns** (`.cursor/rules/architecture.mdc`)
  - SOLID principles
  - Clean Architecture
  - Domain-driven design
  - Dependency injection

- **Testing Standards** (`.cursor/rules/test-standards.mdc`)
  - Test naming conventions
  - Coverage requirements
  - Anti-patterns to avoid

- **API Standards** (`.cursor/rules/api-standards.mdc`)
- **Magic Numbers** (`.cursor/rules/magic-numbers.mdc`)
- **No Backwards Compatibility** (`.cursor/rules/backwards-compatibility.mdc`)

### Workflow Architecture

The code-reviewer system includes two workflows that work together:

#### 1. Review Workflow (`review.yaml`)

Uses an intelligent routing system that optimizes execution based on the review type:

**Router-Based Execution:**

1. **File Discovery**: Lists all `.go` files in the target directory
2. **Smart Routing**: Routes to appropriate execution path based on `review_type` input
3. **Dynamic Execution**:
   - **Single Review Type** (performance, security, monitoring, etc.): Uses dynamic action parameter to execute the specific review type across all files
   - **All Reviews** (default): Runs all seven review types in parallel for each file

**Trigger Support:**

- Can be triggered manually via API
- Can be triggered by signals from the batch review workflow
- Signal payload provides directory and review_type parameters

#### 2. Batch Review Workflow (`review-batch.yaml`)

Enables reviewing multiple directories in parallel using signal-based communication:

**Signal-Based Triggering:**

1. **Input**: Accepts array of directories to review
2. **Collection Processing**: Iterates over each directory in parallel
3. **Signal Emission**: Each iteration sends a `review-directory` signal
4. **Workflow Triggering**: Each signal automatically triggers a new review workflow instance

**Key Benefits:**

- **Efficient**: Single review types use dynamic actions, eliminating code duplication
- **Flexible**: Easy to add new review types without workflow changes
- **Scalable**: Parallel execution maximizes throughput with multiple workflow instances
- **Distributed**: Each directory review runs as an independent workflow
- **Event-Driven**: Signal-based triggering enables loose coupling between workflows

## Quick Start

### Prerequisites

- Compozy installed and running
- Groq API key (or configure a different LLM provider)
- Bun runtime installed

### Setup

1. Set your API key:

```bash
export GROQ_API_KEY="your-groq-api-key"
```

2. Start Compozy server:

```bash
cd examples/code-reviewer
compozy start
```

3. In another terminal, execute a review:

```bash
# Review all aspects (performance + security + monitoring)
curl -X POST http://localhost:5001/api/v0/workflows/review/executions \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "directory": "engine/schema",
      "review_type": "all"
    }
  }'

# Or use the .http file with VS Code REST Client extension
```

## Usage

### Review Types

#### All Reviews (Default)

Runs all eight review types in parallel (performance, security, monitoring, architecture, testing, error_handling, doc_comment, readme):

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/schema",
    "review_type": "all"
  }
}
```

#### Performance Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/agent",
    "review_type": "performance"
  }
}
```

#### Security Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/auth",
    "review_type": "security"
  }
}
```

#### Monitoring Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/workflow",
    "review_type": "monitoring"
  }
}
```

#### Architecture Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/core",
    "review_type": "architecture"
  }
}
```

#### Testing Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/task2",
    "review_type": "testing"
  }
}
```

#### Error Handling Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/llm",
    "review_type": "error_handling"
  }
}
```

#### Documentation Comment Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/infra",
    "review_type": "doc_comment"
  }
}
```

#### README Documentation Review Only

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/agent",
    "review_type": "readme"
  }
}
```

This review type:

- Analyzes the directory structure and all files
- Reads and evaluates the README.md
- Identifies missing documentation, outdated information, and incorrect examples
- Generates a comprehensive review document at `[directory]/_reviews/README_REVIEW.md`

### Batch Review (Signal-Based)

Review multiple directories in parallel by triggering separate workflow instances:

#### Batch All Reviews

```bash
POST /api/v0/workflows/review-batch/executions
{
  "input": {
    "directories": [
      "engine/agent",
      "engine/task",
      "engine/workflow"
    ],
    "review_type": "all"
  }
}
```

This will:

1. Send a signal for each directory
2. Each signal triggers an independent review workflow instance
3. All reviews run in parallel as separate workflows
4. Results are generated independently for each directory

#### Batch Security Review

```bash
POST /api/v0/workflows/review-batch/executions
{
  "input": {
    "directories": [
      "engine/auth",
      "engine/infra/server",
      "engine/webhook"
    ],
    "review_type": "security"
  }
}
```

**Benefits of batch review:**

- Each directory review runs independently
- Failures in one review don't affect others
- Results are isolated and easier to manage
- Scales to many directories efficiently

## Output

Reviews are saved as markdown files in a `reviews/` subdirectory within the analyzed directory. For example, reviewing `engine/schema/cwd.go` produces `engine/schema/reviews/cwd.go.md`.

### Review Report Structure

Each review includes:

- **Summary**: Brief overview of the file's purpose
- **Findings**: Categorized by severity (Critical, High, Medium, Low)
- **Code Examples**: Side-by-side comparisons of current vs. recommended code
- **Rule References**: Links to specific project rules that apply
- **Impact Assessment**: Performance, maintainability, and security implications

Example:

````markdown
# Code Review: cwd.go

**Review Type:** Performance
**Severity:** Medium

## Summary

Brief overview of the file and its purpose

## Findings

### Critical Issues

- [Issue description with rule reference]

### High Priority

- [Issue description]

## Code Examples

```go
// ❌ Current implementation
[problematic code]

// ✅ Recommended fix
[improved code]
```
````

## Rule References

- .cursor/rules/go-coding-standards.mdc#map-operations
- .cursor/rules/architecture.mdc#solid-principles

## Impact Assessment

- Performance impact: [description]
- Maintainability impact: [description]

````

## Configuration

### Custom LLM Provider

Edit `compozy.yaml` to use a different provider:

```yaml
models:
  - provider: openai
    model: gpt-4o
    api_key: "{{ .env.OPENAI_API_KEY }}"
    default: true
````

### Native Tools Configuration

The example uses native file system tools (`cp__list_files`, `cp__read_file`, `cp__write_file`) which are automatically available. Configure the root directory in `compozy.yaml`:

```yaml
runtime:
  type: bun
  native_tools:
    root_dir: ../.. # Adjust to your project root
```

### Autoload Configuration

The agent definitions are automatically loaded from the `agents/` directory:

```yaml
autoload:
  enabled: true
  strict: true
  include:
    - "agents/*.yaml"
  exclude:
    - "**/*~"
    - "**/*.bak"
```

## Project Structure

```
code-reviewer/
├── agents/
│   └── analyzer.yaml          # Agent definition with 3 specialized actions
├── workflows/
│   └── review.yaml            # Workflow with conditional review types
├── compozy.yaml               # Project configuration
├── api.http                   # API test examples
├── entrypoint.ts              # Runtime entry point (minimal)
├── package.json               # Node dependencies
└── README.md                  # This file
```

## Advanced Usage

### Extending the Agent

Add new review types by creating additional actions in `agents/analyzer.yaml`:

```yaml
actions:
  - id: custom_review
    prompt: |-
      Analyze {{ .input.file_path }} for [your specific focus].

      Focus on:
      - [Aspect 1]
      - [Aspect 2]

      Reference: .cursor/rules/[relevant-rule].mdc
```

### Filtering Files

Modify the `cp__list_files` tool parameters in `workflows/review.yaml`:

```yaml
- id: list_files
  type: basic
  tool: cp__list_files
  with:
    dir: "{{ .workflow.input.directory }}"
    exclude: "!*.go" # Only .go files
    # Or use include patterns:
    # include: "**/*_test.go"     # Only test files
```

### Custom Rules Reference

Update the agent's `instructions` section to reference your own coding standards:

```yaml
instructions: |
  **Project Rules Reference:**
  - Custom Standard: ./path/to/your/standard.md
    - [Key points from your standard]
```

## Troubleshooting

### Reviews Not Generated

- Check that the directory path is relative to the `native_tools.root_dir`
- Verify the Groq API key is set correctly
- Check Compozy logs: `compozy logs`

### Empty Review Files

- Increase `tool_execution_timeout` in `compozy.yaml` if reviews are timing out
- Check the LLM provider quota and rate limits

### Permission Errors

Ensure Bun has the necessary permissions in `compozy.yaml`:

```yaml
runtime:
  permissions:
    - --allow-read
    - --allow-write
    - --allow-net
    - --allow-env
```

## Next Steps

- Customize the agent instructions for your specific coding standards
- Add custom review types for your domain-specific requirements
- Integrate with CI/CD pipelines to run automated reviews
- Create aggregation workflows to summarize multiple reviews

## Contributing

Found an issue or have a suggestion? Please open an issue or PR in the main Compozy repository.

## License

This example is part of the Compozy project and follows the same license.
