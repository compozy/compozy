---
title: Slow npm adapter installs no longer fail a session start
type: fix
---

A first `npx -y <adapter>@latest` can take longer than the 30 seconds an adapter gets to start, so CompozyOS stopped the install before the adapter ever ran, and repeat starts could interrupt npm's own recovery. This also blocked Loop agent steps. The package now installs first, with its own five-minute limit, and the 30-second limit starts only after that. This covers built-in and custom providers.

Starts that need an install take turns, and the wait counts toward the five minutes. Versions stay unpinned, nothing in your npm cache is deleted, and a failure now tells a temporary npm problem apart from a package that can't be resolved, in both the error and `compozy doctor`.
