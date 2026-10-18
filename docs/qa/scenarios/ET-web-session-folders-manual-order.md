---
id: ET-web-session-folders-manual-order
area: ET
title: Session folders, manual drag order, and session/folder context menus
persona: Bruno
journey: J-14
expected: In workspace scope the sessions sidebar (+) opens a menu with New session and New folder. New folder asks for a name and adds a collapsible folder (chevron, folder icon, name, count) at the top, expanded by default with a trailing New session row that creates the session inside it. Right-click on a root session row opens the same actions as its … menu plus Move to folder (submenu of folders), Remove from folder when foldered, and Move up/Move down in Manual sort; Shift+F10 opens it from the keyboard. Right-click on a folder header offers New session in folder, Rename folder…, Move folder up/down, and Delete folder… (confirmation; sessions return to the list, none deleted). The sort menu has a fifth option, Manual, which shows folders first and then the root in the operator's order. Dragging a root session onto a folder header moves it into the folder in any sort; dragging between rows in Manual sort reorders within the folder or root with a visible drop line; dragging a folder header reorders folders. Manual reorder (row drag and Move up/Move down) is offered only when the whole container is on screen: not while a filter or a worktree scope narrows the list, and not on a later catalog page; moving into or out of a folder stays available. If the folder list fails to load, the list stays flat and shows "Couldn’t load folders." with Retry, which brings the folders back. The session-cycle shortcut follows the folder order and skips sessions inside a collapsed folder. A rejected mutation rolls the list back and shows an error. Folders, membership, and order persist across reload and daemon restart, reach another open window live, are per profile and workspace, and are also manageable through `compozy session folder|move|reorder` and the `compozy__session_folder_*`, `compozy__session_move`, and `compozy__session_reorder` tools. Children and subagents stay under their parent and are never foldered or dragged on their own. All-workspaces scope keeps rows flat (folders are workspace-scoped).
entry_points: SessionSidebar (+) menu; session row and folder header context menus; sort menu Manual; GET/POST /api/workspaces/{workspace_id}/session-folders; PATCH/DELETE /api/workspaces/{workspace_id}/session-folders/{folder_id}; PUT /api/workspaces/{workspace_id}/session-order/{folders,sessions}; POST /api/workspaces/{workspace_id}/sessions/{session_id}/move; GET /api/workspaces/{workspace_id}/sessions?sort=manual; compozy session folder|move|reorder
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/session-folders/4670c69f1a2b/ (dark-/light-01..06 PNGs)
last_report:
overlaps: ET-web-session-sidebar-threads; RT-session-list-row-actions; ET-web-session-list-bulk-actions
---

Introduced for issue #781. The narrowed-list reorder guard, the folder load failure Retry, and the folder-aware session-cycle shortcut were added after the 2026-10-17 walk and have not been walked yet. Folder state lives in the global SQLite database (`session_folders`, `session_placements`, migration 00139) and never in browser storage; only the per-client folder collapse state is local view state.

2026-10-17 isolated-lab walk (targeted web/api/cli lab, real daemon at this branch, Vite proxied): CLI created folders, moved and reordered sessions, and `session list --sort manual` returned folders first then the pinned root. In the Web sidebar, (+) → New folder created a folder that a second open page showed live; dropping a row on a folder header, Move to folder, and Remove from folder each persisted through the daemon and reached the second page; Delete folder… kept the session at the root; Shift+F10 opened the row menu. A daemon restart preserved folders, membership, and manual order. Light and dark captures cover expanded/collapsed folders, the (+) menu, both context menus, and a drag in progress. Lab teardown clean.
