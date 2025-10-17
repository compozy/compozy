# Code Reviewer Example

An advanced AI-powered code review system for Go projects using Compozy agents. This example demonstrates how to build specialized code analysis workflows that follow your project's coding standards and best practices.

## Features

### Multi-Type Code Analysis

The code-reviewer system supports eight specialized review types:

#### Analysis Agents (Generate Reports)

These agents analyze code and generate detailed review reports:

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

#### Documentation Agents (Modify Files/Generate Reports)

These specialized agents work with documentation:

7. **Documentation Comment Agent** (`doc_comment`) - **Modifies Files Directly**
   - **Adds doc comments** to exported functions, types, constants, variables, and methods
   - Follows Go documentation standards (2-4 lines max)
   - Starts doc comments with element name
   - Skips trivial getters/setters and obvious constructors
   - **Output**: Modified source files with added documentation
   - **Behavior**: Unlike other review types, this agent updates files in-place

8. **README Documentation Review** (`readme`) - **Generates Report**
   - README accuracy vs actual code structure
   - Missing documentation for files/components
   - Outdated API references and examples
   - Incorrect usage patterns
   - Broken links and references
   - Missing setup instructions
   - Incomplete or outdated examples
   - Documentation structure and completeness
   - Cross-reference verification (docs vs code)
   - **Output**: Generates review report in `ai-docs/reviews/readme/`

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

The code-reviewer system includes two workflows and three specialized agents:

#### Agents

1. **Analyzer Agent** (`analyzer.yaml`)
   - Handles 6 review types: performance, security, monitoring, architecture, testing, error_handling
   - Generates detailed markdown reports in `ai-docs/reviews/<type>/`
   - Uses dynamic actions to avoid code duplication

2. **Doc Comment Agent** (`doc_comment.yaml`)
   - Adds doc comments to Go source files
   - **Modifies files directly** (unlike other agents)
   - Follows Go documentation standards (2-4 lines max)
   - Skips trivial elements

3. **README Reviewer Agent** (`readme_reviewer.yaml`)
   - Analyzes directory structure and README documentation
   - Identifies missing, outdated, or incorrect documentation
   - Generates comprehensive review reports

#### 1. Review Workflow (`review.yaml`)

Uses an intelligent routing system that optimizes execution based on the review type:

**Router-Based Execution:**

