# BUG-20260715-taskless-network-wake-run-unreadable: Exposed Network wake run cannot be read

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-run-bounded-live-collaboration, reconcile the durable task-run wake
- **Scenarios:** NB-run-bounded-live-collaboration; NB-agent-manages-participation
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Network usage exposed a durable `task_run_id`, but `agh task run show <id>` failed with `task id is required`. Ada could not follow the public identifier back to its terminal run even though the run itself was present and valid.

## Reproduction

- **Charter:** CH-live-bounds-agent-path · **Tour:** Interrupt Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Settle a Live Network wake and read its `task_run_id` from `agh network usage -o json`.
2. Run `agh task run show <task-run-id> -o json` or `GET /api/task-runs/:id`.

**Expected:** The public run read returns the persisted taskless `network_wake` and omits any absent Task reference.
**Actual:** The service loaded a Task unconditionally and rejected the intentionally empty `task_id`.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md`
- Live CLI and HTTP retest returned `run-1e1b1175e599c620`, session `sess-f6e4d5140f5dc947`, and result wake `wake-aca546f93f266528` without a fabricated `task` object.

## Fix

- **Root cause:** `RunDetail` used the task-backed load path for every run, despite `network_wake` explicitly requiring an empty Task ID.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `TestManagerRunDetailAggregatesRuntimeContextAndOmitsOptionalFields/Should_read_a_taskless_network_wake_without_inventing_a_task_reference`; API/CLI contract suites and generated OpenAPI co-ship the optional reference.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** CLI, HTTP, and UDS return the taskless run by ID; JSON omits `task`, while task-backed Web routes retain their explicit task requirement.
