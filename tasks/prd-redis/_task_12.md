## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs/content/docs</domain>
<type>documentation</type>
<scope>user_documentation</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 12.0: User Documentation [Size: M - 1-2 days]

## Overview

Create comprehensive user-facing documentation for standalone mode, including deployment guides, configuration references, and migration guides. This documentation should help users understand when to use standalone vs distributed mode, how to configure it, and how to migrate between modes.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- Documentation must be accurate, complete, and beginner-friendly
- All code examples must be tested and work correctly
- Configuration examples must be valid YAML
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Create deployment guide for standalone mode
- Create mode configuration reference guide
- Create Redis-specific configuration reference
- Create migration guide from standalone to distributed
- Update existing docs to reference standalone mode
- Update navigation to include new docs
- All examples must be tested and working
- Documentation must follow project style and conventions
</requirements>

## Subtasks

- [ ] 12.1 Create standalone deployment guide
- [ ] 12.2 Create mode configuration guide
- [ ] 12.3 Create Redis configuration reference
- [ ] 12.4 Create migration guide (standalone → distributed)
- [ ] 12.5 Update getting started quickstart guide
- [ ] 12.6 Update distributed mode deployment guide
- [ ] 12.7 Update architecture documentation
- [ ] 12.8 Update configuration overview
- [ ] 12.9 Update troubleshooting guide
- [ ] 12.10 Update FAQ
- [ ] 12.11 Update navigation structure
- [ ] 12.12 Validate all code examples and configuration samples

## Implementation Details

### Documentation Structure

Create new documentation pages under `docs/content/docs/` following the existing structure and style. Update existing pages to reference standalone mode where relevant.

### Relevant Files

**New Documentation Files:**
- `docs/content/docs/deployment/standalone-mode.mdx` - Standalone deployment guide
- `docs/content/docs/configuration/mode-configuration.mdx` - Mode configuration guide
- `docs/content/docs/configuration/redis.mdx` - Redis configuration reference
- `docs/content/docs/guides/migrate-standalone-to-distributed.mdx` - Migration guide

**Files to Update:**
- `docs/content/docs/deployment/distributed-mode.mdx` - Add comparison with standalone
- `docs/content/docs/getting-started/quickstart.mdx` - Add standalone quick start
- `docs/content/docs/architecture/overview.mdx` - Explain both modes
- `docs/content/docs/configuration/overview.mdx` - Reference mode configuration
- `docs/content/docs/troubleshooting/common-issues.mdx` - Add standalone troubleshooting
- `docs/content/docs/faq.mdx` - Add standalone mode FAQs
- `docs/meta.json` or navigation config - Add new pages to nav

### Dependent Files

- Task 3.0 deliverables - Mode-aware cache factory for examples
- Examples from Task 13.0 - Reference in documentation

## Deliverables

### New Documentation Pages

**1. Standalone Deployment Guide** (`deployment/standalone-mode.mdx`):
- When to use standalone mode (use cases, benefits, limitations)
- Requirements (Go 1.25+, PostgreSQL, optional dependencies)
- Quick start installation
- Configuration examples
- Running the server
- Verifying the setup
- Performance expectations
- Troubleshooting common issues

**2. Mode Configuration Guide** (`configuration/mode-configuration.mdx`):
- Overview of deployment modes (standalone vs distributed)
- Global mode configuration
- Component-specific mode overrides
- Mode resolution and inheritance rules
- Configuration examples (pure standalone, pure distributed, mixed)
- Best practices for mode selection
- Configuration validation

**3. Redis Configuration Reference** (`configuration/redis.mdx`):
- Redis configuration structure
- Distributed mode settings (addr, password, TLS)
- Standalone mode settings (persistence config)
- Persistence options (snapshot interval, data directory)
- Mode resolution for Redis
- Performance tuning
- Monitoring and metrics
- Troubleshooting

**4. Migration Guide** (`guides/migrate-standalone-to-distributed.mdx`):
- When to migrate (scaling triggers, use cases)
- Prerequisites (Redis setup, infrastructure)
- Step-by-step migration process
- Configuration changes required
- Data export/import (if applicable)
- Rollback procedures
- Testing and validation
- Common migration issues

### Updated Documentation Pages

**5. Distributed Mode Guide** - Add comparison with standalone mode
**6. Quickstart Guide** - Add standalone quick start option
**7. Architecture Overview** - Explain both deployment modes
**8. Configuration Overview** - Reference mode configuration
**9. Troubleshooting Guide** - Add standalone-specific issues
**10. FAQ** - Add standalone mode questions

### Navigation Updates

- Update docs navigation to include new pages in appropriate sections
- Ensure logical flow from getting started → deployment → configuration → guides

## Tests

Documentation validation checklist:

### Content Quality

- [ ] All documentation is accurate and complete
- [ ] Technical details match implementation
- [ ] Use cases and benefits are clearly explained
- [ ] Limitations and trade-offs are honestly presented
- [ ] Examples are relevant and helpful
- [ ] Troubleshooting covers common issues
- [ ] Links to related documentation work correctly

### Code Examples

- [ ] All YAML configuration examples are valid
- [ ] All CLI commands are correct and tested
- [ ] All code snippets compile and work
- [ ] Configuration examples cover common scenarios
- [ ] Examples follow project conventions

### Configuration Examples Validation

- [ ] Minimal standalone config works
- [ ] Standalone with persistence config works
- [ ] Mixed mode config works
- [ ] Distributed mode config works (existing, unchanged)
- [ ] Invalid configs are rejected with helpful errors

### User Experience

- [ ] Documentation is beginner-friendly
- [ ] Navigation is logical and intuitive
- [ ] Search finds relevant pages
- [ ] Cross-references are helpful
- [ ] Formatting is consistent with existing docs

### Migration Guide Validation

- [ ] Migration steps are clear and complete
- [ ] Prerequisites are listed
- [ ] Configuration changes are accurate
- [ ] Rollback procedure is provided
- [ ] Common issues are addressed

### Completeness

- [ ] All new features are documented
- [ ] All configuration options are documented
- [ ] All CLI flags are documented
- [ ] All error messages are explained
- [ ] All limitations are disclosed

## Success Criteria

- All 4 new documentation pages created and published
- All 6 existing pages updated with standalone mode references
- Navigation updated to include new pages
- All code examples and configuration samples tested and working
- Documentation follows project style and conventions
- Documentation is clear, accurate, and beginner-friendly
- Migration guide provides complete migration path
- Troubleshooting covers common standalone mode issues
- FAQ answers key questions about standalone mode
- Documentation builds successfully with docs tooling
- No broken links in documentation
- Search functionality finds new pages
- Peer review completed and feedback addressed
