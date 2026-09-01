# QA Run Report — 2026-09-01 — Profile extension implementer

- **Scope:** Validate typed Loop Agent inputs through the acting Profile and exercise stock implement-tasks with a Profile-only extension Agent and Agent-local skill.
- **Cadence tier:** targeted
- **Base:** `34208e9990622ee62e9a5cf114386273ae6abfa0` · **Build:** `be2ca774e0ea4c5f1a3aa30fe73bb9110d451735` · **Environment:** isolated integration harness
- **Started:** 2026-09-01T20:00:00Z · **Status:** closed

## Session Matrix & Results

| # | Journey / Scenario | Status | Result |
|---|---|---|---|
| 1 | `LP-implement-tasks-orchestrated-mode` | Pass | Conductor succeeded; three Profile-scoped engineer workers observed the Agent-local sentinel, completed in task order, stopped, and left zero active workers |

## What Was Fixed

- **Root cause 1:** typed Loop entity validation discarded `ProfileID` and resolved only the default workspace lens.
- **Fix 1:** propagate the acting or persisted Profile through Start, DryRun, Fork, automation preflight, response annotations, and daemon entity catalogs.
- **Root cause 2:** daemon-issued exact session IDs were looked up through the ambient CLI Profile. The conductor's first `compozy me` preflight therefore returned exit 69 `session not found`; after exact-ID lookup was repaired, generic nested `session` commands still hid the spawned Profile child.
- **Fix 2:** use all-Profile lookup only for the exact daemon-issued caller Session ID, retain canonical Agent/workspace/active validation, and inherit the validated caller's Profile only for nested `session` commands. Other Agent-facing namespaces remain daemon-authenticated and unchanged.
- **Regressions:** direct real-catalog Profile isolation, service propagation, persisted response scope, exact caller identity, session-only Profile inheritance, secret-safe sandbox diagnostics, and the retained stock implement-tasks E2E.

## Retained Red Evidence

The public E2E now records the command (`/bin/sh`), stdout/stderr, exit code, conductor Session/Agent/intended Profile/Workspace identity, and the first failing step without environment or credential values. Before Fix 2 it reported first boundary `compozy me`, exit `69`, output `{"error":"session not found"}`, and no worker creation.

## Green Evidence

- The conductor preflight and canonical spawn/prompt/stop chain succeed.
- Three engineer children are created with the exact Profile and selected runtime.
- Every child observes the engineer Agent-local sentinel skill.
- Task files settle completed in the expected order; the orchestrated branch settles done and the per-task branch remains not-taken.
- All children are stopped and no worker remains in starting, active, or stopping state after settlement.

## Human Verifications Needed

None.

## Final Status

- **Exit gate:** fresh `make gate` passed; focused CLI/Loop/daemon/acpmock race tests passed; the exact public implement-tasks E2E passed repeatedly on the contribution and integrated heads.
- **Verdict:** ready pending exact-head provider CI.
