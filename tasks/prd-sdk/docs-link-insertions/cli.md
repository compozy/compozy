# CLI Docs Link Insertions
Follow `callout-templates.md` for the Programmatic Alternative section. Keep h2 heading level across CLI pages.

## Before/After Example: cli/overview.mdx
**Before**
```md
## Overview
Compozy CLI helps you manage projects, workflows, and executions from the terminal.
```
**After**
```md
## Overview
Compozy CLI helps you manage projects, workflows, and executions from the terminal.
## Programmatic Alternative
For programmatic workflow execution and project management, see the [Go SDK](/docs/sdk/overview).
- **CLI:** Interactive commands, YAML configuration.
- **SDK:** Type-safe Go code, embedded execution, programmatic control.

Learn more in [SDK Getting Started](/docs/sdk/getting-started).
```

## Command Pages
| Page | Placement | Link Text | Target | Notes |
| --- | --- | --- | --- | --- |
| docs/content/docs/cli/overview.mdx | Add Programmatic Alternative section (see example) | As in example | Multiple | Required |
| docs/content/docs/cli/project-commands.mdx | After summary of `compozy project deploy` | `Prefer automation? Use the [Project builder deploy helpers](/docs/sdk/builders/project#cli-parity).` | /docs/sdk/builders/project#cli-parity | Inline |
| docs/content/docs/cli/workflow-commands.mdx | After execute section | `Execute workflows in Go via the [Workflow builder CLI parity helpers](/docs/sdk/builders/workflow#cli-parity).` | /docs/sdk/builders/workflow#cli-parity | Inline |
| docs/content/docs/cli/executions.mdx | After monitoring paragraph | `Track executions programmatically with the [Client builder execution APIs](/docs/sdk/builders/client#execute).` | /docs/sdk/builders/client#execute | Inline |
| docs/content/docs/cli/dev-commands.mdx | After `compozy dev` description | `Run dev environments in code with the [Compozy lifecycle dev helpers](/docs/sdk/builders/compozy#dev).` | /docs/sdk/builders/compozy#dev | Inline |
| docs/content/docs/cli/config-commands.mdx | After configuration sync section | `Sync configuration through the [Compozy lifecycle config helpers](/docs/sdk/builders/compozy#config-sync).` | /docs/sdk/builders/compozy#config-sync | Inline |
| docs/content/docs/cli/auth-commands.mdx | After auth login section | `Initialize tokens via the [Client builder auth setup](/docs/sdk/builders/client#auth).` | /docs/sdk/builders/client#auth | Inline |
| docs/content/docs/cli/knowledge-commands.mdx | After ingest description | `Automate ingestion using the [Knowledge builder CLI parity helpers](/docs/sdk/builders/knowledge#cli-parity).` | /docs/sdk/builders/knowledge#cli-parity | Inline |
| docs/content/docs/cli/mcp-commands.mdx | After register command | `Register MCP transports programmatically with the [MCP builder CLI parity helpers](/docs/sdk/builders/mcp#cli-parity).` | /docs/sdk/builders/mcp#cli-parity | Inline |
