---
id: LP-web-loops-all-projects
area: LP
title: Loops, Runs and Goals list every project under All projects and open in place
persona: Bruno
journey: J-supervise-loop-request
expected: With the globe on, the Loops catalog reads `GET /api/loops?all_workspaces=true` for one concrete profile — User rows plus each project's rows, each row leading its meta line with a project badge ("User" for user-layer definitions) — and shows one inline "Couldn't load {project}" row with Retry per entry in `errors[]` while healthy rows render; a truncated batch says "Showing the first 2,000 definitions". Under All profiles the toolbar reads "Definitions for {profile}". Runs (and Goals, `origin=session`) read `GET /api/loop-runs?all_workspaces=true` in recency order with a Project column; the one-project Steps view is not offered. A Project filter (multi-select, User last, only under All projects, reset on mode change) narrows both lists server-side through `workspace_ids`. Opening a row, a bell Loop request or node row, a palette Loop row, a session's Goal run, or a task's run opens the item in place on the All projects desktop bound to its project (`?workspace=<id>` or `@user`); the globe stays pressed and the window chrome shows the project badge. In project mode a foreign item still focuses its project first. The loops dock badge counts pending requests of the current breadth and profile lens (`GET /api/loop-requests` under All projects).
entry_points: web Loops window (catalog, Runs); web OS attention bell; web command palette Loops rows; web session Goal strip and timeline Goal marker; web task Loop provenance; web dock
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-web-bell-loop-requests; LP-web-attention-loop-rows; LP-loop-run-deep-link; MS-web-menubar-global-scope-toggle
---

story: As an operator running Loops in several projects, I see every project's Loops, runs and Goals together and act on any of them without leaving All projects.

src: .compozy/tasks/all-projects/_spec.md (slice 3, BR-4/5/10/12/15); `_uiux.md` S5/S8; `_tests.md` W-18, W-23, E-03

2026-10-21 qa-impact (all-projects AW3): added. Walk with two projects plus User-scope runs, one project with a broken loop config (inline error row), and a second profile (All profiles note; dock badge follows the profile lens). Include a bound run permalink and reload. Unit evidence: `loop-catalog.test.tsx`, `loop-runs-location.test.tsx`, `use-loop-request-attention.test.tsx`, `use-menubar-attention-selection.test.tsx`, `-route-preloading.integration.test.tsx`; E2E: `loops.spec.ts` ("Loop runs open in place under All projects…"). Owned by the trailing QA pair.

QA impact 2026-11-01 (responsiveness audit): stream reconciliation retains both
the concrete workspace and All projects query prefixes while coalescing lifecycle
bursts. Confirm latest-run/catalog updates after terminal events, plus request
markers after answering from another surface, without invalidating unrelated run
detail scopes. Existing scenario verdict/history is retained; the enclosing audit
owns the runtime replay and current-head CI. Focused evidence is recorded in
`docs/qa/reports/2026-11-01-performance/loop-loops.md`.
