# `workflow` – _Declarative AI Workflow Orchestration_

> **The workflow package provides a declarative configuration framework for defining and managing complex AI-powered workflows through YAML configuration, enabling seamless orchestration of agents, tools, and tasks with advanced scheduling and validation capabilities.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `workflow` package is the orchestration foundation of Compozy, providing a declarative YAML-based configuration system for defining complex AI workflows. It enables developers to create sophisticated multi-step processes that coordinate AI agents, external tools, and business logic through a powerful configuration-driven approach.

**Key Features:**

- 📝 **Declarative Configuration**: Define workflows through expressive YAML configuration
- 🤖 **AI Agent Coordination**: Seamless integration with multiple AI models and providers
- 🔧 **Tool Integration**: Comprehensive support for external tools and MCP servers
- 📊 **Schema Validation**: JSON Schema-based input/output validation and type safety
- 🕐 **Advanced Scheduling**: Cron-based scheduling with timezone support and overlap policies
- 🔄 **Event-Driven Triggers**: Signal-based triggers for reactive workflow execution
- 📏 **Template Engine**: Go template expressions for dynamic data flow between tasks
- 🎛️ **Conditional Logic**: Sophisticated branching and error handling capabilities

---

## 💡 Motivation

- **Declarative Simplicity**: Define complex workflows through configuration rather than code
- **AI-First Design**: Built specifically for orchestrating AI agents and intelligent processes
- **Scalable Architecture**: Support enterprise-grade workflow automation and orchestration
- **Developer Productivity**: Rapid workflow development with validation and testing support
- **Operational Excellence**: Comprehensive monitoring, scheduling, and error handling

---

## ⚡ Design Highlights

### Configuration-Driven Architecture

Workflows are defined entirely through YAML configuration, enabling rapid development, easy maintenance, and non-technical team collaboration without requiring code changes.

### AI Agent Orchestration

Native support for coordinating multiple AI agents with different capabilities, instructions, and tool access, enabling complex multi-agent workflows and specialized task distribution.

### Advanced Template System

Powerful Go template expressions enable dynamic data flow between tasks, conditional logic, and sophisticated data transformations throughout the workflow execution.

### Comprehensive Validation

JSON Schema-based validation ensures type safety and data integrity at every step, from input validation to task parameter checking and output verification.

### Enterprise Scheduling

Production-ready scheduling with cron expressions, timezone support, overlap policies, and execution windows for automated workflow management.

---

## 🚀 Getting Started

### Basic Workflow Definition

```yaml
# workflows/customer-support.yaml
id: customer-support
version: "1.0.0"
description: "Automated customer support workflow with ticket creation"

# Input validation
config:
  input:
    type: object
    properties:
      customer_email:
        type: string
        format: email
      issue_description:
        type: string
        minLength: 10
    required: [customer_email, issue_description]

# AI agent configuration
agents:
  - id: support-agent
    model: "gpt-4"
    instructions: |
      You are a helpful customer support agent. Analyze customer issues,
      determine priority, and provide clear, empathetic responses.
    temperature: 0.7

# External tool configuration
tools:
  - id: ticket-creator
    description: "Creates support tickets via API"
    input:
      type: object
      properties:
        title: { type: string }
        description: { type: string }
        priority: { type: string, enum: [low, medium, high] }

# Workflow execution sequence
tasks:
  - id: analyze-issue
    type: basic
    agent: support-agent
    with:
      prompt: |
        Analyze this customer issue:
        Customer: {{ .workflow.input.customer_email }}
        Issue: {{ .workflow.input.issue_description }}

        Provide:
        1. Issue category
        2. Priority level
        3. Suggested resolution
    on_success:
      next: create-ticket

  - id: create-ticket
    type: basic
    tool: ticket-creator
    with:
      title: "{{ .tasks.analyze-issue.output.category }}"
      description: "{{ .workflow.input.issue_description }}"
      priority: "{{ .tasks.analyze-issue.output.priority }}"
    final: true

# Output mapping
outputs:
  ticket_id: "{{ .tasks.create-ticket.output.ticket_id }}"
  category: "{{ .tasks.analyze-issue.output.category }}"
  priority: "{{ .tasks.analyze-issue.output.priority }}"
```

