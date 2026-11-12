---
title: Terminal commands can run in a managed worktree
type: fix
---

`compozy__terminal_exec` now accepts a managed worktree by ID or name, the same way opening a terminal does. Use the native tool's `worktree` input, the HTTP/UDS request field, or `compozy terminal exec --worktree <id-or-name>`. Before, a session not bound to a worktree could open a terminal there but could not run a command in it: exec rejected the checkout as `invalid_cwd`.

Relative working directories resolve inside the selected worktree, and the worktree cannot be removed while the command is still running.

Migration notes: no action needed.
