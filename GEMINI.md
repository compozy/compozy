# Development Guide

This file provides comprehensive guidance for working with the Compozy codebase, including development commands, standards, and workflow patterns.

<critical>@.cursor/rules/critical-validation.mdc</critical>
<critical>**YOU MUST ALWAYS** show all recommendations/issues from a Zen MCP review, does not matter if they are related to your task or not, you **NEED TO ALWAYS** show them.</critical>

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

**📁 Complete project structure, technology stack, and architectural patterns:** See @.cursor/rules/project-structure.mdc

## 🚨 CRITICAL: Follow All Development Standards

**📋 MANDATORY: Review and follow ALL established coding standards:**

- **Code Formatting & Line Spacing**: @.cursor/rules/no_linebreaks.mdc - NEVER add blank lines inside function bodies
- **Go Coding Standards**: @.cursor/rules/go-coding-standards.mdc - Function limits, error handling, documentation policy
- **Testing Standards**: @.cursor/rules/test-standard.mdc - MANDATORY `t.Run("Should...")` pattern, testify usage
- **Go Implementation Patterns**: @.cursor/rules/go-patterns.mdc - Canonical implementations of architecture principles
- **GoGraph**: @.cursor/rules/go-graph.mdc - GoGraph is a tool that helps you to understand the dependencies between your codebase.
- **Architecture Principles**: @.cursor/rules/architecture.mdc - SOLID principles, Clean Architecture, DRY
- **Code Quality & Security**: @.cursor/rules/quality-security.mdc - Linting rules, security requirements
- **Required Libraries**: @.cursor/rules/core-libraries.mdc - Mandatory library choices and usage patterns
- **API Development**: @.cursor/rules/api-standards.mdc - RESTful design, versioning, documentation
- **Code Review Process**: @.cursor/rules/review-checklist.mdc - Pre-review requirements and checklist

## Development Workflow

- **New features:** Include comprehensive tests following @.cursor/rules/test-standard.mdc
- **Task completion:** Follow @.claude/commands/task-review.md for mandatory code review workflow via Zen MCP tools
- **Backwards Compatibility:** See @.cursor/rules/backwards-compatibility.mdc - NOT REQUIRED during development phase

## Task Management

For task-based development workflows, see these rule files:

- .claude/commands/prd-create.md - PRD Creation
- .claude/commands/prd-techspec.md - Technical Specifications
- .claude/commands/prd-tasks.md - Task List Generation
- .claude/commands/task-next.md - Task Next
- .claude/commands/task-review.md - Task Completion with Zen MCP code review

## Compozy Configuration Examples

For YAML configuration patterns and examples:

- **Project Configuration**: .cursor/rules/compozy-project-config.mdc - Project setup patterns
- **Task Patterns**: .cursor/rules/compozy-task-patterns.mdc - Workflow task configurations
- **Agent Configuration**: .cursor/rules/compozy-agent-config.mdc - AI agent setup patterns
- **Shared Patterns**: .cursor/rules/compozy-shared-patterns.mdc - MCP, templates, and references
- **Configuration Index**: .cursor/rules/compozy-examples.mdc - Overview and cross-references

**All rule files are located in `.cursor/rules/` and use semantic XML tags for better context and AI understanding.**

The project uses Go 1.24+ features and requires external dependencies to be mocked in tests when necessary.
