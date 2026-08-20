# QA Run Report — 2026-08-20 — Normie-friendly UI

- **Scope:** Targeted living-QA cycle for the normie-friendly UI foundation pass: first-run Home, populated Home, desktop shell scale and navigation, settings aliases, onboarding, transcript, decision dock, and empty-catalog canary.
- **Cadence tier:** targeted
- **Build:** working tree over `origin/main` · **Environment:** fresh isolated lab at `http://127.0.0.1:62884`; manifest-derived runtime, Web proxy, CLI, and HTTP surfaces; normal agent-browser profile. Safari, Firefox, mobile devices, screen-reader speech output, extensions, and network throttling are parity boundaries unless the environment exposes them during execution.
- **Started:** 2026-08-20T17:47:00-03:00 · **Status:** closed
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-20260820-174700-967041-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Automated precondition:** `rtk make gate` ran exactly once and failed after full-gate escalation because `COPY.md` and `PRODUCT.md` are not formatted. Those existing files are outside this QA worker's ownership and were left untouched.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-home-zero-inventory-first-start, CH-untested-019-19-lea |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-untested-069-operate-home-dashboard-cora |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-plain-scale-legibility |
| Théo | Session Operator | desktop / wifi-fast / en-US | CH-session-permission-dock, CH-session-calm-transcript |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-untested-068-operate-desktop-shell-bruno, CH-empty-catalog-first-use |
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-untested-041-administer-runtime-settings-dora |

## Flows in Scope

- `J-operate-home-dashboard` — start honestly from zero, then read real work in seven zones (`../journeys/J-operate-home-dashboard.md`)
- `J-operate-desktop-shell` — reach and operate apps through the desktop registry (`../journeys/J-operate-desktop-shell.md`)
- `J-19` — complete first-run setup over the inert shell (`../journeys/J-19.md`)
- `J-administer-runtime-settings` — find and apply runtime settings (`../journeys/J-administer-runtime-settings.md`)
- `J-14` — supervise a calm, truthful session transcript (`../journeys/J-14.md`)
- `J-answer-agent-requests` — decide permissions and clarifications at the composer (`../journeys/J-answer-agent-requests.md`)
- `J-start-from-empty-catalogs` — reach real next actions from empty catalogs (`../journeys/J-start-from-empty-catalogs.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-home-zero-inventory-first-start | J-operate-home-dashboard / RT-home-zero-inventory-first-run | Lea | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 2 | CH-untested-019-19-lea | J-19 / RT-onboarding-setup-panel-over-shell | Lea | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 3 | CH-plain-scale-legibility | J-operate-desktop-shell / ET-web-geist-wght-medium-510 | Sol | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 4 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-catalog-navigation; ET-web-dock-default-window-size | Bruno | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 5 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-settings-takeover-redesign | Dora | Back-Button Tour | Skipped | Automated precondition failed before session launch. | |
| 6 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 7 | CH-session-permission-dock | J-answer-agent-requests / ET-web-session-permission-dock | Théo | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 8 | CH-untested-069-operate-home-dashboard-cora | J-operate-home-dashboard / RT-home-dashboard-zones | Cora | Feature Tour | Skipped | Automated precondition failed before session launch. | |
| 9 | CH-empty-catalog-first-use | J-start-from-empty-catalogs / TA-web-tasks-zero-inventory-templates; TA-web-jobs-zero-inventory-suggestions; TA-web-triggers-zero-inventory-intro | Bruno | Feature Tour | Skipped | Automated precondition failed before session launch. | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

No session started. The mandatory automated precondition failed before any runtime, Web, or browser process was launched. All scenario files remain `untested`; no historical verdict was changed.

## Experiential Lens Pass

Not run because the automated precondition failed before session launch.

## What Was Fixed

None. No product behavior was exercised.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- Automated precondition: `make gate` failed in BunLint because `COPY.md` and `PRODUCT.md` need formatting. This is pre-existing worktree state, not a browser-session finding.

## Human Verifications Needed

None. The blocked boundary is automated, not a human-only product leg.

## Decisions for a Human

None. No finding entered the fix-loop governor.

## Learnings

- A fresh isolated lab can be allocated safely without starting any owned process; a red automated precondition still ends the behavior-first run before launch.

## Final Status

- **Exit gate (this QA phase):** `rtk make gate` — failed; `make gate-full` intentionally deferred until after deep-review remediation and was not run.
- **Strict evidence audit:** FAIL (exit 2) — C7 missing Web, CLI, and API journey-log evidence because no session started; C12 missing final verification report; C14 missing final `make verify` evidence. Reports: `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-20260820-174700-967041-lab/qa-artifacts/qa/qa-audit-report.{json,md}`.
- **Parity gaps:** No behavior evidence was collected. Chrome/agent-browser, 1280/768 captures, keyboard and screen-reader-readable checks, Safari/Firefox/mobile, reduced motion, live provider behavior, refresh/deep-link confirmation, and the Home/desktop six-lens pass all remain uncovered.
- **Teardown:** PASS — `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-20260820-174700-967041-lab/qa-artifacts/qa/teardown.json` records `clean: true`, no survivors, no listener on port 62884, and no registered PID files.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/9 matrix sessions walked; 9/9 rows closed as Skipped; 0/9 flagged scenarios settled; adjacent canary not walked.
- **Verdict:** not ready — the mandatory automated precondition is red, so behavior-first QA did not start.
