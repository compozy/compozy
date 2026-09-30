---
id: RT-web-session-all-workspaces
area: RT
title: Browse sessions across every workspace without losing healthy groups
persona: Théo
journey: J-respond-to-agent-attention
expected: The Sessions catalog offers one globe toggle — this workspace or every workspace. Both scopes expose bounded pages with explicit previous/next navigation and server search across history. Workspace groups show exact population counts, isolate one workspace's page failure, join and remove workspaces live, persist globally through `shell.sessions.scope`, and open a foreign session in its owner workspace.
entry_points: web Sessions dock item; web session-window sidebar
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-attention-catalog; ET-web-session-cross-workspace-confirm
---

Exercise more than 100 sessions in one workspace, a failing sibling workspace, live workspace
join/removal, collapse, reload, and foreign activation. A failed group must not blank healthy groups,
and actions scoped to the active runtime must never target a foreign row.

QA impact 2026-08-16: Task 03 added the daemon-backed tri-state scope and cursor-complete grouped
catalog. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.

QA impact 2026-08-17: The tri-state pill group collapsed to a single globe toggle and the
agent-grouped `all` view was deleted; `shell.sessions.scope` is now `workspace` (default) or
`all-workspaces`. The narrow breadth lists the workspace's complete catalog — the six-thread cap is
gone. Re-walk: confirm the globe presses both ways, the CLI reports the same value, and the
workspace groups still isolate a failure.

Walk this cycle: blocked-verify — the web unit suites (5211 passing) and the rewritten Playwright
legs cover the globe's pressed states, the daemon round trip, and workspace-group isolation, but an
isolated QA lab with a live daemon was not started, so a persona walk through public entry points
could not meet the qa-execution evidence standard.

2026-08-23 qa-impact (Profiles): **reset from `blocked-verify` to `untested`** — phase 0 deleted the
client-side catalog filtering this row's cross-workspace behavior was built on
(`web/src/systems/session/hooks/use-session-catalog-streams.ts`) and moved narrowing into the
daemon, and the catalog payload gained the profile field. Re-walk workspace breadth with a live
daemon, confirming the browser never receives rows it then hides, that a failing workspace group
still isolates, and that widening workspaces does not widen profiles. The profile axis of the same
stream is owned by `ET-profile-stream-isolation`; what Global means for the data is owned by
`MS-global-scope-no-workspace-work`.

Global consumer acceptance: with the desktop data scope set to Global and no selected project,
open Sessions through the palette and the shell modal while the saved session-list preference is
Workspace. Verify that both issue the canonical profile-scoped `all_workspaces=true` catalog read,
show persisted Global/project sessions, and preserve truthful attention rows. Selecting a project
must return to that workspace's scoped catalog; an unresolved project selection must not become
an implicit aggregate read. A null Global workspace is a valid unscoped destination, not a disabled
catalog. The owning OS attention and palette suites cover this readiness distinction; the real
browser retest remains the acceptance proof.

QA impact 2026-09-29 (#679): With more than 200 sessions across multiple workspaces, verify that
opening a palette catalog fetches one page of at most 100 rows and exact population facets.
Next and Previous must reach older rows; searching a title/agent beyond the first page must find it
without walking intervening pages. Chips retain whole-scope counts while search and badge filters
change the visible page. A catalog wake rereads only the current visible page and preserves its
cursor and keyboard focus. In grouped Sessions, collapse a workspace and confirm no record page
is fetched for it; expand, navigate its history, and verify keyboard cycling uses that visible page.
Refresh failures must retain known rows with Retry, while missing aggregate metadata stays unknown
rather than showing a zero count. Repeat the catalog soak on the final built assets for 60 minutes;
request growth must depend on visible consumers and wakes, not persisted history size.
