# Normalizer Package

The normalizer package provides template-based configuration normalization for Compozy components.
It allows you to use dynamic template expressions in task, agent, and tool configurations that are
resolved at runtime using workflow state and configuration context.

## Purpose

The normalizer enables dynamic configuration values by:

- **Template Resolution**: Processes Go template expressions in configuration fields
- **Context Injection**: Provides access to workflow state, task outputs, configuration properties,
  and runtime data
- **Configuration Normalization**: Ensures configurations are resolved before component execution
- **Cross-Component Communication**: Allows components to reference outputs and properties from
  other components

## Context Hierarchy

The normalizer provides a hierarchical context structure that reflects the Compozy component
relationships:

```
Workflow
  ├── Tasks
  │   ├── Tasks (nested)
  │   ├── Agents
  │   │   └── Actions
  │   └── Tools
  └── Environment Variables
```

### Parent-Child Relationships

- **Workflow → Task**: When a task is executed within a workflow, the workflow becomes its parent
- **Task → Task**: When a task calls another task, the calling task becomes the parent of the called
  task
- **Task → Agent**: When an agent is used by a task, the task becomes its parent
- **Task → Tool**: When a tool is used by a task, the task becomes its parent
- **Agent → Action**: When an action is defined within an agent, the agent becomes its parent

## Template Syntax

Templates use Go template syntax with `{{ }}` delimiters:

```yaml
# Basic template
input:
    message: "Hello {{ .workflow.input.name }}!"

# Accessing task outputs
input:
    data: "{{ .tasks.previous_task.output.result }}"

# Accessing configuration properties
env:
    TASK_TYPE: "{{ .parent.type }}"
    WORKFLOW_VERSION: "{{ .workflow.version }}"
```

## Available Context

### Workflow Context

Access workflow runtime data and configuration properties:

```yaml
# Workflow input (runtime data)
input:
    user_id: "{{ .workflow.input.user_id }}"

# Workflow output (if available)
input:
    final_result: "{{ .workflow.output.summary }}"

# Workflow configuration properties
input:
    workflow_id: "{{ .workflow.id }}"
    workflow_version: "{{ .workflow.version }}"
    workflow_description: "{{ .workflow.description }}"
```

### Parent Context

Access configuration properties and runtime data from the immediate parent component:

```yaml
# For a task's agent or tool:
instructions: |
    Processing data for task: {{ .parent.id }}
    Task type: {{ .parent.type }}
    Task action: {{ .parent.action }}

# For an agent's action:
prompt: |
    Continue the conversation based on:
    Agent ID: {{ .parent.id }}
    Agent instructions: {{ .parent.instructions }}

# Access parent input
input:
    city: "{{ .parent.input.city }}"
```

### Tasks Context

Access both configuration properties and runtime data from other tasks in the workflow:

```yaml
# Task runtime outputs
input:
    processed_data: "{{ .tasks.data_processor.output.result }}"

# Task configuration properties
input:
    depends_on_type: "{{ .tasks.validator.type }}"
    action_to_call: "{{ .tasks.executor.action }}"

# Task runtime inputs
condition: "{{ eq .tasks.analyzer.input.mode \"advanced\" }}"
```

### Parallel Task Output Access

For parallel tasks, sub-task outputs are accessible using a nested structure:

```yaml
# Access sub-task outputs from parallel tasks
input:
    # Format: .tasks.<parallel_task_id>.output.<sub_task_id>.output.<field>
    sentiment_result: "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.sentiment }}"
    keywords_list: "{{ .tasks.process_data_parallel.output.extract_keywords.output.keywords }}"
    confidence_score: "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.confidence }}"

# You can also access entire sub-task outputs
full_sentiment_output: "{{ .tasks.process_data_parallel.output.sentiment_analysis.output }}"
```

This structure allows you to:
- Access individual fields from specific sub-tasks
- Get complete output objects from sub-tasks
- Reference multiple sub-task results in a single configuration

### Current Input Context

Access the current component's input parameters:

```yaml
# Reference your own input parameters
instructions: |
    Process the data with these settings:
    - City: {{ .input.city }}
    - Mode: {{ .input.processing_mode }}
    - Timeout: {{ .input.timeout | default "30" }}
```

### Environment Context

Access merged environment variables from all levels (workflow + parent + current):

```yaml
# Environment variables
env:
    API_URL: "{{ .env.BASE_URL }}/api/v1"
    DEBUG_MODE: "{{ .env.DEBUG | default \"false\" }}"

# Use in other fields
instructions: |
    Connect to: {{ .env.DATABASE_URL }}
    Debug mode: {{ .env.DEBUG }}
```

## Context Structure

