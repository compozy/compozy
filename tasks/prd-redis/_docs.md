# Documentation Plan: Standalone Mode - Redis Alternatives

## Goals

- Document the standalone deployment mode for Compozy
- Provide clear configuration examples for mode inheritance pattern
- Create migration guides from standalone to distributed mode
- Update existing deployment documentation with mode options

## New/Updated Pages

### docs/content/docs/deployment/standalone-mode.mdx (new)
- Purpose: Comprehensive guide to deploying Compozy in standalone mode
- Outline:
  - What is Standalone Mode
  - When to Use Standalone vs Distributed
  - Architecture Overview (miniredis + optional persistence)
  - Quick Start Guide
  - Configuration Reference
  - Memory and Performance Considerations
  - Limitations and Trade-offs
  - Migration to Distributed Mode
- Links: 
  - configuration/mode-configuration.mdx
  - deployment/distributed-mode.mdx
  - deployment/production.mdx

### docs/content/docs/configuration/mode-configuration.mdx (new)
- Purpose: Document the global mode configuration pattern and inheritance
- Outline:
  - Global Mode Configuration
  - Component Mode Inheritance
  - Mode Resolution Priority
  - Configuration Examples (full standalone, full distributed, mixed mode)
  - Per-Component Override Examples
  - Environment Variable Overrides
  - Validation Rules
- Links:
  - configuration/redis.mdx
  - configuration/temporal.mdx
  - configuration/mcp-proxy.mdx
  - deployment/standalone-mode.mdx

### docs/content/docs/configuration/redis.mdx (new)
- Purpose: Complete Redis/cache configuration reference
- Outline:
  - Overview (distributed vs standalone)
  - Distributed Mode Configuration (external Redis)
  - Standalone Mode Configuration (miniredis)
  - Persistence Options (BadgerDB snapshots)
  - Performance Tuning
  - Troubleshooting
- Links:
  - configuration/mode-configuration.mdx
  - deployment/standalone-mode.mdx

### docs/content/docs/deployment/distributed-mode.mdx (update)
- Purpose: Update to clarify distributed mode is for production/scale
- Updates:
  - Add comparison table: standalone vs distributed
  - Add section on when to migrate from standalone
  - Update prerequisites to mention mode configuration
- Links:
  - deployment/standalone-mode.mdx
  - configuration/mode-configuration.mdx

### docs/content/docs/getting-started/quickstart.mdx (update)
- Purpose: Add standalone mode quick start option
- Updates:
  - Add "Option 1: Standalone Mode" section before existing instructions
  - Show simple `mode: standalone` config example
  - Note that standalone is ideal for local development
- Links:
  - deployment/standalone-mode.mdx

### docs/content/docs/deployment/production.mdx (update)
- Purpose: Clarify production deployments should use distributed mode
- Updates:
  - Add warning about standalone mode limitations
  - Add decision matrix: standalone vs distributed
  - Update deployment checklist to include mode selection
- Links:
  - deployment/distributed-mode.mdx
  - deployment/standalone-mode.mdx

## Schema Docs

### docs/content/docs/reference/config-schema.mdx (update)
- Renders `schemas/config.json`
- Notes to highlight:
  - New global `mode` field (standalone | distributed)
  - New `redis` configuration section with mode inheritance
  - New `redis.standalone.persistence` configuration
  - Mode resolution logic explanation
- Add visual diagram showing mode inheritance

## API Docs

No API changes required - standalone mode is transparent to API consumers.

## CLI Docs

### docs/content/docs/cli/start.mdx (update)
- Purpose: Document `--standalone` flag and mode configuration
- Updates:
  - Add `--mode` flag documentation
  - Show examples: `compozy start --mode standalone`
  - Show examples: `compozy start --mode distributed`
  - Note that YAML config takes precedence over flags
- Links:
  - configuration/mode-configuration.mdx
  - deployment/standalone-mode.mdx

