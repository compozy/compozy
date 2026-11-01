---
id: ET-web-session-sidebar-per-window
area: ET
title: Each session window owns its sessions sidebar state
persona: Bruno
journey: J-14
expected: With two session windows side by side, opening or closing the sessions sidebar, collapsing a folder, a child-session thread, or a grouped list's project/profile group in one window leaves the other window unchanged. ⌘B, the palette `sidebar.toggle` command, and the session-cycle shortcut act on the focused window's sidebar. A new session window opens with the sidebar state the operator used last and diverges from then on. A reload restores each window's own sidebar state; closing a window drops its stored sidebar state, while switching to another workspace or profile and back keeps the first one's windows' state. Selecting a session in a window keeps every fold that window had, including a collapsed group. Upgrading from a build with one shared sidebar keeps the operator's open/closed choice and every collapsed folder and thread. Sort order, list scope, and the All profiles lens stay global.
entry_points: web session window topbar (session-sidebar-toggle); ⌘B and palette `sidebar.toggle`; session-cycle shortcut; SessionSidebar folder and thread toggles; localStorage key compozy:session:sidebar:v1 (schema 3, `byScope` keyed by window id)
qa_status: pass
bug_ids: compozy/compozy#843
fix_status: fixed
retest_status: pass
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-evidence/issue-843/01-two-windows-default.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-evidence/issue-843/02-folder-collapsed-right-only.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-evidence/issue-843/03-cmd-b-focused-left-only.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-evidence/issue-843/04-after-reload-restored.png;https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/pr-evidence/issue-843/05-left-closed-pruned.png
last_report:
overlaps: ET-web-session-sidebar-threads; ET-web-session-folders-manual-order
---

Introduced for issue #843. Before the fix one persisted store held the sidebar state for every session window, so a toggle in one window changed all of them.

Walk: open two session windows side by side in one workspace with a folder holding two sessions. Collapse the folder in the right window; the left window's folder stays expanded. Focus the left window and press ⌘B; only the left sidebar closes. Reload; the left sidebar is still closed and the right folder is still collapsed. Close the left window and toggle anything in the right one; the stored state (`compozy:session:sidebar:v1` → `context.byScope`) no longer lists the closed window's id. Open a new session window; it starts with the last-used state. Switch to another workspace, change a sidebar there, and switch back; the original windows keep their own state.

2026-10-19 isolated-lab walk (Playwright daemon-served lab, real daemon built from this branch, temporary `COMPOZY_HOME`): every step above passed. The regression is automated as `Issue 843` in `web/e2e/__tests__/os-shell.spec.ts`; store-level independence, inheritance, partition-scoped pruning (switch away and back), and the lossless v2 → v3 upgrade are owned by `use-session-sidebar-state.test.ts`.

QA impact 2026-10-22 (issue #903): group-by group collapse (grouped by project or profile) joins the per-window rail state, keyed by the list lens (scope, profile lens, grouping). Before, it lived in the session window's content and reset whenever a selection remounted it. Walk: under All projects with Group by project, collapse a project group and a thread, select sessions in another group, reload, then open a new session window. The group and thread stay collapsed throughout; the new window inherits them, and expanding the group there leaves the first window collapsed. Selecting a session never reopens its own collapsed group. The window header already names the session and its project, so selection does not reveal the row. Owning suites: `use-session-sidebar-state.test.ts` and `use-session-list-groups.test.tsx` (W-13).

QA impact 2026-10-23 (Sessions Kanban): the same per-window store now also holds the window's List | Kanban view and the board's "Hide child sessions" filter. Both are window-only: a reload restores them, but they never enter the last-used (shell) state, so a new session window always opens in List with children hidden (#1118). Walks live in `ET-web-sessions-board-toggle` and `ET-web-sessions-board-children`; the store invariant is owned by `use-session-sidebar-state.test.ts` (UT-067).
