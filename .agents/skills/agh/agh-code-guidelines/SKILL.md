---
name: agh-code-guidelines
description: >-
  Go production discipline for AGH. Use when writing or editing non-test Go
  files under cmd or internal, including config, logging, CLI, concurrency, and
  process-lifecycle paths. Do not use for Go tests; pair it with the narrower
  schema, contract, cleanup, or network skill when those domains apply.
trigger: implicit
---

# AGH Code Guidelines

Apply the canonical AGH rules before changing production Go. This file owns the
application order; its references own the rules.

## Procedures

**Step 1: Route the Change**

1. Confirm the target is a production Go file (`cmd/**` or `internal/**`, not `*_test.go`).
2. Activate `golang-pro`. Also activate the matching narrow skill for tests, cleanup paths, schema migrations, API contracts, or `internal/network` work.
3. Read `.agents/skills/agh/agh-code-guidelines/references/coding-style.md` in full. Also read `.agents/skills/agh/agh-code-guidelines/references/concurrency-patterns.md` in full when the change touches goroutines, shared state, detached lifetime, subprocesses, shutdown, timers, mutexes, or channels.

*Done when:* the edit surface and every matching companion domain are named, and every applicable canonical reference is loaded.

**Step 2: Apply the Canonical Rules**

1. Apply every matching rule from the loaded references; do not copy a subset into task notes or local conventions.
2. Trace the change through error identity, cleanup, context, logging, types, configuration lifecycle, CLI boundaries, comments, package boundaries, goroutine ownership, detached execution, and subprocess supervision.
3. Repair violations in the touched behavior. Record unrelated pre-existing violations without expanding the change silently.

*Done when:* every reference heading that intersects the change has been checked and no touched path violates a matching rule.

**Step 3: Audit Ownership**

1. Enumerate each changed error, resource, goroutine, process, and mutable shared state owner.
2. If setup or teardown has more than one fallible step, complete `agh-cleanup-failure-paths` before continuing.
3. Verify that public behavior remains manageable through the required CLI, HTTP, UDS, native-tool, extension, config, docs, and official-skill surfaces.

*Done when:* every changed lifetime has one explicit owner and every public contract has a complete AGH Impact Audit.

**Step 4: Verify Once**

1. Run `make lint` and scoped `go test -race ./<owning-package>/...` for the changed package.
2. Run cross-build or Linux-race parity checks only when the concurrency reference routes the change there.
3. Reserve the single full `make verify` for the task completion gate after source freeze.

*Done when:* scoped lanes are green and exactly one fresh full gate is scheduled or complete for the finished task.

## Error Handling

- **Existing file already violates the rules:** fix what the current behavior touches; record unrelated debt without silently expanding scope.
- **`errors.Is` / `errors.As` is impossible because the dependency returns a string:** wrap once at the boundary in a typed error of yours; downstream code matches on your typed error.
- **Reflection genuinely required (codegen, decoder):** keep a written justification adjacent to the reflection call. Lint exception requires a `//nolint:` directive with a reason.
- **`panic` shows up in seemingly-production code:** confirm whether the path is reachable post-`main`. If it is, replace with explicit error return; if it is genuinely unreachable, mark with `// unreachable: ...` and prefer `panic("invariant: ...")` over `log.Fatal`.
- **CLI command silently ignores a flag:** verify with `cmd.Flags().Changed(name)`; if the flag is meaningfully optional, document the resolution chain and emit an explicit `slog` debug line when the default is taken.
