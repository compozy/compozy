# Core Docs Link Insertions
Refer to `callout-templates.md` for reusable snippets. Apply the entries below without altering existing headings or front matter.

## Before/After Example: workflows.mdx
**Before**
```md
Workflows are the core building blocks of Compozy that orchestrate the execution of AI agents, tools, and tasks. They provide a declarative way to define complex AI-powered applications through YAML configuration.
```
**After**
```md
Workflows are the core building blocks of Compozy that orchestrate the execution of AI agents, tools, and tasks. They provide a declarative way to define complex AI-powered applications through YAML configuration.
> 💡 **Programmatic Alternative:** Build the same workflow with the [Workflow Builder](/docs/sdk/builders/workflow) for type safety and programmatic control.
```

## Getting Started
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/index.mdx | After first overview paragraph | `> 💡 **Programmatic Alternative:** Build any project with the [Compozy Go SDK](/docs/sdk/overview) when you need type safety or automation.` | /docs/sdk/overview | Use info callout |
| docs/content/docs/core/getting-started/installation.mdx | End of CLI installation paragraph | `Prefer code-first setup? Follow [SDK Getting Started](/docs/sdk/getting-started).` | /docs/sdk/getting-started | Inline sentence |
| docs/content/docs/core/getting-started/quick-start.mdx | After "Before you begin" paragraph | `> 💡 **Use Go instead of YAML:** The [Workflow Builder](/docs/sdk/builders/workflow) creates the same project programmatically.` | /docs/sdk/builders/workflow | Callout |
| docs/content/docs/core/getting-started/first-workflow.mdx | After first YAML example | `You can generate this workflow with the [SDK Workflow Builder](/docs/sdk/builders/workflow#basic).` | /docs/sdk/builders/workflow#basic | Inline sentence |
| docs/content/docs/core/getting-started/core-concepts.mdx | In final "See also" list | `SDK Overview` | /docs/sdk/overview | Add bullet |

## Configuration
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/configuration/project-setup.mdx | Repository structure section | `Generate projects from Go with the [Project Builder](/docs/sdk/builders/project).` | /docs/sdk/builders/project | Inline |
| docs/content/docs/core/configuration/workflows.mdx | After first YAML explanation | `> 💡 **Programmatic Alternative:** Build the same workflow with the [Workflow Builder](/docs/sdk/builders/workflow) for type safety.` | /docs/sdk/builders/workflow | Callout |
| docs/content/docs/core/configuration/global.mdx | End of "Configuration precedence" paragraph | `Set defaults in Go using the [Config Manager helpers](/docs/sdk/builders/compozy#configuration).` | /docs/sdk/builders/compozy#configuration | Inline |
| docs/content/docs/core/configuration/providers.mdx | Provider registration list | `Register providers through code with the [Runtime Builder](/docs/sdk/builders/runtime#providers).` | /docs/sdk/builders/runtime#providers | Inline |
| docs/content/docs/core/configuration/runtime.mdx | After introduction | `> 💡 **Programmatic Alternative:** Configure runtimes in Go with the [Runtime Builder](/docs/sdk/builders/runtime).` | /docs/sdk/builders/runtime | Callout |
| docs/content/docs/core/configuration/server.mdx | Server bootstrap section | `Embed the server directly with the [Compozy Lifecycle builder](/docs/sdk/builders/compozy#server).` | /docs/sdk/builders/compozy#server | Inline |
| docs/content/docs/core/configuration/webhooks.mdx | First paragraph describing webhook workflows | `Define webhooks in code via the [Client Builder hooks](/docs/sdk/builders/client#webhooks).` | /docs/sdk/builders/client#webhooks | Inline |
| docs/content/docs/core/configuration/cli.mdx | Summary paragraph | `Prefer automation? Use the [SDK Getting Started](/docs/sdk/getting-started#cli-parity) guide for CLI-equivalent commands.` | /docs/sdk/getting-started#cli-parity | Inline |
| docs/content/docs/core/configuration/autoload.mdx | Section explaining autoload repository layout | `Generate autoload bundles in Go with the [Project Builder](/docs/sdk/builders/project#autoload).` | /docs/sdk/builders/project#autoload | Inline |
| docs/content/docs/core/configuration/monitoring.mdx | Intro paragraph | `> 💡 **Programmatic Alternative:** Wire telemetry with the [Monitoring Builder](/docs/sdk/builders/monitoring).` | /docs/sdk/builders/monitoring | Callout |
| docs/content/docs/core/configuration/providers.mdx | Credentials subsection | `Manage provider secrets via [Compozy Lifecycle](/docs/sdk/builders/compozy#providers).` | /docs/sdk/builders/compozy#providers | Inline |

