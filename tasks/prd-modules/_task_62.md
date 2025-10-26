## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>cross_links</scope>
<complexity>low</complexity>
<dependencies>task_61</dependencies>
</task_context>

# Task 62.0: Cross‑Links Map Core/API/CLI → SDK (S)

## Overview

Create comprehensive cross-linking map connecting existing documentation (Core, API, CLI) to new SDK section, ensuring bidirectional navigation and discoverability.

<critical>
- **ALWAYS READ** tasks/prd-modules/_docs.md (cross-section link updates)
- **MUST** maintain bidirectional links (Core ← → SDK, API ← → SDK, CLI ← → SDK)
- **MUST** preserve existing docs structure (thin updates only)
- **OUTPUT:** Link insertion plan for docs team
</critical>

<requirements>
- Map all relevant Core pages → SDK equivalents
- Map all relevant API pages → SDK client usage
- Map all relevant CLI pages → SDK programmatic alternatives
- Create reverse links: SDK → Core/API/CLI
- Define link placement strategy (callouts, inline, sections)
- Provide exact link text and context
- Create search keywords update plan
</requirements>

## Subtasks

- [ ] 62.1 Identify all Core pages mentioning YAML configuration
- [ ] 62.2 Map Core workflow/agent/task pages → SDK builders
- [ ] 62.3 Map API reference pages → SDK client usage
- [ ] 62.4 Map CLI command pages → SDK programmatic equivalents
- [ ] 62.5 Define link insertion points in existing pages
- [ ] 62.6 Create reverse link map (SDK → Core/API/CLI)
- [ ] 62.7 Plan callout/info boxes for major SDK features
- [ ] 62.8 Update search keywords and tags

## Implementation Details

**Based on:** tasks/prd-modules/_docs.md (cross-section link updates)

### Cross-Link Categories

#### 1. Core → SDK Links

```markdown
# Example: docs/content/docs/core/workflows.mdx

**Existing section on YAML configuration:**
[Add callout box]
> 💡 **Programmatic Alternative:** You can also define workflows using the
> [Go SDK](/docs/sdk/builders/workflow) for type safety and programmatic control.
> See [SDK Overview](/docs/sdk/overview) for when to use SDK vs YAML.

**Inline link additions:**
- workflows.mdx → [SDK Workflow Builder](/docs/sdk/builders/workflow)
- agents.mdx → [SDK Agent Builder](/docs/sdk/builders/agent)
- tasks.mdx → [SDK Task Builders](/docs/sdk/builders/tasks)
- knowledge.mdx → [SDK Knowledge Builders](/docs/sdk/builders/knowledge)
- memory.mdx → [SDK Memory Builders](/docs/sdk/builders/memory)
- mcp.mdx → [SDK MCP Builder](/docs/sdk/builders/mcp)
```

#### 2. API → SDK Links

```markdown
# Example: docs/content/docs/api/overview.mdx

**Add SDK client section:**
## Using the API from Go

The Compozy Go SDK provides a type-safe client for all API operations:

- [Deploy projects](/docs/sdk/builders/client#deploy)
- [Execute workflows](/docs/sdk/builders/client#execute)
- [Query status](/docs/sdk/builders/client#status)

See [SDK Client Builder](/docs/sdk/builders/client) for complete documentation.

**Embedded usage note:**
For embedded usage (no HTTP), see [Compozy Lifecycle](/docs/sdk/builders/compozy).
```

#### 3. CLI → SDK Links

```markdown
# Example: docs/content/docs/cli/overview.mdx

**Add programmatic alternative section:**
## Programmatic Alternative

For programmatic workflow execution and project management,
see the [Go SDK](/docs/sdk/overview).

Key differences:
- **CLI:** Interactive commands, YAML configuration
- **SDK:** Type-safe Go code, embedded execution, programmatic control

Learn more in [SDK Getting Started](/docs/sdk/getting-started).
```

#### 4. SDK → Core/API/CLI Links (Reverse)

```markdown
# In SDK pages, link back to conceptual docs:

**sdk/overview.mdx:**
- Link to Core concepts for YAML-based approach
- Link to API reference for HTTP operations
- Link to CLI for interactive usage

**sdk/builders/*.mdx:**
- Link to corresponding Core pages for concepts
- Link to engine package GoDoc for implementation details
```

### Link Placement Strategy

1. **Callout Boxes** (prominent, top of relevant sections)
   - Core workflow page → SDK workflow builder
   - Core knowledge page → SDK knowledge builders
   - API overview → SDK client

