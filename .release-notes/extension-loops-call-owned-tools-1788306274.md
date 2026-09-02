---
title: An extension Loop can call the tools its own extension ships
type: fix
---

A code-backed extension can contribute both Loops and tools, but the external-source policy stopped a contributed Loop from resolving a tool owned by that same extension, so the action failed with `unknown_action_kind`. The manifest owner is now preserved through resource loading, compilation, executed snapshots, and hydration, and execution adds exactly that same-owner extension source to the normal allow set. (#503)

- Trusted-source status is never granted, and tools from other extensions stay denied.
- Loop schema compilation snapshots the operator registry once per compilation instead of reprojecting it repeatedly.
- Extension installation and enablement retry transient `SQLITE_BUSY` conflicts, and lifecycle tokens prevent stale cleanup from disabling a replacement installation.
