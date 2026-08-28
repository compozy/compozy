# QA Run Report — 2026-08-27 — session-empty-dock

- **Scope:** Session delete keeps the window on /sessions; Sessions dock opens last created catalog row
- **Cadence tier:** targeted
- **Build:** working tree daemon binary plus make web-dev · **Environment:** isolated QA lab, HTTP 51278, web localhost:3000
- **Started:** 2026-08-27T16:56:00-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | returning operator | desktop / wifi-fast / en-US | CH-session-empty-and-dock-last-created |
| Bruno | builder | desktop / wifi-fast / en-US | CH-session-empty-and-dock-last-created |

## Flows in Scope

- `J-15` — permanently delete a session (`../journeys/J-15.md`)
- `J-operate-desktop-shell` — dock launch (`../journeys/J-operate-desktop-shell.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-empty-and-dock-last-created | J-15 / RT-014 | Théo | Feature Tour | Pass | | |
| 2 | CH-session-empty-and-dock-last-created | J-15 / RT-session-delete-keeps-empty-tab | Théo | Feature Tour | Pass | | |
| 3 | CH-session-empty-and-dock-last-created | J-operate-desktop-shell / ET-web-dock-contextual-session-launch | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-empty-and-dock-last-created — Théo / Bruno

- **Ran:** 2026-08-27T17:06Z → 2026-08-27T17:16Z (box respected: yes)
- **Findings:** none that failed the contract
- **Bugs filed/updated:** BUG-20260826-session-delete-return-race remains fixed; the bounce-to-agent path is gone
- **Scenarios settled:** RT-014 → pass; RT-session-delete-keeps-empty-tab → pass; ET-web-dock-contextual-session-launch → pass
- **Paper cuts:** first-run setup still blocks a cold lab; after CLI create the dock needed a reload before the new row appeared in the web catalog (dull)
- **Surprises:** leftover empty /sessions windows from HTTP/UDS deletes stayed on the desktop; dock still opened the last created session instead of focusing those empties
- **Suggested next charter:** row-menu delete from an empty-tab sidebar when several live sessions remain

## What Was Fixed

Delete now retires the matching session tab onto /sessions instead of closing it or opening Agents. The Sessions dock keys off catalog emptiness and created_at.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Bruno | J-operate-desktop-shell after CLI session new | "I created a session in the terminal and the dock still thought the catalog was empty until I refreshed." | dull | watching |

## Runtime Errors Observed

- Isolated daemon logged missing provider on bundled spec-cycle agents until acpmock was configured. Not user-visible after first-run setup.
- No unexpected HTTP 5xx during the walks.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Evidence

- Lab journey log: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/journey-log.jsonl`
- HTTP: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/logs/http-delete.txt`
- Runtime windows: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/logs/http-windows-after.json`
- UDS: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/logs/uds-delete.txt`
- Web independent read: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/logs/web-delete-independent.txt`
- Screenshots: `docs/qa/evidence/2026-08-27-session-empty-dock/`
- Lab verification report: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/verification-report.md`
- Local gate log: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/logs/make-verify.log`
- Teardown: `/Users/pedronauck/dev/qa-labs/compozy-session-empty-dock-20260827-165738-773687-lab/qa-artifacts/qa/teardown.json` (`clean`: true)

## Compozy Impact Audit

- Native tools: no composy__ tool IDs. Window-manager session-delete origin now navigates to /sessions instead of closing.
- Extensibility and hooks: session catalog, dock launcher, WM reconciliation. No config.toml keys. CommandID is window.navigate so hooks do not fire window.closed for this retire.
- Workspace data isolation: catalog pick and window retarget stayed on workspace ws_d0816f751fd36eb1. Deletion only mutated that workspace.
- Official Compozy skill: skills/compozy/ has no session-delete or dock-launch copy that needed an update.

## Final Status

PASS — three of three matrix rows passed on HTTP, UDS/runtime, and Web.

Local scoped `make gate` passed (go-lint, go-test, js-desktop, js-web; exit 0). Lab teardown completed with `clean`: true and no survivors. Auditor C14 passed after `qa/logs/make-verify.log`.
