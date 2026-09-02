# QA Run Report — 2026-09-01 — Window manager review fixes

- **Scope:** Review remediation for structural zoom recovery, captured stale-gesture guards, conflict recovery, solo-frame zoom state, and the Layouts canvas summary.
- **Cadence tier:** targeted
- **Build:** working tree on `0f651ebf5` · **Environment:** fresh isolated targeted lab `compozy-window-manager-review-fixes-20260902-015422-962467-lab`
- **Started:** 2026-09-02T01:53:20Z · **Completed:** 2026-09-02T02:09:44Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Builder | desktop / wifi-fast / en-US | CH-window-manager-review-fixes |

## Flows in Scope

- `J-administer-window-manager` — tune window behavior without partial state (`../journeys/J-administer-window-manager.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-window-manager-review-fixes | J-administer-window-manager / ET-window-zoom-in-place | Bruno | Feature Tour | Pass | | current remediation commit |
| 2 | CH-window-manager-review-fixes | J-administer-window-manager / ET-window-manager-layout-gestures | Bruno | Feature Tour | Pass | | current remediation commit |
| 3 | CH-window-manager-review-fixes | J-administer-window-manager / ET-layout-editor-board-summary | Bruno | Feature Tour | Pass | | current remediation commit |
| 4 | CH-window-manager-review-fixes | J-administer-window-manager / RT-desktop-pager-overview | Bruno | Feature Tour | Pass | adjacent canary | current remediation commit |

## Session Debriefs

- Solo zoom filled one desktop, set the traffic-light `aria-pressed` state, and appeared as `zoomed=true` through the CLI.
- Opening a separate Settings frame ended zoom before the peer became visible. Edge snap, lift to Desktop 2, pager growth, and exact unzoom return all passed.
- A minimized lifted zoom retained Desktop 2 and `return_anchor.zoomed=true`; restore returned the window zoomed to Desktop 2.
- Settings > Layouts reported the selected desktop's live tiled, floating, zoomed, and reference counts. The focused component regression proves a two-zoom draft renders `2 zoomed` rather than the former constant.
- Stale free-drop and top-center zoom guards are covered by the focused interaction regressions, which mutate the window's current node after gesture capture.

## What Was Fixed

The remediation preserves zoom teardown mutations, enforces one visible zoom unit per desktop, chooses an unoccupied restore desktop after unzoom, retains lifted desktops needed by minimized zoom windows, carries captured gesture rebases through queued commands, and keeps conflict recovery active after an unsuccessful refresh.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

The prior zoom walk's A2 step expects a separately opened peer to coexist with zoom until a later head drop. That setup is obsolete under the full-desktop invariant, so it was excluded rather than treated as a product defect; direct stack insertion is verified in the owning Go suite. The strict evidence audit passed, and `qa/teardown.json` records `clean: true` with no survivors.

## Final Status

- **Exit gate (full automated suite):** `make gate` pass — codegen check, zero-issue Go lint, scoped Go race tests, and affected Turbo lanes
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked
- **Verdict:** ready — targeted product walk, strict evidence audit, clean teardown, and local delivery gate pass.