2. **Inline Links** (contextual, within paragraphs)
   - "YAML configuration" → "or use the SDK"
   - "REST API" → "SDK client provides type-safe access"

3. **Section Additions** (new subsections)
   - "Programmatic Alternative" sections in CLI pages
   - "SDK Client" section in API overview

4. **Sidebar Updates** (navigation hints)
   - Add SDK to "See Also" sections
   - Cross-reference related builders

### Search Keywords Update

```yaml
# Update docs search configuration
keywords_by_page:
  core/workflows:
    add: ["go sdk", "programmatic workflow", "type-safe"]

  core/agents:
    add: ["go sdk agent", "agent builder", "programmatic agent"]

  api/overview:
    add: ["go sdk client", "programmatic api", "embedded compozy"]

  cli/overview:
    add: ["sdk alternative", "programmatic cli", "go api"]
```

### Link Inventory

```markdown
# Cross-Link Inventory (complete mapping)

## Core → SDK Links (15+ links)
1. core/workflows.mdx → sdk/builders/workflow.mdx
2. core/agents.mdx → sdk/builders/agent.mdx
3. core/tasks.mdx → sdk/builders/tasks.mdx
4. core/knowledge.mdx → sdk/builders/knowledge.mdx
5. core/memory.mdx → sdk/builders/memory.mdx
6. core/mcp.mdx → sdk/builders/mcp.mdx
7. core/runtime.mdx → sdk/builders/runtime.mdx
8. core/tools.mdx → sdk/builders/tool.mdx
9. core/schemas.mdx → sdk/builders/schema.mdx
10. core/schedules.mdx → sdk/builders/schedule.mdx
... (all core concepts → SDK equivalents)

## API → SDK Links (5+ links)
1. api/overview.mdx → sdk/builders/client.mdx
2. api/deploy.mdx → sdk/builders/client.mdx#deploy
3. api/execute.mdx → sdk/builders/client.mdx#execute
4. api/overview.mdx → sdk/builders/compozy.mdx (embedded)

## CLI → SDK Links (3+ links)
1. cli/overview.mdx → sdk/overview.mdx
2. cli/workflow.mdx → sdk/builders/workflow.mdx
3. cli/deploy.mdx → sdk/getting-started.mdx

## SDK → Core/API/CLI Links (reverse)
1. sdk/overview.mdx → core/concepts.mdx
2. sdk/builders/*.mdx → core/[concept].mdx
3. sdk/builders/client.mdx → api/overview.mdx
4. sdk/getting-started.mdx → cli/overview.mdx
```

### Relevant Files

- All files under docs/content/docs/core/
- All files under docs/content/docs/api/
- All files under docs/content/docs/cli/
- tasks/prd-modules/_docs.md (cross-link specifications)
- Task 61.0 deliverable (SDK section plan)

### Dependent Files

- Task 61.0 deliverable (SDK section structure)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/tasks/prd-modules/docs-cross-links-map.md` (new file)
  - Complete link inventory (50+ bidirectional links)
  - Link placement specifications (callout/inline/section)
  - Exact link text for each insertion point
  - Search keywords update plan
  - Implementation order (Core → API → CLI → SDK reverse)
- `/Users/pedronauck/Dev/compozy/compozy/tasks/prd-modules/docs-link-insertions/` (new directory)
  - Per-file link insertion specifications
  - Before/after examples
  - Callout box templates

## Tests

Cross-link validation:
- [ ] All Core concept pages link to SDK equivalents
- [ ] All API pages link to SDK client
- [ ] All CLI pages link to SDK programmatic alternative
- [ ] All SDK pages link back to conceptual docs
- [ ] Links are bidirectional (no orphans)
- [ ] Link text is clear and actionable
- [ ] Callout boxes have consistent formatting

Quality checks:
- [ ] No broken links (all targets exist in plan)
- [ ] Link placement is logical and non-intrusive
- [ ] Search keywords improve discoverability
- [ ] "See Also" sections are comprehensive

## Success Criteria

- Users can discover SDK from any relevant Core/API/CLI page
- SDK pages link back to conceptual documentation
- Link placement enhances rather than disrupts existing content
- Search finds SDK pages when users search for YAML/API/CLI concepts
- Bidirectional navigation is seamless
- Link inventory is complete (covers all 16 builder categories)
- Implementation plan is actionable for docs team
- Cross-links improve docs ecosystem without fragmenting content
