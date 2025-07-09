# Development Guide

This file provides comprehensive guidance for working with the Compozy codebase, including development commands, standards, and workflow patterns.

<critical>@.cursor/rules/critical-validation.mdc</critical>

## Project Overview

Compozy is a **workflow orchestration engine for AI agents** that enables building AI-powered applications through declarative YAML configuration and a robust Go backend. It integrates with various LLM providers and supports the Model Context Protocol (MCP) for extending AI capabilities.

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

<gograph>
## GoGraph MCP Tools
You have access to GoGraph MCP tools for precise Go code analysis. Use these tools extensively instead of guessing.

## 🔧 Available Tools & Usage Priority

### 🎯 Primary Analysis Tools (Use These First)

- `verify_code_exists` - Verify function/type existence before making suggestions
- `get_package_structure` - Understand package organization and exports
- `get_function_info` - Get detailed function signatures, parameters, and relationships
- `trace_call_chain` - Find API call flows and function dependencies
- `query_dependencies` - Analyze import relationships and package dependencies

### 🔍 Deep Analysis Tools

- `find_implementations` - Locate interface implementations
- `detect_circular_deps` - Identify architectural issues
- `check_test_coverage` - Assess test quality by package
- `detect_code_patterns` - Find design patterns and anti-patterns

### 🚫 What NOT to Use

- `analyze_project` - Project is already analyzed, don't re-run
- `natural_language_query` - Use specific tools instead of NL queries

## 📋 Required Workflow

1. **Before any suggestion**: Use `verify_code_exists` to confirm functions/types exist
2. **For API discovery**: Use `trace_call_chain` from entry points to map call flows
3. **For dependencies**: Use `query_dependencies` to understand package relationships
4. **For structure**: Use `get_package_structure` and `get_function_info` for details

</gograph>

## Architecture & Project Structure

**📁 Complete project structure, technology stack, and architectural patterns:** See @.cursor/rules/project-structure.mdc

## 🚨 CRITICAL: Follow All Development Standards

**📋 MANDATORY: Review and follow ALL established coding standards:**

- **Code Formatting & Line Spacing**: @.cursor/rules/no_linebreaks.mdc - NEVER add blank lines inside function bodies
- **Go Coding Standards**: @.cursor/rules/go-coding-standards.mdc - Function limits, error handling, documentation policy
- **Testing Standards**: @.cursor/rules/testing-standards.mdc - MANDATORY `t.Run("Should...")` pattern, testify usage
- **Test Anti-Patterns**: @.cursor/rules/test-anti-patterns.mdc - CRITICAL anti-patterns to prevent based on codebase analysis
- **Go Implementation Patterns**: @.cursor/rules/go-patterns.mdc - Canonical implementations of architecture principles
- **Architecture Principles**: @.cursor/rules/architecture.mdc - SOLID principles, Clean Architecture, DRY
- **Code Quality & Security**: @.cursor/rules/quality-security.mdc - Linting rules, security requirements
- **Required Libraries**: @.cursor/rules/core-libraries.mdc - Mandatory library choices and usage patterns
- **API Development**: @.cursor/rules/api-standards.mdc - RESTful design, versioning, documentation
- **Code Review Process**: @.cursor/rules/review-checklist.mdc - Pre-review requirements and checklist

## Development Workflow

### Pre-Commit Requirements

**ALWAYS run before committing:**

```bash
make fmt && make lint && make test
```

### Development Process

1. **API changes:** Update Swagger annotations (`swag` comments)
2. **Schema changes:** Create migrations with `make migrate-create name=<name>`
3. **New features:** Include comprehensive tests following @.cursor/rules/testing-standards.mdc
4. **Task completion:** Follow @.claude/commands/task-review.md for mandatory code review workflow via Zen MCP tools
5. **Backwards Compatibility:** See @.cursor/rules/backwards-compatibility.mdc - NOT REQUIRED during development phase

### Key Development Notes

- **Logging:** Use @.cursor/rules/core-libraries.mdc for structured logging patterns
- **Core types:** Use `core.ID` for UUIDs
- **Dependencies:** Mock external dependencies in tests when necessary (see @.cursor/rules/testing-standards.mdc)

## Task Management

For task-based development workflows, see these rule files:

- @.claude/commands/prd-create.md - PRD Creation
- @.claude/commands/prd-techspec.md - Technical Specifications
- @.claude/commands/prd-tasks.md - Task List Generation
- @.claude/commands/task-next.md - Task Next
- @.claude/commands/task-review.md - Task Completion with Zen MCP code review

## Compozy Configuration Examples

For YAML configuration patterns and examples:

- **Project Configuration**: @.cursor/rules/compozy-project-config.mdc - Project setup patterns
- **Task Patterns**: @.cursor/rules/compozy-task-patterns.mdc - Workflow task configurations
- **Agent Configuration**: @.cursor/rules/compozy-agent-config.mdc - AI agent setup patterns
- **Shared Patterns**: @.cursor/rules/compozy-shared-patterns.mdc - MCP, templates, and references
- **Configuration Index**: @.cursor/rules/compozy-examples.mdc - Overview and cross-references

**All rule files are located in `.cursor/rules/` and use semantic XML tags for better context and AI understanding.**

The project uses Go 1.24+ features and requires external dependencies to be mocked in tests when necessary.
