---
title: The compozy command works after installing the macOS app
type: fix
---

Installing CompozyOS from the DMG ran the runtime but left no `compozy` command in your terminal. Only sessions inside the app could find it.

On first launch, the macOS app now links `compozy` into `/usr/local/bin`, or into `~/.local/bin` when that folder is already on your shell's PATH. If it needs administrator rights, it asks first. If the link is missing, use **Install compozy Command…** in the CompozyOS menu to add it. Existing commands are never overwritten. App updates keep the command working without reinstalling, and `compozy --version` now works too.
