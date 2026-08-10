# QA Run Report — 2026-08-10 — loop-browser-runtime-closeout

- **Scope:** Browser closeout for Loop convergence: deck-target ownership/performance and truthful installed Marketplace management
- **Cadence tier:** targeted
- **Build:** `18187504-dirty` · **Environment:** `http://localhost:3002`, fresh isolated daemon, real Web build, no provider-backed leg
- **Started:** `2026-08-10T09:32:13-03:00` · **Status:** behavior complete; final verification pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-window-tabs-keyboard-flow, CH-marketplace-installed-default |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-window-tabs-home-canary |

## Flows in Scope

- `J-organize-tabbed-work` — Group and recover related work without losing topology (`../journeys/J-organize-tabbed-work.md`)
- `J-marketplace-acquisition` — Manage installed capabilities from daemon truth (`../journeys/J-marketplace-acquisition.md`)
- `J-operate-home-dashboard` — Adjacent canary for shell persistence (`../journeys/J-operate-home-dashboard.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-window-tabs-keyboard-flow | J-organize-tabbed-work / ET-window-tab-deck-lifecycle | Bruno | Feature Tour | Pass | | |
| 2 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-marketplace-installed-management | Bruno | Back-Button Tour | Blocked (catalog precondition) | | |
| 3 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-extension-detail | Bruno | Back-Button Tour | Blocked (catalog precondition) | | |
| 4 | CH-window-tabs-home-canary | J-operate-home-dashboard / RT-home-usage-window-persistence | Cora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (catalog precondition) | Blocked (human decision)`

## Session Debriefs

- Bruno formed a Home + Tasks deck, saw and cancelled a live Marketplace insertion preview, then
  committed the insertion. Tasks, Skills, and Home persisted as one ordered deck after reload.
- Bruno toggled the bundled `compozy` skill off, confirmed `Disabled` across reload, and restored it.
  He repeated the lifecycle for `dev-cycle`, inspected its healthy runtime and verified official
  trust, and installed Playwright into the exact workspace MCP scope.
- Cora selected 90d and expanded System after the deck/Marketplace activity. Both preferences
  remained selected after a full reload.

## What Was Fixed

No in-session fixes.

## Paper Cuts

None.

## Runtime Errors Observed

No Web page errors or product console errors. The console contained only the React development-tools
information message.

## Human Verifications Needed

The extension update confirmation still needs a manual walk when an official installed extension
has a newer catalog generation. Both real installed extensions reported `update_available: false`;
fabricating update state would invalidate this behavioral session.

## Decisions for a Human

None.

## Learnings

- The merge target remains stable through preview cancellation, commit, and reload with three tabs.
- Installed action labels now follow daemon state immediately and after remount.
- Truthful absence matters: the UI omitted Update when the daemon had no candidate and the MCP detail
  reported runtime unavailable instead of inventing discovered tools.

## Final Status

- **Exit gate (full automated suite):** Pending final `make gate-full` after QA write-back.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 journeys walked; 4/4 matrix rows terminal (2 pass, 2 blocked-verify).
- **Verdict:** behavior ready with honest catalog-precondition blocks; final readiness depends on
  the strict evidence audit, clean teardown, and the full monorepo gate.
