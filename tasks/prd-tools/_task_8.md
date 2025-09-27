---
status: pending
parallelizable: true
blocked_by: ["1.0", "5.0"]
---

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>documentation</dependencies>
<unblocks>[]</unblocks>
</task_context>

# Task 8.0: Update documentation, templates

## Overview

Revise documentation, CLI templates, and communication assets to reflect cp\_\_ native tools as the default experience. Provide migration steps, configuration guidance, and troubleshooting materials aligned with the new error catalog.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Create or update docs (`docs/native-tools.md`, tutorials, changelog) to explain cp__ tools, configuration options, kill switch usage, and troubleshooting.
- Update CLI scaffolding templates and examples to reference `cp__` identifiers exclusively and remove `@compozy/tool-*` dependencies.
- Produce a migration checklist for internal teams (identify legacy references, run doc scan CI check).
- Coordinate with developer relations to schedule announcement and capture DX feedback survey instrumentation.
- Ensure accessibility compliance in updated docs (heading hierarchy, alt text, contrast checks).
</requirements>

## Subtasks

- [ ] 8.1 Draft comprehensive cp\_\_ tool documentation with configuration and troubleshooting sections.
- [ ] 8.2 Update CLI templates and examples to use cp\_\_ identifiers and remove Bun workspace references.
- [ ] 8.3 Implement CI/doc scan detecting lingering `@compozy/tool-` references.
- [ ] 8.4 Coordinate release notes, changelog entries, and DX survey updates with DevRel.

## Sequencing

- Blocked by: 1.0, 5.0
- Unblocks: None (documentation deliverable)
- Parallelizable: Yes (once dependencies met)

## Implementation Details

Use PRD "Documentation and Developer Experience" requirements and tech spec "Integration Points" plus "Rollout & Verification" guidance. Ensure all references to native tools align with canonical error terminology.

### Relevant Files

- `docs/native-tools.md`
- `pkg/template/templates/*`
- `docs/changelog.md`

### Dependent Files

- `engine/tool/builtin/registry.go`
- `pkg/config/native_tools.go`

## Success Criteria

- All documentation references updated; CI scan passes without `@compozy/tool-` strings.
- Migration guide published alongside release notes.
- Developer feedback loop (survey link) embedded in documentation.
