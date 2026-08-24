# Conjunctive Loop runtime routing QA

- **Date:** 2026-08-24
- **Scenario:** `LP-runtime-selection-overrides`
- **Status:** `blocked-verify`
- **Evidence:** `/home/francisross/dev/qa-labs/compozy-runtime-selection-overrides-20260824-220605-045727-lab/qa-artifacts/qa/runtime-selection-blocked-verify.md`

## Evidence and corrected interpretation

CLI, HTTP, and UDS dry-runs returned the same effective ordered three-rule configuration: legacy
`type=frontend`, matrix `type=frontend + complexity=high`, and exact
`id=task_frontend_high_exact`. This proves rule-stack parity across the three public transports. A
dry-run does not prove per-item `resolved_runtime` fields or provenance.

Real CLI run `looprun-a7c7dc7d16f7ea2e` durably emitted `runtime_applied` for the matrix item with
`codex/gpt-5.6-luna/high`, but the live action did not settle. After cancellation, the daemon log
recorded `loop: transition conflict: Goal session cleanup ... payload changed`. Fresh known-model run
`looprun-1bea18dfb47059ae` did not advance past generation start during bounded polling. The evidence
therefore does not prove a settled mixed batch, its legacy single-selector item, its exact-ID item,
or complete per-field provenance. The scenario remains `blocked-verify`.

## Remaining verification

Run one live mixed batch to a settled terminal state, then inspect every item's `resolved_runtime`
and per-field provenance. The live result must show that the matrix rule applies only when both
selectors match, the legacy single-selector rule still applies, and the exact-ID rule overrides
both for each non-empty field it sets.

## Existing lab isolation and teardown

- Lab: `/home/francisross/dev/qa-labs/compozy-runtime-selection-overrides-20260824-220605-045727-lab`
- Isolated `COMPOZY_HOME`: `/tmp/compozyqa-16a842c9ca08/runtime`
- HTTP port: `38939`
- UDS socket: `/tmp/compozyqa-16a842c9ca08/runtime/compozyd.sock`
- Teardown completed at `2026-08-24T22:15:03Z`.
- `qa/teardown.json` reports `"clean": true` and `survivors: []`, and records the signal sweep for
  registered daemon PID `2452695`.
- No lab daemon, tmux server, browser, watcher, or provider process remains.

No new QA lab was started for this correction.
