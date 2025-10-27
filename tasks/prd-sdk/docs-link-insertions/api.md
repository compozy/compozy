# API Docs Link Insertions
Use `callout-templates.md` for standard snippets. Apply h3 headings when inserting new "Programmatic Alternative" sections in resource pages.

## Before/After Example: api/overview.mdx
**Before**
```md
## Using the API
Compozy exposes a REST API for deploying projects, monitoring executions, and querying results.
```
**After**
```md
## Using the API
Compozy exposes a REST API for deploying projects, monitoring executions, and querying results.
### Using the API from Go
The Compozy Go SDK provides a type-safe client for all API operations:
- [Deploy projects](/docs/sdk/builders/client#deploy)
- [Execute workflows](/docs/sdk/builders/client#execute)
- [Query status](/docs/sdk/builders/client#status)

See [SDK Client Builder](/docs/sdk/builders/client) for complete documentation. For embedded usage (no HTTP), see [Compozy Lifecycle](/docs/sdk/builders/compozy).
```

## Resource Pages
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/api/overview.mdx | Insert h3 section after existing "Using the API" intro | As in example above | Multiple | Matches task spec |
| docs/content/docs/api/operations.mdx | End of first paragraph | `Automate these calls with the [Client Builder operations helpers](/docs/sdk/builders/client#operations).` | /docs/sdk/builders/client#operations | Inline |
| docs/content/docs/api/auth.mdx | Credentials paragraph | `Configure tokens once using the [Client builder auth setup](/docs/sdk/builders/client#auth).` | /docs/sdk/builders/client#auth | Inline |
| docs/content/docs/api/project.mdx | After deploy example | `Deploy projects programmatically via the [Project builder deploy helpers](/docs/sdk/builders/project#deploy).` | /docs/sdk/builders/project#deploy | Inline |
| docs/content/docs/api/workflows.mdx | After execute call description | `Execute from Go with the [Client builder execute helper](/docs/sdk/builders/client#execute).` | /docs/sdk/builders/client#execute | Inline |
| docs/content/docs/api/tasks.mdx | After task invocation section | `Manage tasks through the [Client builder task APIs](/docs/sdk/builders/client#tasks).` | /docs/sdk/builders/client#tasks | Inline |
| docs/content/docs/api/agents.mdx | After agent endpoint summary | `Create agents in Go with the [Agent builder](/docs/sdk/builders/agent#api) and invoke them via the client.` | /docs/sdk/builders/agent#api | Inline |
| docs/content/docs/api/tools.mdx | After tool registration paragraph | `Register tools using the [Tool builder API sync](/docs/sdk/builders/tool#api-sync).` | /docs/sdk/builders/tool#api-sync | Inline |
| docs/content/docs/api/schemas.mdx | After schema endpoint description | `Manage schemas in Go with the [Schema builder API integration](/docs/sdk/builders/schema#api).` | /docs/sdk/builders/schema#api | Inline |
| docs/content/docs/api/models.mdx | After model selection section | `Provision models via the [Model builder client support](/docs/sdk/builders/model#client).` | /docs/sdk/builders/model#client | Inline |
| docs/content/docs/api/knowledge.mdx | After ingestion endpoint description | `Ingest from Go via the [Knowledge builder API helpers](/docs/sdk/builders/knowledge#api).` | /docs/sdk/builders/knowledge#api | Inline |
| docs/content/docs/api/memory.mdx | After overview paragraph | `Configure memory in Go using the [Memory builder API sync](/docs/sdk/builders/memory#api).` | /docs/sdk/builders/memory#api | Inline |
| docs/content/docs/api/memories.mdx | After listing API verbs | `Perform the same operations with the [Client builder memory APIs](/docs/sdk/builders/client#memory).` | /docs/sdk/builders/client#memory | Inline |
| docs/content/docs/api/mcps.mdx | After registration endpoint | `Register MCP transports through the [MCP builder API linkage](/docs/sdk/builders/mcp#api).` | /docs/sdk/builders/mcp#api | Inline |
| docs/content/docs/api/schedules.mdx | After schedule management section | `Control schedules in Go via the [Schedule builder client control](/docs/sdk/builders/schedule#api).` | /docs/sdk/builders/schedule#api | Inline |
| docs/content/docs/api/users.mdx | After user management description | `Manage users programmatically with the [Client builder user management](/docs/sdk/builders/client#users).` | /docs/sdk/builders/client#users | Inline |