### Loading and Executing Workflows

```go
package main

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/workflow"
)

func main() {
    // Load workflow configuration
    cwd, _ := core.CWDFromPath("/path/to/project")
    config, err := workflow.Load(cwd, "workflows/customer-support.yaml")
    if err != nil {
        panic(err)
    }

    // Validate workflow configuration
    if err := config.Validate(); err != nil {
        panic(err)
    }

    // Validate input
    input := &core.Input{
        "customer_email": "user@example.com",
        "issue_description": "I can't access my account after password reset",
    }

    ctx := context.Background()
    if err := config.ValidateInput(ctx, input); err != nil {
        panic(err)
    }

    // Workflow is ready for execution
    fmt.Printf("Workflow %s validated and ready\n", config.ID)
}
```

---

## 📖 Usage

### Library

#### Workflow Configuration Management

```go
// Load workflow configuration
config, err := workflow.Load(cwd, "workflows/my-workflow.yaml")
if err != nil {
    return err
}

// Validate workflow structure
if err := config.Validate(); err != nil {
    return fmt.Errorf("invalid workflow: %w", err)
}

// Apply input defaults from schema
mergedInput, err := config.ApplyInputDefaults(input)
if err != nil {
    return fmt.Errorf("failed to apply defaults: %w", err)
}
```

#### Input Validation and Schema Handling

```go
// Validate workflow input
input := &core.Input{
    "user_id": "12345",
    "action": "process_payment",
    "amount": 99.99,
}

if err := config.ValidateInput(ctx, input); err != nil {
    return fmt.Errorf("invalid input: %w", err)
}

// Check for input schema
if config.HasSchema() {
    fmt.Println("Workflow has input validation schema")
}
```

#### Task Management and Navigation

```go
// Find specific task configuration
taskConfig, err := task.FindConfig(config.Tasks, "analyze-data")
if err != nil {
    return fmt.Errorf("task not found: %w", err)
}

// Determine next task based on execution result
success := true
nextTask := config.DetermineNextTask(taskConfig, success)
if nextTask != nil {
    fmt.Printf("Next task: %s\n", nextTask.ID)
}
```

#### Agent and Tool Discovery

```go
// Find agent configuration
agentConfig, err := workflow.FindAgentConfig[*agent.Config](workflows, "data-analyst")
if err != nil {
    return fmt.Errorf("agent not found: %w", err)
}

// Access workflow components
tools := config.Tools
agents := config.Agents
mcps := config.GetMCPs()
```

---

## 🔧 Configuration

### Complete Workflow Schema

```yaml
# Workflow identification
id: "workflow-id" # Required: Unique identifier
version: "1.0.0" # Optional: Semantic version
description: "Workflow description" # Optional: Human-readable description
author: # Optional: Author information
  name: "Developer Name"
  email: "dev@example.com"

# Configuration and validation
config:
  # Input schema (JSON Schema Draft 7)
  input:
    type: object
    properties:
      param1:
        type: string
        description: "Parameter description"
      param2:
        type: integer
        minimum: 1
        default: 10
    required: [param1]

  # Environment variables
  env:
    API_KEY: "{{ .secrets.API_KEY }}"
    BASE_URL: "https://api.example.com"

# Reusable schemas
schemas:
  - id: user_schema
    type: object
    properties:
      id: { type: string }
      email: { type: string, format: email }
      name: { type: string }

# AI agents
agents:
  - id: "agent-id"
    model: "gpt-4"
    instructions: "Agent instructions"
    temperature: 0.7
    tools: ["tool1", "tool2"]

# External tools
tools:
  - id: "tool-id"
    description: "Tool description"
    input:
      type: object
      properties:
        param: { type: string }

# MCP servers
mcps:
  - id: "mcp-id"
    url: "http://localhost:3000/mcp"
    proto: "2025-03-26"

# Event triggers
triggers:
  - type: signal
    name: "event-name"
    schema:
      type: object
      properties:
        event_data: { type: string }

  - type: webhook
    webhook:
      slug: "provider-events"
      method: POST
      events:
        - name: "checkout.session.completed"
          filter: payload.type == "checkout.session.completed"
          input:
            event_id: "{{ payload.id }}"

# Execution sequence
tasks:
  - id: "task-id"
    type: "basic"
    agent: agent-id
    with:
      prompt: "Task prompt with {{ .workflow.input.param1 }}"
    on_success:
      next: "next-task"
    on_error:
      next: "error-handler"

# Output mapping
outputs:
  result: "{{ .tasks.task-id.output.result }}"
  status: "success"

# Scheduling
schedule:
  cron: "0 9 * * MON-FRI"
  timezone: "America/New_York"
  overlap_policy: "skip"
  enabled: true
```

