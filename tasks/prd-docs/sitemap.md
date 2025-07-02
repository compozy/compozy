## Documentation Site Structure (145 pages)

```
docs.compozy.ai/
├── 1. Getting Started (5 pages)
│   ├── 1.1 Welcome to Compozy
│   ├── 1.2 Quick Start (15 minutes with TUI)
│   ├── 1.3 Installation & Setup
│   ├── 1.4 Core Concepts
│   └── 1.5 First Workflow (interactive walkthrough)
├── 2. Configuration (8 pages)
│   ├── 2.1 Project Setup & Structure
│   ├── 2.2 Configuration Hierarchy & Inheritance
│   ├── 2.3 Component Discovery & AutoLoad
│   ├── 2.4 Environment Management
│   ├── 2.5 Reference Resolution System
│   ├── 2.6 Advanced Configuration Features
│   ├── 2.7 Validation & Error Handling
│   └── 2.8 Schema Definition
├── 3. YAML Authoring & Templates (7 pages)
│   ├── 3.1 YAML Configuration Basics
│   ├── 3.2 Template Engine Overview
│   ├── 3.3 Context & Variables
│   ├── 3.4 Sprig Functions
│   ├── 3.5 Advanced Templating
│   ├── 3.6 Configuration Patterns
│   └── 3.7 Validation & Debugging
├── 4. Tools Development (9 pages)
│   ├── 4.1 Tools Overview
│   ├── 4.2 TypeScript Development
│   ├── 4.3 Configuration & Schemas
│   ├── 4.4 Schema Definition
│   ├── 4.5 Runtime Environment
│   ├── 4.6 External Integrations
│   ├── 4.7 Testing & Debugging
│   ├── 4.8 Performance & Security
│   └── 4.9 Advanced Patterns
├── 5. AI Agents (9 pages)
│   ├── 5.1 Agent Overview
│   ├── 5.2 LLM Integration
│   ├── 5.3 Instructions & Actions
│   ├── 5.4 Tool Integration
│   ├── 5.5 Schema Definition
│   ├── 5.6 Memory Systems
│   ├── 5.7 Multi-Agent Patterns
│   ├── 5.8 Testing & Validation
│   └── 5.9 Performance Optimization
├── 6. Task Types (7 pages)
│   ├── 6.1 Basic Tasks
│   ├── 6.2 Flow Control
│   ├── 6.3 Parallel Processing
│   ├── 6.4 Memory Tasks
│   ├── 6.5 Aggregate Tasks
│   ├── 6.6 Advanced Patterns
│   └── 6.7 Custom Task Types
├── 7. Signals & Communication (6 pages)
│   ├── 7.1 Signal Overview
│   ├── 7.2 Signal Tasks
│   ├── 7.3 Signal-based Triggers
│   ├── 7.4 Wait Tasks
│   ├── 7.5 Event API
│   └── 7.6 Advanced Patterns
├── 8. Memory System (6 pages)
│   ├── 8.1 Memory Concepts
│   ├── 8.2 Configuration
│   ├── 8.3 Operations
│   ├── 8.4 Integration Patterns
│   ├── 8.5 Privacy & Security
│   └── 8.6 Troubleshooting
├── 9. Model Context Protocol (13 pages)
│   ├── 9.1 MCP Overview & Architecture
│   ├── 9.2 MCP Proxy Server
│   ├── 9.3 Transport Configuration
│   ├── 9.4 Schema Definition
│   ├── 9.5 Admin API & Management
│   ├── 9.6 Client Manager & Connection Pooling
│   ├── 9.7 Security & Authentication
│   ├── 9.8 Tool Discovery & Filtering
│   ├── 9.9 Storage Backends
│   ├── 9.10 Monitoring & Metrics
│   ├── 9.11 Integration Patterns
│   ├── 9.12 Development & Debugging
│   └── 9.13 Production Deployment
├── 10. Scheduling & Automation (4 pages)
│   ├── 10.1 Scheduled Workflows
│   ├── 10.2 Schedule Management
│   ├── 10.3 Advanced Scheduling
│   └── 10.4 Production Considerations
├── 11. Temporal Integration (8 pages)
│   ├── 11.1 Temporal Architecture & Enterprise Benefits
│   ├── 11.2 Production Deployment & Infrastructure
│   ├── 11.3 Event-Driven Workflows & Signal Handling
│   ├── 11.4 Schedule Management & Native Temporal Scheduling
│   ├── 11.5 Monitoring, Metrics & Observability
│   ├── 11.6 Scaling & Multi-Tenancy Patterns
│   ├── 11.7 Operational Runbooks & Troubleshooting
│   └── 11.8 Development & Testing with Temporal
├── 12. Development (6 pages)
│   ├── 12.1 Development Setup
│   ├── 12.2 Testing Strategies
│   ├── 12.3 Configuration Management
│   ├── 12.4 Debugging
│   ├── 12.5 Performance
│   └── 12.6 Code Organization
├── 13. Deployment (5 pages)
│   ├── 13.1 Deployment Options
│   ├── 13.2 Infrastructure
│   ├── 13.3 Configuration
│   ├── 13.4 Scaling
│   └── 13.5 Maintenance
├── 14. Monitoring & Operations (4 pages)
│   ├── 14.1 Observability
│   ├── 14.2 Health Checks
│   ├── 14.3 Security
│   └── 14.4 Troubleshooting
├── 15. CLI Reference (8 pages)
│   ├── 15.1 CLI Overview & TUI Philosophy
│   ├── 15.2 Project Commands (init)
│   ├── 15.3 Workflow Commands (list/get/deploy/validate)
│   ├── 15.4 Execution Commands (run create/list/get/cancel)
│   ├── 15.5 Monitoring Commands (run status)
│   ├── 15.6 CI/CD Usage (--no-tui patterns)
│   ├── 15.7 Advanced TUI Features (fuzzy search, vim navigation)
│   └── 15.8 Shell Completions & Configuration
├── 16. API Reference (8 pages)
│   ├── 16.1 API Overview
│   ├── 16.2 Workflows & Executions
│   ├── 16.3 Events & Signals
│   ├── 16.4 Schedules Management
│   ├── 16.5 Component Discovery
│   ├── 16.6 Memory API
│   ├── 16.7 Admin APIs
│   └── 16.8 System & Health
├── 17. Examples & Tutorials (8 pages)
│   ├── 17.1 Basic Examples
│   ├── 17.2 Intermediate Examples
│   ├── 17.3 Advanced Examples
│   ├── 17.4 Use Case: Customer Support
│   ├── 17.5 Use Case: Content Generation
│   ├── 17.6 Use Case: Data Processing
│   ├── 17.7 Integration Examples
│   └── 17.8 Migration Guides
├── 18. Registry (soon)
├── 19. Community & Contributing (4 pages)
│   ├── 19.1 Contributing Guide
│   ├── 19.2 Community Resources
│   ├── 19.3 Documentation
│   └── 19.4 Roadmap & Changelog
└── 20. Configuration Reference (4 pages)
    ├── 20.1 Complete Schema Reference
    ├── 20.2 Environment Variables
    ├── 20.3 Error Codes & Troubleshooting
    └── 20.4 Migration & Versioning
```

