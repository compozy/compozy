## status: pending

<task_context>
<domain>v2/workspace</domain>
<type>implementation</type>
<scope>scaffolding</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 01.0: Workspace + Module Scaffolding (S)

## Overview

Create the Go workspace structure and v2 SDK module foundation. Establish go.work, v2/go.mod, and basic directory structure for all SDK packages.

<critical>
- **ALWAYS READ** tasks/prd-modules/02-architecture.md (Go Workspace Structure section)
- **ALWAYS READ** tasks/prd-modules/04-implementation-plan.md (Phase 0 section)
- **VERIFY** Go 1.25.2 compatibility
- **MUST** follow one-way dependency rule (v2 → engine only)
</critical>

<requirements>
- Create go.work with main module and v2 module
- Create v2/go.mod with correct module path and dependencies
- Create directory structure for all 14 packages
- Create v2/doc.go with package documentation
- Create v2/internal/errors and v2/internal/validate directories
- Verify workspace sync and module resolution
</requirements>

## Subtasks

- [ ] 01.1 Create go.work in repository root with both modules
- [ ] 01.2 Create v2/go.mod with github.com/compozy/compozy/v2 module path
- [ ] 01.3 Create directory structure for all packages (project, model, workflow, agent, task, etc.)
- [ ] 01.4 Create v2/doc.go with SDK package documentation
- [ ] 01.5 Create v2/internal/errors and v2/internal/validate packages
- [ ] 01.6 Run go work sync and verify module resolution

## Implementation Details

Reference: tasks/prd-modules/02-architecture.md

### Workspace Layout

```
compozy/
├── go.work                    # NEW: Workspace definition
├── go.work.sum                # NEW: Workspace checksums
├── go.mod                     # EXISTING: Main module
└── v2/                        # NEW: SDK module
    ├── go.mod                 # Module: github.com/compozy/compozy/v2
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
- `v2/go.mod` (NEW)
- `v2/doc.go` (NEW)

### Dependent Files

- None (foundation task)

## Deliverables

- ✅ `go.work` with both modules configured
- ✅ `v2/go.mod` with correct module path and go 1.25.2
- ✅ Directory structure for all 14 SDK packages created
- ✅ `v2/doc.go` with SDK documentation
- ✅ `v2/internal/errors/` and `v2/internal/validate/` packages
- ✅ Workspace successfully syncs with `go work sync`

## Tests

Reference: tasks/prd-modules/_tests.md

- Unit tests for workspace structure:
  - [ ] Test workspace resolution with `go work sync`
  - [ ] Test module imports work correctly (v2 → main)
  - [ ] Test no circular dependencies exist
  - [ ] Verify all directories are accessible

## Success Criteria

- `go work sync` executes without errors
- `cd v2 && go mod tidy` executes without errors
- All package directories exist and are importable
- v2 module can import engine packages
- No circular dependencies detected
- Documentation in v2/doc.go describes SDK purpose