## Agents
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/agents/overview.mdx | After opening paragraph | `> 💡 **Build agents in Go:** Start with the [Agent Builder](/docs/sdk/builders/agent).` | /docs/sdk/builders/agent | Callout |
| docs/content/docs/core/agents/context.mdx | Context attachment paragraph | `Attach context programmatically with the [Agent Builder context helpers](/docs/sdk/builders/agent#context).` | /docs/sdk/builders/agent#context | Inline |
| docs/content/docs/core/agents/instructions-actions.mdx | Actions explanation | `Define instructions in code through the [Agent Actions DSL](/docs/sdk/builders/agent#actions).` | /docs/sdk/builders/agent#actions | Inline |
| docs/content/docs/core/agents/tools.mdx | Tool registration paragraph | `Register tools via Go using the [Tool Builder](/docs/sdk/builders/tool).` | /docs/sdk/builders/tool | Inline |
| docs/content/docs/core/agents/llm-integration.mdx | Model provider paragraph | `Wire models with the [Model Builder](/docs/sdk/builders/model#llm).` | /docs/sdk/builders/model#llm | Inline |
| docs/content/docs/core/agents/structured-outputs.mdx | Schema mapping paragraph | `Map structured outputs with the [Schema Builder](/docs/sdk/builders/schema#structured-outputs).` | /docs/sdk/builders/schema#structured-outputs | Inline |
| docs/content/docs/core/agents/memory.mdx | Memory integration paragraph | `Attach agent memory through the [Memory Builder](/docs/sdk/builders/memory#agent-memory).` | /docs/sdk/builders/memory#agent-memory | Inline |

## Tasks
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/tasks/overview.mdx | After first paragraph | `> 💡 **Programmatic Alternative:** Build every task with the [Task Builders](/docs/sdk/builders/tasks).` | /docs/sdk/builders/tasks | Callout |
| docs/content/docs/core/tasks/basic-tasks.mdx | After first YAML block | `Produce this task with the [Basic Task builder](/docs/sdk/builders/tasks#basic).` | /docs/sdk/builders/tasks#basic | Inline |
| docs/content/docs/core/tasks/parallel-processing.mdx | After sample configuration | `Create parallel tasks in Go via the [Parallel Task builder](/docs/sdk/builders/tasks#parallel).` | /docs/sdk/builders/tasks#parallel | Inline |
| docs/content/docs/core/tasks/collection-tasks.mdx | After introductory paragraph | `Assemble collections with the [Collection Task builder](/docs/sdk/builders/tasks#collection).` | /docs/sdk/builders/tasks#collection | Inline |
| docs/content/docs/core/tasks/router-tasks.mdx | After router example | `Route with code using the [Router Task builder](/docs/sdk/builders/tasks#router).` | /docs/sdk/builders/tasks#router | Inline |
| docs/content/docs/core/tasks/wait-tasks.mdx | After first YAML block | `Delay execution with the [Wait Task builder](/docs/sdk/builders/tasks#wait).` | /docs/sdk/builders/tasks#wait | Inline |
| docs/content/docs/core/tasks/aggregate-tasks.mdx | After aggregate example | `Merge results via the [Aggregate Task builder](/docs/sdk/builders/tasks#aggregate).` | /docs/sdk/builders/tasks#aggregate | Inline |
| docs/content/docs/core/tasks/composite-tasks.mdx | After composite description | `Compose subgraphs using the [Composite Task builder](/docs/sdk/builders/tasks#composite).` | /docs/sdk/builders/tasks#composite | Inline |
| docs/content/docs/core/tasks/memory-tasks.mdx | After memory flow paragraph | `Capture transcripts with the [Memory Task builder](/docs/sdk/builders/tasks#memory).` | /docs/sdk/builders/tasks#memory | Inline |
| docs/content/docs/core/tasks/signal-tasks.mdx | After signal explanation | `Publish signals with the [Signal Task builder](/docs/sdk/builders/tasks#signal).` | /docs/sdk/builders/tasks#signal | Inline |

## Signals
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/signals/overview.mdx | After introduction | `Coordinate events with the [Signal Task builder](/docs/sdk/builders/tasks#signal).` | /docs/sdk/builders/tasks#signal | Inline |
| docs/content/docs/core/signals/event-api.mdx | After API usage paragraph | `Automate signal publishing via the [Client Builder events helpers](/docs/sdk/builders/client#signals).` | /docs/sdk/builders/client#signals | Inline |
| docs/content/docs/core/signals/signal-triggers.mdx | After trigger YAML block | `Define triggers in Go with the [Workflow Builder triggers](/docs/sdk/builders/workflow#triggers).` | /docs/sdk/builders/workflow#triggers | Inline |
| docs/content/docs/core/signals/signal-tasks.mdx | After first paragraph | `> 💡 **Programmatic Alternative:** Emit signals with the [Signal Task builder](/docs/sdk/builders/tasks#signal).` | /docs/sdk/builders/tasks#signal | Callout |