## Relevant Files

### 1. Getting Started

- `README.md` - Project overview and quick start
- `docs/` - Basic documentation and setup guides
- `.env.example` - Environment configuration template
- `Makefile` - Development commands and setup
- `cluster/docker-compose.yml` - Local development infrastructure

### 2. Configuration

- `engine/core/` - Core configuration types and interfaces (ID, Ref, validation)
- `engine/project/` - Project structure and configuration management
- `engine/config/` - Configuration parsing and hierarchy management
- `engine/schema/` - Configuration schema validation and types
- `pkg/autoload/` - Component discovery and autoload system
- `pkg/env/` - Environment variable management
- `examples/` - Project configuration examples and patterns
- `compozy.yaml` - Main project configuration file examples

### 3. YAML Authoring & Templates

- `engine/template/` - Template engine implementation with Sprig
- `engine/core/config.go` - Configuration structures and validation
- `engine/workflow/config.go` - Workflow-specific configuration
- `examples/` - YAML configuration examples and patterns
- `docs/examples/` - Template usage examples

### 4. Tools Development

- `engine/task/tool/` - Tool execution engine with Deno runtime
- `pkg/deno/` - Deno JavaScript/TypeScript runtime integration
- `engine/core/tool.go` - Tool configuration and schemas
- `pkg/autoload/tool.go` - Tool discovery and loading
- `examples/tools/` - Tool implementation examples

### 5. AI Agents

- `engine/task/agent/` - Agent orchestration and LLM integration
- `engine/llm/` - LLM provider integrations (8 providers)
- `engine/core/agent.go` - Agent configuration and types
- `pkg/autoload/agent.go` - Agent discovery and loading
- `examples/agents/` - Agent configuration examples

### 6. Task Types

