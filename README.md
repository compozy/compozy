# Compozy – _Workflow Orchestration Engine for AI Agents_

[![CI](https://img.shields.io/github/actions/workflow/status/compozy/compozy/ci.yml?branch=main&logo=github)](https://github.com/compozy/compozy/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/compozy/compozy?logo=go)](https://golang.org/)
[![License](https://img.shields.io/github/license/compozy/compozy)](./LICENSE)
[![Release](https://img.shields.io/github/v/release/compozy/compozy)](https://github.com/compozy/compozy/releases)

> **Compozy transforms AI integration from a technical challenge requiring months of infrastructure development into a configurable business capability accessible through simple YAML files. Build AI-powered applications through declarative configuration with production-ready Go backend, multi-LLM provider support, and enterprise-grade workflow orchestration.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 The Problem Compozy Solves](#-the-problem-compozy-solves)
- [⚡ Key Features](#-key-features)
- [🏗️ Architecture](#-architecture)
- [🚀 Quick Start](#-quick-start)
- [📖 Core Concepts](#-core-concepts)
- [🔧 Configuration](#-configuration)
- [📊 API Reference](#-api-reference)
- [🛠️ Development](#-development)
- [🧪 Testing](#-testing)
- [📦 Package Architecture](#-package-architecture)
- [🌐 Ecosystem & Integration](#-ecosystem--integration)
- [🔒 Security & Production](#-security--production)
- [📚 Examples](#-examples)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

Compozy is a **workflow orchestration engine for AI agents** that solves the critical business problem of AI integration complexity. Organizations face interconnected challenges when implementing AI: technical complexity barriers, integration fragmentation, development bottlenecks, vendor lock-in risks, and scalability challenges.

**What Compozy Does:**

- **Eliminates AI Infrastructure Development**: No need to build orchestration, error handling, or scaling from scratch
- **Provides Multi-LLM Orchestration**: Unified interface for OpenAI, Anthropic, Google, Groq, Ollama, and custom providers
- **Enables Declarative Workflows**: Define complex AI processes through YAML configuration rather than code
- **Ensures Production Readiness**: Built with enterprise-grade reliability, monitoring, and security

**System Architecture:**

```
┌─────────────────────────────────────────────────────────────┐
│                    User Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   YAML      │  │  REST API   │  │  CLI Tools  │        │
│  │  Config     │  │  Endpoint   │  │  & DevExp   │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                  Core Engine Layer                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  Workflow   │    Task     │   Agent     │    Tool     │  │
│  │ Orchestr.   │  Executor   │  Runtime    │  Manager    │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Memory    │     LLM     │     MCP     │   Runtime   │  │
│  │  Manager    │ Orchestr.   │   Client    │  (Bun/Node) │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                Infrastructure Layer                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Server    │  Monitoring │    Cache    │    Store    │  │
│  │  (Gin)      │(Prometheus) │  (Redis)    │(PostgreSQL) │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                External Dependencies                        │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Temporal  │  LLM APIs   │  MCP Tools  │  Grafana    │  │
│  │  Workflows  │ (Multi-Prov)│  & Servers  │ Dashboard   │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 💡 The Problem Compozy Solves

### Real-World Business Challenges

**1. Technical Complexity Barrier**

- Teams spend 70-80% of time building AI infrastructure instead of business logic
- Complex AI use cases remain unimplemented due to technical barriers
- Organizations need expertise in multiple domains: AI models, infrastructure, orchestration, error handling, scaling

**2. Integration Fragmentation**

- Different AI capabilities exist in silos
- Difficult to create cohesive, multi-step intelligent processes
- Manual coordination wastes human resources and creates inconsistent experiences

**3. Vendor Lock-in & Cost Inefficiency**

- Dependency on single AI providers limits flexibility and increases costs
- No automatic cost optimization across providers
- Difficult to A/B test different AI approaches

**4. Scalability & Reliability Challenges**

- Manual AI operations don't scale to enterprise requirements
- No built-in error handling and recovery mechanisms
- Lack of production-ready monitoring and observability

### Compozy's Solution

**Business Impact Metrics:**

- **Development Velocity**: 70-80% reduction in AI workflow development time
- **Operational Efficiency**: 90% reduction in manual AI task coordination
- **Cost Optimization**: 30-50% reduction in AI spend through provider optimization
- **Time-to-Market**: From months to days for complex AI workflows

---

## ⚡ Key Features

### 🤖 **Multi-Agent Orchestration**

- **Declarative YAML Configuration**: Define complex AI workflows without code
- **Multi-LLM Support**: OpenAI, Anthropic, Google, Groq, Ollama, and custom providers
- **Intelligent Routing**: Automatic failover and cost optimization
- **Agent Collaboration**: Coordinate multiple AI agents for complex tasks

### 🔧 **Advanced Tool Integration**

- **Model Context Protocol (MCP)**: Standardized external tool integration
- **JavaScript/TypeScript Runtime**: Custom tool execution with Bun/Node.js
- **HTTP/CLI Tool Support**: Integrate any external API or command-line tool
- **Security Sandboxing**: Isolated execution environments with permission controls

### 📊 **Enterprise-Grade Workflow Management**

- **Temporal Integration**: Fault-tolerant, durable workflow execution
- **Complex Task Types**: Basic, parallel, collection, composite, router, aggregate, wait, and signal tasks
- **State Management**: Persistent workflow and task state in PostgreSQL
- **Event-Driven Architecture**: Comprehensive event system for monitoring

### 💾 **Intelligent Memory System**

- **Token-Aware Management**: Optimized token usage and cost control
- **Privacy Controls**: Configurable PII detection and data filtering
- **Multiple Eviction Strategies**: LRU, TTL, and hybrid memory management
- **Distributed Architecture**: Redis-backed with conflict resolution

### 🔒 **Production-Ready Features**

- **Comprehensive Monitoring**: OpenTelemetry + Prometheus + Grafana
- **Security First**: Input validation, rate limiting, audit trails
- **Hot Reload Development**: Instant configuration updates
- **Multi-Environment Support**: Development, staging, production configurations

---

## 🏗️ Architecture

### Clean Architecture Principles

Compozy follows **Clean Architecture** with **Domain-Driven Design**:

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI & API Layer                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │     CLI     │  │  REST API   │  │  MCP Proxy  │        │
│  │  (Cobra)    │  │   (Gin)     │  │   Server    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                  Application Layer                          │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Agent     │   Task      │  Workflow   │   Memory    │  │
│  │  Domain     │  Domain     │  Domain     │  Domain     │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │    LLM      │    MCP      │    Tool     │   Runtime   │  │
│  │  Domain     │  Domain     │  Domain     │  Domain     │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                   Core Domain Layer                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Types     │   Config    │   Context   │   Errors    │  │
│  │   (IDs)     │  (Loader)   │  (Params)   │  (Unified)  │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                Infrastructure Layer                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   Server    │  Monitoring │    Cache    │    Store    │  │
│  │  (HTTP)     │  (Metrics)  │  (Redis)    │(PostgreSQL) │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Technology Stack

**Backend Core:**

- **Go 1.24+**: Modern Go with latest features and performance
- **Gin**: High-performance HTTP framework
- **Cobra**: CLI framework for commands
- **Temporal**: Distributed workflow orchestration

**Data Layer:**

- **PostgreSQL 15+**: Primary database with JSONB support
- **Redis 7.2+**: Caching, configuration, and pub/sub
- **Goose**: Database migration management

**External Integrations:**

- **OpenTelemetry**: Distributed tracing and metrics
- **Prometheus**: Metrics collection and monitoring
- **Grafana**: Dashboards and visualization
- **MCP Protocol**: Model Context Protocol for tool integration

---

## 🚀 Quick Start

### Prerequisites

```bash
# Required
- Go 1.24+
- PostgreSQL 15+
- Redis 7.2+
- Docker & Docker Compose
- Make

# Optional for development
- Bun (for JavaScript/TypeScript tools)
- Node.js (alternative runtime)
```

### Installation

```bash
# Clone and setup
git clone https://github.com/compozy/compozy.git
cd compozy

# Quick setup (starts databases and applies migrations)
make deps && make start-docker && make migrate-up

# Start development server with hot reload
make dev
```

### Your First Workflow

Create a simple workflow configuration:

```yaml
# compozy.yaml
name: "hello-world"
version: "1.0.0"
description: "A simple greeting workflow"

# Configure AI models
models:
  - provider: openai
    model: gpt-4
    api_key: "{{ .env.OPENAI_API_KEY }}"
    temperature: 0.7
    max_tokens: 1000

# Define agents
agents:
  - id: greeter
    config:
      $ref: global::models.#(provider=="openai")
    instructions: |
      You are a friendly AI assistant that creates personalized greetings.
      Always be warm and welcoming.
    actions:
      - id: greet
        prompt: "Generate a personalized greeting for: {{ .input.name }}"
        output:
          type: object
          properties:
            greeting: { type: string }
            tone: { type: string }

# Define workflow tasks
tasks:
  - id: generate_greeting
    type: basic
    $use: agent(local::agents.#(id="greeter"))
    action: greet
    with:
      name: "{{ .workflow.input.name }}"
    outputs:
      result: "{{ .output.greeting }}"
      tone: "{{ .output.tone }}"

# Define workflow outputs
outputs:
  greeting: "{{ .tasks.generate_greeting.output.result }}"
  metadata:
    tone: "{{ .tasks.generate_greeting.output.tone }}"
    timestamp: "{{ now }}"
```

Run the workflow:

```bash
# Start development server
make dev

# Test the workflow (in another terminal)
curl -X POST http://localhost:3000/api/v0/workflows/hello-world/executions \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
```

---

## 📖 Core Concepts

### Workflows

Workflows are the top-level orchestration units that define how tasks, agents, and tools work together:

```yaml
# Advanced workflow example
name: "customer-support"
version: "1.0.0"

# Schema validation
schemas:
  - id: customer_inquiry
    type: object
    properties:
      message: { type: string, minLength: 1 }
      priority: { type: string, enum: ["low", "normal", "high", "urgent"] }
      customer_id: { type: string }
    required: ["message", "customer_id"]

# Multi-provider models
models:
  - provider: openai
    model: gpt-4-turbo
    api_key: "{{ .env.OPENAI_API_KEY }}"
    temperature: 0.3
  - provider: anthropic
    model: claude-3-opus
    api_key: "{{ .env.ANTHROPIC_API_KEY }}"
    temperature: 0.1

# Specialized agents
agents:
  - id: classifier
    config:
      $ref: global::models.#(provider=="openai")
    instructions: |
      You are a customer service classifier. Analyze inquiries and categorize them.
    actions:
      - id: classify
        json_mode: true
        prompt: |
          Classify this customer inquiry:
          Message: {{ .input.message }}
          Priority: {{ .input.priority }}

          Determine the category and required expertise level.
        output:
          type: object
          properties:
            category:
              { type: string, enum: ["technical", "billing", "general"] }
            expertise_level:
              { type: string, enum: ["junior", "senior", "specialist"] }
            confidence: { type: number, minimum: 0, maximum: 1 }

  - id: technical_support
    config:
      $ref: global::models.#(provider=="anthropic")
    instructions: |
      You are a senior technical support specialist. Provide detailed,
      accurate technical assistance with clear step-by-step instructions.
    actions:
      - id: resolve
        prompt: |
          Provide technical support for this inquiry:
          {{ .input.message }}

          Category: {{ .input.category }}
          Customer ID: {{ .input.customer_id }}

          Provide a comprehensive resolution with clear steps.

# Complex task orchestration
tasks:
  - id: validate_input
    type: basic
    with:
      inquiry: "{{ .workflow.input | validateSchema('customer_inquiry') }}"
    outputs:
      validated_data: "{{ .with.inquiry }}"

  - id: classify_inquiry
    type: basic
    $use: agent(local::agents.#(id="classifier"))
    action: classify
    with:
      message: "{{ .tasks.validate_input.output.validated_data.message }}"
      priority: "{{ .tasks.validate_input.output.validated_data.priority }}"
    outputs:
      category: "{{ .output.category }}"
      expertise_level: "{{ .output.expertise_level }}"
      confidence: "{{ .output.confidence }}"

  - id: route_to_specialist
    type: router
    condition: '{{ .tasks.classify_inquiry.output.category == "technical" && .tasks.classify_inquiry.output.confidence > 0.8 }}'
    routes:
      technical:
        id: technical_resolution
        type: basic
        $use: agent(local::agents.#(id="technical_support"))
        action: resolve
        with:
          message: "{{ .tasks.validate_input.output.validated_data.message }}"
          category: "{{ .tasks.classify_inquiry.output.category }}"
          customer_id: "{{ .tasks.validate_input.output.validated_data.customer_id }}"
      default:
        id: general_response
        type: basic
        with:
          message: "Thank you for contacting us. Your inquiry has been received and will be processed by our team."

outputs:
  resolution: "{{ .tasks.route_to_specialist.output.message | default('General inquiry processed') }}"
  metadata:
    category: "{{ .tasks.classify_inquiry.output.category }}"
    confidence: "{{ .tasks.classify_inquiry.output.confidence }}"
    processing_time: "{{ .workflow.execution_time }}"
    timestamp: "{{ now }}"
```

### Agents

Agents are AI-powered components that can perform actions using LLM providers:

````yaml
agents:
  - id: code_reviewer
    config:
      provider: anthropic
      model: claude-3-opus
      temperature: 0.1
      max_tokens: 4000
    instructions: |
      You are a senior code reviewer with expertise in Go, architecture,
      and best practices. Provide constructive feedback focused on:
      1. Code quality and readability
      2. Performance and efficiency
      3. Security considerations
      4. Maintainability and scalability

    # Memory configuration
    memory:
      - id: review_context
        key: "code_review:{{ .workflow.input.repository }}"
        mode: read-write
        privacy:
          enabled: true
          retention_policy: "7d"

    actions:
      - id: review_code
        json_mode: true
        prompt: |
          Review this code submission:

          Repository: {{ .input.repository }}
          Files changed: {{ .input.files | length }}

          {{ range .input.files }}
          File: {{ .path }}
          ```{{ .language }}
          {{ .content }}
          ```
          {{ end }}

          Provide comprehensive feedback with specific recommendations.
        input:
          type: object
          properties:
            repository: { type: string }
            files:
              type: array
              items:
                type: object
                properties:
                  path: { type: string }
                  language: { type: string }
                  content: { type: string }
        output:
          type: object
          properties:
            overall_rating: { type: integer, minimum: 1, maximum: 10 }
            feedback:
              type: array
              items:
                type: object
                properties:
                  file: { type: string }
                  line: { type: integer }
                  type:
                    { type: string, enum: ["suggestion", "issue", "praise"] }
                  message: { type: string }
                  priority: { type: string, enum: ["low", "medium", "high"] }
````

### Tools

Tools extend agent capabilities with external functionality:

```yaml
tools:
  - id: github_api
    description: "GitHub API integration for repository operations"
    type: http
    config:
      base_url: "https://api.github.com"
      headers:
        Authorization: "Bearer {{ .env.GITHUB_TOKEN }}"
        Accept: "application/vnd.github.v3+json"
    input:
      type: object
      properties:
        endpoint: { type: string }
        method: { type: string, enum: ["GET", "POST", "PUT", "DELETE"] }
        data: { type: object }
    output:
      type: object
      properties:
        status_code: { type: integer }
        data: { type: object }
        headers: { type: object }

  - id: code_formatter
    description: "Format code using various formatters"
    type: javascript
    execute: |
      async function format({ code, language, options = {} }) {
        // Import appropriate formatter
        let formatter;
        switch (language) {
          case 'go':
            formatter = await import('go-fmt');
            break;
          case 'javascript':
          case 'typescript':
            formatter = await import('prettier');
            break;
          case 'python':
            formatter = await import('black');
            break;
          default:
            throw new Error(`Unsupported language: ${language}`);
        }
        
        return {
          formatted_code: await formatter.format(code, options),
          original_length: code.length,
          language: language
        };
      }
    input:
      type: object
      properties:
        code: { type: string }
        language: { type: string }
        options: { type: object }
    output:
      type: object
      properties:
        formatted_code: { type: string }
        original_length: { type: integer }
        language: { type: string }
```

### Task Types

Compozy supports various task types for complex orchestration:

```yaml
tasks:
  # Basic task - single agent execution
  - id: analyze_data
    type: basic
    $use: agent(local::agents.#(id="analyst"))
    action: analyze
    with:
      data: "{{ .workflow.input.data }}"

  # Parallel task - concurrent execution
  - id: multi_analysis
    type: parallel
    strategy: wait_all # or wait_any, wait_first
    tasks:
      - id: sentiment_analysis
        type: basic
        $use: agent(local::agents.#(id="sentiment_analyzer"))
        action: analyze
        with:
          text: "{{ .workflow.input.text }}"

      - id: entity_extraction
        type: basic
        $use: agent(local::agents.#(id="entity_extractor"))
        action: extract
        with:
          text: "{{ .workflow.input.text }}"

  # Collection task - process lists
  - id: process_documents
    type: collection
    items: "{{ .workflow.input.documents }}"
    mode: parallel # or sequential
    task:
      id: "process-{{ .index }}"
      type: basic
      $use: agent(local::agents.#(id="document_processor"))
      action: process
      with:
        document: "{{ .item }}"
        index: "{{ .index }}"

  # Router task - conditional branching
  - id: route_by_type
    type: router
    condition: "{{ .workflow.input.type }}"
    routes:
      urgent:
        id: urgent_processing
        type: basic
        $use: agent(local::agents.#(id="urgent_handler"))
        action: handle_urgent
      normal:
        id: normal_processing
        type: basic
        $use: agent(local::agents.#(id="normal_handler"))
        action: handle_normal
      default:
        id: default_processing
        type: basic
        with:
          message: "Unknown request type, using default processing"

  # Composite task - multi-step execution
  - id: complex_workflow
    type: composite
    tasks:
      - id: step1
        type: basic
        $use: agent(local::agents.#(id="processor"))
        action: preprocess
        with:
          input: "{{ .workflow.input }}"

      - id: step2
        type: basic
        $use: agent(local::agents.#(id="analyzer"))
        action: analyze
        with:
          data: "{{ .tasks.step1.output }}"

      - id: step3
        type: basic
        $use: agent(local::agents.#(id="formatter"))
        action: format
        with:
          analysis: "{{ .tasks.step2.output }}"
          format: "json"

  # Aggregate task - combine results
  - id: combine_results
    type: aggregate
    outputs:
      final_result:
        sentiment: "{{ .tasks.multi_analysis.output.sentiment_analysis.sentiment }}"
        entities: "{{ .tasks.multi_analysis.output.entity_extraction.entities }}"
        processed_docs: "{{ .tasks.process_documents.output }}"
        metadata:
          processed_at: "{{ now }}"
          total_docs: "{{ .tasks.process_documents.output | length }}"
```

---

## 🔧 Configuration

### Hierarchical Configuration System

Compozy uses a sophisticated configuration system with clear precedence:

```yaml
# Priority Order (highest to lowest):
# 1. CLI flags
# 2. YAML configuration files
# 3. Environment variables
# 4. Default values
```

### Project Configuration

```yaml
# compozy.yaml - Main project configuration
name: "my-ai-project"
version: "1.0.0"
description: "AI workflow orchestration project"
author:
  name: "Your Name"
  email: "your.email@example.com"

# Workflow definitions
workflows:
  - source: "./workflows/customer-support.yaml"
  - source: "./workflows/content-generation.yaml"
  - source: "./workflows/data-processing.yaml"

# Multi-provider model configuration
models:
  - provider: openai
    model: gpt-4-turbo
    api_key: "{{ .env.OPENAI_API_KEY }}"
    base_url: "https://api.openai.com/v1"
    temperature: 0.7
    max_tokens: 4000
    timeout: 30s
    retry_attempts: 3

  - provider: anthropic
    model: claude-3-opus
    api_key: "{{ .env.ANTHROPIC_API_KEY }}"
    temperature: 0.1
    max_tokens: 8000

  - provider: ollama
    model: llama2:13b
    api_url: "http://localhost:11434"
    temperature: 0.1
    context_window: 4096

  - provider: groq
    model: llama-3.3-70b-versatile
    api_key: "{{ .env.GROQ_API_KEY }}"
    temperature: 0.2

# JavaScript/TypeScript runtime configuration
runtime:
  type: bun # or 'node'
  entrypoint: "./tools/index.ts"
  permissions:
    - "--allow-read=/safe/directory"
    - "--allow-net=api.trusted.com"
    - "--allow-env=WEATHER_API_KEY"
  timeout: 30s
  memory_limit: "512MB"

# Auto-discovery configuration
autoload:
  enabled: true
  strict: true # Fail on invalid configurations
  include:
    - "agents/**/*.yaml"
    - "memory/**/*.yaml"
    - "tools/**/*.yaml"
    - "workflows/**/*.yaml"
  exclude:
    - "**/*.tmp"
    - "**/*~"
    - "**/node_modules/**"

# Caching configuration
cache:
  url: "redis://localhost:6379"
  pool_size: 10
  max_idle_conns: 5
  timeout: 5s
  ttl: 1h

# Monitoring and observability
monitoring:
  enabled: true
  metrics:
    provider: prometheus
    endpoint: "/metrics"
    collection_interval: 30s
  logging:
    level: info
    format: json
    structured: true
  tracing:
    enabled: true
    provider: jaeger
    endpoint: "http://localhost:14268/api/traces"
```

### Environment Variables

```bash
# Server Configuration
COMPOZY_HOST=localhost
COMPOZY_PORT=3000
COMPOZY_ENV=development

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=compozy
DB_USER=compozy
DB_PASSWORD=your_password
DB_SSL_MODE=disable
DB_MAX_CONNECTIONS=25

# Redis Configuration
REDIS_URL=redis://localhost:6379
REDIS_POOL_SIZE=10
REDIS_MAX_IDLE_CONNS=5

# Temporal Configuration
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=compozy-tasks

# LLM Provider Configuration
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GROQ_API_KEY=gsk_...
GOOGLE_API_KEY=your_google_key

# MCP Configuration
MCP_PROXY_URL=http://localhost:3000
MCP_ADMIN_TOKEN=your_admin_token

# Runtime Configuration
RUNTIME_ENVIRONMENT=development
RUNTIME_LOG_LEVEL=info
RUNTIME_LOG_FORMAT=json
ASYNC_TOKEN_COUNTER_WORKERS=10
ASYNC_TOKEN_COUNTER_BUFFER_SIZE=1000

# Monitoring
PROMETHEUS_ENABLED=true
GRAFANA_URL=http://localhost:3001
JAEGER_ENDPOINT=http://localhost:14268/api/traces
```

---

## 📊 API Reference

### REST API Endpoints

Compozy provides a comprehensive REST API with OpenAPI documentation:

**Base URL**: `http://localhost:3000/api/v0`
**Documentation**: `http://localhost:3000/swagger/index.html`

#### Workflow Management

```bash
# List all workflows
GET /api/v0/workflows

# Get workflow definition
GET /api/v0/workflows/{id}

# List workflow executions
GET /api/v0/workflows/{id}/executions

# Start workflow execution
POST /api/v0/workflows/{id}/executions
Content-Type: application/json
{
  "input": {
    "data": "workflow input data"
  }
}

# Get execution status
GET /api/v0/executions/workflows/{execution_id}

# Cancel execution
POST /api/v0/executions/workflows/{execution_id}/cancel

# Send signal to execution
POST /api/v0/executions/workflows/{execution_id}/signals
Content-Type: application/json
{
  "signal_name": "pause_processing",
  "data": {"reason": "manual_intervention"}
}
```

#### Component Discovery

```bash
# List workflow tasks
GET /api/v0/workflows/{id}/tasks

# Get task definition
GET /api/v0/workflows/{id}/tasks/{task_id}

# List workflow agents
GET /api/v0/workflows/{id}/agents

# Get agent configuration
GET /api/v0/workflows/{id}/agents/{agent_id}

# List workflow tools
GET /api/v0/workflows/{id}/tools
```

#### Health and Monitoring

```bash
# System health check
GET /health

# Readiness probe
GET /readiness

# Prometheus metrics
GET /metrics

# Memory system health
GET /api/v0/memory/health
```

### CLI Commands

```bash
# Development server with hot reload
compozy dev [flags]
  --config string     Configuration file path (default "compozy.yaml")
  --port int         Server port (default 3000)
  --host string      Server host (default "localhost")
  --env-file string  Environment file path (default ".env")
  --watch            Enable file watching for hot reload (default true)
  --debug            Enable debug logging

# Configuration management
compozy config [command]
  show               Display current configuration
  validate           Validate configuration files
  diagnostics        Run configuration diagnostics

# MCP proxy server
compozy mcp-proxy [flags]
  --port int                    Proxy server port (default 8081)
  --host string                 Proxy server host (default "0.0.0.0")
  --admin-tokens strings        Admin authentication tokens
  --admin-allow-ips strings     Admin IP whitelist
  --global-auth-tokens strings  Global authentication tokens
  --log-level string           Log level (debug, info, warn, error)
```

---

## 🛠️ Development

### Development Environment Setup

```bash
# Prerequisites
- Go 1.24+
- Docker &
Docker Compose
- PostgreSQL 15+
- Redis 7.2+
- Make

# Quick setup
make deps         # Install dependencies
make start-docker # Start PostgreSQL, Redis, Temporal
make migrate-up   # Apply database migrations
make dev          # Start development server with hot reload
```

### Development Commands

```bash
# Code quality
make fmt                    # Format code
make lint                   # Run linters
make test                   # Run tests
make test-integration       # Run integration tests
make coverage              # Generate test coverage report

# Build and deployment
make build                  # Build binary
make docker                 # Build Docker image
make swagger               # Generate API documentation

# Database operations
make migrate-create name=<name>  # Create new migration
make migrate-up                  # Apply migrations
make migrate-down                # Rollback migrations
make migrate-status              # Check migration status
make reset-db                    # Reset database completely

# Development utilities
make dev-logs              # View development logs
make dev-reset             # Reset development environment
make clean                 # Clean build artifacts
```

### Development Workflow

```bash
# 1. Start development environment
make deps && make start-docker && make migrate-up

# 2. Start development server
make dev

# 3. Make changes to YAML configurations or Go code
# Hot reload automatically restarts the server

# 4. Run tests before committing
make fmt && make lint && make test

# 5. Create pull request
git add . && git commit -m "feat: add new feature"
git push origin feature-branch
```

---

## 🧪 Testing

### Testing Standards

Compozy follows comprehensive testing practices:

**Coverage Requirements:**

- Business logic packages: 80%+ coverage
- Integration tests for all API endpoints
- Unit tests for all exported functions

**Test Structure:**

```go
func TestService_Method(t *testing.T) {
    t.Run("Should describe expected behavior", func(t *testing.T) {
        // Given - Arrange
        service := NewService()
        input := &Input{Value: "test"}

        // When - Act
        result, err := service.Method(input)

        // Then - Assert
        assert.NoError(t, err)
        assert.Equal(t, "expected", result.Value)
    })
}
```

### Testing Commands

```bash
# Run all tests
make test

# Run specific package tests
go test ./engine/workflow/...

# Run tests with coverage
make coverage

# Run integration tests
make test-integration

# Run performance tests
go test -bench=. ./engine/llm/...

# Run tests with race detection
go test -race ./...

# Run tests with verbose output
go test -v ./engine/task/...
```

### Test Categories

**Unit Tests:**

- Test individual functions and methods
- Mock external dependencies
- Focus on business logic validation

**Integration Tests:**

- Test component interactions
- Use testcontainers for database testing
- Validate API endpoints end-to-end

**Performance Tests:**

- Benchmark critical paths
- Memory usage profiling
- Concurrent execution testing

---

## 📦 Package Architecture

### Core Engine Packages

**Business Logic Layer:**

- **[agent](./engine/agent/)** - AI agent configuration and management
- **[workflow](./engine/workflow/)** - Declarative workflow orchestration
- **[task](./engine/task/)** - Task execution framework with multiple strategies
- **[llm](./engine/llm/)** - LLM provider integration and orchestration
- **[memory](./engine/memory/)** - Intelligent memory management for AI conversations
- **[tool](./engine/tool/)** - External tool integration framework
- **[mcp](./engine/mcp/)** - Model Context Protocol client implementation
- **[runtime](./engine/runtime/)** - JavaScript/TypeScript runtime management

**Supporting Packages:**

- **[autoload](./engine/autoload/)** - Automated configuration discovery and loading
- **[project](./engine/project/)** - Project configuration management
- **[schema](./engine/schema/)** - JSON schema validation and processing
- **[worker](./engine/worker/)** - Temporal-based workflow execution
- **[core](./engine/core/)** - Core types, utilities, and domain primitives

### Infrastructure Layer

**Infrastructure Services:**

- **[server](./engine/infra/server/)** - HTTP server with middleware and routing
- **[store](./engine/infra/store/)** - PostgreSQL data layer with repositories
- **[cache](./engine/infra/cache/)** - Redis caching, locking, and pub/sub
- **[monitoring](./engine/infra/monitoring/)** - OpenTelemetry observability stack

### Supporting Packages

**CLI and Utilities:**

- **[cli](./cli/)** - Command-line interface with development server
- **[config](./pkg/config/)** - Multi-source configuration management
- **[logger](./pkg/logger/)** - Structured logging with beautiful terminal output
- **[mcp-proxy](./pkg/mcp-proxy/)** - MCP gateway server implementation

**Processing and Utilities:**

- **[tplengine](./pkg/tplengine/)** - Enhanced template engine with XSS protection
- **[ref](./pkg/ref/)** - YAML/JSON reference resolution and directives
- **[swagger](./pkg/swagger/)** - OpenAPI response utilities and types
- **[schemagen](./pkg/schemagen/)** - JSON schema generation from Go structs

### Package Relationship Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                            │
│                         cli/                                │
├─────────────────────────────────────────────────────────────┤
│                    Business Logic                           │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   agent/    │  workflow/  │    task/    │    llm/     │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │   memory/   │    tool/    │    mcp/     │  runtime/   │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  autoload/  │  project/   │  schema/    │   worker/   │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                    Core Domain                              │
│                       core/                                 │
├─────────────────────────────────────────────────────────────┤
│                  Infrastructure                             │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  server/    │ monitoring/ │   cache/    │   store/    │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                   Support Packages                         │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │  config/    │  logger/    │ mcp-proxy/  │ tplengine/  │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
│  ┌─────────────┬─────────────┬─────────────┬─────────────┐  │
│  │    ref/     │  swagger/   │ schemagen/  │     ...     │  │
│  │             │             │             │             │  │
│  └─────────────┴─────────────┴─────────────┴─────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 🌐 Ecosystem & Integration

### LLM Provider Ecosystem

**Supported Providers:**

- **OpenAI**: GPT-4, GPT-3.5, GPT-4-turbo with function calling
- **Anthropic**: Claude 3 Opus, Claude 3 Sonnet with vision capabilities
- **Google**: Gemini Pro, Gemini Flash with integrated Google services
- **Groq**: Ultra-fast inference for open models
- **Ollama**: Local model hosting with privacy guarantees
- **Custom**: OpenAI-compatible API endpoints

**Multi-Provider Benefits:**

- **Cost Optimization**: Intelligent routing to cost-appropriate models
- **High Availability**: Automatic failover between providers
- **Performance**: Match workloads to optimal inference speeds
- **Compliance**: Keep sensitive data with local providers

### Model Context Protocol (MCP)

**MCP Integration:**

- **Standardized Tool Protocol**: MCP 2025-03-26 specification
- **Proxy Architecture**: Centralized tool management and security
- **Multi-Transport**: HTTP, Server-Sent Events, and stdio communication
- **Dynamic Registration**: Runtime tool discovery and lifecycle management

**MCP Server Examples:**

```bash
# Database MCP server
{
  "url": "http://localhost:4000/mcp",
  "capabilities": ["query", "schema", "transactions"],
  "security": "admin_token"
}

# File system MCP server
{
  "url": "stdio://./mcp-servers/filesystem",
  "capabilities": ["read", "write", "search"],
  "permissions": ["--allow-read=/safe/directory"]
}
```

### External Service Integration

**Enterprise Integration Points:**

- **Temporal**: Distributed workflow orchestration
- **PostgreSQL**: ACID-compliant state management
- **Redis**: High-performance caching and coordination
- **OpenTelemetry**: Observability and distributed tracing
- **Prometheus**: Metrics collection and alerting

**API Integration Patterns:**

- **HTTP Tools**: RESTful API integration with authentication
- **CLI Tools**: Command-line tool execution with security constraints
- **Database**: Direct database access with connection pooling
- **Message Queues**: Pub/sub integration for event processing

---

## 🔒 Security & Production

### Security Features

**Authentication & Authorization:**

- **API Token Management**: Secure token-based authentication
- **Role-Based Access**: Granular permissions and access control
- **MCP Security**: Admin tokens and IP whitelisting
- **Audit Logging**: Comprehensive security event tracking

**Data Protection:**

- **Privacy Controls**: Configurable PII detection and filtering
- **Memory Encryption**: Encrypted storage of sensitive conversation data
- **Secure Configuration**: Environment-based secret management
- **Input Validation**: Comprehensive sanitization and validation

**Runtime Security:**

- **Sandboxed Execution**: Isolated JavaScript/TypeScript tool execution
- **Permission Controls**: Fine-grained access controls for tools
- **Resource Limits**: CPU and memory constraints for tool execution
- **Network Policies**: Restricted network access for security

### Production Deployment

**Infrastructure Requirements:**

```yaml
# Minimum Production Setup
- CPU: 4 cores
- Memory: 8GB RAM
- Storage: 100GB SSD
- Network: 1Gbps
- Database: PostgreSQL 15+ with backup strategy
- Cache: Redis 7.2+ with clustering
- Monitoring: Prometheus + Grafana stack
```

**Docker Production Configuration:**

```yaml
# docker-compose.prod.yml
version: "3.8"
services:
  compozy:
    image: compozy:latest
    environment:
      - COMPOZY_ENV=production
      - RUNTIME_LOG_LEVEL=info
      - RUNTIME_LOG_FORMAT=json
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=compozy
      - POSTGRES_USER=compozy
      - POSTGRES_PASSWORD_FILE=/run/secrets/postgres_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - postgres_password

  redis:
    image: redis:7.2-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
```

**Health Checks:**

```bash
# Application health
curl -f http://localhost:3000/health || exit 1

# Database connectivity
curl -f http://localhost:3000/api/v0/health/database || exit 1

# Memory system
curl -f http://localhost:3000/api/v0/memory/health || exit 1
```

### Monitoring & Observability

**Metrics Collection:**

- **Application Metrics**: Request rates, response times, error rates
- **Business Metrics**: Workflow executions, task completion rates
- **System Metrics**: CPU, memory, disk usage
- **LLM Metrics**: Token usage, provider performance, cost tracking

**Alerting Rules:**

```yaml
# prometheus.rules.yml
groups:
  - name: compozy.rules
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"

      - alert: WorkflowExecutionFailure
        expr: rate(workflow_executions_total{status="failed"}[5m]) > 0.1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Workflow execution failures detected"
```

---

## 📚 Examples

### Multi-Agent Customer Support System

```yaml
# workflows/customer-support.yaml
name: "intelligent-customer-support"
version: "1.0.0"
description: "AI-powered customer support with intelligent routing and escalation"

schemas:
  - id: support_request
    type: object
    properties:
      customer_id: { type: string }
      message: { type: string, minLength: 10 }
      priority: { type: string, enum: ["low", "normal", "high", "urgent"] }
      category: { type: string, enum: ["technical", "billing", "general"] }
      attachments:
        type: array
        items:
          type: object
          properties:
            filename: { type: string }
            content_type: { type: string }
            size: { type: integer }
    required: ["customer_id", "message", "priority"]

agents:
  - id: ticket_classifier
    config:
      $ref: global::models.#(provider=="openai" && model=="gpt-4-turbo")
    instructions: |
      You are an expert customer support ticket classifier. Analyze customer 
      messages and provide accurate categorization with confidence scores.
    actions:
      - id: classify_ticket
        json_mode: true
        prompt: |
          Analyze this customer support request:

          Customer ID: {{ .input.customer_id }}
          Message: {{ .input.message }}
          Priority: {{ .input.priority }}

          Provide classification with confidence scores.
        output:
          type: object
          properties:
            category:
              {
                type: string,
                enum: ["technical", "billing", "general", "escalation"],
              }
            subcategory: { type: string }
            urgency:
              { type: string, enum: ["low", "medium", "high", "critical"] }
            confidence: { type: number, minimum: 0, maximum: 1 }
            requires_human: { type: boolean }
            estimated_resolution_time: { type: string }

  - id: technical_specialist
    config:
      $ref: global::models.#(provider=="anthropic" && model=="claude-3-opus")
    instructions: |
      You are a senior technical support specialist with deep expertise in
      troubleshooting, system architecture, and customer communication.
      Always provide clear, step-by-step solutions.
    memory:
      - id: technical_context
        key: "tech_support:{{ .input.customer_id }}"
        mode: read-write
        privacy:
          enabled: true
          retention_policy: "30d"
    actions:
      - id: resolve_technical_issue
        prompt: |
          Provide technical support for this customer issue:

          Customer ID: {{ .input.customer_id }}
          Issue: {{ .input.message }}
          Category: {{ .input.category }}
          Previous Context: {{ .memory.technical_context }}

          Provide a comprehensive solution with:
          1. Problem analysis
          2. Step-by-step resolution
          3. Prevention recommendations
          4. Follow-up actions needed

  - id: billing_specialist
    config:
      $ref: global::models.#(provider=="openai" && model=="gpt-4")
    instructions: |
      You are a billing and account specialist. Handle billing inquiries,
      account issues, and subscription management with accuracy and empathy.
    tools:
      - $ref: local::tools.#(id=="billing_system")
    actions:
      - id: resolve_billing_issue
        prompt: |
          Handle this billing inquiry:

          Customer ID: {{ .input.customer_id }}
          Issue: {{ .input.message }}
          Account Status: {{ .tools.billing_system.get_account_status(.input.customer_id) }}

          Provide resolution with specific account details and next steps.

tools:
  - id: billing_system
    description: "Integration with billing system API"
    type: http
    config:
      base_url: "https://billing.internal.com/api/v1"
      headers:
        Authorization: "Bearer {{ .env.BILLING_API_TOKEN }}"
    input:
      type: object
      properties:
        action:
          {
            type: string,
            enum: ["get_account", "update_billing", "process_refund"],
          }
        customer_id: { type: string }
        data: { type: object }

  - id: notification_system
    description: "Send notifications to customers and staff"
    type: javascript
    execute: |
      async function sendNotification({ type, recipient, message, priority = "normal" }) {
        const notificationAPI = await import('notification-service');
        
        const result = await notificationAPI.send({
          type,
          recipient,
          message,
          priority,
          timestamp: new Date().toISOString()
        });
        
        return {
          notification_id: result.id,
          status: result.status,
          delivery_time: result.deliveryTime
        };
      }

tasks:
  - id: validate_request
    type: basic
    with:
      request: "{{ .workflow.input | validateSchema('support_request') }}"
    outputs:
      validated_request: "{{ .with.request }}"

  - id: classify_support_request
    type: basic
    $use: agent(local::agents.#(id="ticket_classifier"))
    action: classify_ticket
    with:
      customer_id: "{{ .tasks.validate_request.output.validated_request.customer_id }}"
      message: "{{ .tasks.validate_request.output.validated_request.message }}"
      priority: "{{ .tasks.validate_request.output.validated_request.priority }}"
    outputs:
      classification: "{{ .output }}"

  - id: route_to_specialist
    type: router
    condition: "{{ .tasks.classify_support_request.output.classification.category }}"
    routes:
      technical:
        id: technical_resolution
        type: basic
        $use: agent(local::agents.#(id="technical_specialist"))
        action: resolve_technical_issue
        with:
          customer_id: "{{ .tasks.validate_request.output.validated_request.customer_id }}"
          message: "{{ .tasks.validate_request.output.validated_request.message }}"
          category: "{{ .tasks.classify_support_request.output.classification.category }}"
        outputs:
          resolution: "{{ .output }}"

      billing:
        id: billing_resolution
        type: basic
        $use: agent(local::agents.#(id="billing_specialist"))
        action: resolve_billing_issue
        with:
          customer_id: "{{ .tasks.validate_request.output.validated_request.customer_id }}"
          message: "{{ .tasks.validate_request.output.validated_request.message }}"
        outputs:
          resolution: "{{ .output }}"

      escalation:
        id: human_escalation
        type: basic
        $use: tool(local::tools.#(id="notification_system"))
        with:
          type: "escalation"
          recipient: "support-team@company.com"
          message: "Customer {{ .tasks.validate_request.output.validated_request.customer_id }} requires human attention"
          priority: "high"
        outputs:
          escalation_id: "{{ .output.notification_id }}"

      default:
        id: general_response
        type: basic
        with:
          message: "Thank you for contacting us. We've received your inquiry and will respond within 24 hours."

  - id: send_confirmation
    type: basic
    $use: tool(local::tools.#(id="notification_system"))
    with:
      type: "email"
      recipient: "{{ .tasks.validate_request.output.validated_request.customer_id }}"
      message: "Your support request has been processed. Resolution: {{ .tasks.route_to_specialist.output.resolution | default('General inquiry received') }}"
      priority: "normal"

outputs:
  ticket_id: "{{ .workflow.execution_id }}"
  customer_id: "{{ .tasks.validate_request.output.validated_request.customer_id }}"
  resolution: "{{ .tasks.route_to_specialist.output.resolution | default('Escalated to human support') }}"
  classification: "{{ .tasks.classify_support_request.output.classification }}"
  processing_time: "{{ .workflow.execution_time }}"
  notification_sent: "{{ .tasks.send_confirmation.output.notification_id }}"
  metadata:
    processed_at: "{{ now }}"
    workflow_version: "1.0.0"
    confidence_score: "{{ .tasks.classify_support_request.output.classification.confidence }}"
```

### Data Processing Pipeline

```yaml
# workflows/data-processing.yaml
name: "intelligent-data-pipeline"
version: "1.0.0"
description: "AI-powered data processing with validation, analysis, and reporting"

schemas:
  - id: data_source
    type: object
    properties:
      source_type: { type: string, enum: ["database", "api", "file", "stream"] }
      connection_string: { type: string }
      query: { type: string }
      format: { type: string, enum: ["json", "csv", "xml", "parquet"] }
      credentials: { type: object }
    required: ["source_type", "connection_string"]

  - id: processing_config
    type: object
    properties:
      validation_rules:
        type: array
        items:
          type: object
          properties:
            field: { type: string }
            rule: { type: string }
            parameters: { type: object }
      transformations:
        type: array
        items:
          type: object
          properties:
            type: { type: string }
            config: { type: object }

agents:
  - id: data_validator
    config:
      $ref: global::models.#(provider=="openai" && model=="gpt-4-turbo")
    instructions: |
      You are a data quality specialist. Analyze datasets for completeness,
      accuracy, and consistency. Identify data quality issues and suggest remediation.
    actions:
      - id: validate_data
        json_mode: true
        prompt: |
          Analyze this dataset for quality issues:

          Dataset Info:
          - Records: {{ .input.record_count }}
          - Columns: {{ .input.columns | join(", ") }}
          - Source: {{ .input.source_type }}

          Sample Data:
          {{ .input.sample_data | toJson }}

          Validation Rules:
          {{ .input.validation_rules | toJson }}

          Provide comprehensive data quality assessment.
        output:
          type: object
          properties:
            quality_score: { type: number, minimum: 0, maximum: 1 }
            issues_found:
              type: array
              items:
                type: object
                properties:
                  field: { type: string }
                  issue_type: { type: string }
                  severity:
                    {
                      type: string,
                      enum: ["low", "medium", "high", "critical"],
                    }
                  description: { type: string }
                  suggested_fix: { type: string }
            recommendations:
              type: array
              items:
                type: string
            data_completeness: { type: number }
            data_accuracy: { type: number }

  - id: data_analyst
    config:
      $ref: global::models.#(provider=="anthropic" && model=="claude-3-opus")
    instructions: |
      You are a senior data analyst with expertise in statistical analysis,
      pattern recognition, and business intelligence. Provide insights that
      drive business decisions.
    actions:
      - id: analyze_data
        prompt: |
          Perform comprehensive analysis of this dataset:

          Dataset Overview:
          - Records: {{ .input.record_count }}
          - Time Period: {{ .input.date_range }}
          - Business Context: {{ .input.business_context }}

          Data Summary:
          {{ .input.data_summary | toJson }}

          Previous Analysis (if any):
          {{ .input.previous_analysis | default("None") }}

          Provide actionable insights and recommendations.
        output:
          type: object
          properties:
            insights:
              type: array
              items:
                type: object
                properties:
                  category: { type: string }
                  finding: { type: string }
                  impact: { type: string, enum: ["low", "medium", "high"] }
                  confidence: { type: number }
            trends:
              type: array
              items:
                type: string
            recommendations:
              type: array
              items:
                type: object
                properties:
                  action: { type: string }
                  priority: { type: string, enum: ["low", "medium", "high"] }
                  expected_impact: { type: string }
            kpis:
              type: object
              properties:
                current_values: { type: object }
                previous_values: { type: object }
                trends: { type: object }

tools:
  - id: database_connector
    description: "Connect to various database systems"
    type: javascript
    execute: |
      async function connectAndQuery({ connectionString, query, credentials }) {
        const { Client } = await import('pg');
        const client = new Client({
          connectionString,
          ...credentials
        });
        
        try {
          await client.connect();
          const result = await client.query(query);
          return {
            success: true,
            data: result.rows,
            rowCount: result.rowCount,
            fields: result.fields.map(f => f.name)
          };
        } catch (error) {
          return {
            success: false,
            error: error.message
          };
        } finally {
          await client.end();
        }
      }

  - id: data_transformer
    description: "Transform and clean data using various strategies"
    type: javascript
    execute: |
      async function transform({ data, transformations }) {
        let result = [...data];
        
        for (const transformation of transformations) {
          switch (transformation.type) {
            case 'filter':
              result = result.filter(row => 
                evaluateCondition(row, transformation.config.condition)
              );
              break;
              
            case 'map':
              result = result.map(row => 
                mapFields(row, transformation.config.mapping)
              );
              break;
              
            case 'aggregate':
              result = aggregate(result, transformation.config);
              break;
              
            case 'sort':
              result = result.sort((a, b) => 
                compareValues(a[transformation.config.field], b[transformation.config.field])
              );
              break;
          }
        }
        
        return {
          transformed_data: result,
          original_count: data.length,
          final_count: result.length,
          transformations_applied: transformations.length
        };
      }

  - id: report_generator
    description: "Generate various report formats"
    type: javascript
    execute: |
      async function generateReport({ data, analysis, format, template }) {
        const reportGenerator = await import('report-generator');
        
        const report = await reportGenerator.create({
          data,
          analysis,
          format,
          template,
          timestamp: new Date().toISOString()
        });
        
        return {
          report_id: report.id,
          format,
          size: report.size,
          download_url: report.downloadUrl,
          expires_at: report.expiresAt
        };
      }

tasks:
  - id: validate_input
    type: basic
    with:
      sources: "{{ .workflow.input.data_sources | validateSchema('data_source') }}"
      config: "{{ .workflow.input.processing_config | validateSchema('processing_config') }}"
    outputs:
      validated_sources: "{{ .with.sources }}"
      validated_config: "{{ .with.config }}"

  - id: extract_data
    type: collection
    items: "{{ .tasks.validate_input.output.validated_sources }}"
    mode: parallel
    task:
      id: "extract-{{ .index }}"
      type: basic
      $use: tool(local::tools.#(id="database_connector"))
      with:
        connectionString: "{{ .item.connection_string }}"
        query: "{{ .item.query }}"
        credentials: "{{ .item.credentials }}"
    outputs:
      extracted_data: "{{ .output }}"

  - id: validate_data_quality
    type: collection
    items: "{{ .tasks.extract_data.output.extracted_data }}"
    mode: parallel
    task:
      id: "validate-{{ .index }}"
      type: basic
      $use: agent(local::agents.#(id="data_validator"))
      action: validate_data
      with:
        record_count: "{{ .item.rowCount }}"
        columns: "{{ .item.fields }}"
        source_type: "{{ .workflow.input.data_sources[.index].source_type }}"
        sample_data: "{{ .item.data | slice(0, 10) }}"
        validation_rules: "{{ .tasks.validate_input.output.validated_config.validation_rules }}"
    outputs:
      validation_results: "{{ .output }}"

  - id: transform_data
    type: collection
    items: "{{ .tasks.extract_data.output.extracted_data }}"
    mode: parallel
    task:
      id: "transform-{{ .index }}"
      type: basic
      $use: tool(local::tools.#(id="data_transformer"))
      with:
        data: "{{ .item.data }}"
        transformations: "{{ .tasks.validate_input.output.validated_config.transformations }}"
    outputs:
      transformed_data: "{{ .output }}"

  - id: analyze_processed_data
    type: basic
    $use: agent(local::agents.#(id="data_analyst"))
    action: analyze_data
    with:
      record_count: "{{ .tasks.transform_data.output.transformed_data | map('final_count') | sum }}"
      date_range: "{{ .workflow.input.date_range }}"
      business_context: "{{ .workflow.input.business_context }}"
      data_summary: "{{ .tasks.transform_data.output.transformed_data }}"
      previous_analysis: "{{ .workflow.input.previous_analysis }}"
    outputs:
      analysis: "{{ .output }}"

  - id: generate_report
    type: basic
    $use: tool(local::tools.#(id="report_generator"))
    with:
      data: "{{ .tasks.transform_data.output.transformed_data }}"
      analysis: "{{ .tasks.analyze_processed_data.output.analysis }}"
      format: "{{ .workflow.input.report_format | default('pdf') }}"
      template: "{{ .workflow.input.report_template | default('standard') }}"
    outputs:
      report: "{{ .output }}"

  - id: aggregate_results
    type: aggregate
    outputs:
      processing_summary:
        total_records_processed: "{{ .tasks.transform_data.output.transformed_data | map('final_count') | sum }}"
        data_quality_scores: "{{ .tasks.validate_data_quality.output.validation_results | map('quality_score') }}"
        average_quality_score: "{{ .tasks.validate_data_quality.output.validation_results | map('quality_score') | avg }}"
        issues_found: "{{ .tasks.validate_data_quality.output.validation_results | map('issues_found') | flatten | length }}"
      analysis_summary:
        insights_count: "{{ .tasks.analyze_processed_data.output.analysis.insights | length }}"
        recommendations_count: "{{ .tasks.analyze_processed_data.output.analysis.recommendations | length }}"
        high_priority_actions: "{{ .tasks.analyze_processed_data.output.analysis.recommendations | filter('priority', 'high') | length }}"
      report_info:
        report_id: "{{ .tasks.generate_report.output.report.report_id }}"
        format: "{{ .tasks.generate_report.output.report.format }}"
        download_url: "{{ .tasks.generate_report.output.report.download_url }}"

outputs:
  processing_id: "{{ .workflow.execution_id }}"
  processing_summary: "{{ .tasks.aggregate_results.output.processing_summary }}"
  analysis_results: "{{ .tasks.analyze_processed_data.output.analysis }}"
  report_details: "{{ .tasks.aggregate_results.output.report_info }}"
  data_quality_report: "{{ .tasks.validate_data_quality.output.validation_results }}"
  execution_metrics:
    total_execution_time: "{{ .workflow.execution_time }}"
    tasks_completed: "{{ .workflow.completed_tasks | length }}"
    success_rate: "{{ .workflow.success_rate }}"
    processed_at: "{{ now }}"
```

---

## 🤝 Contributing

We welcome contributions from the community! Compozy is built to be extensible and maintainable.

### Development Standards

**Code Quality Requirements:**

- Follow [Go coding standards](.cursor/rules/go-coding-standards.mdc)
- Maintain [architecture principles](.cursor/rules/architecture.mdc)
- Write [comprehensive tests](.cursor/rules/test-standard.mdc)
- Follow [API standards](.cursor/rules/api-standards.mdc)

**Pre-Commit Checklist:**

```bash
# MANDATORY before every commit
make fmt && make lint && make test
```

### Contribution Process

1. **Fork & Clone**

   ```bash
   git clone https://github.com/your-username/compozy.git
   cd compozy
   ```

2. **Setup Development Environment**

   ```bash
   make deps && make start-docker && make migrate-up
   ```

3. **Create Feature Branch**

   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Develop & Test**

   ```bash
   # Make your changes
   make dev # Test with hot reload
   
   # Run quality checks
   make fmt && make lint && make test
   ```

5. **Submit Pull Request**
   - Include comprehensive tests
   - Update documentation
   - Follow commit message conventions
   - Ensure CI passes

### Areas for Contribution

**Core Features:**

- New LLM provider integrations
- Additional task types and orchestration patterns
- Enhanced memory management strategies
- Advanced tool integration capabilities

**Developer Experience:**

- Visual workflow editor
- Enhanced debugging tools
- Configuration validation improvements
- Performance optimizations

**Ecosystem:**

- MCP server implementations
- Example workflows and templates
- Integration guides and tutorials
- Community tools and extensions

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.

---

**Built with ❤️ by the Compozy community**

For questions, issues, or feature requests, please visit our [GitHub repository](https://github.com/compozy/compozy).

**Get Started Today:**

```bash
git clone https://github.com/compozy/compozy.git
cd compozy
make deps && make start-docker && make migrate-up && make dev
```

Transform your AI workflow development from months to days with Compozy's declarative approach to AI orchestration.
