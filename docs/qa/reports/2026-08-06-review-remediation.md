# QA Run Report — 2026-08-06 — Review remediation

- **Scope:** Fresh-frame window drag grouping plus an adjacent desktop-shell reload canary.
- **Cadence tier:** targeted
- **Build:** working tree after `f8afbc10` · **Environment:** isolated lab `compozy-review-remediation-20260806-140818-700665-lab`, daemon `:52234`, Web `:3000`
- **Started:** 2026-08-06T11:07:48-03:00 · **Status:** walks complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | fresh isolated workspace | desktop / wifi-fast / en-US | CH-window-tabs-keyboard-flow |
| Cora | fresh isolated workspace | laptop / wifi-fast / en-US | CH-window-tabs-home-canary |

## Flows in Scope

- `J-organize-tabbed-work` — group related windows using the frame committed by the current render.
- `J-operate-home-dashboard` — adjacent shell canary after grouping and reload.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-window-tabs-keyboard-flow | J-organize-tabbed-work / ET-window-tab-deck-lifecycle | Bruno | Feature Tour | Pass | | working tree |
| 2 | CH-window-tabs-home-canary | J-operate-home-dashboard / RT-home-usage-window-persistence | Cora | Feature Tour | Pass | | not applicable |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-window-tabs-keyboard-flow — Bruno

- **Ran:** 2026-08-06T11:10:00-03:00 → 2026-08-06T11:16:00-03:00 (box respected: yes)
- **Findings:** No product failure. Moving Settings exposed the Tasks head; dragging back showed “Group as tabs” and release created one ordered frame. Reload retained Tasks + Settings.
- **Bugs filed/updated:** none.
- **Scenarios settled:** ET-window-tab-deck-lifecycle → pass.
- **Paper cuts:** none.
- **Surprises:** The browser harness exposes `capture_screenshot`, not `screenshot`; the first capture call failed after the affordance was observed, but its `finally` block released the pointer and the goal-state captures succeeded.
- **Suggested next charter:** none for this remediation.

Edge probes were clean: full reload, repeated dock activation, a 1280×800 viewport, reduced motion,
and switching to an adjacent Home window. Evidence:
`docs/qa/evidence/2026-08-06-review-remediation/bruno-grouped-deck.png` and
`docs/qa/evidence/2026-08-06-review-remediation/bruno-grouped-deck-reload.png`.

### CH-window-tabs-home-canary — Cora

- **Ran:** 2026-08-06T11:16:00-03:00 → 2026-08-06T11:18:00-03:00 (box respected: yes)
- **Findings:** none.
- **Bugs filed/updated:** none.
- **Scenarios settled:** RT-home-usage-window-persistence remained pass as an unchanged adjacent canary.
- **Paper cuts:** none.
- **Surprises:** none.
- **Suggested next charter:** none for this remediation.

Cora selected 90d, expanded System, and reloaded. `Usage · 90d`, the expanded daemon facts, and
`cost unavailable` all persisted. Evidence:
`docs/qa/evidence/2026-08-06-review-remediation/cora-home-canary-reload.png`.

## What Was Fixed

- Animation-frame callbacks and merge-target frame reads now refresh during the layout phase before coordinator callbacks can observe them.

## Paper Cuts

None recorded.

## Runtime Errors Observed

No product errors were observed. The first strict evidence audit correctly remained blocked on the
not-yet-run final repository gate; it will be repeated after `make gate-full` creates final evidence.

## Human Verifications Needed

None expected.

## Decisions for a Human

None expected.

## Learnings

- Layout-phase ref updates prevent the coordinator from observing a callback or frame from the
  previous commit during the browser's pre-paint window.

## Final Status

- **Exit gate (full automated suite):** pending rebase and final `make gate-full`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 targeted journeys walked
- **Verdict:** behavior passes; repository readiness remains pending rebase, final gate, and strict evidence re-audit
- **Teardown:** pass — `/Users/pedronauck/dev/qa-labs/compozy-review-remediation-20260806-140818-700665-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with zero survivors.
