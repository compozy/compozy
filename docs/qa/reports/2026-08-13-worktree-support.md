# QA Run Report — 2026-08-13 — worktree support

- **Scope:** Native Git worktree lifecycle, bound sessions, per-run task and Loop isolation, assisted exit, Forge degradation, configuration, hooks, and the adjacent workspace-add canary.
- **Cadence tier:** targeted
- **Build:** `d7869a8` + terminal review batch · **Environment:** fresh isolated macOS arm64 labs with unique runtime homes, ports, and sockets; operator `HOME` retained for native CLI provider login.
- **Started:** 2026-08-13T07:55:22Z · **Status:** PASS

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
| 1 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-cli-lifecycle | Ada | Feature | Pass | | |
| 2 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-api-surface-parity | Ada | Feature | Fixed | BUG-20260813-base-ref-accepted-before-validation | 0d54b6fe |
| 3 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-create-adopt | Ada | Feature | Fixed | BUG-20260813-pending-worktree-marked-missing; BUG-20260813-base-ref-accepted-before-validation | b6eb94d0; 0d54b6fe |
| 4 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-nested-navigation | Ada | Feature | Fixed | BUG-20260813-desktop-shell-context-order; BUG-20260813-pending-worktree-marked-missing | 8ec45d75; b6eb94d0 |
| 5 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-lifecycle | Théo | Garbage | Pass | | |
| 6 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-resume-refusal | Théo | Garbage | Pass | | |
| 7 | CH-worktree-binding-containment | J-worktree-management / RT-session-worktree-fork | Théo | Garbage | Pass | | |
| 8 | CH-worktree-binding-containment | J-worktree-management / RT-worktree-web-session-environment | Théo | Garbage | Pass | | |
| 9 | CH-worktree-binding-containment | J-worktree-management / RT-worktree-web-composer-binding-fork | Théo | Garbage | Pass | | |
| 10 | CH-worktree-bootstrap-hooks | J-worktree-management / MS-worktree-config-bootstrap | Dora | Garbage | Fixed | BUG-20260813-default-home-global-config-reclassified; BUG-20260813-worktree-config-paths-not-mutable | 2e741d9d; a216668f |
| 11 | CH-worktree-bootstrap-hooks | J-worktree-management / ET-worktree-hook-event-contract | Dora | Garbage | Pass | | |
| 12 | CH-worktree-bootstrap-hooks | J-isolated-task-loop-execution / TA-task-per-run-worktree-isolation | Dora | Garbage | Fixed | BUG-20260813-native-claim-skips-run-start | e59a03b6 |
| 13 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-commit-scope | Bruno | Interrupt | Pass | | |
| 14 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-push-publish | Bruno | Interrupt | Pass | | |
| 15 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-pr-idempotency | Bruno | Interrupt | Pass | | |
| 16 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-exit-merged-cleanup | Bruno | Interrupt | Pass | | |
| 17 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-exit-ladder | Bruno | Interrupt | Pass | | |
| 18 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-exit-progress | Bruno | Interrupt | Pass | | |
| 19 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-merged-cleanup | Bruno | Interrupt | Pass | | |
| 20 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-removal-two-step | Bruno | Interrupt | Pass | | |
| 21 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-web-missing-resolution | Bruno | Interrupt | Pass | | |
| 22 | CH-worktree-destructive-recovery | J-worktree-management / RT-worktree-reconcile-branch-safety | Bruno | Interrupt | Pass | | |
| 23 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-task-per-run-worktree-isolation | Bruno | Multi-Tab | Fixed | BUG-20260813-native-claim-skips-run-start | e59a03b6 |
| 24 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-task-fanout-worktree-isolation | Bruno | Multi-Tab | Fixed | BUG-20260813-native-claim-skips-run-start | e59a03b6 |
| 25 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-worktree-web-task-policy | Bruno | Multi-Tab | Pass | | |
| 26 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / TA-worktree-web-fanout-isolation | Bruno | Multi-Tab | Fixed | BUG-20260813-web-fanout-missing-intent-identity | 207bc4a7 |
| 27 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / LP-loop-environment-resolution | Bruno | Multi-Tab | Pass | | |
| 28 | CH-worktree-fanout-exit-removal | J-isolated-task-loop-execution / LP-worktree-web-loop-environment | Bruno | Multi-Tab | Pass | | |
| 29 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-ladder | Bruno | Multi-Tab | Pass | | |
| 30 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-commit-pr | Bruno | Multi-Tab | Fixed | BUG-20260813-worktree-exit-menu-crash | d7869a8 |
| 31 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-exit-progress | Bruno | Multi-Tab | Pass | | |
| 32 | CH-worktree-fanout-exit-removal | J-worktree-management / RT-worktree-web-removal-two-step | Bruno | Multi-Tab | Pass | | |
| 33 | CH-worktree-forge-credential-boundary | J-worktree-management / ET-worktree-forge-provider-boundary | Dora | Garbage | Pass | | |
| 34 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-exit-pr-idempotency | Dora | Garbage | Pass | | |
| 35 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-exit-browser-fallback | Dora | Garbage | Pass | | |
| 36 | CH-worktree-forge-credential-boundary | J-worktree-management / RT-worktree-web-exit-commit-pr | Dora | Garbage | Pass | | |
| 37 | CH-add-workspace-from-root | J-add-workspace-by-browsing / RT-038 | Lea | Feature | Pass | | |
| 38 | CH-add-workspace-from-root | J-add-workspace-by-browsing / MS-051 | Lea | Feature | Pass | | |
| 39 | CH-add-workspace-from-root | J-add-workspace-by-browsing / MS-web-workspace-add-directory-browser | Lea | Feature | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-worktree-lifecycle-surface-parity — Ada

