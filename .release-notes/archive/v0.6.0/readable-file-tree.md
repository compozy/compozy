---
title: File trees stay readable on deep paths
type: fix
---

The file tree in a session's **Changes** panel and in the **Files** app keeps names readable on long, deeply nested paths. Before, names collapsed to a couple of characters with the ellipsis drawn over the text, directory names were clipped on the left, indentation pushed rows against the right edge, and the git status and line counts overlapped the name.

Folders that contain a single folder now collapse into one row, names truncate with one clean ellipsis that keeps the extension, and the status column never overlaps the name. The diff list in **Changes** keeps the file name and its nearest folders visible and shortens the shared prefix instead.

Migration notes: no action needed.