1. **File Discovery**: Lists all `.go` files in the target directory
2. **Smart Routing**: Routes to appropriate execution path based on `review_type` input
3. **Dynamic Execution**:
   - **Analysis Reviews** (performance, security, monitoring, architecture, testing, error_handling): Uses analyzer agent with dynamic action parameter
   - **Doc Comment Review**: Uses doc_comment agent to modify files directly
   - **README Review**: Uses readme_reviewer agent to analyze documentation
   - **All Reviews** (default): Runs all review types in parallel

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
- [Ollama](https://ollama.ai) installed and running locally
- Bun runtime installed

> **Note:** Ollama must be running locally with the required model pulled. By default, this example uses `llama3.1:70b`, but you can use any model you have locally.

### Setup

1. Install and start Ollama:

```bash
# Install Ollama (if not already installed)
# Visit: https://ollama.ai

# Pull the model
ollama pull llama3.1:70b

# Verify Ollama is running
ollama list
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

Runs all eight review types in parallel:

- **6 Analysis Reviews**: performance, security, monitoring, architecture, testing, error_handling (generate reports)
- **1 Documentation Agent**: doc_comment (modifies files)
- **1 README Review**: readme (generates report)

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/schema",
    "review_type": "all"
  }
}
```

**⚠️ Note:** When running "all" reviews, the doc_comment agent will modify your source files. Ensure you have uncommitted changes in version control to review the modifications.

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

**⚠️ Note:** This review type **modifies files directly** by adding doc comments.

```bash
POST /api/v0/workflows/review/executions
{
  "input": {
    "directory": "engine/infra",
    "review_type": "doc_comment"
  }
}
```

This review type:

- **Reads all Go files** in the specified directory
- **Identifies undocumented exported elements** (functions, types, methods, constants, variables)
- **Adds concise doc comments** (2-4 lines max) directly to the source files
- **Skips trivial elements** (obvious getters/setters, simple constructors)
- **Preserves formatting** and maintains code structure
- **Returns summary** of how many doc comments were added per file

**⚠️ Important:** Unlike other review types that generate reports, this agent modifies your source files. Use version control to review changes before committing.

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
- Generates a comprehensive review document at `ai-docs/reviews/readme/<num>_README_REVIEW.md`

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

### Analysis Reports (Most Review Types)

Code analysis reviews are saved centrally in the `ai-docs/reviews/<review_type>/` directory from the project root. Each review file is named with a sequential number and the full file path (sanitized):

**Naming Format:** `<num>_<full_path_sanitized>.md`

**Examples:**

- `engine/schema/cwd.go` → `ai-docs/reviews/performance/001_engine_schema_cwd.go.md`
- `engine/agent/uc/create.go` → `ai-docs/reviews/security/003_engine_agent_uc_create.go.md`
- README review → `ai-docs/reviews/readme/001_engine_agent_README_REVIEW.md`

### File Modifications (Doc Comment Review)

The `doc_comment` review type **does not generate reports**. Instead, it:

- **Modifies source files directly** by adding doc comments
- **Updates files in-place** at their original locations
- **Returns console output** summarizing changes made

**Example output:**

```
✓ Doc comments added to: engine/agent/service.go
  - Documented 5 functions
  - Documented 2 types
  - Documented 1 interface
  - Total: 8 doc comments added
```

**⚠️ Important:** Use `git diff` to review changes after running doc_comment reviews:

```bash
git diff engine/agent/service.go
```

### Review Report Structure

Each review includes:

- **File Path**: Full path of the reviewed file
- **Review Type**: Type of analysis performed
- **Summary**: Brief overview of the file's purpose
- **Findings**: Categorized by severity with emoji indicators
  - 🔴 **Critical Issues**: Must fix immediately
  - 🟠 **High Priority Issues**: Significant problems
  - 🟡 **Medium Priority Issues**: Important improvements
  - 🔵 **Low Priority / Suggestions**: Nice-to-have enhancements
- **Inline Code Examples**: Each issue includes before/after code snippets
- **Rule References**: Links to specific project rules
- **Impact Assessment**: Performance, maintainability, security implications
- **Recommendations**: Prioritized action items
- **Positive Aspects**: What the code does well

Example:

````markdown
# Code Review: cwd.go

**File Path:** `engine/schema/cwd.go`
**Review Type:** Performance
**Review Date:** 2024-10-16
**Severity:** Medium

## Summary

Brief overview of the file and its purpose

## Findings

### 🔴 Critical Issues

- **[Issue Title]**
  - **Problem**: [Clear description]
  - **Impact**: [Why this is critical]
  - **Fix**: [Specific recommendation]
  - **Rule Reference**: `.cursor/rules/go-coding-standards.mdc`

  ```go
  // ❌ Current implementation
  [problematic code from the file]

  // ✅ Recommended fix
  [improved code with explanation]
  ```

### 🟠 High Priority Issues

- **[Issue Title]**
  - **Problem**: [Clear description]
  - **Impact**: [Why this matters]
  - **Fix**: [Specific recommendation]

  ```go
  // ❌ Current implementation
  [problematic code]

  // ✅ Recommended fix
  [improved code]
  ```

## Rule References

- `.cursor/rules/go-coding-standards.mdc`: Map operations, Context propagation
- `.cursor/rules/architecture.mdc`: SOLID principles

## Impact Assessment

- **Performance Impact**: [How this affects runtime]
- **Maintainability Impact**: [How this affects maintenance]

## Recommendations

### Immediate Actions (Critical/High Priority)

1. [Most urgent fix]
2. [Second priority]

### Short-term Improvements (Medium Priority)

1. [Important enhancement]

## Positive Aspects

- [What the code does well]
- [Good practices to maintain]
````

## Configuration

### Custom LLM Provider

This example uses Ollama by default, but you can configure any LLM provider.

**Using a different Ollama model:**

```yaml
models:
  - provider: ollama
    model: codellama:34b # Or any other local model
    # api_url: http://localhost:11434  # Optional: defaults to localhost:11434
    default: true
```

**Using a cloud provider (OpenAI, Anthropic, Groq, etc.):**

```yaml
models:
  - provider: openai
    model: gpt-4o
    api_key: "{{ .env.OPENAI_API_KEY }}"
    default: true
```

Available providers: `ollama`, `openai`, `anthropic`, `google`, `groq`, `deepseek`, `xai``

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
│   ├── analyzer.yaml          # Code analysis agent (6 review types)
│   ├── doc_comment.yaml       # Documentation agent (adds doc comments)
│   └── readme_reviewer.yaml   # README analysis agent
├── workflows/
│   ├── review.yaml            # Main review workflow with routing
│   └── review-batch.yaml      # Batch review workflow (multiple dirs)
├── compozy.yaml               # Project configuration
├── api.http                   # API test examples
├── entrypoint.ts              # Runtime entry point (minimal)
├── package.json               # Node dependencies
└── README.md                  # This file
```

## Advanced Usage

### Using Doc Comment Agent Effectively

The doc_comment agent modifies files directly, so use it carefully:

**Best Practices:**

1. **Use Version Control**: Always commit your work before running doc_comment reviews

   ```bash
   git commit -am "Before doc comment additions"
   compozy workflow execute review --directory "engine/agent" --review-type "doc_comment"
   git diff # Review changes
   ```

2. **Start Small**: Test on a single package first

   ```bash
   # Test on one package
   --directory "engine/core"
   
   # If results are good, expand to larger directories
   --directory "engine"
   ```

3. **Review Changes**: Always review generated doc comments before committing

   ```bash
   git diff engine/agent/service.go
   # Edit any comments that need adjustment
   git add -p # Stage changes selectively
   ```

4. **Combine with Analysis**: Run doc_comment first, then run other reviews

   ```bash
   # Step 1: Add documentation
   --review-type "doc_comment"
   
   # Step 2: Run analysis reviews
   --review-type "all" # This includes doc_comment again
   ```

5. **Skip Trivial Packages**: Don't run on packages with obvious functionality
   - Simple utility packages
   - Test helpers
   - Generated code

### Extending the Agents

#### Add New Analysis Review Types

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

#### Create Custom Documentation Agents

Create specialized documentation agents similar to `doc_comment.yaml`:

```yaml
resource: agent
id: my_custom_doc_agent
description: Custom documentation agent
version: 1.0.0

instructions: |
  # Your custom instructions

actions:
  - id: add_custom_docs
    prompt: |-
      # Your custom prompt
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
- Verify Ollama is running: `ollama list`
- Ensure the model is pulled: `ollama pull llama3.1:70b`
- Check Compozy logs: `compozy logs`

### Empty Review Files

- Increase `tool_execution_timeout` in `compozy.yaml` if reviews are timing out
- For Ollama: Ensure you have enough RAM/VRAM for the model size
- For cloud providers: Check the LLM provider quota and rate limits
- Consider using a smaller model if running into resource constraints (e.g., `llama3.1:8b`)

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

```

```
