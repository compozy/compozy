# Compozy Development Guide

Compozy is a **workflow orchestration engine for AI agents, tasks, and tools** that enables developers to build sophisticated AI-powered applications through declarative YAML configuration and a robust Go backend.

## Project Vision & Goals

### Core Mission

- **Simplify AI Workflow Orchestration**: Provide a declarative approach to building complex AI agent workflows
- **Enable Multi-Agent Collaboration**: Support parallel and sequential task execution with proper dependency management
- **Runtime Flexibility**: Execute tools and agents in secure, isolated Deno environments
- **Developer Experience**: Offer comprehensive APIs, clear documentation, and robust development tools
- **Production Ready**: Built for scalability with enterprise-grade infrastructure (PostgreSQL, Temporal, NATS)

### Key Features

- **Declarative Workflows**: Define complex AI workflows using YAML configuration
- **Multi-Runtime Support**: Execute tools in Deno with configurable permissions
- **Temporal Integration**: Reliable workflow orchestration with built-in retry and error handling
- **RESTful API**: Comprehensive API for workflow management and monitoring
- **Real-time Monitoring**: Event streaming and execution tracking via NATS
- **Schema Validation**: Type-safe configurations with JSON Schema validation

## Architecture Overview

### Core Components

```
compozy/
├── engine/           # Core domain logic and business rules
│   ├── agent/        # AI agent management and execution
│   ├── task/         # Task orchestration and lifecycle
│   ├── tool/         # Tool execution and management
│   ├── workflow/     # Workflow definition and execution
│   ├── runtime/      # Deno runtime integration for tool execution
│   ├── core/         # Shared domain models and utilities
│   ├── infra/        # Infrastructure layer (server, database, messaging)
│   └── schema/       # Configuration schema validation
├── cli/              # Command-line interface
├── pkg/              # Reusable packages
│   ├── logger/       # Structured logging
│   ├── tplengine/    # Template engine for dynamic configuration
│   ├── schemagen/    # JSON schema generation utilities
│   └── utils/        # Common utilities
└── test/             # Comprehensive test suite
    ├── e2e/          # End-to-end tests
    ├── integration/  # Integration tests
    └── helpers/      # Test utilities and fixtures
```

### Technology Stack

**Backend Infrastructure:**

- **Go 1.24+**: Primary language with focus on performance and reliability
- **PostgreSQL**: Primary data store with migrations via Goose
- **Temporal**: Workflow orchestration engine for reliable execution
- **NATS**: Message streaming for real-time events and logging
- **Gin**: HTTP router for RESTful API

**Runtime Environment:**

- **Deno**: Secure JavaScript/TypeScript runtime for tool execution
- **Docker Compose**: Local development environment orchestration

**Development Tools:**

- **golangci-lint**: Comprehensive code quality enforcement
- **Swagger/OpenAPI**: API documentation generation
- **testify**: Testing framework with mocks and assertions

## Development Standards

### Code Quality Standards

#### Linting Rules (`.golangci.yml`)

- **Function Length**: Maximum 80 lines or 50 statements
- **Line Length**: Maximum 120 characters
- **Cyclomatic Complexity**: Maximum 15
- **Error Handling**: All errors must be checked (`errcheck`)
- **Security**: Security analysis via `gosec`
- **Performance**: Optimized imports and unused code removal

#### Go Best Practices

- **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` for error context
- **Dependency Injection**: Constructor functions with interface-based dependencies
- **Naked Returns**: Prohibited in functions longer than a few lines
- **Context Propagation**: Always pass `context.Context` as first parameter for external calls
- **Interface Design**: Small, focused interfaces following the "accept interfaces, return structs" principle

### Testing Strategy

#### Test Organization

```
test/
├── e2e/              # Full system integration tests
│   └── worker/       # Worker execution scenarios
├── integration/      # Database and external service tests
│   └── repo/         # Repository layer tests
└── helpers/          # Shared test utilities and fixtures
```

#### Testing Standards

- **Test Naming**: Use `t.Run("Should...")` pattern for clear behavior description
- **Test Isolation**: Each test should be independent and repeatable
- **Mock Usage**: Use testify mocks for external dependencies
- **Coverage**: Aim for comprehensive coverage of business logic
- **Performance Tests**: Separate worker tests for performance validation

#### Test Commands

```bash
# Fast development tests (excludes slow integration/worker tests)
make test

# Full test suite including integration tests
make test-all

# Worker-specific performance tests
make test-worker

# Single package testing
go test ./path/to/package
```

### File Organization Standards

#### Package Structure

- **Domain-Driven Design**: Group by feature/domain (agent, task, tool, workflow)
- **Layered Architecture**: Separate concerns (uc/, router/, services/, fixtures/)
- **Interface Separation**: Define interfaces in separate files from implementations
- **Test Collocation**: Place `*_test.go` files alongside implementation files

#### Naming Conventions

- **Packages**: Lowercase, descriptive names without underscores
- **Files**: Lowercase with underscores for separation (e.g., `task_executor.go`)
- **Interfaces**: Descriptive names ending in common patterns (Manager, Service, Repository)
- **Constants**: CamelCase with descriptive prefixes

### Configuration Management

#### Project Configuration (`compozy.yaml`)

```yaml
name: project-name
version: 0.1.0
description: Project description

