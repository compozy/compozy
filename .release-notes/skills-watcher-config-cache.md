---
title: Idle skill watcher polls use less CPU
type: fix
---

CompozyOS reuses workspace and profile skill settings while their configuration files stay unchanged, reducing idle daemon CPU use with many workspaces. Configuration edits, including file creation and deletion, still take effect on the next skill watcher poll.

Migration notes: no action needed.