### Schedule Configuration

```yaml
schedule:
  # Cron expression (required)
  cron: "0 9 * * MON-FRI" # 9 AM on weekdays

  # Timezone (optional, default UTC)
  timezone: "America/New_York"

  # Overlap policy (optional, default skip)
  overlap_policy: "skip" # skip, allow, buffer_one, cancel_other

  # Schedule enabled state (optional, default true)
  enabled: true

  # Execution window (optional)
  start_at: "2024-01-01T00:00:00Z"
  end_at: "2024-12-31T23:59:59Z"

  # Random execution delay (optional)
  jitter: "5m"

  # Default input for scheduled runs (optional)
  input:
    source: "scheduled"
    priority: "normal"
```

### Trigger Configuration

```yaml
triggers:
  - type: signal
    name: "user-registered"
    schema:
      type: object
      properties:
        user_id:
          type: string
        email:
          type: string
          format: email
        registration_date:
          type: string
          format: date-time
      required: [user_id, email]

  - type: signal
    name: "order-completed"
    schema: order_schema
```

---

## 🎨 Examples

### Content Generation Workflow

```yaml
id: content-generation
version: "1.0.0"
description: "AI-powered content generation and optimization workflow"

config:
  input:
    type: object
    properties:
      topic:
        type: string
        description: "Content topic"
      audience:
        type: string
        enum: [general, technical, business]
        default: general
      word_count:
        type: integer
        minimum: 100
        maximum: 5000
        default: 1000
    required: [topic]

agents:
  - id: content-writer
    model: "gpt-4"
    instructions: |
      You are a skilled content writer. Create engaging, well-structured
      content tailored to the specified audience and word count.
    temperature: 0.8

  - id: content-optimizer
    model: "gpt-4"
    instructions: |
      You are a content optimization expert. Improve readability,
      SEO, and engagement while maintaining the original message.
    temperature: 0.6

tools:
  - id: plagiarism-checker
    description: "Check content for plagiarism"
    input:
      type: object
      properties:
        content: { type: string }
    output:
      type: object
      properties:
        is_original: { type: boolean }
        similarity_score: { type: number }

tasks:
  - id: generate-outline
    type: basic
    agent: content-writer
    with:
      prompt: |
        Create a detailed outline for {{ .workflow.input.topic }}
        Target audience: {{ .workflow.input.audience }}
        Word count: {{ .workflow.input.word_count }}
    on_success:
      next: write-content

  - id: write-content
    type: basic
    agent: content-writer
    with:
      prompt: |
        Write content based on this outline:
        {{ .tasks.generate-outline.output.outline }}

        Requirements:
        - Topic: {{ .workflow.input.topic }}
        - Audience: {{ .workflow.input.audience }}
        - Word count: ~{{ .workflow.input.word_count }} words
    on_success:
      next: check-plagiarism

  - id: check-plagiarism
    type: basic
    tool: plagiarism-checker
    with:
      content: "{{ .tasks.write-content.output.content }}"
    on_success:
      next: optimize-content
    on_error:
      next: revise-content

  - id: optimize-content
    type: basic
    agent: content-optimizer
    with:
      prompt: |
        Optimize this content for readability and SEO:
        {{ .tasks.write-content.output.content }}

        Keep the same tone and target audience: {{ .workflow.input.audience }}
    final: true

  - id: revise-content
    type: basic
    agent: content-writer
    with:
      prompt: |
        Revise this content to be more original:
        {{ .tasks.write-content.output.content }}

        Plagiarism score: {{ .tasks.check-plagiarism.output.similarity_score }}
    on_success:
      next: optimize-content

outputs:
  content: "{{ .tasks.optimize-content.output.content }}"
  word_count: "{{ .tasks.optimize-content.output.word_count }}"
  readability_score: "{{ .tasks.optimize-content.output.readability_score }}"
  is_original: "{{ .tasks.check-plagiarism.output.is_original }}"
```

