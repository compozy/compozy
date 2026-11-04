---
title: Long data upgrades show progress instead of looking stuck
type: fix
---

The first start after upgrading could spend several minutes moving telemetry out of a large database. During that time the desktop app sat on "Preparing CompozyOS" with no sign of progress, and quitting it made the upgrade look failed.

- **Visible progress.** The desktop boot screen now shows "Upgrading data…" with a progress bar, and `compozy daemon bootstrap` reports "Upgrading CompozyOS data (N%)", while that one-time move runs.
- **No false failures.** If the desktop app starts while a CompozyOS runtime is still booting, it now waits for that runtime instead of trying five more launches and reporting a failure.
