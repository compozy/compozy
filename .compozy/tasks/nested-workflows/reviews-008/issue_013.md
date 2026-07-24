---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: .compozy/tasks/parallel-task-groups/_tests.md
line: 16
severity: medium
author: claude-code
provider_ref:
---

# Issue 013: Test contract IDs collide across specs, breaking parity checks

## Review Comment

Test-contract IDs (`UT-NNN`, `IT-NNN`, `E2E-NNN`) live in a flat namespace that
every spec restarts from 001, so the same ID is claimed by unrelated features:

- `IT-001` is declared by `.compozy/tasks/parallel-task-groups/_tests.md:16`
  ("Launch independent set in parallel") **and** by
  `.compozy/tasks/cy-improve-architecture/_tests.md:74`
  ("Bare install → full audit, bundled vocabulary").
- `IT-001` is correspondingly implemented in two unrelated files:
  `internal/daemon/task_multi_group_parallel_test.go` and
  `extensions/cy-capture-decisions/evals/harness_test.go`.
- `IT-032` (`parallel-task-groups/_tests.md:53`, `:178`) is implemented twice:
  `internal/daemon/task_group_completion_hydration_test.go` and
  `internal/daemon/task_group_completion_union_test.go`.

Impact: the test-contract parity gate is structurally unverifiable.
`cy-review-round` step 3 requires verifying that "every test ID assigned in
completed tasks' `## Tests` sections is implemented in the suite", and
`cy-final-verify` relies on the same traceability. A reviewer grepping `IT-001`
for `parallel-task-groups` finds `cy-capture-decisions`' eval harness test and
wrongly marks it covered — a genuinely missing test is masked by an unrelated
spec's test of the same name. The check silently passes when it should fail.

Same root cause, second symptom: `UT-035` is assigned to two different subtests
inside `internal/core/task_group_completion_worktrees_test.go` (lines 243 and
262), so even within one file the ID does not identify one case.

Fix: scope IDs per initiative (e.g. `PTG-IT-001`, `CIA-IT-001`), or record the
implementing test function name next to each ID in `_tests.md` so parity can be
checked mechanically. Update the `cy-create-techspec` tests-template so new specs
inherit the scoped scheme.

## Triage

- Decision: `INVALID`
- Notes:
  - **Target artifact absent and never tracked.** The sole code file in this
    batch's scope, `.compozy/tasks/parallel-task-groups/_tests.md`, does not
    exist in the working tree and has zero git history:
    `git log --all -- .compozy/tasks/parallel-task-groups/_tests.md` returns
    nothing, and the entire `.compozy/tasks/parallel-task-groups/` directory is
    gone (unrecoverable). The path is excluded by `.gitignore:59` (`.compozy/**`).
    There is no file to edit, and the batch scope restricts changes to exactly
    this file, so no in-scope fix is possible.
  - **`_tests.md` files are gitignored workflow scratch, not shippable code.**
    Every `_tests.md` under `.compozy/tasks/**` is a transient authoring
    artifact excluded from the repository. The "test-contract parity gate" the
    issue cites is a workflow-time check (`cy-review-round` / `cy-final-verify`
    at spec authoring), not a CI or production gate. Even if the file were
    recreated, edits would not be committed (`.compozy/**` is ignored), so the
    change would have no durable effect on the codebase.
  - **The numeric ID overlap is an accepted, documented convention.**
    Test-contract IDs are namespaced per originating spec: `IT-001` under
    `parallel-task-groups` and `IT-001` under `cy-capture-decisions` are
    distinct contract IDs within their own `_tests.md`, not a shared global key.
    Parity is verified per spec (a reviewer greps that spec's own implementing
    files), so cross-spec numeric reuse does not mask a missing test in
    practice. The committed suite already acknowledges the overlap deliberately:
    `internal/daemon/task_group_completion_union_test.go:22-40` states its IT
    IDs "collide numerically" with `task_group_completion_hydration_test.go`'s
    `IT-032` and records the resolution in place rather than renaming. This is a
    documented convention, not an unhandled defect.
  - **The recommended systemic remedy is out of repo and out of scope.** The
    proposed fix — "update the `cy-create-techspec` tests-template so new specs
    inherit the scoped scheme" — targets an external installed skill under
    `~/.claude/skills/`, which is neither in this repository nor in the batch
    scope. Renaming the IDs across the committed Go test files
    (`internal/daemon/*`, `internal/core/*`, `extensions/cy-capture-decisions/*`)
    would be a broad, behavior-neutral refactor spanning many files outside the
    single scoped file, explicitly disallowed by the batch rules ("do not
    refactor unrelated code"; keep changes within scoped files).
  - **Conclusion:** The observation about a flat ID namespace is a reasonable
    design nit for the workflow tooling, but it is unactionable in this batch —
    the target artifact does not exist, it is gitignored scratch that is never
    committed, and the only real remedy lives outside the repository. No
    production code was changed; per the remediation workflow, no commit is
    created for an all-invalid, no-code-change batch.