```go
{
  "workflow": {
    // Runtime data
    "input":  map[string]any,      // Workflow input parameters
    "output": map[string]any,      // Workflow output (if available)
    
    // Configuration properties
    "id":          string,         // Workflow ID
    "version":     string,         // Workflow version
    "description": string,         // Workflow description
    "author":      map[string]any, // Author information
    // ... other workflow config properties
  },
  "parent": {
    // All configuration properties of the parent component
    "id":           string,        // Parent ID
    "type":         string,        // Parent type (for tasks)
    "action":       string,        // Parent action (for basic tasks)
    "condition":    string,        // Parent condition (for router tasks)
    "instructions": string,        // Parent instructions (for agents)
    "execute":      string,        // Parent execute path (for tools)
    // ... other parent config properties
    
    // Runtime data
    "input":  map[string]any,      // Parent input parameters
    "output": map[string]any,      // Parent output (if available)
  },
  "tasks": {
    "task_id": {
      // Runtime data
      "input":  map[string]any,    // Task input parameters
      "output": map[string]any,    // Task output results
      
      // Configuration properties
      "id":     string,            // Task ID
      "type":   string,            // Task type (basic/router)
      "action": string,            // Task action (for basic tasks)
      "final":  bool,              // Whether task is final
      // ... other task config properties
    }
  },
  "input": map[string]any,         // Current component input
  "env": map[string]string,        // Merged environment variables
}
```

## Usage Examples

### Basic Task Configuration

```yaml
# task.yaml
id: send_notification
type: basic
action: notify_user
with:
    recipient: "{{ .workflow.input.email }}"
    subject: "Processing completed for {{ .workflow.input.request_id }}"
    workflow_version: "{{ .workflow.version }}"
env:
    API_KEY: "{{ .workflow.input.api_key }}"
```

### Task Referencing Other Tasks

```yaml
# analysis_task.yaml
id: analyze_results
type: basic
with:
    data: "{{ .tasks.data_collector.output.dataset }}"
    previous_action: "{{ .tasks.data_collector.action }}"
    analyzer_type: "{{ .tasks.config_loader.type }}"
    threshold: "{{ .tasks.config_loader.output.settings.threshold }}"
```

### Parallel Task Results Aggregation

```yaml
# aggregator_task.yaml
id: aggregate_analysis_results
type: basic
action: aggregate
with:
    # Access individual sub-task outputs from parallel execution
    sentiment: "{{ .tasks.parallel_processor.output.sentiment_analysis.output.sentiment }}"
    keywords: "{{ .tasks.parallel_processor.output.keyword_extraction.output.keywords }}"
    confidence: "{{ .tasks.parallel_processor.output.sentiment_analysis.output.confidence }}"
    
    # Access entire sub-task output objects
    full_sentiment_data: "{{ .tasks.parallel_processor.output.sentiment_analysis.output }}"
    
    # Combine results from multiple sub-tasks
    summary:
        sentiment: "{{ .tasks.parallel_processor.output.sentiment_analysis.output.sentiment }}"
        keyword_count: "{{ len .tasks.parallel_processor.output.keyword_extraction.output.keywords }}"
        processing_time: "{{ .tasks.parallel_processor.output.performance_monitor.output.duration }}"
```

### Task Calling Another Task

```yaml
# subtask.yaml
id: process_item
type: basic
action: process
with:
    # Access parent task's properties
    item: "{{ .parent.input.current_item }}"
    batch_id: "{{ .parent.id }}"
    parent_action: "{{ .parent.action }}"

    # Access workflow context through parent hierarchy
    workflow_id: "{{ .workflow.id }}"

    # Access sibling tasks (other tasks in the workflow)
    config: "{{ .tasks.config_loader.output.settings }}"
env:
    PARENT_TASK: "{{ .parent.id }}"
    PARENT_TYPE: "{{ .parent.type }}"
```

```yaml
# parent_task.yaml
id: batch_processor
type: basic
action: batch_process
with:
    items: "{{ .workflow.input.items }}"
# This task would call process_item for each item
```

### Agent Configuration

```yaml
# agent.yaml
id: data_processor
config:
    model: gpt-4
instructions: |
    You are processing data for the {{ .parent.id }} task.
    The task type is {{ .parent.type }}.

    Workflow context:
    - Workflow ID: {{ .workflow.id }}
    - Workflow Version: {{ .workflow.version }}

    Data to process: {{ .tasks.data_fetcher.output.raw_data }}

actions:
    - id: process_city
      prompt: |
          Process the data for city: {{ .parent.input.city }}
          Using agent: {{ .parent.id }}
          With instructions: {{ .parent.instructions }}
```

### Tool Configuration

```yaml
# tool.yaml
id: api_caller
execute: "{{ .env.SCRIPTS_PATH }}/api_call.ts"
description: "API caller for {{ .parent.id }} task of type {{ .parent.type }}"
with:
    endpoint: "{{ .env.API_BASE_URL }}/{{ .input.endpoint_path }}"
    task_action: "{{ .parent.action }}"
    workflow_id: "{{ .workflow.id }}"
    headers:
        authorization: "Bearer {{ .parent.input.token }}"
        x-workflow-id: "{{ .workflow.id }}"
        x-task-id: "{{ .parent.id }}"
```