- **Ran:** 2026-08-13T08:39Z → 10:02Z (box respected: yes)
- **Findings:** CLI, HTTP, UDS, native tools, catalog SSE, and browser state agreed after remediation. Create, cancel, adopt, inspect, missing-base refusal, dirty refusal, force removal, and workspace isolation were exercised against the same Git fixture.
- **Bugs filed/updated:** BUG-20260813-pending-worktree-marked-missing; BUG-20260813-base-ref-accepted-before-validation
- **Scenarios settled:** lifecycle, parity, create/adopt, and nested navigation → pass after retest

### CH-worktree-binding-containment — Théo

- **Ran:** 2026-08-13T10:03Z → 11:21Z (box respected: yes)
- **Findings:** Fresh per-run sessions kept distinct worktree bindings and Web context selection showed the exact bound checkout. Runtime and Web E2E covered fork, missing-resume refusal, cwd containment, and server-side filtering.
- **Bugs filed/updated:** BUG-20260813-native-claim-skips-run-start
- **Scenarios settled:** binding lifecycle, resume refusal, fork, composer, and session environment → pass

### CH-worktree-bootstrap-hooks — Dora

- **Ran:** 2026-08-13T08:31Z → 10:00Z (box respected: yes)
- **Findings:** The isolated daemon retained native provider login, loaded the correct global/workspace layers, applied every worktree setting live, canceled a slow setup cleanly, and kept native event/approval boundaries intact.
- **Bugs filed/updated:** BUG-20260813-default-home-global-config-reclassified; BUG-20260813-worktree-config-paths-not-mutable
- **Scenarios settled:** config/bootstrap, hooks, and per-run isolation → pass after retest

### CH-worktree-destructive-recovery — Bruno

- **Ran:** 2026-08-13T08:39Z → 10:01Z (box respected: yes)
- **Findings:** The real CLI exposed exact dirty counts, commit scope, one exit operation id, forge absence, safe cleanup evidence, and forced-removal risk. Runtime and browser E2E supplied interruption, progress replay, merged evidence, and missing restoration coverage.
- **Bugs filed/updated:** none
- **Scenarios settled:** all exit, removal, missing, and reconciliation rows → pass

### CH-worktree-fanout-exit-removal — Bruno

- **Ran:** 2026-08-13T09:55Z → 11:24Z (box respected: yes)
- **Findings:** The first Web request revealed a missing intent identity; the first real worker replay then revealed native claim/start divergence. After both fixes, Web returned two attributed results and two real workers ran to completion in distinct branches, worktrees, and sessions.
- **Bugs filed/updated:** BUG-20260813-web-fanout-missing-intent-identity; BUG-20260813-native-claim-skips-run-start
- **Scenarios settled:** task policy, fan-out, per-run, Loop environment, and shared exit/removal rows → pass after retest

### Terminal current-tree re-walk — Ada and Bruno

- **Ran:** 2026-08-13T15:10Z → 15:39Z (box respected: yes)
- **Findings:** A fresh isolated fixture confirmed nested selection, named task policy, Loop node environment, a two-run per-run fan-out, commit scope, and the zero-credential browser exit tier. Opening Git actions after the commit exposed one new route crash, fixed and re-walked in the same lab.
- **Bugs filed/updated:** BUG-20260813-worktree-exit-menu-crash
- **Scenarios settled:** all five scenarios reset by the terminal review batch → pass

### CH-worktree-forge-credential-boundary — Dora

- **Ran:** 2026-08-13T08:43Z → 10:01Z (box respected: yes)
- **Findings:** The zero-credential lab preserved local commit/cleanup actions, returned the typed forge refusal, and emitted no credential. Runtime E2E covered idempotent provider-backed request reuse and safe browser fallback.
- **Bugs filed/updated:** none
- **Scenarios settled:** forge boundary, PR idempotency, browser fallback, and Web exit dialog → pass

### CH-add-workspace-from-root — Lea

- **Ran:** 2026-08-13T08:31Z → 08:39Z (box respected: yes)
- **Findings:** The adjacent canary browsed the filesystem, registered the feature fixture once, activated it, and survived a fresh read.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-038, MS-051, and MS-web-workspace-add-directory-browser → pass

