---
id: ET-web-sessions-board-children
area: ET
title: Child sessions get their own cards, link to their parent, and can be hidden
persona: Bruno
journey:
expected: Every managed child session has its own card in its own column with "↳ <parent title>" in row 3, also when the parent is archived. A parent card shows its direct, non-archived children count with an accent dot when any child needs you; its tooltip lists the children per column. "Hide child sessions" (pressed state, remembered per window) removes every session with a parent from the cards and from the column counts (`children=exclude` on rows and facets); the existing subagents filter keeps its meaning. Provider-native subagents never get cards.
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

Walk: from an orchestrator session spawn three children (`compozy session new --parent <orchestrator>`), and make one of them ask a question. Open the board: the parent card shows the children count 3 with the accent dot, and its tooltip reads 1 Needs you · 2 Working; the questioning child sits in Needs you with "↳ <orchestrator title>". Press "Hide child sessions": the three child cards leave and the Needs you and Working counts drop by their numbers. Reload; the filter is still on for this window only. Archive the orchestrator: its children keep "↳ <title>".

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
