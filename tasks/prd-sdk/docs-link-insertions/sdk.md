# SDK Docs Reverse Link Insertions
These instructions ensure every SDK page points back to Core, API, and CLI counterparts.

## Before/After Example: sdk/overview.mdx
**Before**
```md
## When to Use the SDK
Use the SDK when you need type-safe composition, embedded execution, or programmatic deployment.
```
**After**
```md
## When to Use the SDK
Use the SDK when you need type-safe composition, embedded execution, or programmatic deployment.
See the YAML-first approach in [Core Concepts](/docs/core/getting-started/core-concepts), the [API overview](/docs/api/overview) for REST usage, and the [CLI overview](/docs/cli/overview) for interactive commands.
```

## Page-Level Mapping
| SDK Page | Placement | Link Text | Target(s) | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/sdk/overview.mdx | End of "When to use" section | `See the YAML-first approach in [Core Concepts](/docs/core/getting-started/core-concepts), the [API overview](/docs/api/overview), and the [CLI overview](/docs/cli/overview).` | Multiple | Single sentence |
| docs/content/docs/sdk/getting-started.mdx | Hybrid workflow paragraph | `For YAML-based steps, revisit [Core Quick Start](/docs/core/getting-started/quick-start) and [CLI Installation](/docs/core/getting-started/installation).` | Two links | Inline |
| docs/content/docs/sdk/architecture.mdx | Context-first section | `Compare with [Core runtime configuration](/docs/core/configuration/runtime).` | Single | Inline |
| docs/content/docs/sdk/entities.mdx | Per category summary list | `Link each bullet to the corresponding Core concept (agents, tasks overview, knowledge overview, memory overview, tools overview, signals overview, MCP overview, attachments overview).` | Multiple | Use existing bulleted list |
| docs/content/docs/sdk/builders/project.mdx | Opening paragraph | `See YAML setup in [Project Setup](/docs/core/configuration/project-setup) and REST calls in [Project API](/docs/api/project).` | Two | Inline |
| docs/content/docs/sdk/builders/model.mdx | Provider section | `Cross-reference [LLM Integration](/docs/core/agents/llm-integration) and [Models API](/docs/api/models).` | Two | Inline |
| docs/content/docs/sdk/builders/workflow.mdx | Overview paragraph | `Link back to [Workflow configuration](/docs/core/configuration/workflows) and [Workflow CLI commands](/docs/cli/workflow-commands).` | Two | Inline |
| docs/content/docs/sdk/builders/agent.mdx | Overview paragraph | `Reference [Agents overview](/docs/core/agents/overview) and [Agents API](/docs/api/agents).` | Two | Inline |
| docs/content/docs/sdk/builders/tasks.mdx | Intro paragraph | `Point to [Core task overview](/docs/core/tasks/overview) for YAML context.` | One | Inline |
| docs/content/docs/sdk/builders/tasks.mdx | Each subtype subsection | `Add "See YAML example" link to the matching Core page (basic, parallel, collection, router, wait, aggregate, composite, memory, signal).` | Nine | Inline |
| docs/content/docs/sdk/builders/knowledge.mdx | Overview paragraph | `Link to [Knowledge overview](/docs/core/knowledge/overview) and [Knowledge CLI commands](/docs/cli/knowledge-commands).` | Two | Inline |
| docs/content/docs/sdk/builders/memory.mdx | Overview paragraph | `Link to [Memory overview](/docs/core/memory/overview) and [Memory API](/docs/api/memory).` | Two | Inline |
| docs/content/docs/sdk/builders/mcp.mdx | Overview paragraph | `Reference [MCP overview](/docs/core/mcp/overview) and [MCP CLI commands](/docs/cli/mcp-commands).` | Two | Inline |
| docs/content/docs/sdk/builders/runtime.mdx | Intro paragraph | `Link to [Runtime configuration](/docs/core/configuration/runtime).` | One | Inline |
| docs/content/docs/sdk/builders/tool.mdx | Overview paragraph | `Link to [Tools overview](/docs/core/tools/overview) and [Tools API](/docs/api/tools).` | Two | Inline |
| docs/content/docs/sdk/builders/schema.mdx | Schema summary | `Reference [Schema YAML guidance](/docs/core/tasks/overview#schema) and [Schemas API](/docs/api/schemas).` | Two | Inline |
| docs/content/docs/sdk/builders/schedule.mdx | Intro paragraph | `Link to [Workflow scheduling](/docs/core/configuration/workflows#scheduling) and [Schedules API](/docs/api/schedules).` | Two | Inline |
| docs/content/docs/sdk/builders/monitoring.mdx | Telemetry paragraph | `Link to [Monitoring configuration](/docs/core/configuration/monitoring) and [Streaming telemetry](/docs/core/metrics/streaming-telemetry).` | Two | Inline |
| docs/content/docs/sdk/builders/client.mdx | Overview | `Reference [API overview](/docs/api/overview) for REST parity.` | One | Inline |
| docs/content/docs/sdk/builders/compozy.mdx | Deployment section | `Link to [Docker deployment](/docs/core/deployment/docker) and [CLI dev commands](/docs/cli/dev-commands).` | Two | Inline |
| docs/content/docs/sdk/examples.mdx | Opening paragraph | `Point to [First Workflow tutorial](/docs/core/getting-started/first-workflow) and [CLI workflow commands](/docs/cli/workflow-commands).` | Two | Inline |
| docs/content/docs/sdk/migration.mdx | YAML vs Go table | `Link to [YAML templates overview](/docs/core/yaml-templates/overview) and [Project commands](/docs/cli/project-commands).` | Two | Inline |
| docs/content/docs/sdk/testing.mdx | Test setup paragraph | `Reference [Test standards](/docs/core/getting-started/first-workflow#testing) if present; otherwise point to CLI dev commands for context.` | One | Inline |
| docs/content/docs/sdk/troubleshooting.mdx | Intro paragraph | `Link to [Memory privacy](/docs/core/memory/privacy-security) and [CLI overview](/docs/cli/overview) for hybrid fixes.` | Two | Inline |
