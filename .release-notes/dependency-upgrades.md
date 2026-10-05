---
title: Updated runtime dependencies and preserved profile icons
type: fix
---

Upgrade Go runtime dependencies and the web/desktop toolchain, including the native TypeScript
checker, Bubble Tea installation wizard and CEL expression engine. Source builds require
Go 1.27.1. Existing profile icons retain their appearance when Lucide renames their symbols.

Migration notes: profile reads and writes translate `album`, `book-marked`, `building-2`,
`flip-horizontal-2`, `flip-vertical-2` and `trash-2` to their canonical Lucide names without
discarding profile state. The boundary decoder is scheduled for removal in v0.5.0 after the
persisted names have been migrated. No operator action is required for this release.
