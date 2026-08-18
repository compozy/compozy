---
title: Resource-only extensions need no toolchain
type: fix
---

An extension that ships only declared resources — agents, skills, Loops, automations, layouts — can now use `build`, `dev`, `reload`, and `dev --watch` without installing a Go or TypeScript toolchain. The passive build path validates and publishes those resources without running build or describe subprocesses, and active development links project them into the linked workspace while preserving deterministic generations, atomic reload, and last-good fallback. The Go and TypeScript paths are unchanged, and the resource-only path fails closed. (#423)