### Data Processing Pipeline

```yaml
id: data-processing-pipeline
version: "1.2.0"
description: "ETL pipeline with validation and transformation"

config:
  input:
    type: object
    properties:
      data_source:
        type: string
        enum: [database, api, file]
      source_path:
        type: string
      transformation_rules:
        type: array
        items:
          type: object
          properties:
            field: { type: string }
            operation: { type: string }
            value: { type: string }
    required: [data_source, source_path]

schemas:
  - id: data_record
    type: object
    properties:
      id: { type: string }
      timestamp: { type: string, format: date-time }
      data: { type: object }
      status: { type: string, enum: [valid, invalid, processed] }

tools:
  - id: data-extractor
    description: "Extract data from various sources"
    timeout: "5m"
    input:
      type: object
      properties:
        source: { type: string }
        path: { type: string }
    output:
      type: object
      properties:
        records: { type: array }
        metadata: { type: object }

  - id: data-validator
    description: "Validate data against schema"
    input:
      type: object
      properties:
        records: { type: array }
        schema: { type: object }

  - id: data-transformer
    description: "Transform data using rules"
    input:
      type: object
      properties:
        records: { type: array }
        rules: { type: array }

  - id: data-loader
    description: "Load data to destination"
    input:
      type: object
      properties:
        records: { type: array }
        destination: { type: string }

tasks:
  - id: extract-data
    type: basic
    tool: data-extractor
    with:
      source: "{{ .workflow.input.data_source }}"
      path: "{{ .workflow.input.source_path }}"
    on_success:
      next: validate-data

  - id: validate-data
    type: basic
    tool: data-validator
    with:
      records: "{{ .tasks.extract-data.output.records }}"
      schema: '{{ .local.schemas.#(id="data_record") }}'
    on_success:
      next: transform-data

  - id: transform-data
    type: basic
    tool: data-transformer
    with:
      records: "{{ .tasks.validate-data.output.valid_records }}"
      rules: "{{ .workflow.input.transformation_rules }}"
    on_success:
      next: load-data

  - id: load-data
    type: basic
    tool: data-loader
    with:
      records: "{{ .tasks.transform-data.output.transformed_records }}"
      destination: "processed_data_table"
    final: true

outputs:
  processed_count: "{{ .tasks.load-data.output.processed_count }}"
  validation_errors: "{{ .tasks.validate-data.output.errors }}"
  transformation_summary: "{{ .tasks.transform-data.output.summary }}"

schedule:
  cron: "0 2 * * *" # Daily at 2 AM
  timezone: "UTC"
  overlap_policy: "skip"
  enabled: true
```

### Event-Driven User Onboarding