workflows:
    - source: ./workflow.yaml

models:
    - provider: ollama|openai|groq
      model: model-name
      api_key: "{{ .env.API_KEY }}"

runtime:
    permissions:
        - --allow-read
        - --allow-net
        - --allow-env
```

#### Environment Variables

- **Development**: Use `.env` files for local configuration
- **Production**: Environment-based configuration for security
- **Secrets**: Never commit API keys or sensitive data

## Build & Development Workflow

### Development Commands

```bash
# Development server with hot reload
make dev

# Development with specific example
make dev-weather (should be run in a no block-way)

# Build production binary
make build

# Code formatting and linting
make fmt
make lint

# Database management
make start-docker # Start PostgreSQL, Temporal, NATS
make reset-db     # Reset database schema
make migrate-up   # Apply pending migrations
```

### API Development

#### Swagger Documentation

- **Auto-generation**: Documentation generated from Go annotations
- **Interactive UI**: Available at `/swagger/index.html` during development
- **Validation**: Use `make swagger-validate` to ensure documentation accuracy

#### API Standards

- **RESTful Design**: Follow REST principles for resource management
- **Consistent Responses**: Standardized response format with status, message, data, error
- **Error Handling**: Proper HTTP status codes with detailed error information
- **Versioning**: API versioned at `/api/v0/`

### Database Standards

#### Migration Management

```bash
# Create new migration
make migrate-create name=migration_name

# Check migration status
make migrate-status

# Apply/rollback migrations
make migrate-up
make migrate-down
```

#### Schema Design

- **Normalized Structure**: Proper foreign key relationships
- **Indexing**: Strategic indexing for query performance
- **Constraints**: Appropriate constraints for data integrity
- **Audit Fields**: Created/updated timestamps where applicable

## Runtime Architecture

### Deno Integration

#### Tool Execution

- **Security**: Configurable Deno permissions per project
- **Isolation**: Each tool execution runs in a separate process
- **Communication**: JSON-based stdin/stdout communication
- **Error Handling**: Comprehensive error reporting with stack traces

#### Performance Considerations

- **Process Reuse**: Binary compilation for faster startup
- **Memory Management**: Buffer pooling for efficient memory usage
- **Timeout Handling**: Configurable timeouts for tool execution
- **Resource Limits**: Controlled resource allocation

### Workflow Orchestration

#### Temporal Integration

- **Reliability**: Automatic retry and error recovery
- **Scalability**: Distributed workflow execution
- **Monitoring**: Built-in workflow state tracking
- **Versioning**: Workflow version management for updates

#### Event Streaming

- **Real-time Updates**: NATS-based event streaming
- **Structured Events**: Consistent event format for monitoring
- **Debugging**: Comprehensive logging for troubleshooting

## Performance & Monitoring

### Logging Standards

- **Structured Logging**: JSON-formatted logs for production
- **Log Levels**: Appropriate use of debug, info, warn, error levels
- **Context**: Include relevant context (IDs, operations) in log messages
- **Performance**: Avoid excessive logging in hot paths

### Metrics & Observability

- **Health Checks**: `/health` endpoint for service monitoring
- **Execution Tracking**: Detailed execution metrics via API
- **Error Rates**: Monitor failure rates and error patterns
- **Performance Metrics**: Track execution times and resource usage

## Contributing Guidelines

### Code Review Process

1. **Pre-commit**: Run `make lint` and `make test` before submitting
2. **Documentation**: Update Swagger annotations for API changes
3. **Testing**: Include tests for new functionality
4. **Migration**: Create database migrations for schema changes
5. **Backward Compatibility**: Maintain API compatibility where possible

### Development Environment Setup

```bash
# Install dependencies
make deps

# Start infrastructure
make start-docker

# Run migrations
make migrate-up

# Start development server
make dev
```

### Best Practices

- **Small Commits**: Atomic commits with clear messages
- **Feature Branches**: Use feature branches for development
- **Documentation**: Keep README and API docs updated
- **Security**: Follow security best practices for external inputs
- **Performance**: Consider performance implications of changes

## Debugging & Troubleshooting

### Common Issues

- **Deno Runtime**: Ensure Deno is installed and accessible in PATH
- **Database Connection**: Verify PostgreSQL is running and accessible
- **Temporal Worker**: Check Temporal server connectivity
- **Port Conflicts**: Ensure development ports (3001) are available

### Debugging Tools

- **Logs**: Use `--debug` flag for verbose logging
- **Database**: Direct PostgreSQL access for data inspection
- **API Testing**: Swagger UI for interactive API testing
- **Workflow Monitoring**: Temporal Web UI for workflow state inspection

---

## Quick Reference

### Essential Commands

```bash
# Start development
make dev

# Run tests
make test

# Format code
make fmt

# Generate docs
make swagger-gen

# Reset environment
make reset-docker && make migrate-up
```

### Key Directories

- `engine/`: Core business logic
- `cli/`: Command-line interface
- `test/`: Comprehensive test suite
- `pkg/`: Reusable packages
- `docs/`: API documentation

This guide serves as the foundation for consistent, high-quality development on the Compozy platform. For specific technical questions, refer to the code documentation and test examples.
