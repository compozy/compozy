---
id: RT-home-dashboard-zones
area: RT
title: Home dashboard renders seven truthful zones
persona: Cora
journey: J-operate-home-dashboard
expected: On a workspace that has recorded any work, Home window (`/`) renders pagemeta → Needs you → KPI strip → Working now | Network → Pulse → Outcomes | Usage & cost → Agents | Activity → System, each backed by `GET /api/observe/overview` plus existing status/sessions/tasks/network/agents/logs reads; empty windows show honest empty states (no invented metrics); insights with no data are omitted; a workspace with no recorded work at all renders the zero-inventory start instead of the seven zones (`RT-home-zero-inventory-first-run` owns that read); the head carries glyph + Live pill + one primary New session action and the body renders no H1. Zone queries follow the menubar Global switch: Global (`~`) uses the home-scope filter; a project workspace uses that workspace id. Toggling Global does not invent a home row in the workspace menu.
entry_points: web `/` (dashboard OS window); `GET /api/observe/overview` (HTTP+UDS)
qa_status: skipped
bug_ids: BUG-20260813-retry-leaves-blank-route
fix_status: fixed
retest_status: pass
fix_commits: a97e07f
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-project-normal.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-daemon-unavailable.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-retry-recovered.png; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: RT-home-zero-inventory-first-run
---

story: As an end user I open Home and see what agents did, what needs me, and what it costs — all from persisted daemon data.

New behavior shipped 2026-07-23 (dashboard redesign): zones replace the old daemon-status + 4-metric home. Verify zone order, truthful empties (no runs → axis-free empty chart; no attention → quiet "Nothing needs you right now"), pulse insights omission when the window is empty, and that magnitude charts stay neutral-ink while outcome bars alone use success/danger.

QA completion 2026-07-29: the isolated Home window rendered all seven zones in order from a fresh
`observe-overview/v1` read. No-attention, no-live-work, and no-outcome states were truthful; usage
showed retained tokens without inventing unavailable cost; System exposed the degraded daemon status.
The window head kept its glyph, live connection indicator, and one primary New session action, while
the dashboard body contained no H1. HTTP and UDS normalized to the same overview payload.

2026-08-12 qa-impact: Home scope follows the menubar Global switch (store `scope`), not home-dir detection. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: Home rendered all seven truthful zones for project `tmp`, then exposed a recoverable workspace error while the daemon was stopped. After the daemon returned, Retry left the route blank; filed `BUG-20260813-retry-leaves-blank-route`.

2026-08-13 fix re-walk: after `a97e07f`, the same daemon interruption retained a visible recovery boundary and Retry rendered the complete project-scoped Home dashboard after restart. The route no longer entered a blank pending shell.

2026-08-20 retry: skipped by explicit user instruction. No current behavioral verdict is claimed.

2026-08-13 Global runtime-binding re-walk: reset after the Agents zone was found to query an empty
project id while Global was active. The canonical daemon-served Home E2E now proves the overview and
all six agent rows read from the hidden operator-home runtime binding while the menubar remains
Global. `web/e2e/__tests__/dashboard.spec.ts` passed against a production Web build.

2026-08-20 qa-impact: reset by the normie-friendly UI foundation pass. The seven zones are now
conditional — `home-dashboard.tsx` routes to `HomeFirstRun` when `hasNoRecordedWork(overview)` holds,
so this file's contract gained a precondition and `RT-home-zero-inventory-first-run` took the zero
read. The zone error copy also changed ("Unable to load the home overview" / "The daemon did not
return the overview" → "Couldn't load the overview" / "Try again in a moment"), and the System zone's
`Daemon` label is now `Runtime`. Re-walk the populated path on a workspace with real work; the
`BUG-20260813-retry-leaves-blank-route` fix fields are retained history, not a fresh claim.
