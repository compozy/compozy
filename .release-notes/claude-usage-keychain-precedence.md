---
title: Claude usage reads the macOS Keychain login past a stale credential file
type: fix
---

On macOS, an expired, malformed or unscoped `.credentials.json` left in Claude Code's
config directory no longer reports Claude usage as signed out while Claude Code is
logged in through the Keychain. With Keychain access allowed, CompozyOS reads the
Keychain login through its existing silent, cached path; without consent it asks for
Keychain access instead of asking you to log in again. A usable credential file still
takes precedence and never triggers a Keychain read. Linux and Windows are unchanged.

Migration notes: no action needed. Existing consent settings and credential stores
are preserved.
