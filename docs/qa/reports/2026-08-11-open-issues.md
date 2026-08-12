# QA Run Report — 2026-08-11 — open issues

- **Scope:** Targeted branch validation for issues #341, #342, #344, #345, and #346: runtime-selector scroll containment, filesystem-root discovery, durable session rename, desktop page zoom, and floating-window visual ordering.
- **Cadence tier:** targeted
- **Build:** `ab319b53` + working tree · **Environment:** isolated lab `compozy-open-issues-20260812-002435-338441`
- **Started:** 2026-08-12T00:22:30Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-runtime-selector-scroll-boundary |
| Lea | New User | laptop / wifi-fast / en-US | CH-add-workspace-from-root |
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-rename-session-parity, CH-desktop-page-zoom |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-window-layer-seam |

## Flows in Scope

- `J-17` — launch a session, then choose its next-prompt runtime (`../journeys/J-17-session-create-unified-selector.md`)
- `J-add-workspace-by-browsing` — register a project from any local root (`../journeys/J-add-workspace-by-browsing.md`)
- `J-rename-session` — change a durable display name without changing session work (`../journeys/J-rename-session.md`)
- `J-administer-window-manager` — preserve structural behavior while changing presentation (`../journeys/J-administer-window-manager.md`)
- `J-desktop-attach-daily` — operate the native window on the owned runtime (`../journeys/J-desktop-attach-daily.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-runtime-selector-scroll-boundary | J-17 / RT-068 | Sol | Feature Tour | Pass | #341 | working tree |
| 2 | CH-add-workspace-from-root | J-add-workspace-by-browsing / RT-038; MS-051; MS-web-workspace-add-directory-browser | Lea | Feature Tour | Pass | #342 | working tree |
| 3 | CH-rename-session-parity | J-rename-session / RT-session-rename-durable | Dora | Feature Tour | Pass | #344 | working tree |
| 4 | CH-window-layer-seam | J-administer-window-manager / ET-window-manager-layout-gestures | Bruno | Feature Tour | Pass | #346 | working tree |
| 5 | CH-desktop-page-zoom | J-desktop-attach-daily / APP-desktop-page-zoom | Dora | Feature Tour | Pass | #345 | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Runtime selector:** the live list had 19,740px of content in a 328px scroll owner. It reached the 19,412px lower boundary; a second wheel gesture stayed there and left `document.scrollY` at `0`.
- **Filesystem roots:** `GET /api/fs/browse` returned `roots: ["/"]` with the requested home entries. The Add workspace dialog exposed `/` under Locations, browsed back to the isolated project, and registered it once.
- **Session rename:** Web, `compozy session rename`, and workspace-scoped HTTP PATCH renamed the same stopped user session. After a daemon restart, the catalog still returned the final name with the original session id, stopped state, and workspace id.
- **Window layers:** a real four-window grid rendered tiled windows at layer `1` and its shared seams at layer `2`; floating the active Tasks window raised it to layer `7`, keeping it above the seam without letting focused tiled windows cover resize targets.
- **Desktop page zoom:** the native Tauri main window accepted Command-plus, Command-minus, and Command-zero. Captures show whole-page enlargement, reduction, and exact reset while the in-product window layout remained intact.

## What Was Fixed

No QA-discovered follow-up fixes were needed.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

None recorded yet.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded yet.

## Learnings

- The filesystem root belongs to the daemon response instead of browser guesses, so HTTP, UDS, Web, and onboarding share one operating-system view.
- The session name is workspace-scoped data. Restart evidence preserved the name without changing identity or ownership.
- Window layers must encode semantics: tiled content stays below seams, and floating content stays above both.
- Network throttling was skipped because none of these fixes changes request timing, retries, or offline recovery.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate-full` completed its full `make verify` lane; 21,519 Go tests passed with 2 platform skips, plus codegen, installer, product-language, zero-warning lint, typecheck, frontend tests/builds, Go build, and boundaries. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/final-make-verify.log`.
- **Issues by user impact:** 0 critical · 0 major · 0 minor · 0 paper cuts
- **Coverage:** 5/5 journeys walked; required `web`, `cli`, `api`, and `runtime` surfaces recorded in `journey-log.jsonl`
- **Verdict:** PASS
