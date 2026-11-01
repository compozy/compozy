---
id: ET-web-sessions-board-children
area: ET
title: Child sessions get their own cards, link to their parent, and can be hidden
persona: Bruno
journey:
expected: Every managed child session has its own card in its own column with "↳ <parent title>" in row 3, also when the parent is archived. A parent card shows its direct, non-archived children count with an accent dot when any child needs you; its tooltip lists the children per column. A forked or continued session is top-level: its card shows the fork/continue glyph with the source title ("Forked from <title>" in its accessible name), it is not counted in the source's rollup, and "Hide child sessions" keeps it. "Hide child sessions" starts pressed in a new window (#1118); turning it off is remembered per window. While pressed it removes every child session (provenance, spawn, subagent and recovery lineage) from the cards and from the column counts (`children=exclude` on rows and facets); the existing subagents filter keeps its meaning. Provider-native subagents never get cards.
entry_points: SessionBoardCard (session-board-card-children-<id>, session-board-card-parent-<id>); board head sessions-board-hide-children; GET /api/sessions?children=exclude; GET /api/sessions/facets?children=exclude
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-sessions-board-toggle; ET-web-session-sidebar-threads
---

Introduced by the Sessions Kanban spec (US-014, E2E-007, visual contract VC-27, VC-30).

Walk: from an orchestrator session spawn three children (`compozy session new --parent <orchestrator>`), and make one of them ask a question. Open the board in a new window: "Hide child sessions" is pressed, no child card shows, and the parent card shows the children count 3 with the accent dot; its tooltip reads 1 Needs you · 2 Working. Press "Hide child sessions": the three child cards join their columns, the Needs you and Working counts rise by their numbers, and the questioning child sits in Needs you with "↳ <orchestrator title>". Reload; the filter is still off for this window only. Archive the orchestrator: its children keep "↳ <title>".

Owning automated coverage: E2E-007 (`web/e2e`), `session-board-card.test.ts` (UT-070 rollup), `sessions-board.test.tsx` (parent link and highlighted rollup render), `use-session-sidebar-state.test.ts` (per-window filter).

Accessibility regression: inspect the card button's accessible name. A child includes "Child of
{parent title}"; a parent includes "3 child sessions, A child needs you". One child reads
"1 child session". No family metadata means no family phrase. Existing title, status, waiting
signal and relative time remain in the name. Covered by `sessions-board.test.tsx`.


QA impact 2026-10-26 (session responsiveness): repeat this journey while many sessions
are active. Confirm that detail, parent/child titles and counts, board pages, exact
totals, and usage updates remain current across profiles and workspaces. Exercise
usage bursts and hide/restore the streaming window; unrelated transcript/Goal reads
must not refetch for usage-only signals. Current measurements and verification limits
are in the [responsiveness report](../reports/2026-10-26-session-responsiveness.md).
Targeted re-walk 2026-10-26: the final local daemon and production Web build displayed
the three-child rollup and Needs you indicator. Hiding children removed the child
cards and changed exact counts from Needs you 1 / Done 195 to Needs you 0 / Done 193,
matching the public facets API. Current-head full E2E delivery remains owned by PR
#983 CI; see the report for the isolated dataset and verification limits.

QA impact 2026-10-30 (#1074, forks are not child sessions): fork the orchestrator from its row
menu and continue another session. On the board the fork card shows the fork glyph and the
orchestrator's title, its accessible name reads "Forked from <title>", and the orchestrator's child
count excludes it. Press "Hide child sessions": the spawned children leave, the fork and the
continuation stay, and the column counts and `GET /api/sessions/facets?children=exclude` agree.
Owning automated coverage: `TestSessionCatalogBoard` (store SQL filter, totals and rollups),
`TestManagerBoardCatalog` (active overlay), `session-board-card.test.ts` and
`sessions-board.test.tsx` (card lineage).

QA impact 2026-10-31 (#1118, default on): a window with no stored state, and a stored entry without
`hideChildren`, start with the filter on. A stored `false` stays off: the old default wrote it for
every window that mounted the rail, so windows that existed before the upgrade stay off until turned
on, while new windows start on because the shell entry never seeds this filter. Walk: seed
`compozy:session:sidebar:v1` with `byScope.shell.hideChildren: false` and a window whose entry has
`hideChildren: false`; reload; that window stays off and a new session window shows it on. Owning
coverage: `use-session-sidebar-state.test.ts` (UT-067 and the upgrade case) and E2E-007.
