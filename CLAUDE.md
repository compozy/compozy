# Development Guide

This file provides comprehensive guidance for working with the Compozy codebase, including development commands, standards, and workflow patterns.

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
make test-all

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

**📁 Complete project structure, technology stack, and architectural patterns:** See [project-structure.mdc](mdc:.cursor/rules/project-structure.mdc)

## 🚨 CRITICAL: Follow All Development Standards

**📋 MANDATORY: Review and follow ALL established coding standards:**

- **Code Formatting & Line Spacing**: [no_linebreaks.mdc](mdc:.cursor/rules/no_linebreaks.mdc) - NEVER add blank lines inside function bodies
- **Go Coding Standards**: [go-coding-standards.mdc](mdc:.cursor/rules/go-coding-standards.mdc) - Function limits, error handling, documentation policy
- **Testing Standards**: [testing-standards.mdc](mdc:.cursor/rules/testing-standards.mdc) - MANDATORY `t.Run("Should...")` pattern, testify usage
- **Go Implementation Patterns**: [go-patterns.mdc](mdc:.cursor/rules/go-patterns.mdc) - Canonical implementations of architecture principles
- **Architecture Principles**: [architecture.mdc](mdc:.cursor/rules/architecture.mdc) - SOLID principles, Clean Architecture, DRY
- **Code Quality & Security**: [quality-security.mdc](mdc:.cursor/rules/quality-security.mdc) - Linting rules, security requirements
- **Required Libraries**: [core-libraries.mdc](mdc:.cursor/rules/core-libraries.mdc) - Mandatory library choices and usage patterns
- **API Development**: [api-standards.mdc](mdc:.cursor/rules/api-standards.mdc) - RESTful design, versioning, documentation
- **Code Review Process**: [review-checklist.mdc](mdc:.cursor/rules/review-checklist.mdc) - Pre-review requirements and checklist

## Development Workflow

### Pre-Commit Requirements

**ALWAYS run before committing:**

```bash
make fmt && make lint && make test
```

### Development Process

1. **API changes:** Update Swagger annotations (`swag` comments)
2. **Schema changes:** Create migrations with `make migrate-create name=<name>`
3. **New features:** Include comprehensive tests following [testing-standards.mdc](mdc:.cursor/rules/testing-standards.mdc)
4. **Task completion:** Follow [task-completion.mdc](mdc:.cursor/rules/task-completion.mdc) for mandatory code review workflow via Zen MCP tools
5. **Backwards Compatibility:** See [backwards-compatibility.mdc](mdc:.cursor/rules/backwards-compatibility.mdc) - NOT REQUIRED during development phase

### Key Development Notes

- **Logging:** Use [core-libraries.mdc](mdc:.cursor/rules/core-libraries.mdc) for structured logging patterns
- **Core types:** Use `core.ID` for UUIDs, `core.Ref` for polymorphic references
- **Dependencies:** Mock external dependencies in tests when necessary (see [testing-standards.mdc](mdc:.cursor/rules/testing-standards.mdc))

## Task Management

For task-based development workflows, see these rule files:

- [prd-create.mdc](mdc:.cursor/rules/prd-create.mdc) - PRD Creation
- [prd-tech-spec.mdc](mdc:.cursor/rules/prd-tech-spec.mdc) - Technical Specifications
- [task-generate-list.mdc](mdc:.cursor/rules/task-generate-list.mdc) - Task List Generation
- [task-developing.mdc](mdc:.cursor/rules/task-developing.mdc) - Task Development
- [task-completion.mdc](mdc:.cursor/rules/task-completion.mdc) - Task Completion with Zen MCP code review

## Rule Management

The development rules are actively maintained and improved:

- **Rule Management**: [cursor_rules.mdc](mdc:.cursor/rules/cursor_rules.mdc) - Comprehensive guidelines for creating, maintaining, and improving rules

## Compozy Configuration Examples

For YAML configuration patterns and examples:

- **Project Configuration**: [compozy-project-config.mdc](mdc:.cursor/rules/compozy-project-config.mdc) - Project setup patterns
- **Task Patterns**: [compozy-task-patterns.mdc](mdc:.cursor/rules/compozy-task-patterns.mdc) - Workflow task configurations
- **Agent Configuration**: [compozy-agent-config.mdc](mdc:.cursor/rules/compozy-agent-config.mdc) - AI agent setup patterns
- **Shared Patterns**: [compozy-shared-patterns.mdc](mdc:.cursor/rules/compozy-shared-patterns.mdc) - MCP, templates, and references
- **Configuration Index**: [compozy-examples.mdc](mdc:.cursor/rules/compozy-examples.mdc) - Overview and cross-references

**All rule files are located in `.cursor/rules/` and use semantic XML tags for better context and AI understanding.**

The project uses Go 1.24+ features and requires external dependencies to be mocked in tests when necessary.
