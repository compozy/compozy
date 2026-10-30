---
id: ET-web-sessions-board-drawer
area: ET
title: Open, step through, resize and close a session in the Kanban board drawer
persona: Bruno
journey:
expected: Clicking a board card opens that session in a drawer beside the live board (#1052) instead of switching the window to List. The drawer is the full session view — transcript, composer, runtime selector, Goal controls, subagents, terminal blocks, approvals and clarifications — headed by the session's own head (state mark, title, PR/status chips, Stop, context toggle) plus Previous session, Next session, Open full session, the session's … menu and Close; no sessions rail and no List | Kanban switch inside it. The board keeps its own head ("Sessions", Group by, Sort by, New session), stays live and highlights the open card (aria-current). Another card swaps the drawer; composer drafts stay per session and the card shows "Draft". Previous/Next and Ctrl+Shift+↑/↓ (the session-cycle shortcuts, also when remapped) walk the cards in board order outside text entry. Escape, Close and a click on the empty board close it — the click never while the composer holds unsent input, never at the end of a drag — and focus returns to the card even after it moved column. The window route carries `drawer=open`: Back reopens a closed drawer, Forward/Back walk opened sessions, a reload and a window restore keep it open. The left edge resizes by drag and by ←/→/Home/End; the width persists per window and seeds new windows. A window narrower than 960 px shows the drawer edge to edge. A child session's head starts with its parent's title, which opens the parent in the drawer. Archive or delete of the open session (drawer … or card menu) closes the drawer and focuses the next card in its column. "Open full session" and the List switch show the session in List and drop `drawer=open`. A card whose session another window already shows brings that window forward in List. Streaming in the drawer does not reread the board's catalogs per transcript burst.
entry_points: web session window Kanban view (sessions-board, session-board-drawer-head, session-board-drawer-controls); window route `/agents/<agent>/sessions/<id>?drawer=open`; localStorage compozy:session:sidebar:v1 (`byScope.<window>.drawerWidth`); shortcuts `session.cycle.next` / `session.cycle.previous`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-10-30-issue-1052-kanban-session-drawer.md
last_report: docs/qa/reports/2026-10-30-issue-1052-kanban-session-drawer.md
overlaps: ET-web-sessions-board-toggle; ET-web-sessions-board-card-actions-group-sort; ET-web-sessions-board-children; ET-web-session-sidebar-per-window
---

Introduced by #1052 (Kanban session drawer).

Walk: in a project with a running session, a session waiting on you, a finished session and a
parent with a running child, open a session window and switch it to Kanban (1600×1000).

1. Click the Working card. The drawer opens beside the board with the full session; the board head
   still reads "Sessions"; the card is highlighted; the URL ends with `drawer=open`.
2. Type a draft, click another card: the drawer swaps; the first card shows "Draft". Click the empty
   board: nothing closes while the second composer holds text; clear it and click again: it closes
   and focus lands on the card.
3. Reopen, press Ctrl+Shift+↓ with focus on the drawer head: the next card opens; Ctrl+Shift+↑ goes
   back. In the composer the chord does nothing.
4. Drag the left edge wider, then Tab to it and press ←; reload: same width, drawer still open.
5. Escape: drawer closes, focus on the card. Browser Back: it reopens on that session.
6. Prompt a session that finishes while open: its card moves Working → Done, the drawer stays open
   and the highlight follows.
7. Open the child session: its head starts with the parent's title; select it to open the parent.
8. Archive the open session from the drawer's …: the drawer closes, focus is on the next card in its
   column. Delete another open session: same landing; Escape in the delete confirmation only closes
   the confirmation.
9. Resize the window below 960 px: the drawer covers the board; Escape returns to the board.
10. Switch the scope to All projects and open a card from another project; with All profiles, open a
    session of another profile: it shows the owner banner read-only, as in List.
11. Select "Open full session": the window shows the session in List; the URL drops `drawer=open`.

Owning automated coverage: `session-board-window.test.tsx` (open/close through the route, Escape and
focus return, empty-board click and unsent-input guard, Previous/Next and the shortcut, archive and
delete focus landing, Open full session), `resizable-drawer.test.tsx` (container breakpoint, custom
head, resolved return focus, keyboard resize), `use-session-window-sidebar.test.tsx` (card opens the
drawer, owned-elsewhere session focuses its window in List, in-drawer navigation keeps the drawer, no
rail reads), `use-session-sidebar-state.test.ts` (per-window drawer width), `use-session-live-tail.test.tsx`
(catalog reread only outside the drawer), `use-session-topbar-slot.test.tsx` (drawer head drops rail
and view toggles), `session-deeplink-search.test.ts` (`drawer=open` survives the route), E2E-005
(`web/e2e/__tests__/sessions-board.spec.ts`).
