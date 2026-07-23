---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/daemon/task_multi_group_parallel.go
line: 66
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Re-launch recovery decision path ships entirely untested

## Review Comment

The durable-side-effect re-launch recovery logic — the whole US-001.EC-6
"Uncertain-Outcome Recovery" table and ADR-008 — is implemented but has **zero
test coverage**. The following contracted test IDs from `_tests.md` are missing:
`IT-022` (pending → re-attach), `IT-023` (completed-success → report & refuse,
`--new` starts fresh), `IT-024` (completed-failure → report terminal partial
record), `IT-026` (`--new` fresh namespace), and `E2E-012` (re-issue while
active re-attaches; after completion requires `--new`). `IT-025` (plan-drift
rejection on the launch path) is only covered indirectly by the unit test
`UT-080`, never through an actual re-launch.

Verified gaps:
- `taskMultiGroupRelaunchGate` (`task_multi_group_parallel.go:66`) and the
  terminal-relaunch problems `parallel_task_groups_selection_completed` /
  `parallel_task_groups_selection_terminal` (`:120-166`) have **no** references
  in any `_test.go` file.
- `req.NewRun` / the `--new` gate bypass (`task_multi.go:129`) is never set true
  in any daemon or CLI test.
- `FindRunBySelectionFingerprint` is exercised only by a store-layer round-trip
  unit test (`internal/store/globaldb/runs_test.go`), never driven through the
  launch gate that consumes it.

Why this matters: this path creates and reports durable state (worktrees,
branches, fresh run namespaces) and is precisely the machinery that prevents a
silent duplicate launch or an overwrite of already-produced branches. The code
reads correctly (the gate is serialized under `taskGroupSelectionMu` and the
fingerprint formula matches the spec), but an entire recovery contract shipping
unverified is exactly where a future refactor regresses silently.

Suggested fix: add integration coverage that drives `StartTaskRunMultiple`
twice with an equivalent selection and asserts each recovery-table row —
active → returns the existing run id with no second worktree (IT-022);
terminal-completed → `parallel_task_groups_selection_completed` and `--new`
starts a fresh namespace without touching the completed branches (IT-023/026);
terminal-partial → `parallel_task_groups_selection_terminal` reporting
succeeded/failed/preserved paths (IT-024); and a launch-path plan-checksum drift
→ `task_group_dependencies_unmet` with `plan_changed=true` (IT-025). Add the
CLI-facing `E2E-012` journey once the daemon paths are covered.

## Triage

- Decision: `VALID`
- Root cause: the re-launch recovery decision path (`taskMultiGroupRelaunchGate`,
  `taskMultiGroupTerminalRelaunchProblem`, and the `--new` bypass at
  `task_multi.go:129`) had zero references in any `_test.go`. The store-layer
  `FindRunBySelectionFingerprint` was covered only by a globaldb round-trip, never
  driven through the gate that consumes it.
- Fix: added `TestRunManagerTaskMultiGroupParallelRelaunchRecovery` in
  `internal/daemon/task_multi_group_parallel_test.go`, driving
  `StartTaskRunMultiple` twice with an equivalent selection to assert each recovery
  row end-to-end:
  - IT-022 — an active selection re-attaches (returns the existing run id) and no
    second child is launched (`executed == len(groups)`).
  - IT-023 — a completed selection is refused with
    `parallel_task_groups_selection_completed` (`new_required=true`,
    `result_branches` reported).
  - IT-026 — `--new` mints a fresh run id and fresh branch namespace while the
    prior completed branches keep their exact SHAs.
  - IT-024 — a partial-terminal selection is reported via
    `parallel_task_groups_selection_terminal` (succeeded/failed/preserved_paths).
  - IT-025 — a re-launch after plan drift (TG-002 gains an unmet dependency edge
    in `_task_groups.md`) is rejected on the launch path with
    `task_group_dependencies_unmet`, with TG-002 in the rejected map.
- Supporting refactor (same file, test-only): the request builder was extracted to
  `taskMultiGroupRequest` (now `NewRun`-aware) and `attemptTaskMultiGroupParallelRun`,
  plus `independentTaskGroupSpec` / `writeTaskGroupPlanFile` fixture helpers.
- Scope note on `plan_changed=true`: that detail belongs to the child-start
  revalidation path (`taskGroupDependenciesProblem`, already unit-covered by
  UT-080). The parent launch path emits `task_group_dependencies_unmet` without a
  `plan_changed` flag (there is no captured "previous" preflight to compare
  against), so IT-025 asserts the code the production launch path actually
  produces rather than forcing an unrelated production change.
- Scope note on E2E-012: it is the CLI wrapper over these same daemon behaviors
  (re-attach while active; refuse + `--new` after completion). The CLI journey
  lives outside this batch's single in-scope file (`task_multi_group_parallel.go`);
  its underlying daemon contract is now covered by IT-022 and IT-023/IT-026.
- Verified: the Go gate passes — `make lint` reports `0 issues.`, the full Go
  suite passes with `-race` (`✓ internal/daemon`, including the five new IT
  subtests), and `make go-build` produces `bin/compozy` (exit 0). The only failing
  `make verify` step is `frontend-e2e`: Playwright's `global.setup.ts` cannot start
  the daemon inside this sandboxed review worktree
  (`daemon.json: no such file or directory`, daemon start exit code 2). That is a
  pre-existing environment/bootstrap limitation of the E2E harness in the isolated
  worktree, unrelated to this Go-only change (no assertion failure in any Go test).
