---
title: SQLite upgrades resume safely after an interruption
type: fix
---

If CompozyOS stops while rebuilding SQLite tables during an upgrade, restarting it retries the
interrupted migration with your session catalog and derivation history intact.