```yaml
id: user-onboarding
version: "1.0.0"
description: "Automated user onboarding with personalized communication"

config:
  input:
    type: object
    properties:
      user_id:
        type: string
      email:
        type: string
        format: email
      user_type:
        type: string
        enum: [individual, business]
        default: individual
      preferences:
        type: object
        properties:
          communication_frequency:
            type: string
            enum: [daily, weekly, monthly]
            default: weekly
    required: [user_id, email]

agents:
  - id: onboarding-specialist
    model: "gpt-4"
    instructions: |
      You are a friendly onboarding specialist. Create personalized
      welcome messages and guide users through their first steps.
    temperature: 0.7

tools:
  - id: email-sender
    description: "Send personalized emails"
    input:
      type: object
      properties:
        to: { type: string }
        subject: { type: string }
        body: { type: string }
        template: { type: string }

  - id: user-profiler
    description: "Create user profile and preferences"
    input:
      type: object
      properties:
        user_id: { type: string }
        email: { type: string }
        user_type: { type: string }
        preferences: { type: object }

triggers:
  - type: signal
    name: user-registered
    schema: workflow_input

  - type: signal
    name: email-verified
    schema:
      type: object
      properties:
        user_id: { type: string }
        verified_at: { type: string, format: date-time }

tasks:
  - id: create-profile
    type: basic
    tool: user-profiler
    with:
      user_id: "{{ .workflow.input.user_id }}"
      email: "{{ .workflow.input.email }}"
      user_type: "{{ .workflow.input.user_type }}"
      preferences: "{{ .workflow.input.preferences }}"
    on_success:
      next: generate-welcome-message

  - id: generate-welcome-message
    type: basic
    agent: onboarding-specialist
    with:
      prompt: |
        Create a personalized welcome message for:
        - User type: {{ .workflow.input.user_type }}
        - Email: {{ .workflow.input.email }}
        - Preferences: {{ .workflow.input.preferences }}

        Include:
        1. Warm welcome
        2. Next steps guide
        3. Key features highlight
        4. Support contact information
    on_success:
      next: send-welcome-email

  - id: send-welcome-email
    type: basic
    tool: email-sender
    with:
      to: "{{ .workflow.input.email }}"
      subject: "Welcome to Our Platform!"
      body: "{{ .tasks.generate-welcome-message.output.message }}"
      template: "welcome-{{ .workflow.input.user_type }}"
    on_success:
      next: wait-for-verification

  - id: wait-for-verification
    type: wait
    for: "email-verified"
    timeout: "7d"
    filter:
      user_id: "{{ .workflow.input.user_id }}"
    on_success:
      next: send-getting-started
    on_timeout:
      next: send-reminder

  - id: send-reminder
    type: basic
    tool: email-sender
    with:
      to: "{{ .workflow.input.email }}"
      subject: "Don't forget to verify your email"
      template: "email-reminder"
    final: true

  - id: send-getting-started
    type: basic
    agent: onboarding-specialist
    with:
      prompt: |
        Create a getting started guide for {{ .workflow.input.user_type }} user.
        Include step-by-step instructions for key features.
    on_success:
      next: send-followup-email

  - id: send-followup-email
    type: basic
    tool: email-sender
    with:
      to: "{{ .workflow.input.email }}"
      subject: "Getting Started Guide"
      body: "{{ .tasks.send-getting-started.output.guide }}"
      template: "getting-started"
    final: true

outputs:
  onboarding_status: "{{ .workflow.status }}"
  user_profile_created: "{{ .tasks.create-profile.output.success }}"
  email_verified: "{{ .tasks.wait-for-verification.output.completed }}"
  welcome_sent: "{{ .tasks.send-welcome-email.output.sent }}"
```

---

## 📚 API Reference

### Core Types

#### `Config`

Main workflow configuration struct.

```go
type Config struct {
    ID          string        `json:"id"`
    Version     string        `json:"version"`
    Description string        `json:"description"`
    Author      *core.Author  `json:"author"`
    Schemas     []schema.Schema `json:"schemas"`
    Opts        Opts          `json:"config"`
    Tools       []tool.Config `json:"tools"`
    Agents      []agent.Config `json:"agents"`
    MCPs        []mcp.Config  `json:"mcps"`
    Triggers    []Trigger     `json:"triggers"`
    Tasks       []task.Config `json:"tasks"`
    Outputs     *core.Output  `json:"outputs"`
    Schedule    *Schedule     `json:"schedule"`
}
```

**Key Methods:**

- `Validate() error` - Validates workflow configuration
- `ValidateInput(ctx context.Context, input *core.Input) error` - Validates input parameters
- `ApplyInputDefaults(input *core.Input) (*core.Input, error)` - Applies schema defaults
- `DetermineNextTask(taskConfig *task.Config, success bool) *task.Config` - Determines next task
- `GetTasks() []task.Config` - Gets workflow tasks
- `GetMCPs() []mcp.Config` - Gets MCP configurations
- `HasSchema() bool` - Checks if workflow has input schema

#### `Schedule`

Scheduling configuration for automated execution.

```go
type Schedule struct {
    Cron          string        `yaml:"cron"`
    Timezone      string        `yaml:"timezone"`
    Enabled       *bool         `yaml:"enabled"`
    Jitter        string        `yaml:"jitter"`
    OverlapPolicy OverlapPolicy `yaml:"overlap_policy"`
    StartAt       *time.Time    `yaml:"start_at"`
    EndAt         *time.Time    `yaml:"end_at"`
    Input         map[string]any `yaml:"input"`
}
```

