---
title: Terminal history stays outside project directories
type: breaking
---

Terminal journal databases now live in `~/.compozy/workspaces/<workspace-id>/compozy.db`, beside
the workspace's retained terminal artifacts and recordings. The configured `COMPOZY_HOME` is
respected. Terminal use no longer creates SQLite databases or WAL/shared-memory sidecars in the
project's `.compozy/` directory.

## Migration notes

This is an explicitly approved storage hard cut. Existing `<project>/.compozy/compozy.db` files
are not imported, read, modified or deleted. The new journal starts empty, so old command history
and its recording/artifact index are not available through the journal after upgrading. Retain
the old files if you need that history; never remove SQLite files while an older daemon uses them.
New history persists across restarts and remains isolated by workspace and profile.
