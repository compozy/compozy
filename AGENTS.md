# Development Guide

This file provides comprehensive guidance for working with the Compozy codebase, including development commands, standards, and workflow patterns.

<critical>
- **MANDOTORY READ THIS FILES** before proceed
  - Go coding standards: [go-coding-standards.mdc](.cursor/rules/go-coding-standards.mdc)
  - Architecture patterns: [architecture.mdc](.cursor/rules/architecture.mdc)
  - Testing requirements: [test-standards.mdc](.cursor/rules/test-standards.mdc)
  - No Backwards Compatibility: [backwards-compatibility.mdc](.cursor/rules/backwards-compatibility.mdc)
  - Constants & magic numbers: [magic-numbers.mdc](.cursor/rules/magic-numbers.mdc)
  - API standards: [api-standards.mdc](.cursor/rules/api-standards.mdc)
  - Global Config: [global-config.mdc](.cursor/rules/global-config.mdc)
  - Logger Config: [logger-config.mdc](.cursor/rules/logger-config.mdc)
  - Magic Numbers: [magic-numbers.mdc](.cursor/rules/magic-numbers.mdc)
  - No Line Breaks: [no-linebreaks.mdc](.cursor/rules/no-linebreaks.mdc)

**Quick Read Command:**
Use this one-liner to read all at once:

```bash
cat .cursor/rules/{go-coding-standards,architecture,test-standards,backwards-compatibility,magic-numbers,api-standards,global-config,logger-config,no-linebreaks}.mdc
```

- **MANDATORY REQUIREMENTS:**
  - **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
  - **ALWAYS** verify against PRD and tech specs - NEVER make assumptions (if applied)
  - **NEVER** use workarounds, especially in tests - implement proper solutions
  - **NEVER** create functions with more than 50 lines of code, always prefer to create smaller functions (PRIORITY HIGH)
  - **MUST** use `logger.FromContext(ctx)` - NEVER pass a logger as a parameter or via DI
  - **MUST** use `config.FromContext(ctx)` to read configuration in all code paths
  - **NEVER** use any global configuration singleton.
  - **MUST** inherit context properly - NEVER use context.Background() in runtime code paths
  - **MUST** run `make lint` and `make test` before completing ANY subtask
  - **ALWAYS CHECK** the .cursor/rules/zen-mcp-tools.mdc if you are using Zen MCP tools
  - **ALWAYS CHECK** the .cursor/rules/test-standards.mdc if you are writing tests
  - **YOU CAN ONLY** finish a task if `make lint` and `make tests` are passing, your task should not finish before this
  - **MUST** inherit context properly - NEVER use context.Background() in runtime code paths
  - **YOU MUST NEED** to use perplexity and context7 to get updated information when dealing with external libraries or tools and **NEVER RELY** only in the models data

### For complex/big tasks

- **YOU MUST** use Zen MCP (with Gemini 2.5 Pro) debug, analyze or tracer complex flow **BEFORE INITIATE A TASK**
- **YOU MUST** use Zen MCP (with Gemini 2.5 Pro and O3) codereview tool **AFTER FINISH A TASK**
- **YOU MUST ALWAYS** show all recommendations/issues from a Zen MCP review, does not matter if they are related to your task or not, you **NEED TO ALWAYS** show them.

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Project Overview

Compozy is a **Next-level Agentic Orchestration Platform** that enables building AI-powered applications through declarative YAML configuration and a robust Go backend. It integrates with various LLM providers and supports the Model Context Protocol (MCP) for extending AI capabilities.

## Development Commands

### Essential Commands

```bash
# Quick setup
make deps && make start-docker && make migrate-up
# Start development server with hot reload
make dev
# Run tests (excludes E2E/slow tests)
make test
# Run all tests including E2E
make test
# Format and lint code (ALWAYS run before committing)
make fmt && make lint
# Run specific test
go test -v ./engine/task -run TestExecutor_Execute
```

### Database Commands

```bash
make migrate-up     # Apply migrations
make migrate-down   # Rollback last migration
make migrate-status # Check migration status
make reset-db       # Reset database completely
```

## Architecture & Project Structure

**📁 Complete project structure, technology stack, and architectural patterns:** See .cursor/rules/project-structure.mdc

## Compozy Configuration Examples

For YAML configuration patterns and examples you can see .cursor/rules/compozy/\*.mdc files

## Rules Files

**All rule files are located in `.cursor/rules/` and use semantic XML tags for better context and AI understanding.**
