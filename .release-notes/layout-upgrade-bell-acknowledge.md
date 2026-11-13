---
title: The layout upgrade notice reads atomically and toasts once
type: fix
---

Marking the "Your windows are now tabs" bell notice read now records every listed
upgrade together before updating the stored layouts, so a failure no longer leaves
the bell partly read; retrying finishes the update. Upgrades over several projects
show one toast naming them instead of one identical toast each, and an upgrade that
was already announced does not toast again after a reload.

Migration notes: no action needed.
