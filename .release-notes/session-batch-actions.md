---
title: Batch session actions match the single-session menu and run in parallel
type: fix
---

Selecting several sessions now offers the actions that apply to multiple sessions, using the single-session menu's names. Mark as seen, Make top-level and Copy titles join the selection menu. Batch Stop, Archive, Unarchive, Delete, Mark as seen, Make top-level and Later actions run in parallel instead of one at a time. Batch worktree actions run in parallel too. At most three requests run at once, every running session shows its own progress, and actions that would overlap a running batch are blocked until it finishes.

Migration notes: no action needed.
