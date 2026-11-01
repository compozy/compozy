# Stays-on agents QA walk

Date: 2026-11-01. Branch: `feat/stays-on-qa`. Evidence commit: `a64b1c981fa4`.
Evidence table: `.compozy/tasks/always-on-agent/qa/pr-evidence.md` (13 screens, light and dark).

## Lab

Isolated `COMPOZY_HOME` seeded with `scripts/demo-seed`, a daemon built from this branch, the built web
bundle, and Playwright Chromium at 1440x900. `[providers.claude] command` pointed at `acpmock-driver`, so
no real provider ran. Triage, relayed asks and reports used the real daemon paths with scripted replies.
Teardown reported `TEARDOWN_ALL_CLEAN=true`, with no process or port listener left behind.

## Journeys

| # | Journey | Result |
|---|---|---|
| 1 | Create an agent, Stays on and On demand | Pass |
| 2 | Agent profile, where it works, what it does, memory, Check-ins, Activity | Pass for agents with a project; fails for Global-only and On demand agents, see Open defects |
| 3 | Add to a project, Global, pause, resume, archive, cap | Pass. Global was created from the create screen and offered in the picker. Remove from a project is the archive action |
| 4 | Sidebar row with triangle, Kanban avatar row, Dashboard first | Pass |
| 5 | Report entry, While you were away, relayed approval, failed answer | Pass |
| 6 | No HEARTBEAT.md reads all day with all five sources | Pass for a project agent; a Global-only agent can't open the page |

## Defects fixed

Each fix has a focused test except the hover-card plural, a copy change verified in the browser.

- `f7a8bbf3d` fix: hide stays-on main sessions from the session catalog list
- `4a6fef558` fix: accept the Global scope on agent instance routes
- `2e2487068` fix: keep a cleanly stopped main session out of the failed state
- `11e90692f` fix: run checks for stays-on agents defined outside the workspace
- `2f917de93` fix: let the prompt harness accept triage sessions
- `9d9147a9f` fix: open a cleanly stopped main session instead of a startup failure
- `10183640d` test: repair stopped main session thread case
- `301b10320` fix: render stays-on reports in the main session transcript
- `f5d9906aa` fix: open agent editors in the agent's project under All projects
- `1300ed91f` fix: read check-ins through the daemon contract
- `1a9b61852` fix: keep Add to a project in the Where it works topbar
- `7fc9e2389` fix: deliver reports with prompts on idle main sessions
- `6b13d0dec` fix: count quiet check-ins in the away read
- `e53e4593a` fix: stop triage runs from waking the main session
- `11042786b` fix: phrase a short absence in the away digest
- `b280406e0` fix: label a skipped check-in as skipped
- `a64b1c981` fix: show agent labels in Dashboard live work

Test repair: the stopped-session fix was first committed with a failing new test and a
failing typecheck. The following test commit repairs it; both were green before the walk continued.

## Open defects

1. **Global-only and On demand agents can't open What it does or Check-ins under All projects.** The page
   shows "Couldn't load this page". Soul and heartbeat authoring require a workspace ID. Agents with a
   project placement now borrow it. The schema allows an empty `workspace_id`, but those tables have no
   profile column, so storing `''` would share revisions and snapshots across profiles. Fixing it needs a
   storage decision, and likely a migration, so it was left alone. Tests that pin the current rule:
   `internal/soul/authoring_test.go:159`, `internal/heartbeat/authoring_status_test.go:145`.
2. **`TestAgentInstanceApprovalAdmission` fails on a clean checkout of the QA base.** Archive returns 500
   "session reconciliation unavailable" for the approve-all and operator cases. Not caused by this branch.
3. **Report reservation race.** A prompt judged idle that loses a race to a new turn falls back to steer and
   leaves its reports `reserved` until the daemon restarts. The window is narrow and no report is lost.
4. **Permission asks read "Needs sign-in" on session rows.** The `waiting-for-auth` badge covers an edit
   approval, so the row's label misdescribes it.
5. **No default provider gives a raw error.** With no `defaults.provider`, the create screen says the
   default model will be used, then shows the raw `api: invalid create agent request` text. Seen by
   skipping onboarding, which normally sets a provider.
6. **A digest counts a picked proposal as a decision.** The digest said "one decision for you" when the
   proposal already read "You picked".
7. **Daemon log noise.** Global sessions log `store: event summary workspace_id is required`.

W-3 `include: []` handling and the pipeline nits were out of scope; they land separately.

## Unverified

- Real providers. Every turn used the mock, so read-only triage against live providers wasn't re-walked.
- Expired, provider-ended and interrupted approvals; rotation and the superseded read-only state.
- The `+N` overflow panel, collapsed project signal, flat-mode project switching, the project `+` menu, a
  live pose change without a reload, and keyboard paths.
- Fill with AI: the first request sent the wrong field, and it wasn't retried after the harness fix.
- Editing What it does, Check-ins and identity in place: pages were read, saves weren't exercised.
- Item 13 of `ET-web-agents-team-profile` beyond the all-day reading; `include: []` is W-3.
- All profiles, extension-owned agents and composer agent mentions.
- Draft comparison was by eye against `web/src/drafts/` and `_uiux.md`, not a pixel diff.
- No `make gate`, E2E or repo-wide tests were run, as the brief requires. CI owns those.
