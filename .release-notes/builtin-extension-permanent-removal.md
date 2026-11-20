---
title: Remove built-in extensions permanently and restore them
type: feature
---

Removing a built-in extension, such as `spec-cycle`, used to last only until the next daemon start,
when CompozyOS enrolled it again. You can now remove a built-in permanently, and it stays removed
across restarts until you restore it. Use **Remove permanently** in the remove dialog, or run
`compozy extension remove <name> --permanent`. Over HTTP and UDS, send
`DELETE /api/extensions/<name>?permanent=true`. Permanent removal needs operator access. Plain
`remove` works as before, and the built-in returns at the next start.

A permanent removal deletes the same things a plain removal does. Specs and workspace files are not
touched, and the extension's MCP overrides and input values are kept.

To bring a built-in back, use **Restore** in the **Removed built-ins** section of the installed
list, which appears while at least one built-in is permanently removed. From the command line,
`compozy extension restore <name>` restores it and `compozy extension removed [-o json]` lists the
removed built-ins. The HTTP and UDS routes are `POST /api/extensions/<name>/restore` and
`GET /api/extensions/removed-builtins`. Agents can list and restore removed built-ins with
`compozy__extensions_removed_list` and `compozy__extensions_restore`, but no tool removes a built-in
permanently.

If a restore fails, the built-in stays removed and the call returns `extension_restore_failed`. An
install that conflicts with an existing extension still returns 409 and now carries the
`extension_exists` code.
