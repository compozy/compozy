---
id: ET-web-sessions-board-paging-archive
area: ET
title: Board columns page by 50 and archived sessions live in the Archive bar
persona: Bruno
journey:
expected: Each board column loads 50 cards and ends with "Load more N of M" while the column holds more; M is the `facets.board` count and stays the full column total. Loading more appends the next page without reordering the loaded cards. Done starts expanded and folds into a narrow rail (disclosure button, glyph, vertical name, count). Archived sessions never appear in the columns; an "Archive N" bar at the bottom, collapsed by default and absent when N = 0, lists them as cards when expanded, and clicking one opens it. Archiving a session from its menu removes its card from its column. A failed column read shows "Couldn't load … sessions." with Retry while the other columns keep working.
entry_points: SessionsBoardColumn (sessions-board-load-more-<column>, sessions-board-column-toggle-done); SessionsBoardArchiveBar (sessions-board-archive-toggle); GET /api/sessions?board_column=done&limit=50&cursor=…; GET /api/sessions?archive=only
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-sessions-board-toggle
---

Introduced by the Sessions Kanban spec (US-012.EC-4, US-016, E2E-006, visual contract VC-21..VC-26).

Walk: seed 60 done sessions and 3 archived ones in one workspace and open the board. Done reads 60 in its header, shows 50 cards and "Load more 50 of 60"; Load more shows all 60 and the button disappears. Collapse Done and expand it again. The bottom bar reads "Archive 3"; expand it and see 3 cards; click one and the window opens it in List. Archive a done session from the list, switch back to Kanban: Done reads 59 and the bar reads "Archive 4". In a workspace with no archived sessions the bar is absent.

Owning automated coverage: E2E-006 (`web/e2e`), `sessions-board.test.tsx` (UT-071 bar absent at zero, Load more progress, Done disclosure, archive expansion).

QA impact 2026-10-24 (review remediation; integrated re-walk pending): open a
session in a second window, switch that window to Kanban, then click its card
from the first window. The existing destination must receive focus in List mode.
Focused evidence: the existing session-window navigation suite covers the
destination-window presentation change before retargeting.


QA impact 2026-10-26 (session responsiveness): repeat this journey while many sessions
are active. Confirm that detail, parent/child titles and counts, board pages, exact
totals, and usage updates remain current across profiles and workspaces. Exercise
usage bursts and hide/restore the streaming window; unrelated transcript/Goal reads
must not refetch for usage-only signals. Current measurements and verification limits
are in the [responsiveness report](../reports/2026-10-26-session-responsiveness.md).
Targeted re-walk 2026-10-26: the final local daemon and production Web build passed
with 64 active idle sessions, 1,140 retained rows, and 128 real rename events. The Done
column loaded 50 → 100 → 150 → 195 cards with exact server counts; the three archived
cards and board mode survived the relevant disclosure/reload checks. Current-head
full E2E delivery remains owned by PR #983 CI; see the report for fixture limits.
