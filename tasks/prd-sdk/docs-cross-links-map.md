# Cross-Link Implementation Map: Core/API/CLI ↔ SDK

## Purpose
This deliverable gives the docs team a step-by-step plan to add bidirectional navigation between existing Core, API, and CLI content and the upcoming SDK section. It preserves current page structure while inserting targeted callouts, inline references, and "See also" notes that surface SDK alternatives without duplicating technical material.

## Implementation Order
1. Core pages (configuration → agents → tasks → knowledge/memory → MCP/signals → attachments/tools → YAML templates → deployment/metrics) so shared callout patterns stay consistent.
2. API reference pages, grouping shared sections (overview first, then resource endpoints) to reuse the SDK client snippet.
3. CLI reference pages, finishing with overview so the diff preview shows consolidated programmatic guidance.
4. SDK section pages, adding reverse links and ensuring anchors exist for every target referenced by Core/API/CLI.
5. Update docs search keyword configuration and sidebar "See also" blocks in a single pass to prevent drift.

## Link Placement Patterns
- **Callout boxes**: Prominent entry points near the top of concept pages introducing YAML-first workflows; use the standard info variant with the 💡 emoji prefix.
- **Inline mentions**: Single-sentence additions at the end of paragraphs that currently mention YAML files, REST calls, or CLI commands.
- **Programmatic alternative subsections**: Dedicated h2 blocks (CLI) or h3 blocks (API resource pages) summarizing why/when to use the SDK.
- **See also lists**: Append short bullet lists to existing navigation/footer sections when present.

