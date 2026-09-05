---
name: eng-code-guidelines
description: >-
  Go production discipline for Compozy. Use when writing or editing non-test Go
  files under cmd or internal, including config, logging, CLI, concurrency, and
  process-lifecycle paths. Do not use for Go tests; pair it with the narrower
  schema, contract, cleanup, or network skill when those domains apply.
trigger: implicit
---

# Compozy Code Guidelines

Apply repository-specific Go conventions to production files under `cmd/` and `internal/`. Read the relevant sections of `references/coding-style.md`; for goroutines, shared state, detached execution, subprocesses, shutdown, timers, locks, or channels, also use `references/concurrency-patterns.md`.

- Trace the changed behavior through its applicable error, context, configuration, CLI, logging, type, and resource boundaries. Fix touched violations without expanding into unrelated debt.
- Each changed resource, goroutine, process, or mutable shared state has an owner. Use `eng-cleanup-failure-paths` when multiple fallible acquisitions make partial failure relevant.
- Add a companion skill only for a distinct unresolved concern. `golang-master` supplies deeper language guidance when needed; it is not an automatic prerequisite.
- For public behavior/contract changes, update the owning `docs/_memory/change-impact.md` audit. Cite an existing spec/task/PR audit instead of writing one per skill.

Run the focused owning check and reuse current scoped lint/race evidence. Cross-build and Linux-race parity checks follow the concurrency reference's applicable branches. Root `make gate` and PR CI policy apply at the enclosing commit/push or PR-delivery stage, not after each edit.

## Specific failure cases

- A dependency with only string errors may need one typed wrapper at its boundary so downstream code can match error identity.
- Reflection in codegen/decoders needs an adjacent reason; lint exceptions retain a justified `//nolint:` directive.
- A reachable production panic needs an explicit error path. For a proven unreachable invariant, prefer `panic("invariant: ...")` with its explanation over `log.Fatal`.
- For silently ignored CLI flags, check `cmd.Flags().Changed(name)`, the documented resolution chain, and the default-resolution debug log.