## What Was Fixed

- `BUG-20260813-default-home-global-config-reclassified` — the default operator-home workspace no longer reclassifies the operator's global config as a workspace overlay when the active runtime uses an isolated `COMPOZY_HOME`. The config and daemon owning suites now cover both same-file deduplication and distinct isolated-home topology.
- `BUG-20260813-desktop-shell-context-order` — the desktop now mounts its OS shell provider before focused-window worktree projection reads it. A browser replay reached the live desktop.
- `BUG-20260813-pending-worktree-marked-missing` — catalog reconciliation now marks only vanished `ready` checkouts as missing. A clean Web replay moved an accepted row through pending to ready and Git reported the checkout.
- `BUG-20260813-worktree-config-paths-not-mutable` — the shared CLI/native typed mutation policy now
  includes every `[worktrees]` lifecycle setting. Public set/unset applied live and browser
  cancellation removed a checkout while its configured setup command was still running.
- `BUG-20260813-base-ref-accepted-before-validation` — branch/base identity now resolves before
  pending persistence. HTTP returns the canonical refusal synchronously and the Web maps it to the
  Base ref field without leaving a phantom creation.
- `BUG-20260813-web-fanout-missing-intent-identity` — the Web owns a stable request identity for the
  current fan-out draft, reuses it on unchanged retry, and rotates it after an edit. The live
  browser request returned `201` with two distinct child keys.
- `BUG-20260813-native-claim-skips-run-start` — native worker claims now perform the run-start
  transition and hand execution to the materialized bound session. Two live workers completed in
  separate run branches and worktrees while their bootstrap sessions stayed out of task execution.
- `BUG-20260813-worktree-exit-menu-crash` — the exit menu now places its label inside the Base UI
  group it requires. The exact clean-branch menu that crashed before now opens, and the same
  worktree's HTTP plan exposes the expected zero-credential compare URL.

## Paper Cuts

| Persona | Where | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Bruno | Fan-out result | The result initially says `Worktree pending` because attribution is created at claim time, not enqueue time. | dull | watched; the run detail and worktree menu update once materialization starts |
| Dora | Zero-credential PR action | The raw CLI refusal is terse, while the Web presents the intended browser fallback. | dull | accepted; the structured error is deterministic and the Web owns operator guidance |

## Runtime Errors Observed

- The first two bootstrap attempts stopped before readiness with `gateway settings are global-only`; both labs were torn down with `clean: true` before the fix replay.
- A full-package `internal/daemon` race run timed out nine `HarnessReentryBridgeScenarios` cases under package-wide load. The complete harness suite passed in isolation with `-race` in 17.281s; the official cached gate remains the terminal authority.
- The daemon restart needed to load the worktree fix ended the development proxy's existing SSE connections. The old page retained its last pending snapshot; reloading opened fresh streams and the clean replay converged. This was kept separate from the original domain race.
- Contextual CLI config reads initially repeated the daemon's operator-home/global-file
  misclassification. The shared display/management load policy now suppresses that overlay only
  when the resolved workspace is the operator home.
- One live native-tool descriptor projection returned a transient `502` while a worker process was
  still establishing its hosted MCP bind; the bind completed milliseconds later and both workers
  proceeded. No task or binding state was lost.
- The terminal current-tree re-walk found `MenuGroupContext is missing` when opening Git actions.
  The issue was product code, not the lab: a red component regression test reproduced it before
  `d7869a8`, and the same browser journey passed after the fix.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- Native task tools are not just alternate CLI commands: the scheduler consumes them directly, so
  a lifecycle transition present only in the CLI path is a runtime defect.
- Fan-out idempotency is draft identity, not button-click identity. Keeping it in the draft store
  makes network retry safe without making later edited submissions collide.
- The strongest worktree evidence paired a public UI or CLI observable with a second public read
  and the actual Git checkout; optimistic state alone exposed both high-impact defects in this run.
- The zero-credential forge tier is a first-class path. Local exit remains useful and truthful even
  when request creation is unavailable.

## Final Status

- **Required E2E lanes:** `make test-e2e-runtime` and `make test-e2e-web` — PASS on 2026-08-13.
- **Exit gate (full automated suite):** PASS — terminal `make gate-full` on the frozen tree. Final verify evidence: `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/final-make-verify.log`.
- **Strict real-scenario audit:** PASS — the terminal strict audit accepted the behavioral evidence and final verify log with zero blockers.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/teardown.json` records `clean: true`, zero survivors, and the daemon and Web server stopped.
- **Issues by user impact:** Blocks-Completion 8 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0; all eight fixed and re-walked.
- **Coverage:** 39/39 matrix rows terminal; 33 distinct in-scope scenarios pass; no skipped or blocked rows.
- **Verdict:** PASS — ready to ship; terminal review, behavioral re-walks, strict audit, full gate, and teardown are complete.