## Knowledge
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/knowledge/overview.mdx | After overview paragraph | `> 💡 **Programmatic Alternative:** Manage knowledge with the [Knowledge Builders](/docs/sdk/builders/knowledge).` | /docs/sdk/builders/knowledge | Callout |
| docs/content/docs/core/knowledge/ingestion.mdx | In ingestion workflow paragraph | `Ingest sources with the [Knowledge Ingestion builder](/docs/sdk/builders/knowledge#ingestion).` | /docs/sdk/builders/knowledge#ingestion | Inline |
| docs/content/docs/core/knowledge/configuration.mdx | Configuration example paragraph | `Set pipelines via the [Knowledge Config builder](/docs/sdk/builders/knowledge#configuration).` | /docs/sdk/builders/knowledge#configuration | Inline |
| docs/content/docs/core/knowledge/retrieval-injection.mdx | Retrieval explanation | `Inject context with the [Knowledge Retrieval helpers](/docs/sdk/builders/knowledge#retrieval).` | /docs/sdk/builders/knowledge#retrieval | Inline |
| docs/content/docs/core/knowledge/observability.mdx | Observability intro | `Monitor embeddings with the [Monitoring Builder knowledge metrics](/docs/sdk/builders/monitoring#knowledge).` | /docs/sdk/builders/monitoring#knowledge | Inline |

## Memory
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/memory/overview.mdx | After overview paragraph | `> 💡 **Programmatic Alternative:** Configure memory using the [Memory Builders](/docs/sdk/builders/memory).` | /docs/sdk/builders/memory | Callout |
| docs/content/docs/core/memory/configuration.mdx | Configuration YAML block | `Mirror this config in Go via the [Memory Config builder](/docs/sdk/builders/memory#configuration).` | /docs/sdk/builders/memory#configuration | Inline |
| docs/content/docs/core/memory/operations.mdx | Operations summary paragraph | `Perform operations through the [Client builder memory APIs](/docs/sdk/builders/client#memory).` | /docs/sdk/builders/client#memory | Inline |
| docs/content/docs/core/memory/integration-patterns.mdx | Integration overview | `Wire integrations using [Compozy Lifecycle memory hooks](/docs/sdk/builders/compozy#memory).` | /docs/sdk/builders/compozy#memory | Inline |
| docs/content/docs/core/memory/privacy-security.mdx | Privacy guidance | `Enforce policies with the [Memory Builder privacy options](/docs/sdk/builders/memory#privacy).` | /docs/sdk/builders/memory#privacy | Inline |

## MCP
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/mcp/overview.mdx | After overview paragraph | `> 💡 **Programmatic Alternative:** Register MCP clients with the [MCP Builder](/docs/sdk/builders/mcp).` | /docs/sdk/builders/mcp | Callout |
| docs/content/docs/core/mcp/integration-patterns.mdx | Integration descriptions | `Apply patterns through the [MCP Builder patterns](/docs/sdk/builders/mcp#patterns).` | /docs/sdk/builders/mcp#patterns | Inline |
| docs/content/docs/core/mcp/admin-api.mdx | Admin API usage | `Administer MCP via the [Client builder MCP admin helpers](/docs/sdk/builders/client#mcp-admin).` | /docs/sdk/builders/client#mcp-admin | Inline |
| docs/content/docs/core/mcp/transport-configuration.mdx | Transport type paragraph | `Configure transports with the [MCP transport helpers](/docs/sdk/builders/mcp#transport).` | /docs/sdk/builders/mcp#transport | Inline |
| docs/content/docs/core/mcp/migration-notes.mdx | Migration summary | `See the Go path in [SDK Migration](/docs/sdk/migration#mcp).` | /docs/sdk/migration#mcp | Inline |
| docs/content/docs/core/mcp/security-authentication.mdx | Security guidance | `Manage authentication via the [MCP Builder security options](/docs/sdk/builders/mcp#security).` | /docs/sdk/builders/mcp#security | Inline |

## Attachments
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/attachments/overview.mdx | After overview paragraph | `Link attachments through the [Knowledge attachments builder](/docs/sdk/builders/knowledge#attachments).` | /docs/sdk/builders/knowledge#attachments | Inline |
| docs/content/docs/core/attachments/types-and-sources.mdx | Source list | `Register sources with the [Knowledge ingestion builder](/docs/sdk/builders/knowledge#sources).` | /docs/sdk/builders/knowledge#sources | Inline |
| docs/content/docs/core/attachments/examples.mdx | First example block | `See code samples in [SDK Examples](/docs/sdk/examples#attachments).` | /docs/sdk/examples#attachments | Inline |
| docs/content/docs/core/attachments/llm-integration.mdx | LLM integration paragraph | `Bind attachments in Go using the [Model Builder attachments guidance](/docs/sdk/builders/model#attachments).` | /docs/sdk/builders/model#attachments | Inline |
| docs/content/docs/core/attachments/security-and-limits.mdx | Limits paragraph | `Enforce limits via the [Monitoring Builder attachments section](/docs/sdk/builders/monitoring#attachments).` | /docs/sdk/builders/monitoring#attachments | Inline |

