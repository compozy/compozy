# QA Run Report — 2026-08-13 — worktree support

- **Scope:** Native Git worktree lifecycle, bound sessions, per-run task and Loop isolation, assisted exit, Forge degradation, configuration, hooks, and the adjacent workspace-add canary.
- **Cadence tier:** targeted
- **Build:** `2989d853f6afb02dd2704fa8eed7e1f6c7706488` + Task 10 QA remediation diff · **Environment:** fresh isolated macOS arm64 labs with unique runtime homes, ports, and sockets; operator `HOME` retained for native CLI provider login.
- **Started:** 2026-08-13T07:55:22Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-worktree-lifecycle-surface-parity |
| Théo | Safety-conscious developer | desktop / wifi-fast / en-US | CH-worktree-binding-containment |
| Dora | Technical operator | desktop / wifi-fast or flaky / en-US | CH-worktree-bootstrap-hooks, CH-worktree-forge-credential-boundary |
| Bruno | Recovering operator | desktop / flaky or wifi-fast / en-US | CH-worktree-destructive-recovery, CH-worktree-fanout-exit-removal |
| Lea | New User | laptop / wifi-fast / en-US | CH-add-workspace-from-root |

## Flows in Scope

- `J-worktree-management` — create or adopt an isolated checkout, work, exit, and remove it without losing history (`../journeys/J-worktree-management.md`).
- `J-isolated-task-loop-execution` — preserve task and Loop worktree attribution from enqueue to cleanup (`../journeys/J-isolated-task-loop-execution.md`).
- `J-add-workspace-by-browsing` — register a project from a filesystem root as the adjacent workspace canary (`../journeys/J-add-workspace-by-browsing.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-cli-lifecycle | Ada | Feature | Pending | | |
| 2 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-api-surface-parity | Ada | Feature | Pending | | |
| 3 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-create-adopt | Ada | Feature | Pending | BUG-20260813-pending-worktree-marked-missing | pending |
| 4 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-nested-navigation | Ada | Feature | Pending | BUG-20260813-desktop-shell-context-order; BUG-20260813-pending-worktree-marked-missing | 8ec45d75; pending |
| 5 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-lifecycle | Théo | Garbage | Pending | | |
| 6 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-resume-refusal | Théo | Garbage | Pending | | |
| 7 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-fork | Théo | Garbage | Pending | | |
| 8 | CH-worktree-binding-containment | J-worktree-management / RT-worktree-web-session-environment | Théo | Garbage | Pending | | |
| 9 | CH-worktree-binding-containment | J-worktree-management / RT-worktree-web-composer-binding-fork | Théo | Garbage | Pending | | |
| 10 | CH-worktree-bootstrap-hooks | J-worktree-management / MS-worktree-config-bootstrap | Dora | Garbage | Pending | | |
| 11 | CH-worktree-bootstrap-hooks | J-worktree-management / ET-worktree-hook-event-contract | Dora | Garbage | Pending | | |
| 12 | CH-worktree-bootstrap-hooks | J-isolated-task-loop-execution / TA-task-per-run-worktree-isolation | Dora | Garbage | Pending | | |
| 13 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-commit-scope | Bruno | Interrupt | Pending | | |
| 14 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-push-publish | Bruno | Interrupt | Pending | | |
| 15 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-pr-idempotency | Bruno | Interrupt | Pending | | |
| 16 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-merged-cleanup | Bruno | Interrupt | Pending | | |
| 17 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-exit-ladder | Bruno | Interrupt | Pending | | |
| 18 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-exit-progress | Bruno | Interrupt | Pending | | |
| 19 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-merged-cleanup | Bruno | Interrupt | Pending | | |
| 20 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-removal-two-step | Bruno | Interrupt | Pending | | |
| 21 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-missing-resolution | Bruno | Interrupt | Pending | | |
| 22 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-reconcile-branch-safety | Bruno | Interrupt | Pending | | |
| 23 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-task-per-run-worktree-isolation | Bruno | Multi-Tab | Pending | | |
| 24 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-task-fanout-worktree-isolation | Bruno | Multi-Tab | Pending | | |
| 25 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-worktree-web-task-policy | Bruno | Multi-Tab | Pending | | |
| 26 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-worktree-web-fanout-isolation | Bruno | Multi-Tab | Pending | | |
| 27 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / LP-loop-environment-resolution | Bruno | Multi-Tab | Pending | | |
| 28 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / LP-worktree-web-loop-environment | Bruno | Multi-Tab | Pending | | |
| 29 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-ladder | Bruno | Multi-Tab | Pending | | |
| 30 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-commit-pr | Bruno | Multi-Tab | Pending | | |
| 31 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-progress | Bruno | Multi-Tab | Pending | | |
| 32 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-removal-two-step | Bruno | Multi-Tab | Pending | | |
| 33 | CH-worktree-forge-credential-boundary | J-worktree-management / ET-worktree-forge-provider-boundary | Dora | Garbage | Pending | | |
| 34 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-exit-pr-idempotency | Dora | Garbage | Pending | | |
| 35 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-exit-browser-fallback | Dora | Garbage | Pending | | |
| 36 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-web-exit-commit-pr | Dora | Garbage | Pending | | |
| 37 | CH-add-workspace-from-root | J-add-workspace-by-browsing / RT-038 | Lea | Feature | Pending | | |
| 38 | CH-add-workspace-from-root | J-add-workspace-by-browsing / MS-051 | Lea | Feature | Pending | | |
| 39 | CH-add-workspace-from-root | J-add-workspace-by-browsing / MS-web-workspace-add-directory-browser | Lea | Feature | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Pending execution.

## What Was Fixed

- `BUG-20260813-default-home-global-config-reclassified` — the default operator-home workspace no longer reclassifies the operator's global config as a workspace overlay when the active runtime uses an isolated `COMPOZY_HOME`. The config and daemon owning suites now cover both same-file deduplication and distinct isolated-home topology.
- `BUG-20260813-desktop-shell-context-order` — the desktop now mounts its OS shell provider before focused-window worktree projection reads it. A browser replay reached the live desktop.
- `BUG-20260813-pending-worktree-marked-missing` — catalog reconciliation now marks only vanished `ready` checkouts as missing. A clean Web replay moved an accepted row through pending to ready and Git reported the checkout.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- The first two bootstrap attempts stopped before readiness with `gateway settings are global-only`; both labs were torn down with `clean: true` before the fix replay.
- A full-package `internal/daemon` race run timed out nine `HarnessReentryBridgeScenarios` cases under package-wide load. The complete harness suite passed in isolation with `-race` in 17.281s; the official cached gate remains the terminal authority.
- The daemon restart needed to load the worktree fix ended the development proxy's existing SSE connections. The old page retained its last pending snapshot; reloading opened fresh streams and the clean replay converged. This was kept separate from the original domain race.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

Pending execution.

## Final Status

Pending the terminal matrix, full gate, strict audit, and clean teardown.
