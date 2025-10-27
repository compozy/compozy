# Documentation Plan: Temporal Standalone Mode

## Goals

- Define all documentation updates required to ship Temporal standalone mode using `temporal.NewServer()`
- Provide precise file paths, page outlines, and cross-link updates
- Ensure users understand when to use standalone vs remote mode
- Clarify that standalone uses production-grade Temporal server code

## New/Updated Pages

### 1. docs/content/docs/deployment/temporal-modes.mdx (NEW)
- **Purpose:** Comprehensive guide to Temporal connection modes
- **Outline:**
  - Overview of Remote vs Standalone modes
  - When to use each mode
  - Remote Mode (Production)
    - Requirements (external Temporal cluster)
    - Configuration example
    - High availability setup
  - Standalone Mode (Development/Testing)
    - What is it (embedded `temporal.NewServer()`)
    - Why NOT Temporalite (deprecated)
    - In-memory vs file-based persistence
    - Configuration examples
    - Web UI access (http://localhost:8233)
    - Limitations and warnings
    - Performance characteristics
  - Architecture comparison (diagram: 4 services in standalone)
  - Migration between modes
  - Troubleshooting common issues (port conflicts, startup timeouts)
- **Links:**
  - Link to Configuration Reference
  - Link to Quick Start guide
  - Link to Temporal official docs
  - Link to GitHub reference implementation

### 2. docs/content/docs/configuration/temporal.mdx (UPDATE)
- **Purpose:** Update Temporal configuration reference with new mode fields
- **Updates:**
  - Add `mode` field documentation
  - Add `standalone` configuration section
  - Add examples for both modes
  - Add warning callouts about production use
  - Document all 4 service ports (7233-7236)
  - Document UI server (port 8233)
- **New Sections:**
  - "Mode Selection" (remote vs standalone)
  - "Standalone Configuration" (database_file, frontend_port, bind_ip, enable_ui, ui_port, log_level)
  - "Environment Variables" (TEMPORAL_MODE, TEMPORAL_STANDALONE_*)
  - "Port Configuration" (frontend, history, matching, worker, UI)

### 3. docs/content/docs/quick-start/index.mdx (UPDATE)
- **Purpose:** Update quick start to use standalone mode by default for better onboarding
- **Updates:**
  - Change from "Run Docker Compose" to "Run with standalone mode"
  - Add note about Docker still being an option
  - Show standalone mode in example compozy.yaml
  - Update "What's Next" section to mention production deployment
  - Add note about Web UI at http://localhost:8233
- **New Content:**
  - Step: "Start Compozy with standalone Temporal"
  - Callout: "For production, see Deployment → Temporal Modes"
  - Tip: "Access Temporal Web UI at http://localhost:8233"

### 4. docs/content/docs/deployment/production.mdx (UPDATE)
- **Purpose:** Emphasize remote mode requirement for production
- **Updates:**
  - Add "Temporal Configuration" section
  - Explicitly state standalone mode is NOT for production
  - Explain why (single node, SQLite limitations, no HA)
  - Link to temporal-modes.mdx for details
  - Show production-ready remote mode configuration
  - Recommend external Temporal cluster setup

### 5. docs/content/docs/cli/compozy-start.mdx (UPDATE)
- **Purpose:** Document `--temporal-mode` CLI flag
- **Updates:**
  - Add flag description
  - Show usage examples
  - Note precedence (CLI > env > config file)
  - Document Web UI access when standalone mode enabled

### 6. docs/content/docs/architecture/embedded-temporal.mdx (NEW)
- **Purpose:** Technical deep-dive on embedded Temporal implementation
- **Outline:**
  - Architecture overview
  - Four-service design (frontend, history, matching, worker)
  - SQLite persistence layer
  - Namespace management
  - Web UI server integration
  - Port allocation strategy
  - Startup and shutdown lifecycle
  - Comparison with external Temporal cluster
  - Performance characteristics
  - Security considerations (localhost-only by default)
- **Audience:** Advanced users, contributors
- **Links:**
  - Link to Temporal server documentation
  - Link to reference GitHub implementation

## Schema Docs

### 1. docs/content/docs/reference/schemas/config.mdx (UPDATE)
- **Renders:** `schemas/config.json`
- **Notes:**
  - Highlight new `temporal.mode` enum
  - Highlight new `temporal.standalone` object
  - Add validation notes (mode must be "remote" or "standalone")
  - Document all standalone fields with examples

### 2. schemas/config.json (UPDATE via schemagen)
- **Updates Required:**
  - Add `mode` property to `temporal` object (enum: ["remote", "standalone"], default: "remote")
  - Add `standalone` property to `temporal` object (object type)
  - Define `standalone` schema with properties:
    - `database_file` (string, default ":memory:")
    - `frontend_port` (integer, min 0, max 65535, default 7233)
    - `bind_ip` (string, default "127.0.0.1")
    - `enable_ui` (boolean, default true)
    - `ui_port` (integer, min 0, max 65535, default 8233)
    - `log_level` (enum: ["debug", "info", "warn", "error"], default "warn")

## API Docs

No API changes required. Temporal mode is a server configuration concern only.

## CLI Docs

### 1. docs/content/docs/cli/global-flags.mdx (UPDATE)
- **Add Flag:** `--temporal-mode`
  - Description: "Temporal connection mode (remote or standalone)"
  - Type: string
  - Default: "remote"
  - Env var: TEMPORAL_MODE
  - Example: `compozy start --temporal-mode=standalone`
  - Note: "Standalone mode starts embedded Temporal server"

### 2. docs/content/docs/cli/compozy-config.mdx (UPDATE)
- **Purpose:** Show standalone mode in `compozy config show` output
- **Updates:**
  - Add example output showing temporal.mode field
  - Add example output showing temporal.standalone fields
  - Show warning message when standalone mode active

## Cross-page Updates

### 1. docs/content/docs/architecture/overview.mdx (UPDATE)
- **Update:** Infrastructure diagram showing optional embedded Temporal
- **Note:** Add annotation "Temporal (remote or embedded via temporal.NewServer())"
- **Add:** Brief mention of standalone mode in architecture section

### 2. docs/content/index.mdx (Homepage) (UPDATE)
- **Update:** Add bullet point under features: "Zero-dependency local development with embedded Temporal server"
- **Update:** Quick start code snippet to show standalone mode
- **Add:** "No Docker required for local development" callout

### 3. docs/content/docs/installation/docker.mdx (UPDATE)
- **Note:** Mention that Docker Compose for Temporal is optional with standalone mode
- **Add:** Link to temporal-modes.mdx for comparison
- **Keep:** Docker instructions for those who prefer external Temporal

### 4. docs/content/docs/troubleshooting/temporal.mdx (NEW or UPDATE)
- **Add Section:** "Standalone Mode Issues"
- **Common Issues:**
  - Port conflicts (7233-7236, 8233)
  - Startup timeout
  - SQLite file permissions
  - Database corruption recovery
- **Solutions:**
  - How to configure alternative ports
  - How to increase startup timeout
  - How to fix permissions
  - How to recover from corruption

## Navigation & Indexing

### Update docs/source.config.ts

**Add new page to Deployment section:**
```typescript
{
  title: "Temporal Modes",
  url: "/docs/deployment/temporal-modes",
  description: "Choose between remote and standalone Temporal modes"
}
```

**Add new page to Architecture section:**
```typescript
{
  title: "Embedded Temporal",
  url: "/docs/architecture/embedded-temporal",
  description: "Technical deep-dive on embedded Temporal server implementation"
}
```

**Add new troubleshooting page:**
```typescript
{
  title: "Temporal Troubleshooting",
  url: "/docs/troubleshooting/temporal",
  description: "Common Temporal issues and solutions"
}
```

**Ensure order:**
- Deployment section: Production → **Temporal Modes** (NEW) → Docker → Kubernetes
- Architecture section: Overview → Components → **Embedded Temporal** (NEW) → Workflows → Tasks
- Troubleshooting section: General → **Temporal** (NEW) → Database → Performance

## Acceptance Criteria

- [ ] All 7 new/updated content pages exist with correct outlines
- [ ] Schema docs render the updated config.json with temporal mode fields
- [ ] CLI docs show `--temporal-mode` flag with examples
- [ ] Quick start uses standalone mode by default
- [ ] Production docs explicitly warn against standalone mode
- [ ] Embedded Temporal architecture doc explains four-service design
- [ ] Troubleshooting guide covers common standalone mode issues
- [ ] Cross-page links verified (no 404s)
- [ ] Navigation sidebar shows all new pages in correct sections
- [ ] Docs dev server builds without errors
- [ ] Search index includes "standalone", "embedded temporal", "temporal.NewServer"
- [ ] Code examples in docs are syntactically correct
- [ ] All diagrams clearly show standalone vs remote architecture

## Implementation Notes

**Priority Order:**
1. Schema updates (schemas/config.json) - Foundation for all docs
2. Configuration reference (temporal.mdx) - Core documentation
3. Temporal modes guide (temporal-modes.mdx) - Deep dive
4. Embedded Temporal architecture (embedded-temporal.mdx) - Technical details
5. Quick start update - Onboarding experience
6. Production docs update - Safety warnings
7. Troubleshooting guide - Support resource
8. CLI docs - Reference material
9. Cross-page updates - Consistency
10. Navigation config - Discoverability

**Content Guidelines:**
- Use warning callouts for "Standalone mode is NOT for production"
- Use info callouts to explain "Uses production-grade temporal.NewServer()"
- Use code blocks with YAML syntax highlighting
- Include "See also" sections linking related docs
- Use tables for mode comparison (Remote vs Standalone)
- Use diagrams to show four-service architecture
- Keep explanations concise (prefer bullet points over paragraphs)
- Mention Web UI prominently (differentiator from test-only solutions)

**Visual Assets Needed:**
- Diagram: Temporal architecture (remote: external cluster; standalone: 4 services embedded)
- Diagram: Port allocation (7233-7236 for services, 8233 for UI)
- Screenshot: `compozy config show` output with standalone mode
- Screenshot: Temporal Web UI at localhost:8233
- Optional: Animated GIF showing instant startup with standalone mode

**Testing:**
- Run `cd docs && npm run dev` to verify local build
- Test all internal links
- Verify schema rendering with updated config.json
- Check mobile responsiveness of tables and diagrams
- Verify code block syntax highlighting

**Key Messages to Emphasize:**
1. Standalone mode uses production-grade `temporal.NewServer()`, NOT deprecated Temporalite
2. Web UI included by default for better debugging experience
3. Four-service architecture mirrors production deployment
4. Zero Docker dependency for local development
5. NOT for production (single node, SQLite limitations)

## Related Planning Artifacts
- tasks/prd-temporal/_techspec.md
- tasks/prd-temporal/_examples.md  
- tasks/prd-temporal/_tests.md

## References
- GitHub Reference: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go
- Temporal Server Docs: https://docs.temporal.io/self-hosted-guide
- Temporalite Deprecation: https://github.com/temporalio/temporalite#deprecation-notice
