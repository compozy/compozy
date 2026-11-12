---
title: The daemon starts when a disabled provider has no definition
type: fix
---

A profile with `[providers.<id>] enabled = false` for a provider that has no built-in or configured definition no longer stops the daemon from starting. Disabled providers are no longer validated until you enable them, and provider usage skips a provider it cannot resolve and logs a warning instead of failing for every provider. An enabled provider, or the default provider, must still be fully defined.

Migration notes: no action needed.
