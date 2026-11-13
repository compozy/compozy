---
title: Claude usage follows the current macOS Keychain login
type: fix
---

Claude Code usage on macOS now prefers the current Keychain login over a leftover
credential file. A valid file remains available when the Keychain cannot be read;
an expired file no longer hides a request for Keychain access. Linux and Windows
keep their existing file-based behavior.

Migration notes: no action needed. Existing consent settings and credential stores
are preserved.
