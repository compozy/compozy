## status: pending

<task_context>
<domain>documentation</domain>
<type>validation</type>
<scope>user_documentation</scope>
<complexity>low</complexity>
<dependencies>all_previous_phases</dependencies>
</task_context>

# Task 26.0: Documentation Validation

## Overview

Final validation of all documentation to ensure accuracy, completeness, and quality. Verify all code examples work and all links are valid.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 6.5 before start
- **DEPENDENCIES:** All previous tasks (1.0-25.0) must be completed
- **BLOCKING:** This is the final validation gate before ship
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about documentation validation:
- check if documentation site has automated link checking
- verify code examples are complete and runnable
</research>

<requirements>
- All documentation updated with new mode names
- No broken links in documentation
- All code examples are valid and tested
- Migration guide is complete and accurate
- Documentation is clear and user-friendly
</requirements>

## Subtasks

- [ ] 26.1 Check for broken links in documentation
- [ ] 26.2 Validate all code examples work
- [ ] 26.3 Verify no "standalone" references remain (except historical)
- [ ] 26.4 Review migration guide completeness
- [ ] 26.5 Validate CLI help text accuracy

## Implementation Details

See `_techspec.md` Phase 6.5 for complete implementation details.

### Validation Commands

**Check broken links:**
```bash
cd docs
npm run lint:links  # If available
```

**Validate code examples:**
```bash
npm run test:examples  # If available
# Or manually test each example
```

**Find remaining "standalone" references:**
```bash
grep -r "standalone" docs/ examples/ README.md --exclude-dir=.git
# Should only show historical context or migration guides
```

### Relevant Files

**Documentation files:**
- `docs/content/docs/deployment/memory-mode.mdx`
- `docs/content/docs/deployment/persistent-mode.mdx`
- `docs/content/docs/deployment/distributed-mode.mdx`
- `docs/content/docs/configuration/mode-configuration.mdx`
- `docs/content/docs/guides/mode-migration-guide.mdx`
- `docs/content/docs/quick-start/index.mdx`
- `cli/help/global-flags.md`

**Example files:**
- `examples/memory-mode/`
- `examples/persistent-mode/`
- `examples/distributed-mode/`
- `examples/README.md`

### Dependent Files

All documentation from Tasks 4.1-4.5

## Deliverables

- All documentation validated and accurate
- No broken links in docs
- All code examples tested and working
- Migration guide complete and helpful
- CHANGELOG entry written
- Documentation is ship-ready

## Tests

- Documentation validation:
  - [ ] All links in documentation are valid
  - [ ] All code examples are syntactically correct
  - [ ] All code examples execute successfully
  - [ ] No "standalone" references except in migration contexts
  - [ ] Migration guide covers all scenarios
  - [ ] Quick start guide works as documented
  - [ ] Mode comparison tables are accurate
  - [ ] CLI help text matches implementation
  - [ ] CHANGELOG entry is complete

## Success Criteria

- ✅ All documentation links are valid
- ✅ All code examples work correctly
- ✅ No inappropriate "standalone" references
- ✅ Migration guide is complete and tested
- ✅ Documentation is clear and user-friendly
- ✅ CHANGELOG entry is written
- ✅ All documentation is ship-ready