#### `Trigger`

Event trigger configuration.

```go
type Trigger struct {
    Type   TriggerType   `json:"type"`
    Name   string        `json:"name"`
    Schema *schema.Schema `json:"schema"`
}
```

#### `State`

Runtime workflow state management.

```go
type State struct {
    WorkflowID      string         `json:"workflow_id"`
    WorkflowExecID  core.ID        `json:"workflow_exec_id"`
    Input           *core.Input    `json:"input"`
    Tasks           map[string]*task.State `json:"tasks"`
    Status          string         `json:"status"`
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
}
```

### Functions

#### `Load(cwd *core.PathCWD, path string) (*Config, error)`

Loads workflow configuration from file.

```go
cwd, _ := core.CWDFromPath("/project")
config, err := workflow.Load(cwd, "workflows/my-workflow.yaml")
```

#### `FindConfig(workflows []*Config, workflowID string) (*Config, error)`

Finds workflow configuration by ID.

```go
config, err := workflow.FindConfig(workflows, "my-workflow")
```

#### `FindAgentConfig[C core.Config](workflows []*Config, agentID string) (C, error)`

Finds agent configuration across workflows.

```go
agentConfig, err := workflow.FindAgentConfig[*agent.Config](workflows, "my-agent")
```

#### `WorkflowsFromProject(projectConfig *project.Config) ([]*Config, error)`

Loads all workflows from the project configuration.

```go
workflows, err := workflow.WorkflowsFromProject(projectConfig)
```

### Validation

#### `NewWorkflowValidator(config *Config) *Validator`

Creates comprehensive workflow validator.

#### `NewInputValidator(config *Config, input *core.Input) *InputValidator`

Creates input validator for workflow.

#### `ValidateSchedule(cfg *Schedule) error`

Validates schedule configuration.

---

## 🧪 Testing

### Unit Tests

```go
func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "Should validate valid workflow",
            config: &Config{
                ID:          "test-workflow",
                Description: "Test workflow",
            Tasks: []task.Config{
                {
                    ID:   "test-task",
                    Type: "basic",
                    Agent: &agent.Config{ID: "test-agent"}, // or Agent: selector ID
                },
            },
            },
            wantErr: false,
        },
        {
            name: "Should reject workflow without tasks",
            config: &Config{
                ID:          "test-workflow",
                Description: "Test workflow",
                Tasks:       []task.Config{},
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

```go
func TestWorkflow_LoadAndValidate(t *testing.T) {
    // Setup test workflow file
    workflowPath := createTestWorkflow(t, `
id: test-workflow
description: Test workflow
tasks:
  - id: test-task
    type: basic
    agent: test-agent
    with:
      prompt: "Test prompt"
`)

    // Load workflow
    cwd, _ := core.CWDFromPath(filepath.Dir(workflowPath))
    config, err := workflow.Load(cwd, filepath.Base(workflowPath))
    require.NoError(t, err)

    // Validate configuration
    assert.Equal(t, "test-workflow", config.ID)
    assert.Len(t, config.Tasks, 1)
    assert.Equal(t, "test-task", config.Tasks[0].ID)

    // Validate workflow structure
    err = config.Validate()
    assert.NoError(t, err)
}
```

### Schedule Tests

```go
func TestSchedule_Validate(t *testing.T) {
    tests := []struct {
        name     string
        schedule *Schedule
        wantErr  bool
    }{
        {
            name: "Should validate valid cron schedule",
            schedule: &Schedule{
                Cron:     "0 9 * * MON-FRI",
                Timezone: "UTC",
                Enabled:  &[]bool{true}[0],
            },
            wantErr: false,
        },
        {
            name: "Should reject invalid cron expression",
            schedule: &Schedule{
                Cron:     "invalid-cron",
                Timezone: "UTC",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := workflow.ValidateSchedule(tt.schedule)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Best Practices

1. **Validate configuration** before execution
2. **Test with realistic data** and edge cases
3. **Use schema validation** for type safety
4. **Test error scenarios** and recovery paths
5. **Mock external dependencies** in unit tests
6. **Test template expressions** with various inputs
7. **Verify task sequencing** and conditional logic

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
