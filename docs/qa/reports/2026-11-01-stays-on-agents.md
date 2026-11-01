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

## B5 remediation

Worktree: `stays-on-fix-b5`, branch `feat/stays-on-fix-b5`.

| Item | Change | Evidence |
|---|---|---|
| 1 | Migration 00172 adds profile ownership to soul/heartbeat history, snapshots and digest uniqueness. Profile-specific historical source paths retain their named profile; other existing rows retain the default profile. Authoring, rollback and purge receive explicit profile identity. User-scope HTTP editors work without a project, and Web no longer borrows a placement. | Migration preservation/reopen and canonical migration stream checks passed before the named-profile backfill extension. Two-profile authoring suites and User-scope HTTP read/write/history/rollback checks passed with real SQLite and race. Named-profile migration preservation and public runtime checks passed. |
| 2 | Archive accepts a starter without optional reconciliation when no main is bound; recovery-capable starters still resolve and stop unpublished mains. | Original TestAgentInstanceApprovalAdmission passed with race; commit ebb054854. |
| 3 | A prompt falling back to staged steer releases its report reservations in the admission transaction. | Existing reservation suites passed with race; commit 51c4055a8. |
| 4 | Permission badges read Needs your approval. | Existing badge suite and focused lint passed; commit 6207d4669. |
| 5 | Create explains an absent default provider and asks for a provider in Model; helper copy describes configured defaults conditionally. | Adapter and screen suites passed, 53 tests, plus focused lint; commit f2494fd1e. |
| Digest | Picked proposals no longer count as pending decisions and keep their cards visible. | Existing away suite and focused lint passed; commit 3db879212. |

Validation uses focused checks only, as required by the worker brief. `make codegen`,
`go build -p 1 ./...`, the canonical authoring/API suites and unchanged
`TestAgentInstanceApprovalAdmission` passed with race on the final resolver behavior. Full Soul and
Heartbeat owning suites passed. Native/extension heartbeat and harness integration checks passed. The final serial run passed
codegen-check, Web typecheck and all 137 tests across the six affected Web suites
(`term-4b4161556513`). The optional complete SQLite suite exceeded its ten-minute package budget in unrelated migration
suites; focused B5 cases passed. Scoped Go lint identified local constant, large-parameter and formatting issues, followed by
three additional constant/fixture findings. They are repaired; final scoped lint passed with
`0 issues` and exit code 0 (`term-09c88f47b9ab`). A fresh full Go build and focused API/native/session race tests passed
after those repairs.

Live isolated API verification passed User SOUL/HEARTBEAT open, save, reread and history for an
On demand definition and a Global-only Stays on definition. Both profiles maintain separate
revision IDs, and foreign rollback returns 404 for both sidecars. Global definitions remain shared;
profile ownership applies to persisted history and snapshots. The first walk exposed the public
User resolver error and was stopped; production now uses ResolveUser for @user, with the canonical
HTTP suite reproducing that boundary. The borrowed-placement Web workaround is removed.

The affected native Soul/Heartbeat tools and extension/session/scheduler/harness consumers forward
profile identity. No new native ID, hook or config key is introduced. Migration 00172 is additive;
all earlier migration checksums remain unchanged. Official agent-definition skill guidance and the
owning QA scenarios are updated. This is the bounded B5 delta to the approved always-on-agent impact
entry; the orchestrator-owned central impact record is untouched.

The targeted lab and immutable API receipts are recorded in
[the remediation report](2026-11-01-stays-on-b5-remediation.md). Browser verification passed in an isolated Chrome profile: both agent kinds opened both editors,
SOUL saves survived reload, absent HEARTBEAT displayed all day and five checked sources, and the
missing-provider flow recovered after model selection. Independent API rereads confirmed the UI
writes. Targeted teardown returned clean=true with zero survivors. No real provider turn,
global gate, E2E run or push was performed. The authoring fix is committed as `9d766918a`. Strict QA audit retains C14 solely for the global
gate prohibited by the worker brief; the evidence records this explicit exception without claiming
a gate pass. The follow-up `73a23ce17` records the lint repairs and completed isolated QA. Final scoped lint,
API and daemon race checks passed. All requested items 1–5 and the picked-proposal digest fix are
implemented and committed; no implementation or targeted QA remains pending.
