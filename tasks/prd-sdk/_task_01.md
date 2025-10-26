## status: pending

<task_context>
<domain>sdk/workspace</domain>
<type>implementation</type>
<scope>scaffolding</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 01.0: Workspace + Module Scaffolding (S)

## Overview

Create the Go workspace structure and SDK module foundation. Establish go.work, sdk/go.mod, and basic directory structure for all SDK packages.

<critical>
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md (Go Workspace Structure section)
- **ALWAYS READ** tasks/prd-sdk/04-implementation-plan.md (Phase 0 section)
- **VERIFY** Go 1.25.2 compatibility
- **MUST** follow one-way dependency rule (sdk → engine only)
</critical>

<requirements>
- Create go.work with main module and sdk module
- Create sdk/go.mod with correct module path and dependencies
- Create directory structure for all 14 packages
- Create sdk/doc.go with package documentation
- Create sdk/internal/errors and sdk/internal/validate directories
- Verify workspace sync and module resolution
</requirements>

## Subtasks

- [ ] 01.1 Create go.work in repository root with both modules
- [ ] 01.2 Create sdk/go.mod with github.com/compozy/compozy/sdk module path
- [ ] 01.3 Create directory structure for all packages (project, model, workflow, agent, task, etc.)
- [ ] 01.4 Create sdk/doc.go with SDK package documentation
- [ ] 01.5 Create sdk/internal/errors and sdk/internal/validate packages
- [ ] 01.6 Run go work sync and verify module resolution

## Implementation Details

Reference: tasks/prd-sdk/02-architecture.md

### Workspace Layout

```
compozy/
├── go.work                    # NEW: Workspace definition
├── go.work.sum                # NEW: Workspace checksums
├── go.mod                     # EXISTING: Main module
└── sdk/                        # NEW: SDK module
    ├── go.mod                 # Module: github.com/compozy/compozy/sdk
    ├── doc.go                 # Package documentation
    ├── internal/
    │   ├── errors/
    │   └── validate/
    ├── project/
    ├── model/
    ├── workflow/
    ├── agent/
    ├── task/
    ├── knowledge/
    ├── memory/
    ├── mcp/
    ├── runtime/
    ├── tool/
    ├── schema/
    ├── schedule/
    ├── monitoring/
    └── compozy/
```

### Relevant Files

- `go.work` (NEW)
- `sdk/go.mod` (NEW)
- `sdk/doc.go` (NEW)

### Dependent Files

- None (foundation task)

## Deliverables

- ✅ `go.work` with both modules configured
- ✅ `sdk/go.mod` with correct module path and go 1.25.2
- ✅ Directory structure for all 14 SDK packages created
- ✅ `sdk/doc.go` with SDK documentation
- ✅ `sdk/internal/errors/` and `sdk/internal/validate/` packages
- ✅ Workspace successfully syncs with `go work sync`

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for workspace structure:
  - [ ] Test workspace resolution with `go work sync`
  - [ ] Test module imports work correctly (sdk → main)
  - [ ] Test no circular dependencies exist
  - [ ] Verify all directories are accessible

## Success Criteria

- `go work sync` executes without errors
- `cd sdk && go mod tidy` executes without errors
- All package directories exist and are importable
- sdk module can import engine packages
- No circular dependencies detected
- Documentation in sdk/doc.go describes SDK purpose
