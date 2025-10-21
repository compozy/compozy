# Refactoring check.go to Clean Architecture - Task Summary

## Relevant Files

### Source File to Refactor

- `scripts/markdown/check.go` - 3,055-line monolithic script to be refactored

### Target Architecture Structure

```
scripts/markdown/
├── cmd/check/              # Application Layer (CLI Entry)
├── ui/                     # Presentation Layer (UI Components)
├── core/                   # Domain Layer (Business Logic)
├── infrastructure/         # Infrastructure Layer (External Dependencies)
└── shared/                 # Shared Kernel (Common Utilities)
```

### Related Project Standards

- `.cursor/rules/go-coding-standards.mdc` - Go coding standards
- `.cursor/rules/architecture.mdc` - Architecture patterns
- `.cursor/rules/test-standards.mdc` - Testing requirements

## Tasks

- [ ] 1.0 Foundation Setup - Create Directory Structure and Domain Models (M)
- [ ] 2.0 Infrastructure Layer - File System and Command Execution Abstractions (M)
- [ ] 3.0 Core Business Logic - Extract Services and Use Cases (L)
- [ ] 4.0 UI Layer - Form Builders and Components (M)
- [ ] 5.0 Application Wiring - Dependency Injection and Integration (M)
- [ ] 6.0 Testing and Migration - Comprehensive Tests and Cleanup (L)

Notes on sizing:

- S = Small (≤ half-day)
- M = Medium (1–2 days)
- L = Large (3+ days)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Do not split one deliverable across multiple parent tasks; avoid cross-task coupling
- Each parent task must include unit test subtasks where applicable
- Each generated `/_task_<num>.md` must contain explicit Deliverables and Tests sections
- Functions must be < 50 lines (project-wide constraint)
- All code must pass `make lint` and `make test` before marking as complete

## Execution Plan

### Critical Path (Sequential Dependencies)

1.0 (Foundation) → 3.0 (Core Logic) → 5.0 (Wiring) → 6.0 (Testing & Migration)

### Parallel Track A (after 1.0)

- 2.0 (Infrastructure Layer) - Can be developed independently after domain models exist

### Parallel Track B (after 1.0)

- 4.0 (UI Layer) - Can be developed independently after domain models exist

### Parallelization Strategy

```
Start: 1.0 (Foundation)
  ↓
Split into 3 parallel tracks:
  ├─→ 2.0 (Infrastructure) ─┐
  ├─→ 3.0 (Core Logic)      ├─→ 5.0 (Wiring) → 6.0 (Testing)
  └─→ 4.0 (UI Layer)        ─┘
```

**Key Benefit**: Tasks 2.0, 3.0, and 4.0 can be executed in parallel by different developers or sessions, reducing overall delivery time from ~10 days to ~6 days.

## Batch Plan (Grouped Commits)

- [ ] Batch 1 — Foundation Layer: 1.0
  - Commit message: "refactor(check): create clean architecture foundation with domain models"

- [ ] Batch 2 — Infrastructure & Core (Parallel): 2.0, 3.0
  - Commit message: "refactor(check): implement infrastructure abstractions and core business logic"

- [ ] Batch 3 — UI Layer: 4.0
  - Commit message: "refactor(check): extract UI components and form builders"

- [ ] Batch 4 — Integration & Testing: 5.0, 6.0
  - Commit message: "refactor(check): wire dependencies and add comprehensive tests"

## Architecture Overview

### Current State (Problem)

- **Single File**: 3,055 lines, 158 functions
- **Violations**: SRP, DIP, ISP, OCP
- **Issues**: Tight coupling, no testability, hard to extend

### Target State (Solution)

**5-Layer Clean Architecture**:

1. **cmd/** - Application entry point, CLI setup, DI container
2. **ui/** - Presentation layer (tea/, forms/, styles/)
3. **core/** - Domain layer (models/, services/, usecases/)
4. **infrastructure/** - External dependencies (filesystem/, execution/, logging/, prompts/)
5. **shared/** - Shared utilities (errors/, types/, utils/)

### Key Design Principles Applied

- **SOLID Compliance**: Each layer respects SRP, OCP, LSP, ISP, DIP
- **Dependency Inversion**: High-level modules depend on abstractions
- **Interface Segregation**: Small, focused interfaces
- **Clean Separation**: Business logic independent of UI and infrastructure

## Quality Requirements

### Mandatory Checks Before Task Completion

- [ ] All functions < 50 lines
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] All interfaces properly defined
- [ ] Context properly inherited (no `context.Background()` in runtime)
- [ ] Logger from context (`logger.FromContext(ctx)`)
- [ ] Config from context (`config.FromContext(ctx)`)

### Testing Strategy

- **Unit Tests**: Each component tested in isolation
- **Integration Tests**: Layer interactions validated
- **E2E Tests**: Full workflow preserved
- **Performance Tests**: No regression vs. original implementation

## Risk Mitigation

### Backward Compatibility

- Original CLI interface preserved
- Functional equivalence guaranteed
- No breaking changes to user experience

### Rollback Strategy

- Keep original `check.go` as `check.go.bak` during refactoring
- Each batch is independently reversible
- Feature flags for gradual rollout (if needed)

## Success Criteria

### Functional Requirements

- [ ] All existing functionality preserved
- [ ] CLI interface unchanged
- [ ] Performance characteristics maintained
- [ ] Memory usage not significantly increased

### Architectural Requirements

- [ ] SOLID principles followed throughout
- [ ] Clean architecture layers properly separated
- [ ] All dependencies injected through constructors
- [ ] No circular dependencies between layers

### Quality Metrics

- [ ] Cyclomatic complexity < 10 per function
- [ ] Function length < 50 lines
- [ ] Test coverage > 80%
- [ ] Zero linting errors

## Timeline Estimate

**Sequential Execution**: ~10 days

- Day 1-2: Task 1.0 (Foundation)
- Day 3-4: Task 2.0 (Infrastructure) OR Task 3.0 (Core) OR Task 4.0 (UI)
- Day 5-6: Remaining parallel tasks
- Day 7-8: Task 5.0 (Wiring)
- Day 9-10: Task 6.0 (Testing & Migration)

**Parallel Execution**: ~6 days (with 3 concurrent tracks)

- Day 1-2: Task 1.0 (Foundation)
- Day 3-4: Tasks 2.0, 3.0, 4.0 (Parallel)
- Day 5: Task 5.0 (Wiring)
- Day 6: Task 6.0 (Testing & Migration)

## Notes

- **Context Inheritance**: All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- **Testing Context**: Tests MUST use `t.Context()` instead of `context.Background()`
- **No Global State**: Never use global configuration singletons
- **Go Version**: Project uses Go 1.25.2 - use modern patterns like `sync.WaitGroup.Go()`
- **Dependency Updates**: Run `make deps` if new dependencies are added
- **Scoped Testing**: During development, use scoped test commands for faster feedback
  - Example: `gotestsum --format pkgname -- -race -parallel=4 ./scripts/markdown/...`
- **Final Validation**: Run `make fmt && make lint && make test` before marking any task as completed

## References

- Technical Specification: `tasks/prd-check/_techspec.md`
- Go Coding Standards: `.cursor/rules/go-coding-standards.mdc`
- Architecture Patterns: `.cursor/rules/architecture.mdc`
- Testing Requirements: `.cursor/rules/test-standards.mdc`