## Core → SDK Link Inventory (Placement + Text)
1. docs/content/docs/core/index.mdx — Insert introductory callout: `> 💡 **Programmatic Alternative:** Build any project with the [Compozy Go SDK](/docs/sdk/overview) when you need type safety or automation.` (after first paragraph).
2. docs/content/docs/core/getting-started/installation.mdx — Inline sentence after CLI install paragraph: `Prefer code-first setup? Follow [SDK Getting Started](/docs/sdk/getting-started).`
3. docs/content/docs/core/getting-started/quick-start.mdx — Callout after "Before you begin": `> 💡 **Use Go instead of YAML:** The [Workflow Builder](/docs/sdk/builders/workflow) creates the same project programmatically.`
4. docs/content/docs/core/getting-started/first-workflow.mdx — Inline note after YAML example: `You can generate this workflow with the [SDK Workflow Builder](/docs/sdk/builders/workflow#basic).`
5. docs/content/docs/core/getting-started/core-concepts.mdx — Add "See also" bullet linking to [SDK Overview](/docs/sdk/overview) in final summary list.
6. docs/content/docs/core/configuration/project-setup.mdx — Inline link in repo structure section: `Generate projects from Go with the [Project Builder](/docs/sdk/builders/project).`
7. docs/content/docs/core/configuration/workflows.mdx — Callout after opening YAML description (matches example spec).
8. docs/content/docs/core/configuration/global.mdx — Inline mention in "Configuration precedence": `Set defaults in Go using the [Config Manager helpers](/docs/sdk/builders/compozy#configuration).`
9. docs/content/docs/core/configuration/providers.mdx — Inline link in provider registration list: `Register providers through code with the [Runtime Builder](/docs/sdk/builders/runtime#providers).`
10. docs/content/docs/core/configuration/runtime.mdx — Callout referencing [SDK Runtime Builder](/docs/sdk/builders/runtime).
11. docs/content/docs/core/configuration/server.mdx — Inline sentence in server bootstrap section: `Embed the server directly with the [Compozy Lifecycle builder](/docs/sdk/builders/compozy#server).`
12. docs/content/docs/core/configuration/webhooks.mdx — Inline link: `Define webhooks in code via the [Client Builder hooks](/docs/sdk/builders/client#webhooks).`
13. docs/content/docs/core/configuration/cli.mdx — Programmatic alternative mention pointing to [SDK Getting Started](/docs/sdk/getting-started#cli-parity).
14. docs/content/docs/core/configuration/autoload.mdx — Inline note mapping to [Project Builder](/docs/sdk/builders/project#autoload).
15. docs/content/docs/core/configuration/monitoring.mdx — Callout referencing [Monitoring Builder](/docs/sdk/builders/monitoring).
16. docs/content/docs/core/agents/overview.mdx — Callout: `> 💡 **Build agents in Go:** Start with the [Agent Builder](/docs/sdk/builders/agent).`
17. docs/content/docs/core/agents/context.mdx — Inline sentence linking to [Agent Builder context helpers](/docs/sdk/builders/agent#context).
18. docs/content/docs/core/agents/instructions-actions.mdx — Inline link to [Agent Actions DSL](/docs/sdk/builders/agent#actions).
19. docs/content/docs/core/agents/tools.mdx — Inline mention to [Tool Builder](/docs/sdk/builders/tool).
20. docs/content/docs/core/agents/llm-integration.mdx — Inline note to [Model Builder](/docs/sdk/builders/model#llm).
21. docs/content/docs/core/agents/structured-outputs.mdx — Inline note referencing [Schema Builder](/docs/sdk/builders/schema#structured-outputs).
22. docs/content/docs/core/agents/memory.mdx — Inline note linking to [Memory Builder](/docs/sdk/builders/memory#agent-memory).
23. docs/content/docs/core/tasks/overview.mdx — Callout linking to [Task Builders](/docs/sdk/builders/tasks).
24. docs/content/docs/core/tasks/basic-tasks.mdx — Inline note after first YAML snippet: `Produce the same task with the [Basic Task builder](/docs/sdk/builders/tasks#basic).`
25. docs/content/docs/core/tasks/parallel-processing.mdx — Inline note pointing to [Parallel Task builder](/docs/sdk/builders/tasks#parallel).
26. docs/content/docs/core/tasks/collection-tasks.mdx — Inline note to [Collection Task builder](/docs/sdk/builders/tasks#collection).
27. docs/content/docs/core/tasks/router-tasks.mdx — Inline note to [Router Task builder](/docs/sdk/builders/tasks#router).
28. docs/content/docs/core/tasks/wait-tasks.mdx — Inline note to [Wait Task builder](/docs/sdk/builders/tasks#wait).
29. docs/content/docs/core/tasks/aggregate-tasks.mdx — Inline note to [Aggregate Task builder](/docs/sdk/builders/tasks#aggregate).
30. docs/content/docs/core/tasks/composite-tasks.mdx — Inline note to [Composite Task builder](/docs/sdk/builders/tasks#composite).
31. docs/content/docs/core/tasks/memory-tasks.mdx — Inline note to [Memory Task builder](/docs/sdk/builders/tasks#memory).
32. docs/content/docs/core/tasks/signal-tasks.mdx — Inline note to [Signal Task builder](/docs/sdk/builders/tasks#signal).
33. docs/content/docs/core/signals/overview.mdx — Inline addition referencing [Signal utilities](/docs/sdk/builders/tasks#signal).
34. docs/content/docs/core/signals/event-api.mdx — Inline note pointing to [Client Builder events](/docs/sdk/builders/client#signals).
35. docs/content/docs/core/signals/signal-triggers.mdx — Inline link to [Workflow Builder triggers](/docs/sdk/builders/workflow#triggers).
36. docs/content/docs/core/signals/signal-tasks.mdx — Already covered but add callout reminding of SDK alternative (consistent with tasks entry, ensures emphasis).
37. docs/content/docs/core/knowledge/overview.mdx — Callout to [Knowledge Builders](/docs/sdk/builders/knowledge).
38. docs/content/docs/core/knowledge/ingestion.mdx — Inline note referencing [Knowledge Ingestion builder](/docs/sdk/builders/knowledge#ingestion).
39. docs/content/docs/core/knowledge/configuration.mdx — Inline note to [Knowledge Config builder](/docs/sdk/builders/knowledge#configuration).
40. docs/content/docs/core/knowledge/retrieval-injection.mdx — Inline note to [Knowledge Retrieval helpers](/docs/sdk/builders/knowledge#retrieval).
41. docs/content/docs/core/knowledge/observability.mdx — Inline note to [Monitoring Builder knowledge metrics](/docs/sdk/builders/monitoring#knowledge).
42. docs/content/docs/core/memory/overview.mdx — Callout to [Memory Builders](/docs/sdk/builders/memory).
43. docs/content/docs/core/memory/configuration.mdx — Inline note referencing [Memory Config builder](/docs/sdk/builders/memory#configuration).
44. docs/content/docs/core/memory/operations.mdx — Inline note to [Client builder memory APIs](/docs/sdk/builders/client#memory).
45. docs/content/docs/core/memory/integration-patterns.mdx — Inline note to [Compozy lifecycle memory hooks](/docs/sdk/builders/compozy#memory).
46. docs/content/docs/core/memory/privacy-security.mdx — Inline link to [Memory Builder privacy options](/docs/sdk/builders/memory#privacy).
47. docs/content/docs/core/mcp/overview.mdx — Callout: `> 💡 **SDK alternative:** Register MCP clients with the [MCP Builder](/docs/sdk/builders/mcp).`
48. docs/content/docs/core/mcp/integration-patterns.mdx — Inline note linking to [MCP Builder patterns](/docs/sdk/builders/mcp#patterns).
49. docs/content/docs/core/mcp/admin-api.mdx — Inline note referencing [Client builder MCP admin](/docs/sdk/builders/client#mcp-admin).
50. docs/content/docs/core/mcp/transport-configuration.mdx — Inline note to [MCP transport helpers](/docs/sdk/builders/mcp#transport).
51. docs/content/docs/core/mcp/migration-notes.mdx — Inline link to [Migration guide](/docs/sdk/migration#mcp).
52. docs/content/docs/core/mcp/security-authentication.mdx — Inline note to [MCP Builder auth options](/docs/sdk/builders/mcp#security).
53. docs/content/docs/core/attachments/overview.mdx — Inline link to [Knowledge attachments builder](/docs/sdk/builders/knowledge#attachments).
54. docs/content/docs/core/attachments/types-and-sources.mdx — Inline note to [Knowledge ingestion builder](/docs/sdk/builders/knowledge#sources).
55. docs/content/docs/core/attachments/examples.mdx — Inline note to [SDK examples](/docs/sdk/examples#attachments).
56. docs/content/docs/core/attachments/llm-integration.mdx — Inline note pointing to [Model Builder attachments guidance](/docs/sdk/builders/model#attachments).
57. docs/content/docs/core/attachments/security-and-limits.mdx — Inline note referencing [Monitoring Builder limits section](/docs/sdk/builders/monitoring#attachments).
58. docs/content/docs/core/tools/overview.mdx — Callout linking to [Tool Builder](/docs/sdk/builders/tool).
59. docs/content/docs/core/tools/call-workflow.mdx — Inline note referencing [Client builder execute workflows](/docs/sdk/builders/client#execute).
60. docs/content/docs/core/tools/call-workflows.mdx — Inline note to same target but highlight batch execution anchor (#batch).
61. docs/content/docs/core/tools/call-agent.mdx — Inline note to [Client builder invoke agents](/docs/sdk/builders/client#agents).
62. docs/content/docs/core/tools/call-agents.mdx — Inline note referencing [Agent builder invocation helpers](/docs/sdk/builders/agent#invocation).
63. docs/content/docs/core/tools/call-task.mdx — Inline note to [Client builder run task](/docs/sdk/builders/client#tasks).
64. docs/content/docs/core/tools/call-tasks.mdx — Inline note to [Task builder execution helpers](/docs/sdk/builders/tasks#invoke).
65. docs/content/docs/core/tools/runtime-environment.mdx — Inline link to [Runtime builder sandbox](/docs/sdk/builders/runtime#sandbox).
66. docs/content/docs/core/tools/typescript-development.mdx — Inline note comparing to [Go SDK tool builders](/docs/sdk/builders/tool#typescript-parity).
67. docs/content/docs/core/yaml-templates/overview.mdx — Inline sentence: `Prefer code generation? Use [Workflow templates in Go](/docs/sdk/builders/workflow#templates).`
68. docs/content/docs/core/yaml-templates/yaml-basics.mdx — Inline note linking to [SDK Migration guide](/docs/sdk/migration#yaml-vs-go).
69. docs/content/docs/core/yaml-templates/context-variables.mdx — Inline note referencing [Workflow builder context injection](/docs/sdk/builders/workflow#context).
70. docs/content/docs/core/yaml-templates/directives.mdx — Inline note linking to [Project builder directives](/docs/sdk/builders/project#directives).
71. docs/content/docs/core/yaml-templates/sprig-functions.mdx — Inline note to [SDK expression helpers](/docs/sdk/builders/workflow#expressions).
72. docs/content/docs/core/deployment/docker.mdx — Inline mention: `Deploy workflows directly from Go with [Compozy lifecycle deploy](/docs/sdk/builders/compozy#deploy).`
73. docs/content/docs/core/deployment/kubernetes.mdx — Inline note to [Compozy lifecycle helm helpers](/docs/sdk/builders/compozy#kubernetes).
74. docs/content/docs/core/metrics/monitor-usage.mdx — Inline note referencing [Monitoring builder usage metrics](/docs/sdk/builders/monitoring#usage).
75. docs/content/docs/core/metrics/streaming-telemetry.mdx — Inline note to [Monitoring builder streaming telemetry](/docs/sdk/builders/monitoring#streaming).

## API → SDK Link Inventory
1. docs/content/docs/api/overview.mdx — Add "Using the API from Go" subsection (per spec) with links to SDK client, deploy, execute, status, lifecycle.
2. docs/content/docs/api/operations.mdx — Inline sentence at top: `Automate these calls with the [Client Builder operations helpers](/docs/sdk/builders/client#operations).`
3. docs/content/docs/api/auth.mdx — Inline note referencing [Client builder auth setup](/docs/sdk/builders/client#auth).
4. docs/content/docs/api/project.mdx — Inline note linking to [Project builder deploy](/docs/sdk/builders/project#deploy).
5. docs/content/docs/api/workflows.mdx — Inline note to [Client builder execute workflows](/docs/sdk/builders/client#execute).
6. docs/content/docs/api/tasks.mdx — Inline note referencing [Client builder task APIs](/docs/sdk/builders/client#tasks).
7. docs/content/docs/api/agents.mdx — Inline note to [Agent builder + client invoke](/docs/sdk/builders/agent#api).
8. docs/content/docs/api/tools.mdx — Inline note pointing to [Tool builder registration](/docs/sdk/builders/tool#api-sync).
9. docs/content/docs/api/schemas.mdx — Inline note referencing [Schema builder API sync](/docs/sdk/builders/schema#api).
10. docs/content/docs/api/models.mdx — Inline note linking to [Model builder client usage](/docs/sdk/builders/model#client).
11. docs/content/docs/api/knowledge.mdx — Inline note to [Knowledge builder API usage](/docs/sdk/builders/knowledge#api).
12. docs/content/docs/api/memory.mdx — Inline note referencing [Memory builder API sync](/docs/sdk/builders/memory#api).
13. docs/content/docs/api/memories.mdx — Inline note to [Client builder memory APIs](/docs/sdk/builders/client#memory).
14. docs/content/docs/api/mcps.mdx — Inline note referencing [MCP builder API linkage](/docs/sdk/builders/mcp#api).
15. docs/content/docs/api/schedules.mdx — Inline note to [Schedule builder client control](/docs/sdk/builders/schedule#api).
16. docs/content/docs/api/users.mdx — Inline note referencing [Client builder user management](/docs/sdk/builders/client#users).

## CLI → SDK Link Inventory
1. docs/content/docs/cli/overview.mdx — Add "Programmatic Alternative" section linking to SDK overview and getting started (per spec).
2. docs/content/docs/cli/project-commands.mdx — Inline note: `Prefer automation? Use the [Project builder deploy helpers](/docs/sdk/builders/project#cli-parity).`
3. docs/content/docs/cli/workflow-commands.mdx — Inline mention to [Workflow builder execute helpers](/docs/sdk/builders/workflow#cli-parity).
4. docs/content/docs/cli/executions.mdx — Inline note referencing [Client builder execution APIs](/docs/sdk/builders/client#execute).
5. docs/content/docs/cli/dev-commands.mdx — Inline note to [Compozy lifecycle dev helpers](/docs/sdk/builders/compozy#dev).
6. docs/content/docs/cli/config-commands.mdx — Inline mention linking to [Config helpers in SDK](/docs/sdk/builders/compozy#config-sync).
7. docs/content/docs/cli/auth-commands.mdx — Inline note referencing [Client builder auth setup](/docs/sdk/builders/client#auth).
8. docs/content/docs/cli/knowledge-commands.mdx — Inline note to [Knowledge builder ingestion helpers](/docs/sdk/builders/knowledge#cli-parity).
9. docs/content/docs/cli/mcp-commands.mdx — Inline note to [MCP builder registration](/docs/sdk/builders/mcp#cli-parity).

## Reverse Map: SDK → Core/API/CLI
- sdk/overview.mdx — Add "See also" list linking to Core core-concepts, API overview, CLI overview.
- sdk/getting-started.mdx — Inline links back to Core quick-start (YAML path) and CLI installation for hybrid workflows.
- sdk/architecture.mdx — Reference Core configuration/runtime pages for YAML-based deployments.
- sdk/entities.mdx — For each category, link to the matching Core concept page (agents, tasks overview, knowledge overview, memory overview, tools overview, signals overview, MCP overview, attachments overview).
- sdk/builders/project.mdx — Link to Core project-setup, CLI project commands, API project endpoint.
- sdk/builders/model.mdx — Link to Core agents/llm-integration, API models.
- sdk/builders/workflow.mdx — Link to Core configuration/workflows, CLI workflow commands.
- sdk/builders/agent.mdx — Link to Core agents overview, API agents, CLI agents section (workflow commands call-out).
- sdk/builders/tasks.mdx — Link to Core tasks overview + each specific subtype page.
- sdk/builders/knowledge.mdx — Link to Core knowledge overview and ingestion, CLI knowledge commands, API knowledge endpoint.
- sdk/builders/memory.mdx — Link to Core memory overview, API memory, CLI auth (for secrets) where relevant.
- sdk/builders/mcp.mdx — Link to Core MCP overview, API mcps, CLI mcp commands.
- sdk/builders/runtime.mdx — Link to Core configuration/runtime.
- sdk/builders/tool.mdx — Link to Core tools overview, API tools, CLI project/workflow commands (tool packaging).
- sdk/builders/schema.mdx — Link to Core schema references, API schemas.
- sdk/builders/schedule.mdx — Link to Core configuration/workflows scheduling section, API schedules.
- sdk/builders/monitoring.mdx — Link to Core monitoring pages, metrics.
- sdk/builders/client.mdx — Link to API overview, CLI workflow commands for parity.
- sdk/builders/compozy.mdx — Link to Core deployment Docker/Kubernetes, CLI dev commands.
- sdk/examples.mdx — Link back to Core examples (First workflow) and CLI quick-start.
- sdk/migration.mdx — Link to Core YAML templates and configuration docs, CLI project commands.
- sdk/testing.mdx — Link to Core testing guidance if exists (if not, reference CLI test harness info) and API operations for contract testing.
- sdk/troubleshooting.mdx — Link to Core troubleshooting anchors (memory privacy, signals) and CLI troubleshooting sections (if any) or CLI overview.

## Callout & Inline Text Guidelines
- Callout title always "Programmatic Alternative" or "Use the Go SDK" to maintain consistency.
- Inline sentences end with period and sit in the same paragraph to respect the no extra line break rule.
- Use anchors (`#basic`, `#parallel`, etc.) in SDK pages; include them in the builder markdown front matter to ensure autogen TOC alignment.

## Search Keyword Update Plan
Update docs search configuration (meta or keywords file maintained by docs team) with the following additions:
- core/index: add `"go sdk", "code-first", "programmatic workflows"`.
- core/configuration/workflows: add `"sdk workflow builder", "go workflows"`.
- core/agents/overview: add `"sdk agent", "agent builder", "code-first agent"`.
- core/tasks/overview: add `"sdk tasks", "go task builder"`.
- core/knowledge/overview: add `"sdk knowledge", "ingestion builder"`.
- core/memory/overview: add `"sdk memory", "context-aware cache"`.
- core/mcp/overview: add `"sdk mcp", "mcp builder"`.
- core/tools/overview: add `"sdk tools", "programmatic tool"`.
- core/yaml-templates/overview: add `"yaml vs sdk", "sdk migration"`.
- api/overview: add `"go sdk client", "sdk deploy"`.
- api/agents, api/tasks, api/workflows, api/knowledge, api/memory, api/mcps: add `"sdk <resource>", "go client <resource>"`.
- api/schedules: add `"sdk schedules", "go schedule builder"`.
- api/users: add `"sdk user management", "programmatic auth"`.
- cli/overview: add `"sdk alternative", "programmatic cli", "code-first"`.
- cli/* command pages: add `"sdk parity", "go automation"` plus resource-specific terms (`"project builder"`, `"workflow builder"`, `"knowledge ingestion"`).
- sdk/overview: add `"yaml vs sdk", "code-first compozy"`.
- sdk/builders/client: add `"api parity", "go client"`.
- sdk/builders/tasks: add `"basic task builder", "parallel task builder"`.
- sdk/builders/knowledge: add `"rag builder", "knowledge ingestion sdk"`.
- sdk/migration: add `"yaml migration", "hybrid projects"`.
- sdk/troubleshooting: add `"missing context", "sdk errors"`.

Coordinate updates in the docs search configuration file (docs/content/docs/meta.json keywords block or equivalent data source) so analytics can measure SDK discoverability.

## Quality & Validation Checklist
- Verify every Core/API/CLI page cited above receives its planned link and that no duplicate anchors are introduced.
- Confirm each SDK builder page exposes the anchors referenced by Core/API/CLI (e.g., `#basic`, `#parallel`, `#attachments`); update builder outlines if anchors are missing.
- Ensure every new callout uses the standard info style and respects house formatting (emoji + bold title + concise one-line body).
- After adding links, run the docs site locally to spot check navigation loops: Core → SDK → Core, API → SDK → API, CLI → SDK → CLI.
- Validate search results by rebuilding the search index (docs team command) and confirming queries for \"sdk\" plus each resource surface both YAML and Go SDK pages.

## Dependencies & Follow-up
- Coordinate with Task 61 outcomes to reuse builder anchor naming.
- Update Task tracking (_task_62.md, _tasks.md) once the docs team applies these instructions.
- Prepare a short changelog entry for the docs release summarizing \"Added Go SDK cross-links across Core/API/CLI\" so downstream docs consumers know to expect SDK references.
