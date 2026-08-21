---
id: LP-web-runs-roster-rerank
area: LP
title: The runs roster leads with the runs that need a person
persona: Lea
journey: J-supervise-loop-steady-state
expected: Rows on /loop-runs lead with a plain-words outcome or status, needs-you rows group first and read visibly distinct from active and terminal ones, and the columns are Loop · Status/needs-you · Progress (steps and round) · Started · Duration with the run id demoted to secondary text; the grouping and the progress numbers come from the server's extended list read applied before pagination — no client-side page sort and no per-row follow-up request; a fresh workspace shows an empty roster that explains how to start rather than a blank table; a workspace with dozens of active runs stays readable with the needs-you group still on top; and a lost connection reads as connecting or offline, never as an empty roster.
entry_points: web /loop-runs; GET /api/workspaces/:workspace_id/loop-runs
qa_status: untested
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-runs-roster-server-ordering; LP-web-runs-breadcrumb; LP-web-run-default-read-briefing
---

story: As someone supervising several runs I open the roster and the runs that are waiting on me are already at the top, described in words I use — I never sort a table or decode a run id to find out who needs me.

The re-rank is server truth rendered, not a client behaviour. Walk it with more runs than one page holds and a needs-you run seeded last: if the page sorts what it loaded, that run sits below the fold, and that is the failure this scenario exists to catch. Watch the network while the page loads — one list read, no per-row request for pending counts.

steps:
1. Open `/loop-runs` in a workspace holding needs-you, active and terminal runs, with more runs than one page.
2. Confirm the needs-you group leads, reads distinct from the others, and that the run seeded last is still in it.
3. Confirm each row leads with a plain outcome or status and that the run id is secondary, not the leading text.
4. Confirm Progress shows steps and round from the served values, and that no row triggers its own request.
5. Open a fresh workspace and confirm the empty roster explains how to start instead of rendering a blank table.
6. Load a 30-run workspace and confirm the grouping and readability hold at that scale.
7. Drop the connection and confirm the roster says connecting or offline rather than showing an empty state.

src: web/src/systems/os/apps/loops/ (runs roster); visual contract docs/design/opendesign/loop-legibility/loop-legibility-runs-roster.html (VC-33..VC-36)

QA impact 2026-08-21: Task 05 re-ranked the roster from the server-owned extended read (`_uiux.md` S6). Planning flag only; the loop's QA phase owns the walk and evidence.
