# Compozy SDK Examples

This directory contains runnable Go programs demonstrating core patterns in the Compozy SDK. Each example follows the context-first approach required across the project: loggers and configuration managers are attached to `context.Context` before any builders are invoked, and all resources are constructed by calling `Build(ctx)`.

## 01. Simple Workflow

The file `01_simple_workflow.go` is the "hello world" example. It shows how to:

- Bootstrap a context with logger and configuration manager
- Configure an OpenAI model using the model builder
- Define an agent action with a JSON Schema output
- Register an agent and basic task
- Assemble a workflow and project configuration
- Handle aggregated validation failures via `BuildError`

### Prerequisites

Set your OpenAI API key before running the example:

```bash
export OPENAI_API_KEY="sk-..."
```

### Run the Example

Execute the program from the repository root:

```bash
go run ./sdk/examples/01_simple_workflow.go
```

The program logs each build step, prints a summary of the resulting project configuration, and warns when required environment variables are missing.

### What's Next

Future examples in this directory will cover parallel tasks, knowledge bases, memory, MCP integrations, and more advanced orchestration scenarios. Each example will continue to reinforce context-first patterns and proper error handling with `BuildError`.