### docs/content/docs/cli/config.mdx (update)
- Purpose: Document config validation for mode settings
- Updates:
  - Add mode configuration validation examples
  - Show `compozy config show` output with mode fields
  - Show `compozy config diagnostics` mode resolution output
- Links:
  - configuration/mode-configuration.mdx

## Cross-page Updates

### docs/content/docs/concepts/architecture.mdx (update)
- Add section on "Deployment Modes"
- Update architecture diagrams to show both standalone and distributed options

### docs/content/docs/configuration/temporal.mdx (update)
- Document mode inheritance from global config
- Add examples showing `temporal.mode` override

### docs/content/docs/configuration/mcp-proxy.mdx (update)
- Document mode inheritance from global config
- Add examples showing `mcpproxy.mode` override

### docs/content/docs/troubleshooting/common-issues.mdx (update)
- Add section: "Redis Connection Issues in Standalone Mode"
- Add section: "Mode Configuration Validation Errors"
- Add section: "Snapshot/Persistence Failures"

## Navigation & Indexing

Update `docs/source.config.ts`:

```typescript
// Deployment section
{
  title: "Deployment",
  pages: [
    "deployment/standalone-mode",     // NEW
    "deployment/distributed-mode",    // UPDATED
    "deployment/production",
    "deployment/docker",
    "deployment/kubernetes"
  ]
}

// Configuration section
{
  title: "Configuration",
  pages: [
    "configuration/overview",
    "configuration/mode-configuration", // NEW
    "configuration/redis",              // NEW
    "configuration/temporal",
    "configuration/database",
    // ... existing pages
  ]
}
```

## Migration Guide

### docs/content/docs/guides/migrate-standalone-to-distributed.mdx (new)
- Purpose: Step-by-step guide for migrating from standalone to distributed
- Outline:
  - Prerequisites (Redis, updated config)
  - Step 1: Provision External Redis
  - Step 2: Update Configuration (change mode)
  - Step 3: Export Critical Data (if needed)
  - Step 4: Restart Services
  - Step 5: Verify Functionality
  - Step 6: Clean Up Standalone Data
  - Rollback Procedure
  - Troubleshooting Common Issues
- Links:
  - deployment/distributed-mode.mdx
  - configuration/redis.mdx

## Acceptance Criteria

- [ ] All new pages exist with complete outlines and working examples
- [ ] Cross-links between standalone, distributed, and mode configuration docs are bidirectional
- [ ] Configuration schema docs render the new mode and redis fields correctly
- [ ] CLI documentation shows mode flags and configuration examples
- [ ] Navigation in `source.config.ts` includes new pages in logical order
- [ ] Migration guide provides clear, testable steps
- [ ] Docs dev server builds without warnings or missing routes
- [ ] All code examples in docs are syntactically correct and follow project standards
- [ ] Performance and limitations clearly documented for standalone mode
- [ ] Decision matrices help users choose appropriate deployment mode

## Visual Assets Needed

1. **Architecture Diagram**: Standalone vs Distributed comparison
   - Location: `docs/public/images/deployment/`
   - Shows: miniredis + optional BadgerDB vs external Redis

2. **Mode Inheritance Diagram**: Configuration resolution flow
   - Location: `docs/public/images/configuration/`
   - Shows: Global mode → Component modes → Default fallback

3. **Decision Matrix**: When to use standalone vs distributed
   - Location: Inline in docs as table
   - Criteria: Team size, workload, durability, budget, complexity

## Documentation Review Checklist

- [ ] Technical accuracy verified against implementation
- [ ] Configuration examples tested and validated
- [ ] Migration guide steps tested end-to-end
- [ ] Performance numbers and limitations are accurate
- [ ] Security considerations documented
- [ ] Troubleshooting section covers common issues
- [ ] Links to external resources (miniredis, BadgerDB) are current
- [ ] Code snippets follow project coding standards
- [ ] YAML examples follow configuration best practices

