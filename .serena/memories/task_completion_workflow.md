# Task Completion Workflow

## What to do when a task is completed

### 1. Code Quality Checks (MANDATORY)

```bash
make fmt && make lint # Format and lint code
```

### 2. Testing

```bash
make test          # Run all tests
make test-coverage # Run with coverage if needed
```

### 3. Build Verification

```bash
make build # Ensure build works
```

### 4. Schema Generation (if applicable)

```bash
make schemagen # Update schemas if needed
```

### 5. Documentation Updates (if applicable)

```bash
make swagger # Update API documentation
```

### 6. Git Workflow

```bash
git add .
git commit -m "descriptive commit message"
# Follow conventional commit format if applicable
```

### 7. Final Verification

- Ensure all tests pass
- Ensure linting passes
- Check that changes align with project standards
- Verify backwards compatibility if required

## Code Review Process

- Use Zen MCP for comprehensive code review
- Review architectural compliance
- Check security implications
- Validate performance impact
- Ensure proper error handling
- Verify test coverage

## Critical Requirements

- **ALWAYS** use RepoPrompt for complex analysis
- **ALWAYS** use Serena MCP for code discovery and editing
- **ALWAYS** use Zen MCP (with Gemini 2.5 Pro) for debugging and complex flow analysis
- **ALWAYS** show all recommendations/issues from Zen MCP reviews regardless of relevance to current task
