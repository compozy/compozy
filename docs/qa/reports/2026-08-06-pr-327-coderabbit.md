# QA Run Report — 2026-08-06 — PR 327 CodeRabbit

- **Scope:** PR 327 remediation for session route reconciliation, hierarchy filtering/cycles, and collapsed-list focus containment
- **Cadence tier:** targeted
- **Build:** `31c4b7b2` plus the frozen remediation working tree · **Environment:** isolated daemon on `127.0.0.1:58025`, Vite on `localhost:3000`, normal-profile Chrome via browser-use
- **Started:** 2026-08-06T22:39:44Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-sidebar-navigation, CH-session-catalog-navigation |

## Flows in Scope

- `J-14` — audit and navigate finished session work (`../journeys/J-14-read-a-finished-transcript.md`)
- `J-operate-desktop-shell` — reach sessions through coherent shell entry points (`../journeys/J-operate-desktop-shell.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-sidebar-navigation | J-14 / ET-web-session-sidebar-threads | Bruno | Feature Tour | Pass | | |
| 2 | CH-session-catalog-navigation | J-operate-desktop-shell / ET-web-sessions-catalog-modal | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-sidebar-navigation — Bruno

- **Ran:** 2026-08-06T22:47:00Z → 2026-08-06T22:53:00Z (box respected: yes)
- **Findings:** none. A live root → child → grandchild thread preserved its full ancestor path under a grandchild-only filter. The root toggle stayed keyboard-reachable while collapsed child controls left the accessibility tree. The final Lucide List glyph rendered with truthful open-state label and `aria-pressed` semantics.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-web-session-sidebar-threads → pass
- **Paper cuts:** none
- **Surprises:** the exact stale child-route branch cannot be authored through the current public session UI; its red-before/green-after coordinator regression is the proof for that boundary.
- **Suggested next charter:** none for this remediation

### CH-session-catalog-navigation — Bruno

- **Ran:** 2026-08-06T22:44:00Z → 2026-08-06T22:47:00Z (box respected: yes)
- **Findings:** none. Dock Sessions showed four live sessions, filtering for the grandchild retained the root and intermediate parent, and collapsed thread/group bodies were inert while their toggles remained reachable.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-web-sessions-catalog-modal → pass
- **Paper cuts:** none
- **Surprises:** malformed cyclic lineage is not constructible through the public CLI or Web surfaces; the canonical hierarchy regression owns that anomalous-state boundary.
- **Suggested next charter:** none for this remediation

## What Was Fixed

- Session route reconciliation, cyclic lineage visibility, complete filtered ancestry, and collapsed-content focus containment were fixed before this QA walk. Their canonical regressions failed before the fixes and pass on the current build.
- Window-manager validation now rejects instance retargeting on pop navigation. HTTP and UDS root-filter parity, the existing topbar toggle, and both topbar toggle states are covered in their owning suites.
- The session-window sidebar toggle now uses Lucide List, matching the list it reveals without suggesting a terminal surface.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A malformed lineage cycle needs automated boundary coverage because legitimate public creation cannot write one.
- Production parity deviations: Chrome only, desktop 1920×963, wifi-fast, local Vite rather than a packaged Web artifact, and no Safari/Firefox/mobile/network-throttle sweep. The real daemon, real SQLite store, public CLI setup, and public Web surfaces were used without mocks.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — PASS for the final content fingerprint; current evidence confirmed by `make gate-status`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 journeys walked; the exact malformed-cycle and stale-child-route boundaries were confirmed by red-before/green-after canonical regressions
- **Verdict:** ready — both changed Web journeys passed with live evidence and no open finding.
