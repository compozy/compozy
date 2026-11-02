---
id: RT-home-dashboard-zones
area: RT
title: Home dashboard renders seven truthful zones
persona: Cora
journey: J-operate-home-dashboard
expected: On a workspace that has recorded any work, Home window (`/`) renders pagemeta → Needs you → KPI strip → Working now → Pulse → Outcomes | Usage & cost → Agents | Activity → System, each backed by `GET /api/observe/overview` plus existing status/sessions/tasks/agents/logs reads; empty windows show honest empty states (no invented metrics); insights with no data are omitted; a workspace with no recorded work at all renders the zero-inventory start instead of the seven zones (`RT-home-zero-inventory-first-run` owns that read); the head carries glyph + Live pill + one primary New session action and the body renders no H1. Zone queries follow the menubar destination: under All projects the header reads "All projects", the overview reads the explicit aggregate (`all_workspaces=true`, never the overview omission, which is User's tasks only) and Working now lists every project's live sessions and task runs (`all_workspaces=true`) with a project badge on each card ("User" for work outside a project) that opens the item in place on the All projects desktop; User reads `@user`; a project uses that workspace id. Toggling All projects does not invent a home row in the workspace menu.
entry_points: web `/` (dashboard OS window); `GET /api/observe/overview` (HTTP+UDS)
qa_status: untested
bug_ids: BUG-20260813-retry-leaves-blank-route
fix_status: fixed
retest_status:
fix_commits: a97e07f
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-project-normal.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-daemon-unavailable.png; /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-retry-recovered.png; docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-10-02-untested.md
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


2026-09-10 issue 606: Needs you shows unread notifications for the selected workspace/global and
profile lens. Mark as read removes one occurrence without Retry/Approve/Reject/Open. Clear all
includes every current occurrence, even beyond the 12-row display and inbox page limits. Verify
bell/Home receipt consistency, exact counts, reload, failed acknowledgement, and a new failed run
reappearing. Source task decisions remain available in Open inbox. Local QA is deferred by explicit
user instruction; the existing SQLite/overview tests and CI own validation for this change.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA impact 2026-10-07 (memory removal): the System zone renders five tiles (daemon, providers, scheduler, hooks, retention); the Memory tile and `status.memory` were removed, and Activity no longer folds `memory.*` events as quiet (historical `memory.*` rows still within retention render as ordinary rows). The skip above still stands; the next cycle that walks this scenario should expect five tiles.

2026-10-21 qa-impact (all-projects AW2, Tasks + Home): **reset to `untested`**. Under All projects
Home now asks for the aggregates explicitly — `GET /api/observe/overview?all_workspaces=true` and
`GET /api/observe/tasks/dashboard?all_workspaces=true` — instead of the omission that only read User
tasks. Walk with two projects plus User work: the header reads "All projects", the KPI strip and
Needs you count every project, Working now badges each session and task-run card with its project (or
User), and opening a card keeps All projects on and shows the project badge in the window chrome.
Repeat in project mode (no badges, that project only) and with User focused (`@user`). The Needs you
items carry no project in the overview payload, so their Open lands unbound on the All projects desktop.

QA impact 2026-11-01 (responsiveness audit): while Home observes task lifecycle activity,
exercise a burst with aggregate responses still pending. The first event must start a read
immediately; later events must produce one trailing read after it settles, with final attention,
outcome, task-dashboard and inbox data matching the latest event. A failed leading response must
not discard that trailing read. Switch profiles away and back while reads are pending, then close
Home; old stream callbacks must not change the replacement view and queued aggregate changes
must be reconciled after the leading read settles. Repeat closure with disabled observers and
without initial query data; disabled data must remain invalidated until re-enabled. A pending
old scope must settle through its original QueryClient without consuming a replacement scope's
trailing refresh. Connectivity-only reads use runtime identity; System still reads full status.
Focused evidence: dashboard stream/model suites and the reproducible QueryClient benchmark in
`web/e2e/scripts/benchmark-home-live.ts`; these do not substitute for the isolated browser/daemon
walk. See `../reports/2026-11-01-performance/usage-dashboard.md` for current verification status.
