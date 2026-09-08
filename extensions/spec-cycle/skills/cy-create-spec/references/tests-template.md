# Test Specification Template

Structure for `_tests.md` — the canonical test contract that ships alongside `_spec.md`. Every test case for the feature lives here with a stable ID: `cy-create-tasks` assigns each ID to exactly one task, implementers write exactly the assigned cases, and review rounds check the shipped suite against this document. Existing suite/gate/probe evidence may own a behavior without a new test ID.

## ID Rules

- `UT-NNN` unit, `IT-NNN` integration, `E2E-NNN` end-to-end — zero-padded, sequential within each prefix.
- IDs are permanent once tasks reference them: never renumber or reuse. Mark a dropped case `(withdrawn)` in place instead of deleting the number.

## Document Skeleton

```markdown
# Test Specification: [Feature Name]

Canonical test contract for [feature]. Companion to `_spec.md`.
Derived from `_user_stories.md` (behavior), `_spec.md` Part II (components),
`_dx.md` (CLI/API journeys), and `_uiux.md` (browser journeys) when present.

## Strategy

- Frameworks and harnesses: [test framework, fixture strategy, fakes at I/O boundaries]
- Execution: [how the unit / integration / e2e suites run in this repository]
- Conventions: [table-driven style, parallelism, naming patterns to follow]

## Coverage Matrix

| Source        | Behavior         | Unit           | Integration | E2E     |
| ------------- | ---------------- | -------------- | ----------- | ------- |
| US-001        | [story summary]  | UT-001, UT-002 | IT-001      | E2E-001 |
| US-001.EC-1   | [edge case]      | UT-003         | —           | —       |
| [Component A] | [responsibility] | UT-010–UT-014  | IT-002      | —       |

## Unit Tests

### [Component A] (Spec: [Part II section name])

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

## Coverage Decisions

For each distinct changed invariant, name its owning layer and canonical suite or stronger check. Reuse existing cases; add a new ID only for a gap. Public commands and failure shapes use their actual documented contract. Integration/E2E cases cover necessary boundary/journey risks; they do not duplicate every unit invariant. A row may cite existing coverage or explain why no automated test is needed.

Completeness is checked per owner, not per count: every `US-NNN` story and `US-NNN.EC-N` edge case has a row whose owner is a test ID, existing coverage cited by path, or a one-line reason no automated test is needed. A row with none of those is a gap. A component whose only cases are happy-path is a gap unless its failure modes are owned elsewhere in the matrix.

## Case-Writing Rules

- Concrete or nothing: name the real function, route, or command, the actual input values, and the exact expected output or error. "Verify error handling" is not a case; "POST /runs with an unknown workflow id returns 404 with code=workflow_not_found" is.
- Tag every unit case with its class: `happy`, `error`, `boundary`, `concurrency`, `idempotency`, `ordering`, or `state`.
- One observable behavior per case — a case that needs "and" twice is two cases.
- Unit cases fake only I/O boundaries. Integration cases use real wiring between components. E2E cases go through the public surface (CLI, API, UI) exactly as a user would — for CLI/API that means the `_dx.md` transcripts verbatim.
- Cover distinct relevant failure modes at their owning layer; interrupted flows, permission denials, and concurrent actors require cases when the changed contract makes them meaningful.
