---
id: ET-web-session-folders-manual-order
area: ET
title: Session folders, manual drag order, and session/folder context menus
persona: Bruno
journey: J-14
expected: In workspace scope the sessions sidebar (+) opens a menu with New session and New folder. New folder asks for a name and adds a collapsible folder (chevron, folder icon, name, count) at the top, expanded by default with a trailing New session row that creates the session inside it. Right-click on a root session row opens the same actions as its … menu plus Move to folder (submenu of folders), Remove from folder when foldered, and Move up/Move down in Manual sort; Shift+F10 opens it from the keyboard. Right-click on a folder header offers New session in folder, Rename folder…, Move folder up/down, and Delete folder… (confirmation; sessions return to the list, none deleted). The sort menu has a fifth option, Manual, which shows folders first and then the root in the operator's order. Dragging a root session onto a folder header moves it into the folder in any sort; dragging between rows in Manual sort reorders within the folder or root with a visible drop line; dragging a folder header reorders folders. A rejected mutation rolls the list back and shows an error. Folders, membership, and order persist across reload and daemon restart, reach another open window live, are per profile and workspace, and are also manageable through `compozy session folder|move|reorder` and the `compozy__session_folder_*`, `compozy__session_move`, and `compozy__session_reorder` tools. Children and subagents stay under their parent and are never foldered or dragged on their own. All-workspaces scope keeps rows flat (folders are workspace-scoped).
entry_points: SessionSidebar (+) menu; session row and folder header context menus; sort menu Manual; GET/POST /api/workspaces/{workspace_id}/session-folders; PATCH/DELETE /api/workspaces/{workspace_id}/session-folders/{folder_id}; PUT /api/workspaces/{workspace_id}/session-order/{folders,sessions}; POST /api/workspaces/{workspace_id}/sessions/{session_id}/move; GET /api/workspaces/{workspace_id}/sessions?sort=manual; compozy session folder|move|reorder
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-sidebar-threads; RT-session-list-row-actions; ET-web-session-list-bulk-actions
---

Introduced for issue #781. Folder state lives in the global SQLite database (`session_folders`, `session_placements`, migration 00139) and never in browser storage; only the per-client folder collapse state is local view state.
