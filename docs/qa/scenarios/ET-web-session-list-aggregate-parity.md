---
id: ET-web-session-list-aggregate-parity
area: ET
title: Session list organization under All projects and All profiles, adopting sessions as children, and copying IDs
persona: Bruno
journey: J-14
expected: Folders, manual order, multi-select with bulk verbs, folder moves from the row and selection menus, and the list keyboard shortcuts work in every Group by group that resolves to one (workspace, profile) pair. A group that spans pairs — a project under All profiles, a profile under All projects — splits into one subgroup per pair (a single pair renders without a second header). One selection spans every expanded group; Stop, Archive, Unarchive, and Delete route each session to its owner profile, and Move to folder needs the selection inside one pair, otherwise it says "Select sessions from one project and profile to move them." The flat aggregate list keeps New folder visible but disabled with "Folders live inside one project and profile. Group by … to add one." and Manual sort shows a note instead of silently reordering. Every group header has a "+" after its count (a sibling of the collapse toggle) offering New session and New folder in that group's project and profile; when the group spans profiles (or projects) the New folder dialog asks for the missing axis, prefilled with the creation destination. New session from a group or a folder opens creation on that project and profile. Dropping a top-level session onto another row's middle (or anywhere on it outside Manual sort) nests it as that row's child in one daemon write, with a highlighted target and an Undo toast; sessions that already have a parent, subagents, archived sessions, other pairs, and the session's own descendants never offer the drop. A nested provenance child offers Make top-level in its row menus. Both the … and right-click row menus offer Copy session ID (the `sess-…` ID) and Copy title, each confirmed by a toast.
entry_points: SessionSidebar and Sessions modal under the topbar globe (All projects) and profile lens (All profiles); Sort menu Group by; group header +; row … and context menus; selection bar; drag onto a row; PUT/DELETE /api/workspaces/{workspace_id}/sessions/{session_id}/parent; compozy session reparent; compozy__session_reparent
qa_status: untested
overlaps: ET-web-session-folders-manual-order; ET-web-session-list-bulk-actions; RT-session-list-row-actions; ET-profile-web-aggregate-owner-surfaces
---

Introduced for issue #904. Walk every breadth: one project + one profile, All projects + one
profile, one project + All profiles, and All projects + All profiles, each flat and grouped.
Use an isolated lab with two projects, two profiles, sessions in each pair, one parented child,
and one subagent.

1. One project, one profile: folders, Manual order, selection, and the toolbar (+) menu behave
   as in ET-web-session-folders-manual-order. Copy session ID and Copy title from both menus
   paste the exact ID and the shown title.
2. All projects, flat: rows name their project; (+) shows New folder disabled with its reason;
   Manual sort shows the note. Group by project: each project group shows folders and serves
   Manual order; its header + creates a session or folder in that project.
3. One project, All profiles, Group by profile: each profile group is one pair with folders,
   Manual order, and its own header +. Select rows from two profile groups: the selection bar
   counts both; Stop/Archive route per owner; Move to folder explains the cross-pair refusal.
4. All projects + All profiles, Group by project: each project group splits into profile
   subgroups (one subgroup renders inline). The project header + New folder asks for the
   profile; the folder appears in that profile's subgroup once it holds a session.
5. Drag a top-level session onto another row of the same pair: the row highlights; dropping
   nests it, the Undo toast detaches it again. `compozy session list -o json` shows the
   provenance lineage, root, and depth before and after. Dragging a child, a subagent, or onto
   one of the source's descendants offers no drop. Make top-level on the nested row returns it.
6. Reload and restart the daemon: folders, order, and adopted lineage persist; a second window
   updates live.

7. Attempt to reparent a session with a descendant in another workspace: expect
   `session_reparent_cross_workspace_subtree` and unchanged source/descendant lineage.
   Native calls targeting either a source or parent in another profile must be denied.
