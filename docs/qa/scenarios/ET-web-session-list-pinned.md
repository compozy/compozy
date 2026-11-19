---
id: ET-web-session-list-pinned
area: ET
title: Pin sessions above the session list's project groups and folders
persona: Bruno
journey: J-14
expected: Every session row in the Active session list offers Pin to top and Unpin, as a quiet pin button beside its … (revealed on hover and keyboard focus) and as an item in the row context menu and … menu. Pinning moves that session into a Pinned section at the top of the list, below search and above every project group, folder, and flat row, and carries its subtree: every descendant (spawned, provenance and recovery children, their continuations, and subagents in the parent's roster) renders nested under it in Pinned at any depth with the usual expansion depth, and leaves its project group and folder while the pin lasts; a fork, or a continuation of the pinned session itself, stays in place. Its parent, folder, and project group stay where they were and no fold changes. A pinned session inside another pin's subtree shows once, under the outer pin. Pinned shows a pin glyph, "Pinned", and the number of pins (carried rows are not counted), folds like a group (Collapse all / Expand all include it, a search opens it), puts a new pin on top, follows the operator's order (drag a pin between pins, or Move ▸ Move up/Move down; persisted, identical after reload and in other windows; only where the list reads one project and profile, and off during a search), pages by pin past 100 pins with whole subtrees per page with its own Previous/Next sessions (the count reads "100+"), keeps its rows with a Retry when a refresh fails, and is absent when nothing is pinned. In All projects it sits above every project group and shows every project's pins; a project's list shows only that project's pins. Group and folder counts exclude pinned sessions and every session they carry, so a project whose only visible sessions are pinned or carried disappears while the pin lasts and comes back on Unpin; with every session pinned, the list shows Pinned instead of "No sessions". Unpin returns the session and its subtree to their project group and folder at the positions they held, including after a sibling was reordered meanwhile; an inner pin then gets its own place in Pinned. After a pin change, focus lands on the session's row in its new place and a polite live region announces "Pinned <title>." or "Unpinned <title>."; a failed change shows "Couldn't pin <title>. Try again.". Pinned rows offer no folder items; carried rows are not dragged and offer no order items. Subagent sessions and always-on agents' main sessions cannot be pinned (no affordance; the API refuses with `subagent_pin_follows_parent` / `session_pin_instance_main`). A pin survives stop, archive, Later, reload, and daemon restart; archived or parked pinned sessions show a pin mark with Unpin in the Archived and Later views and return to Pinned when restored. The Kanban board ignores pins. Pins reach other open windows through the catalog stream and are manageable through `compozy session pin|unpin`, `compozy session reorder --pinned`, `compozy session list --pinned`, `compozy__session_list` `pinned`, and `compozy__session_reorder` `pinned`.
entry_points: SessionSidebar row pin button; row context menu and … menu (Pin to top / Unpin, Move ▸ Move up/Move down on a pin); Pinned section header and drag between pins; PUT /api/workspaces/{workspace_id}/session-order/sessions with pinned=true; POST/DELETE /api/workspaces/{workspace_id}/sessions/{session_id}/pin; GET /api/workspaces/{workspace_id}/sessions?pinned=only&sort=pinned; compozy session pin|unpin; compozy session list --pinned
qa_status: pending
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pin-sessions/88426ae3176c/ (01..11 PNGs, dark)
last_report:
overlaps: ET-web-session-folders-manual-order; ET-web-session-list-aggregate-parity; RT-session-list-row-actions; ET-web-session-list-bulk-actions
---

Introduced for issue #1827. The pin lives in the global SQLite database (`sessions.pinned_at`,
migration 00206), never in browser storage; only the Pinned fold is local view state, stored with
the group folds.

Walk: in a lab with two projects (one holding a single session) and a project with a folder of
three sessions and a parent with children, pin a child deep in a folder and confirm it rises alone
while its parent, siblings, folder, and folds stay put; pin the single-session project's session
in All projects and confirm the project group hides, then Unpin and confirm it returns; reorder a
sibling inside the folder while a member is pinned, Unpin, and confirm the member returns to its
slot; Tab to a row's pin button and press Enter, then open the row menu with Shift+F10 and choose
Unpin, confirming focus and the announcement each time; reload and restart the daemon and confirm
the pins persist.

Owning coverage: the store and session manager catalog suites (filter over durable and active
sessions, `pinned` order and cursor, facets, Later count, refusals, no `updated_at` bump, manual
order slot preservation), the sessions HTTP handler suite, the CLI session suite, and the web
SessionList and `use-session-list-groups` suites (Pinned section, hidden sole-session group, counts,
empty state, menu items, visible order).

Issue #1859 changed the rule: a pin carries its subtree, and Pinned has a manual order
(`sessions.pin_position`, migration 00209, backfilled in pin-time order). The carried set is resolved
by the daemon's lineage-aware pinned filter, so the Web, CLI, and native tools agree; the
`sort=pinned` cursor anchors a pin, never a carried row.

Walk (#1859): pin a parent with children, a grandchild, a continued child, a fork, and a subagent;
confirm the subtree nests under the pin in Pinned, the fork stays in its group, and the group and
folder counts drop by the carried rows; pin the grandchild too and confirm it shows once; drag a
pin above another and use Move up from the keyboard, reload and open a second window to confirm
the order; Unpin and confirm every descendant returns to its group, folder, and slot.

Known limits: there is no bulk Pin/Unpin; pinned rows in the Pinned section are not part of a bulk
selection; the Pinned order cannot be changed in an aggregate (All projects or all profiles) list
or during a search; subagent sessions follow their parent. An archived or Later pin carries nothing
until it is back, and a child started in another project stays in that project, so a carried row
whose pin is not in the read lists as a root in Pinned without Move up/Move down or drag. A Pinned
page returns each pin's whole subtree, however large.

2026-11-18 isolated-lab walk (daemon and `web/dist` built from the branch at 88426ae31, demo-seeded
Northstar Pay home on its own port and socket, plus a `launch-notes` project holding one unbound
session, headless Chromium): All projects grouped by project. Pinning "Rehearse the cutover in
staging", a child inside the Queue cutover folder of platform-infra, moved only that row into
Pinned; its parent, sibling, folder and folds stayed, and the folder and project counts dropped by
one (4→3, 8→7). Pinning the only session of `launch-notes` with Tab to its pin button and Enter
removed the project group (0 group nodes), moved focus to the row in Pinned and announced "Pinned
Plan the launch announcement."; Unpin from the keyboard brought the group back in its place and
announced "Unpinned …". Unpinning the deep session returned it to its folder slot under its parent.
The row menu offered Pin to top and, on a pinned row, Unpin. Lab torn down after the walk. Not
walked: paging past 100 pins, reload and daemon restart persistence (owned by the store suite).
