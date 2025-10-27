# Task 9.0: Examples

**Size:** L (2-3 days)  
**Priority:** MEDIUM - User education  
**Dependencies:** Task 5.0

## Overview

Create 7 example projects demonstrating various standalone mode configurations and use cases.

## Deliverables

- [ ] `examples/temporal-standalone/basic/` - Basic setup
- [ ] `examples/temporal-standalone/persistent/` - File-based persistence
- [ ] `examples/temporal-standalone/custom-ports/` - Custom port configuration
- [ ] `examples/temporal-standalone/no-ui/` - UI disabled
- [ ] `examples/temporal-standalone/debugging/` - Development with UI
- [ ] `examples/temporal-standalone/migration-from-remote/` - Mode migration
- [ ] `examples/temporal-standalone/integration-testing/` - Testing patterns

## Acceptance Criteria

- [ ] All 7 examples created
- [ ] Each example includes README.md with clear instructions
- [ ] Each example includes compozy.yaml configuration
- [ ] Each example includes workflow.yaml
- [ ] All examples tested and working
- [ ] READMEs explain use case and key concepts
- [ ] No linter errors

## Content Outline

See `_examples.md` for complete example specifications.

**basic/:**
- In-memory mode
- Default ports
- UI enabled
- Simple workflow

**persistent/:**
- File-based SQLite
- Demonstrates persistence across restarts
- Workflow execution before/after restart

**custom-ports/:**
- Custom FrontendPort (17233)
- Custom UIPort (18233)
- Port conflict resolution example

**no-ui/:**
- EnableUI=false
- Minimal configuration
- Suitable for CI/testing

**debugging/:**
- Full UI enabled
- Detailed logging
- Step-by-step debugging workflow
- UI navigation guide

**migration-from-remote/:**
- Side-by-side configs (remote vs standalone)
- Migration checklist
- Testing strategy

**integration-testing/:**
- Testing patterns
- Test fixtures
- CI configuration example
- Teardown strategies

## Example Structure

Each example should have:
```
examples/temporal-standalone/<name>/
├── README.md           # Clear instructions
├── compozy.yaml        # Configuration
├── workflow.yaml       # Example workflow
├── .env.example        # Environment template (if needed)
└── tools/              # Custom tools (if needed)
    └── example.ts
```

## Files to Create

- 7 example directories with complete projects
- Each with README, configs, and workflow

## Validation

```bash
# Test each example
cd examples/temporal-standalone/basic
compozy start

# Verify workflow execution
curl -X POST http://localhost:3000/api/workflows/{id}/execute

# Check UI access
open http://localhost:8233
```
