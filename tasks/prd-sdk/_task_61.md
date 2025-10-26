## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>docs_planning</scope>
<complexity>low</complexity>
<dependencies>task_53,task_54,task_55</dependencies>
</task_context>

# Task 61.0: SDK Docs Section Plan Finalization (S)

## Overview

Finalize the implementation plan for the new "SDK" top-level documentation section, including navigation structure, page outlines, and integration with existing docs.

<critical>
- **ALWAYS READ** tasks/prd-sdk/_docs.md before starting
- **ALWAYS READ** tasks/prd-sdk/01-executive-summary.md (SDK value props)
- **MUST** create NEW top-level section (not modify existing docs)
- **MUST** maintain DRY principle (link to PRD modules, no duplication)
- **OUTPUT:** Implementation plan for docs team (not actual docs)
</critical>

<requirements>
- Create complete navigation structure for SDK section
- Define all page outlines with content sources
- Plan cross-links to existing docs (Core, API, CLI)
- Specify meta.json updates for docs site
- Document content organization and DRY strategy
- Create implementation checklist for docs team
</requirements>

## Subtasks

- [ ] 61.1 Define SDK section navigation structure (meta.json)
- [ ] 61.2 Create page outlines for all SDK documentation pages
- [ ] 61.3 Map content sources (PRD modules → docs pages)
- [ ] 61.4 Plan cross-links from Core/API/CLI to SDK
- [ ] 61.5 Define builders/ subdirectory structure
- [ ] 61.6 Create docs site integration plan
- [ ] 61.7 Document content guidelines (DRY, linking strategy)
- [ ] 61.8 Create implementation checklist for docs team

## Implementation Details

**Based on:** tasks/prd-sdk/_docs.md, tasks/prd-sdk/01-executive-summary.md

### SDK Section Structure

```
docs/content/docs/sdk/
├── meta.json                           # Navigation configuration
├── overview.mdx                        # What is the SDK, when to use it
├── getting-started.mdx                 # Quick start + context setup
├── architecture.mdx                    # SDK → Engine integration
├── entities.mdx                        # 16 categories / 30 builders inventory
├── builders/                           # Detailed builder docs
│   ├── meta.json
│   ├── project.mdx
│   ├── model.mdx
│   ├── workflow.mdx
│   ├── agent.mdx
│   ├── tasks.mdx                      # All 9 types
│   ├── knowledge.mdx                  # All 5 builders
│   ├── memory.mdx                     # Config + reference
│   ├── mcp.mdx
│   ├── runtime.mdx
│   ├── tool.mdx
│   ├── schema.mdx
│   ├── schedule.mdx
│   ├── monitoring.mdx
│   ├── client.mdx
│   └── compozy.mdx                    # Embedded lifecycle
├── examples.mdx                        # Examples index + runnable commands
├── migration.mdx                       # YAML → SDK migration guide
├── testing.mdx                         # Testing guidance
└── troubleshooting.mdx                 # Common errors + fixes
```

### Navigation Configuration

```json
// docs/content/docs/sdk/meta.json
{
  "title": "SDK",
  "description": "Compozy GO SDK documentation",
  "icon": "Code2",
  "root": true,
  "pages": [
    "overview",
    "getting-started",
    "architecture",
    "entities",
    "builders",
    "examples",
    "migration",
    "testing",
    "troubleshooting"
  ]
}

// docs/content/docs/sdk/builders/meta.json
{
  "title": "Builders",
  "pages": [
    "project",
    "model",
    "workflow",
    "agent",
    "tasks",
    "knowledge",
    "memory",
    "mcp",
    "runtime",
    "tool",
    "schema",
    "schedule",
    "monitoring",
    "client",
    "compozy"
  ]
}
```

### Page Outlines

Each page outline specifies:
- Purpose
- Content source (PRD module references)
- Sections
- Cross-links
- Code examples strategy

### Content Guidelines

1. **DRY Principle:**
   - Link to PRD modules for technical details
   - Don't duplicate API signatures (link to GoDoc)
   - Don't duplicate examples (link to sdk/examples/)

2. **Linking Strategy:**
   - Technical details → tasks/prd-sdk/*.md
   - API reference → GoDoc (pkg.go.dev)
   - Examples → sdk/examples/*.go
   - Cross-sections → relative links

3. **Code Examples:**
   - Minimal inline examples
   - Full examples in sdk/examples/
   - Always show context setup
   - Include import statements

### Relevant Files

- tasks/prd-sdk/_docs.md (source plan)
- tasks/prd-sdk/01-executive-summary.md (value props)
- tasks/prd-sdk/06-migration-guide.md (migration content)
- Task 53/54/55 deliverables (migration + troubleshooting guides)

### Dependent Files

- Task 53.0 deliverable (migration basics)
- Task 54.0 deliverable (migration advanced)
- Task 55.0 deliverable (troubleshooting)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/tasks/prd-sdk/docs-implementation-plan.md` (new file)
  - Complete navigation structure (meta.json specs)
  - All page outlines with content sources
  - Cross-link mapping (Core/API/CLI → SDK)
  - Content guidelines and DRY strategy
  - Implementation checklist for docs team
  - File creation order and dependencies
- `/Users/pedronauck/Dev/compozy/compozy/tasks/prd-sdk/docs-page-outlines/` (new directory)
  - Individual outline files for each page (15+ pages)
  - Section-by-section content specifications
  - Source references to PRD modules

## Tests

Documentation plan validation:
- [ ] All 16 builder categories have corresponding pages
- [ ] Navigation structure is complete and logical
- [ ] Every page outline specifies content sources
- [ ] Cross-links are bidirectional (SDK ← → Core/API/CLI)
- [ ] DRY strategy prevents content duplication
- [ ] Examples strategy links to sdk/examples/
- [ ] Implementation checklist is actionable

Completeness checks:
- [ ] meta.json configurations are valid JSON
- [ ] All PRD module references are correct
- [ ] Page dependency order is clear
- [ ] Content guidelines cover all edge cases

## Success Criteria

- Docs team can implement SDK section from this plan alone
- Navigation structure is clear and user-friendly
- Every page has complete outline with source references
- DRY strategy prevents duplicate maintenance
- Cross-links improve docs discoverability
- Implementation checklist provides clear execution path
- Plan accounts for all 30 builders across 16 categories
- Content organization matches user mental models
- Examples are discoverable and runnable
