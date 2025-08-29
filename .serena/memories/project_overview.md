# Compozy Project Overview

## Purpose

Compozy is a next-level agentic orchestration platform that enables building AI-powered applications through declarative YAML configuration and a robust Go backend. It integrates with various LLM providers and supports the Model Context Protocol (MCP) for extending AI capabilities.

## Key Features

- **Declarative Workflows**: Define complex AI workflows with simple, human-readable YAML
- **Developer-Focused**: Comprehensive CLI with hot-reloading for seamless development experience
- **Advanced Task Orchestration**: 8 powerful task types including parallel, sequential, and conditional execution
- **Extensible Tools**: Write custom tools in TypeScript/JavaScript to extend agent capabilities
- **Multi-Model Support**: Integrates with 7+ LLM providers like OpenAI, Anthropic, Google, and local models
- **Enterprise-Ready**: Built on Temporal for production with persistence, monitoring, and security features
- **High Performance**: Built with Go at its core for exceptional speed and efficiency

## Tech Stack

- **Language**: Go 1.25.0+
- **Database**: PostgreSQL with pgx driver
- **Cache**: Redis
- **Workflow Engine**: Temporal
- **Web Framework**: Gin
- **Template Engine**: Go templates with Sprig functions
- **Testing**: Testify
- **CLI**: Cobra
- **Configuration**: YAML with go-yaml
- **LLM Integration**: Multiple providers via custom adapters
- **Containerization**: Docker/Docker Compose

## Architecture Overview

The project follows clean architecture principles with these main domains:

- `engine/`: Core business logic
  - `agent/`: AI agent management
  - `task/`: Task execution and orchestration
  - `workflow/`: Workflow management
  - `tool/`: Tool system
  - `llm/`: LLM provider integrations
  - `runtime/`: Runtime execution environment
  - `infra/`: Infrastructure layer (database, cache, etc.)
- `cli/`: Command-line interface
- `pkg/`: Shared packages and utilities
- `examples/`: Example workflows and configurations
