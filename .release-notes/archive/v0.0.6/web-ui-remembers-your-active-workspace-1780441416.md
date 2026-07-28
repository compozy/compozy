---
title: Web UI remembers your active workspace
type: fix
---

The web UI now remembers your active workspace across reloads and browser restarts, so a refresh no longer snaps you back to the first workspace. Direct and shared session links resolve a session's owning workspace from its ID and load reliably regardless of which workspace is selected, and the UI redirects to the agent page when an opened session belongs to a different workspace than the active one.