### Agent Action Configuration

```yaml
# Within an agent configuration
actions:
    - id: analyze_data
      prompt: |
          Analyze the following data:

          Agent context:
          - Agent ID: {{ .parent.id }}
          - Agent Instructions: {{ .parent.instructions }}
          - Agent Model: {{ .parent.config.model }}

          Task context:
          - City to analyze: {{ .parent.input.city }}
          - Analysis mode: {{ .parent.input.mode }}

          Previous results: {{ .tasks.preprocessing.output.summary }}
      with:
          context: "{{ .parent.input.context }}"
          threshold: "{{ .parent.input.threshold | default \"0.8\" }}"
```

## Common Patterns

### Accessing Parent Properties

```yaml
# In an agent/tool used by a task
instructions: |
    Task ID: {{ .parent.id }}
    Task Type: {{ .parent.type }}
    Task Action: {{ .parent.action }}
    Task Final: {{ .parent.final }}

# In an agent action
prompt: |
    Agent ID: {{ .parent.id }}
    Agent Instructions: {{ .parent.instructions }}
    Agent Model: {{ .parent.config.model }}
```

### Accessing Workflow Properties

```yaml
input:
    workflow_info:
        id: "{{ .workflow.id }}"
        version: "{{ .workflow.version }}"
        description: "{{ .workflow.description }}"
        author_name: "{{ .workflow.author.name }}"
        author_email: "{{ .workflow.author.email }}"
```

### Accessing Task Properties

```yaml
input:
    # Access another task's configuration
    dependent_task_type: "{{ .tasks.validator.type }}"
    dependent_task_action: "{{ .tasks.validator.action }}"

    # Access another task's runtime data
    validation_result: "{{ .tasks.validator.output.is_valid }}"
    validation_input: "{{ .tasks.validator.input.data }}"
```

## Template Functions

The normalizer supports Sprig template functions for advanced operations:

### String Operations

```yaml
input:
    # Uppercase
    name: "{{ .workflow.input.name | upper }}"

    # String replacement
    cleaned_id: "{{ .parent.id | replace \"-\" \"_\" }}"

    # Trimming
    trimmed: "{{ .workflow.description | trim }}"
```

### Conditional Logic

```yaml
input:
    # If-else based on parent type
    mode: "{{ if eq .parent.type \"router\" }}branching{{ else }}linear{{ end }}"

    # Default values
    timeout: "{{ .workflow.input.timeout | default \"30\" }}"

    # Check parent properties
    is_final_task: "{{ .parent.final }}"
```

### Data Manipulation

```yaml
input:
    # JSON encoding
    config: "{{ .parent | toJson }}"

    # Array operations (if tasks returns a list)
    first_item: "{{ index .tasks.list_processor.output.items 0 }}"

    # Access nested properties safely
    nested_value: "{{ .workflow.author.email | default \"no-reply@example.com\" }}"
```

## Best Practices

### 1. Use Configuration Properties for Static Values

```yaml
# Good: Use configuration properties that don't change
input:
    task_type: "{{ .parent.type }}"
    workflow_version: "{{ .workflow.version }}"

# Avoid: Don't rely on runtime state for configuration
input:
    task_type: "{{ if .parent.output }}processed{{ else }}pending{{ end }}"
```

### 2. Understand Parent Context

```yaml
# In a tool/agent, parent is the task
instructions: |
    Processing for task: {{ .parent.id }}
    Task action: {{ .parent.action }}

# In an agent action, parent is the agent
prompt: |
    Using agent: {{ .parent.id }}
    With model: {{ .parent.config.model }}
```

### 3. Handle Missing Values Gracefully

```yaml
# Always provide defaults for optional values
env:
    DEBUG: "{{ .workflow.input.debug | default \"false\" }}"
    VERSION: "{{ .workflow.version | default \"1.0.0\" }}"
```

### 4. Use Descriptive Property Access

```yaml
# Good: Clear what property you're accessing
input:
    executor_action: "{{ .tasks.executor.action }}"
    validator_type: "{{ .tasks.validator.type }}"

# Avoid: Ambiguous property access
input:
    value: "{{ .tasks.task1.property }}"
```

## Error Handling

The normalizer will return detailed error messages for:

- **Template Parsing Errors**: Invalid Go template syntax
- **Context Access Errors**: Attempting to access non-existent fields
- **Type Conversion Errors**: Invalid type operations in templates

Example error:

```
failed to normalize task config input: failed to parse template in input[data]: 
template: :1:2: executing "" at <.parent.nonexistent>: 
map has no entry for key "nonexistent"
```

## Integration

The normalizer is automatically used in the component execution flow:

1. **Configuration Loading**: Component configurations are loaded with their properties
2. **Context Building**: Runtime state and configuration properties provide template context
3. **Template Resolution**: All template expressions are resolved with access to configuration
   properties
4. **Execution**: Normalized configurations are used to execute components

This ensures that all dynamic values, including configuration properties, are resolved before
execution begins.
