# Reviewing Agent-Generated Tests

Use this reference when authoring or reviewing tests whose oracle, placement, or use of mocks needs attention. The same behavioral contract applies regardless of who wrote the test. This reference adds no approval stage, fixed report format, or companion-test quota.

## Invariant and owning suite

Before adding or changing a test, name the behavior that must hold, the lowest layer that can observe it, and the canonical suite. Reuse the task's existing evidence and the repository's test conventions.

Extend an existing file when it owns the invariant. When none fits, explain the new file's responsibility and use the established layout. A missing suite alone does not require user confirmation. Ask only when an unresolved product contract or action beyond existing authorization changes the outcome.

Keep one owner per invariant. Additional layers earn coverage for distinct wiring, persistence, transport, or user-journey behavior, not because a unit test exists.

## Build an independent oracle

Derive expected results from the behavior contract, independently calculated examples, or known input/output cases. Do not copy the production constant or algorithm merely to make the assertion agree with the implementation.

A test should detect a plausible defect in the system under test. For example, an adapter test may supply an I/O response and verify the adapter's transformation or error handling. Asserting only a value placed directly into a mock proves nothing about the adapter.

Keep mocks at unit-test I/O boundaries. Use real collaborators when their interaction is the invariant. Avoid mocking internal helpers or asserting call order unless that order is an observable protocol requirement.

## Match execution to the claim

| Claimed behavior | Evidence that can establish it |
| --- | --- |
| Pure transformation or local decision | Execute the actual implementation in its unit suite with independent expected results. |
| Database behavior | Run against the supported database through the real data layer and appropriate disposable fixtures. |
| HTTP integration | Exercise the actual handler chain and verify the response and relevant resulting state. |
| External service compatibility | Use authorized live-service evidence when claiming live compatibility; label replay/cassette evidence as recorded, not current provider verification. |
| User interaction | Exercise the app's public surface through the existing integration/end-to-end suite or the app itself. |

Run the changed tests and the applicable real integration or app validation for the requested change. Reuse existing coverage and trustworthy results for unchanged inputs. A unit test does not require a new integration companion when an existing run already owns that boundary, or when no integration behavior is involved. A new test file is not a substitute for executing the product path being claimed.

## Resolve a failing test

Read the relevant failure and trace it to the test's expectation and owning production behavior. Preserve the observed evidence; a fixed-format transcript or full-file reread is unnecessary when the cause is already clear.

If the test exposes a bug or regression, fix production code and rerun the affected checks. Do not weaken the assertion to accept the defect. Correct a test only with evidence that its expectation is wrong or the authorized contract intentionally changed; explain that evidence and preserve valid coverage. Existing authorization governs the repair, without an additional confirmation for each edit.

## Artifact and negative coverage

Test prose, CSS, generated output, configuration, or snapshots only when the artifact itself is the product contract and an existing build, typecheck, renderer, or other owning gate is insufficient. Prefer focused assertions that identify the failure; a snapshot is an option, not a default requirement even for public artifacts.

Cover rejection, boundary, recovery, and cancellation behavior where the contract has those cases and coverage is missing. Add them to the owning suite; there is no one-negative-per-positive rule. Removing a test usually leaves the remaining suite green and says nothing about that test's value. When the oracle is uncertain, reproduce the known defect or introduce a relevant production mutation in an isolated fixture and confirm the test detects it. Mutation testing is diagnostic, not a required extra run for every file.

## Review and completion

Review the changed tests for useful failure detection, correct ownership, independent expectations, and uncovered relevant failure paths. Reuse nearby fixtures and avoid counts of assertions, files, or coverage percentage as acceptance criteria.

Finish with the requested behavior verified and material gaps stated. A concise command/result summary is sufficient; no mandatory prompt block belongs in every PR or AGENTS.md. For further evidence and evaluation methods, consult the relevant entries in `sources.md` and `llm-eval.md`.
