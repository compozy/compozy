# Agentic Orchestration Example

This example demonstrates comprehensive use of Compozy's built-in orchestration tools for agents, tasks, and workflows. All resources are auto-discovered and can be executed directly through the Tasks API or composed into complex orchestration patterns.

## Highlights

### Agent Orchestration

- **`cp__call_agent`** – Single agent execution (demonstrated in `topic-research`)
- **`cp__call_agents`** – Parallel multi-agent orchestration (demonstrated in `topic-research-multi-agent`)
- Three specialized research agents (web, academic, industry) working independently or in parallel

### Task Orchestration

- **`cp__call_task`** – Single task invocation (demonstrated in `research-orchestrated`)
- **`cp__call_tasks`** – Parallel task execution (demonstrated in `research-pipeline`)
- Reusable tasks: text summarization, keyword extraction, sentiment analysis, translation

### Workflow Orchestration

- **`cp__call_workflow`** – Single workflow invocation (demonstrated in `analyze-with-workflow`)
- **`cp__call_workflows`** – Parallel workflow execution (demonstrated in `batch-analysis`)
- Composable workflows: content analysis, multi-agent research synthesis

### Key Features

- Agents and tasks auto-discovered from `agents/*.yaml`, `tasks/*.yaml`
- Workflows explicitly declared in `compozy.yaml` with source paths
- Built-in tools used within task prompts (not as tool definitions)
- REST client snippets in [api.http](./api.http) for both sync and async execution patterns

## Prerequisites

- A running Compozy backend (`make dev` from the repository root starts the services).
- A Groq API key exported as `GROQ_API_KEY` (the manifest uses `openai/gpt-oss-120b` via Groq by default).  
  Adjust `compozy.yaml` if you prefer a different provider/model.

```bash
export GROQ_API_KEY=grq_...
```

## Run the tasks

1. Start the example runtime:

   ```bash
   cd examples/agentic
   ../../compozy dev
   ```

2. Execute the sample requests from [api.http](./api.http) or use curl examples below.

3. For async execution, poll `GET /executions/tasks/{exec_id}` until status is `completed`.

## Usage Examples

### Agent Orchestration

**Single Agent (`cp__call_agent`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/topic-research/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{ "with": { "topic": "Serverless architecture" } }'
```

**Multiple Agents (`cp__call_agents`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/topic-research-multi-agent/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{ "with": { "topic": "Microservices patterns" } }'
```

### Task Orchestration

**Single Task (`cp__call_task`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/research-orchestrated/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{ "with": { "topic": "Edge computing", "summary_length": 2 } }'
```

**Parallel Tasks (`cp__call_tasks`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/research-pipeline/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{ "with": { "topic": "GraphQL APIs", "target_language": "French" } }'
```

### Workflow Orchestration

**Single Workflow (`cp__call_workflow`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/analyze-with-workflow/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{
    "with": {
      "text": "Cloud computing enables scalable infrastructure...",
      "max_summary_length": 2,
      "keyword_count": 5
    }
  }'
```

**Parallel Workflows (`cp__call_workflows`):**

```bash
curl -X POST http://localhost:5001/api/v0/tasks/batch-analysis/executions/sync \
  -H 'Content-Type: application/json' \
  -d '{
    "with": {
      "texts": [
        "AI is transforming industries...",
        "Blockchain offers decentralized trust...",
        "Quantum computing promises breakthroughs..."
      ],
      "max_summary_length": 2
    }
  }'
```

## Project Structure

```
agentic/
├── agents/                          # Agent definitions
│   ├── research-web.yaml           # Web research specialist
│   ├── research-academic.yaml      # Academic research specialist
│   └── research-industry.yaml      # Industry trends analyst
├── tasks/                           # Task definitions
│   ├── topic-research.yaml         # Single agent call (cp__call_agent)
│   ├── topic-research-multi-agent.yaml  # Multi-agent orchestration (cp__call_agents)
│   ├── summarize-text.yaml         # Reusable text summarization
│   ├── extract-keywords.yaml       # Reusable keyword extraction
│   ├── sentiment-analysis.yaml     # Reusable sentiment analysis
│   ├── translate-text.yaml         # Reusable text translation
│   ├── research-orchestrated.yaml  # Task chaining (cp__call_task)
│   ├── research-pipeline.yaml      # Parallel tasks (cp__call_tasks)
│   ├── analyze-with-workflow.yaml  # Workflow invocation (cp__call_workflow)
│   └── batch-analysis.yaml         # Parallel workflows (cp__call_workflows)
├── workflows/                       # Workflow definitions
│   ├── content-analysis.yaml       # Simple analysis workflow
│   └── multi-research.yaml         # Multi-agent research workflow
├── compozy.yaml                     # Project configuration
└── api.http                         # REST client test examples
```

## Key Concepts

### Built-in Tools Usage

All built-in orchestration tools (`cp__call_agent`, `cp__call_agents`, `cp__call_task`, `cp__call_tasks`, `cp__call_workflow`, `cp__call_workflows`) are used **within task prompts**, not as tool definitions in the `tools` array.

**Example from `research-pipeline.yaml`:**

```yaml
prompt: |
  Step 1: Call cp__call_agent to research the topic.
  Step 2: Call cp__call_tasks with parallel analysis tasks.

  Invoke cp__call_tasks with:
  {
    "tasks": [
      { "task_id": "extract-keywords", "with": {...} },
      { "task_id": "sentiment-analysis", "with": {...} },
      { "task_id": "translate-text", "with": {...} }
    ]
  }
```

### Template Context

Tasks use standard template variables to access input and state:

- `.workflow.input.X` – Access workflow/task input parameters
- `.tasks.X.output` – Access previous task outputs
- `.env.X` – Access environment variables

### Output Mapping

Tasks can extract and transform outputs using template functions:

```yaml
outputs:
  keywords: '{{ fromJson .output | get "keywords" }}'
  count: "{{ len (fromJson .output) }}"
```

## Customization

Feel free to:

- Add new specialized agents with different research perspectives
- Create additional reusable tasks for text processing
- Compose new orchestration patterns using the built-in tools
- Adjust timeouts and concurrency limits in `compozy.yaml`
- Experiment with different prompt strategies for orchestration
