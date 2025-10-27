# Task 8.0: Documentation

## status: completed

**Size:** L (2-3 days)  
**Priority:** HIGH - User-facing  
**Dependencies:** Task 5.0

## Overview

Create comprehensive documentation covering temporal modes, architecture, configuration, and troubleshooting.

## Deliverables

- [x] `docs/content/docs/deployment/temporal-modes.mdx` - Mode selection guide
- [x] `docs/content/docs/architecture/embedded-temporal.mdx` - Architecture deep-dive
- [x] `docs/content/docs/configuration/temporal.mdx` - Update config reference
- [x] `docs/content/docs/quick-start/index.mdx` - Update quick start
- [x] `docs/content/docs/deployment/production.mdx` - Update production guide
- [x] `docs/content/docs/cli/compozy-start.mdx` - Update CLI docs
- [x] `docs/content/docs/troubleshooting/temporal.mdx` - Troubleshooting guide

## Acceptance Criteria

- [x] All 7 pages created/updated
- [x] Navigation config updated
- [x] Code examples tested
- [x] YAML configuration examples included
- [x] Architecture diagrams clear
- [x] Troubleshooting section comprehensive
- [x] Production warnings prominent
- [x] Links validated
- [x] Docs site builds successfully

## Content Outline

See `_docs.md` for complete content specifications.

**temporal-modes.mdx:**
- Overview of remote vs standalone
- When to use each mode
- Configuration examples
- Migration guide

**embedded-temporal.mdx:**
- Four-service architecture
- SQLite persistence design
- Port allocation
- Lifecycle management
- UI server integration

**Configuration reference updates:**
- Mode field documentation
- Standalone config fields
- Default values
- Validation rules

**Quick start updates:**
- Add standalone mode quick start
- Update setup instructions

**Production guide updates:**
- Warning against standalone in production
- Remote mode recommended practices

**CLI documentation:**
- Document all --temporal-* flags
- Usage examples

**Troubleshooting:**
- Port conflicts
- SQLite errors
- Startup timeouts
- UI not accessible
- Performance issues

## Files to Create/Modify

- `docs/content/docs/deployment/temporal-modes.mdx` (new)
- `docs/content/docs/architecture/embedded-temporal.mdx` (new)
- `docs/content/docs/troubleshooting/temporal.mdx` (new)
- `docs/content/docs/configuration/temporal.mdx` (update)
- `docs/content/docs/quick-start/index.mdx` (update)
- `docs/content/docs/deployment/production.mdx` (update)
- `docs/content/docs/cli/compozy-start.mdx` (update)
- `docs/source.config.ts` (navigation)

## Validation

```bash
# Build docs site
cd docs && npm run build

# Check for broken links
npm run lint

# Preview locally
npm run dev
```
