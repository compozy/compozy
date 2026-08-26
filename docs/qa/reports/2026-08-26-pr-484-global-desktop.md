# QA Run Report — 2026-08-26 — PR 484 Global Desktop

- **Scope:** PR #484 remediation for first-run Global desktop, command palette, and native shortcut recovery.
- **Cadence tier:** targeted
- **Build:** `d0ee1a6ce5f400a945b6b9dbc744b1acede52c3a` plus the current reviewed working tree · **Environment:** fresh isolated daemon at `http://127.0.0.1:62605`; current-source Vite Web at `http://127.0.0.1:41739`; native behavior separately covered by packaged Electron E2E.
- **Started:** 2026-08-26T11:56:10Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-onboarding-global-skip, CH-home-zero-inventory-first-start |

## Flows in Scope

- `J-19` — finish onboarding into a usable Global desktop (`../journeys/J-19-onboarding-default-model.md`).
- `J-operate-home-dashboard` — enter Home with no work and receive honest first steps.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-onboarding-global-skip | J-19 / RT-onboarding-skip-to-global | Lea | Interrupt Tour | Pass | | working tree |
| 2 | CH-home-zero-inventory-first-start | J-operate-home-dashboard / RT-home-zero-inventory-first-run | Lea | Feature Tour | Fixed | BUG-20260826-onboarding-pulse-hides-empty-home | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-onboarding-global-skip — Lea

- **Ran:** 2026-08-26T11:59:32Z → 2026-08-26T12:03:37Z (box respected: yes)
- **Findings:** No functional or trust finding. Add/remove completed without leaving a workspace; Skip landed on a usable Global desktop.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-onboarding-skip-to-global → pass
- **Paper cuts:** none
- **Surprises:** The add/remove interruption created two legitimate workspace pulse events, so Home correctly rendered populated zones instead of the pristine zero-inventory state.
- **Suggested next charter:** CH-home-zero-inventory-first-start in a separate untouched lab.

### CH-home-zero-inventory-first-start — Lea

- **Ran:** 2026-08-26T12:05:05Z → 2026-08-26T12:08:04Z (box respected: yes)
- **Findings:** `BUG-20260826-onboarding-pulse-hides-empty-home` (Friction): onboarding-created pulse activity suppresses the first-run Home state despite no work.
- **Bugs filed/updated:** BUG-20260826-onboarding-pulse-hides-empty-home
- **Scenarios settled:** RT-home-zero-inventory-first-run → fail; fix and same-persona retest pending
- **Paper cuts:** none beyond the functional finding
- **Surprises:** The supposedly pristine onboarding path itself emits two pulse events.
- **Suggested next charter:** Re-run this charter from a fresh home after the predicate fix.

### CH-home-zero-inventory-first-start — Lea (same-persona retest)

- **Ran:** 2026-08-26T12:30:31Z → 2026-08-26T12:34:00Z (box respected: yes)
- **Findings:** Fixed. A pristine onboarding Skip opened the Global desktop, Home showed “No agent work yet” with exactly the three promised actions, and the state survived refresh.
- **Independent reads:** Public HTTP and CLI overview reads both reported zero session events and no busiest pulse bucket; HTTP and CLI workspace lists remained empty.
- **Bugs filed/updated:** BUG-20260826-onboarding-pulse-hides-empty-home → retest passed in the current working tree; commit pending.
- **Scenarios settled:** RT-home-zero-inventory-first-run → Fixed in this run; tracker commit metadata pending.
- **Evidence:** `docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-home-zero-inventory-first-start-fixed.png`; lab API/CLI summaries under `qa-artifacts/qa/`.
- **Paper cuts:** none
- **Surprises:** One discarded lab served stale JavaScript embedded in an older daemon binary. Hash comparison proved the mismatch; its browser observations were invalidated, the lab was torn down cleanly, and this retest used the required manifest-derived Vite proxy.
- **Suggested next charter:** none for this fix.

## What Was Fixed

### BUG-20260826-onboarding-pulse-hides-empty-home: First-run setup activity hides the empty Home state

- **Symptom:** A new user with no workspace or agent work saw seven zero-filled Home zones instead of one clear starting point.
- **Root cause:** The overview pulse counted global configuration events whose `session_id` is empty as agent activity.
- **Fix:** Count only session-owned events in overview pulse aggregation; current working tree, commit pending.
- **Regression test:** `internal/store/globaldb/global_db_observe_overview_test.go`, canonical `TestObserveOverviewEventAndSessionAggregates` suite.
- **Retested:** Pass under Lea after pristine onboarding, refresh, HTTP overview read, CLI overview read, and empty workspace reads.

### BUG-20260820-global-home-deleted-onboarding: Removing the suggested Home folder disables the desktop

- **Symptom:** Finishing setup without a project could leave Global unable to open desktop apps and the palette.
- **Root cause:** The Global desktop partition was not consistently recognized by the window-manager and command-palette runtime paths.
- **Fix:** Current PR #484 working tree; commit pending after final verification.
- **Regression test:** Packaged Electron E2E-027 through E2E-030 plus canonical Go and Web suites.
- **Retested:** Pending persona walk.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

No product runtime error remained in the valid retest. A prior lab accidentally exercised stale embedded Web assets; that environment-only result was discarded after bundle-hash comparison and clean teardown.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None.

## Learnings

- Browser QA against a locally built daemon must use the manifest-derived Vite proxy; the daemon's embedded Web module can lag the current working tree even when `web/dist` is current.

## Final Status

- **Exit gate (full automated suite):** pending
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** pending
- **Verdict:** pending
