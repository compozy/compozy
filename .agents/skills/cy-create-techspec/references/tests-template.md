# Test Specification Template

Structure for `_tests.md` — the canonical test contract that ships alongside `_techspec.md`. Every test case for the feature lives here with a stable ID: `cy-create-tasks` assigns each ID to exactly one task, implementers write exactly the assigned cases, and review rounds check the shipped suite against this document. A behavior without a test ID here is a behavior nobody committed to verifying.

## ID Rules

- `UT-NNN` unit, `IT-NNN` integration, `E2E-NNN` end-to-end — zero-padded, sequential within each prefix.
- IDs are permanent once tasks reference them: never renumber or reuse. Mark a dropped case `(withdrawn)` in place instead of deleting the number.

## Document Skeleton

```markdown
# Test Specification: [Feature Name]

Canonical test contract for [feature]. Companion to `_techspec.md`.
Derived from `_user_stories.md` (behavior) and `_techspec.md` (components).

## Strategy

- Frameworks and harnesses: [test framework, fixture strategy, fakes at I/O boundaries]
- Execution: [how the unit / integration / e2e suites run in this repository]
- Conventions: [table-driven style, parallelism, naming patterns to follow]

## Coverage Matrix

| Source        | Behavior          | Unit           | Integration | E2E     |
|---------------|-------------------|----------------|-------------|---------|
| US-001        | [story summary]   | UT-001, UT-002 | IT-001      | E2E-001 |
| US-001.EC-1   | [edge case]       | UT-003         | —           | —       |
| [Component A] | [responsibility]  | UT-010–UT-014  | IT-002      | —       |

## Unit Tests

### [Component A] (TechSpec: [section name])

- **UT-001** (happy): [target function/behavior] — given [concrete input/state], produces [concrete expected output].
- **UT-002** (error): [target] — given [invalid input], returns [the specific error].
- **UT-003** (boundary): [target] — at [exact boundary value], behaves [expected result].

## Integration Tests

### [Boundary or flow]

- **IT-001**: [components wired together] — setup [fixtures/state]; do [action]; expect [observable result across the boundary].

## End-to-End Tests

### [User journey] (US-001, US-003)

- **E2E-001**: [entry point] → [user-visible steps] → [final observable outcome].
```

## Coverage Demands

The matrix is the completion gate for this document:

- Every `US-NNN` story **and** every `US-NNN.EC-N` edge case from `_user_stories.md` has its own row with at least one test ID.
- Every component and interface in the TechSpec has a row with unit coverage that includes its error paths — a component whose only cases are happy-path is uncovered.
- Every API endpoint, CLI verb, or message contract in the TechSpec has cases for its success shape and each documented failure shape.
- Every user journey has at least one end-to-end or integration case following it start to finish.
- An empty cell in a populated row is fine; a row with no IDs at all is a hole — fill it or annotate the row with the reason it needs no test.

## Case-Writing Rules

- Concrete or nothing: name the real function, route, or command, the actual input values, and the exact expected output or error. "Verify error handling" is not a case; "POST /runs with an unknown workflow id returns 404 with code=workflow_not_found" is.
- Tag every unit case with its class: `happy`, `error`, `boundary`, `concurrency`, `idempotency`, `ordering`, or `state`.
- One observable behavior per case — a case that needs "and" twice is two cases.
- Unit cases fake only I/O boundaries. Integration cases use real wiring between components. E2E cases go through the public surface (CLI, API, UI) exactly as a user would.
- Cover failure paths at every level, not just unit: interrupted flows, permission denials, and concurrent actors mirror the edge-case classes from `_user_stories.md`.
