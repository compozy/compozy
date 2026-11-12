---
title: The terminal launcher folds worktrees under their project
type: feat
---

Under **All projects**, the **Terminal** dock item's launcher now lists each project with its worktrees folded underneath, using the same branched tree as child sessions in the session list. Before, every worktree of every project appeared in one flat list, so a project with many worktrees pushed the others out of view.

Unfold a project by clicking its worktree count, by tabbing to it and pressing Enter or Space, or with → and ← on the selected project. When you search, projects with matching worktrees unfold to show those worktrees and the first match is selected, so Enter opens it. Selecting a project still opens a terminal in its root.

Migration notes: no action needed.