- `engine/task/` - All task type implementations
- `engine/task/basic/` - Basic task execution
- `engine/task/flow/` - Flow control tasks (if, loop, switch)
- `engine/task/parallel/` - Parallel execution patterns
- `engine/task/memory/` - Memory integration tasks
- `engine/task/aggregate/` - Data aggregation tasks

### 7. Signals & Communication

- `engine/signal/` - Signal system implementation
- `engine/task/signal/` - Signal task types
- `engine/task/wait/` - Wait task implementation
- `engine/worker/dispatcher.go` - Event-driven signal dispatcher
- `api/handlers/events.go` - Event API endpoints

### 8. Memory System

- `engine/memory/` - Complete memory system implementation
- `engine/memory/workflows.go` - Temporal-powered memory workflows
- `engine/memory/manager.go` - Memory lifecycle management
- `pkg/redis/` - Redis integration for distributed storage
- `api/handlers/memory.go` - Memory API endpoints

### 9. Model Context Protocol (MCP)

- `pkg/mcp-proxy/` - Complete MCP proxy server implementation
- `engine/mcp/` - MCP client integration and configuration
- `pkg/mcp-proxy/README.md` - MCP proxy documentation
- `pkg/mcp-proxy/types.go` - MCP types and definitions
- `api/handlers/mcp.go` - MCP admin API endpoints

### 10. Scheduling & Automation

- `engine/workflow/schedule/` - Native Temporal scheduling
- `engine/workflow/schedule/manager.go` - Schedule lifecycle management
- `api/handlers/schedules.go` - Schedule management API
- `engine/cron/` - CRON expression handling
- `examples/scheduled/` - Scheduled workflow examples

### 11. Temporal Integration

- `engine/worker/` - Temporal workflow and activity implementation
- `engine/worker/workflows.go` - Universal CompozyWorkflow
- `engine/worker/dispatcher.go` - Signal-based dispatcher workflow
- `engine/infra/monitoring/interceptor/temporal.go` - Metrics interceptor
- `cluster/docker-compose.yml` - Temporal infrastructure setup

### 12. Development

- `Makefile` - Development commands and workflows
- `test/` - Testing infrastructure and patterns
- `test/integration/` - Integration test suites
- `.github/workflows/` - CI/CD pipeline configuration
- `tools/` - Development tools and utilities

### 13. Deployment

- `cluster/` - Complete deployment configurations
- `cluster/docker-compose.yml` - Production Docker setup
- `cluster/k8s/` - Kubernetes deployment manifests
- `scripts/` - Deployment and operational scripts
- `docs/deployment/` - Deployment guides and examples

### 14. Monitoring & Operations

- `engine/infra/monitoring/` - Metrics and observability
- `engine/infra/health/` - Health check implementations
- `pkg/prometheus/` - Prometheus metrics integration
- `api/handlers/health.go` - Health API endpoints
- `engine/infra/logging/` - Structured logging configuration

### 15. CLI Reference

- `cli/` - CLI command implementations
- `cli/root.go` - Root command with TUI-first approach
- `cli/cmd/workflow/` - Workflow management commands
- `cli/cmd/run/` - Execution management commands
- `cli/shared/` - Shared components (client, output, styles)
- `cli/internal/tui/` - TUI components with Bubble Tea
- `docs/cli/` - CLI usage documentation

### 16. API Reference

- `api/` - Complete REST API implementation
- `api/handlers/` - All API endpoint handlers
- `api/middleware/` - Authentication and middleware
- `api/swagger/` - OpenAPI/Swagger documentation
- `pkg/http/` - HTTP utilities and helpers

### 17. Examples & Tutorials

- `examples/` - Comprehensive example projects
- `examples/basic/` - Basic workflow examples
- `examples/advanced/` - Complex workflow patterns
- `examples/integrations/` - Third-party integration examples
- `docs/tutorials/` - Step-by-step tutorial guides

### 18. Registry

- Registry feature not implemented yet

### 19. Community & Contributing

- `CONTRIBUTING.md` - Contribution guidelines
- `CODE_OF_CONDUCT.md` - Community standards
- `.github/` - GitHub templates and workflows
- `docs/development/` - Development setup guides
- `CHANGELOG.md` - Version history and changes

### 20. Configuration Reference

- `engine/core/config.go` - Complete configuration schema
- `pkg/env/` - Environment variable handling
- `engine/errors/` - Error codes and handling
- `api/swagger/swagger.yaml` - Complete API schema
- `docs/reference/` - Reference documentation
- `CHANGELOG.md` - Version history and migration guides