## Tools
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/tools/overview.mdx | After overview paragraph | `> 💡 **Programmatic Alternative:** Build tools with the [Tool Builder](/docs/sdk/builders/tool).` | /docs/sdk/builders/tool | Callout |
| docs/content/docs/core/tools/call-workflow.mdx | After invoke paragraph | `Execute workflows in Go with the [Client builder execute helper](/docs/sdk/builders/client#execute).` | /docs/sdk/builders/client#execute | Inline |
| docs/content/docs/core/tools/call-workflows.mdx | Batch invocation paragraph | `Batch executions via [Client builder batch helpers](/docs/sdk/builders/client#batch).` | /docs/sdk/builders/client#batch | Inline |
| docs/content/docs/core/tools/call-agent.mdx | Invocation overview | `Invoke agents programmatically with the [Client builder agents API](/docs/sdk/builders/client#agents).` | /docs/sdk/builders/client#agents | Inline |
| docs/content/docs/core/tools/call-agents.mdx | Multi-agent paragraph | `Coordinate multiple agents via the [Agent builder invocation helpers](/docs/sdk/builders/agent#invocation).` | /docs/sdk/builders/agent#invocation | Inline |
| docs/content/docs/core/tools/call-task.mdx | Task execution paragraph | `Run tasks through the [Client builder task API](/docs/sdk/builders/client#tasks).` | /docs/sdk/builders/client#tasks | Inline |
| docs/content/docs/core/tools/call-tasks.mdx | Batch task paragraph | `Control batches via the [Task builder invocation helpers](/docs/sdk/builders/tasks#invoke).` | /docs/sdk/builders/tasks#invoke | Inline |
| docs/content/docs/core/tools/runtime-environment.mdx | Sandbox description | `Configure runtime sandboxes with the [Runtime builder sandbox options](/docs/sdk/builders/runtime#sandbox).` | /docs/sdk/builders/runtime#sandbox | Inline |
| docs/content/docs/core/tools/typescript-development.mdx | Comparison section | `For Go parity, use the [Tool Builder](/docs/sdk/builders/tool#typescript-parity).` | /docs/sdk/builders/tool#typescript-parity | Inline |

## YAML Templates
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/yaml-templates/overview.mdx | After overview paragraph | `Prefer code generation? Use [Workflow templates in Go](/docs/sdk/builders/workflow#templates).` | /docs/sdk/builders/workflow#templates | Inline |
| docs/content/docs/core/yaml-templates/yaml-basics.mdx | Summary paragraph | `See [SDK Migration](/docs/sdk/migration#yaml-vs-go) for code-based equivalents.` | /docs/sdk/migration#yaml-vs-go | Inline |
| docs/content/docs/core/yaml-templates/context-variables.mdx | Variable description paragraph | `Inject the same values with [Workflow builder context injection](/docs/sdk/builders/workflow#context).` | /docs/sdk/builders/workflow#context | Inline |
| docs/content/docs/core/yaml-templates/directives.mdx | Directives list | `Generate directives using the [Project builder directives helpers](/docs/sdk/builders/project#directives).` | /docs/sdk/builders/project#directives | Inline |
| docs/content/docs/core/yaml-templates/sprig-functions.mdx | Function usage paragraph | `Translate Sprig usage with [Workflow builder expression helpers](/docs/sdk/builders/workflow#expressions).` | /docs/sdk/builders/workflow#expressions | Inline |

## Deployment & Metrics
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/core/deployment/docker.mdx | Deployment summary paragraph | `Deploy directly from Go with [Compozy lifecycle deploy](/docs/sdk/builders/compozy#deploy).` | /docs/sdk/builders/compozy#deploy | Inline |
| docs/content/docs/core/deployment/kubernetes.mdx | Helm section | `Manage Kubernetes rollouts via [Compozy lifecycle Kubernetes helpers](/docs/sdk/builders/compozy#kubernetes).` | /docs/sdk/builders/compozy#kubernetes | Inline |
| docs/content/docs/core/metrics/monitor-usage.mdx | Usage metrics paragraph | `Track usage programmatically through the [Monitoring builder usage metrics](/docs/sdk/builders/monitoring#usage).` | /docs/sdk/builders/monitoring#usage | Inline |
| docs/content/docs/core/metrics/streaming-telemetry.mdx | Streaming intro | `Stream telemetry from Go with the [Monitoring builder streaming telemetry](/docs/sdk/builders/monitoring#streaming).` | /docs/sdk/builders/monitoring#streaming | Inline |
