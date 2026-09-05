---
name: golang-master
description: "Write or review production Go: errors, concurrency, context, types, generics, safety, tests, benchmarks, profiling, modernization, and module structure. Project/framework conventions and third-party API lookup have separate owners."
allowed-tools: Read, Grep, Glob, Bash(go:*)
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
  credits: Distilled from samber/cc-skills-golang and Jeffallan's golang-pro (both MIT)
---

# Golang Master

Language-level doctrine for **Go 1.21+** (1.22–1.26 features flagged inline) in any Go codebase. This skill is the generic floor; a project's own guidelines skill overrides it wherever the two conflict, and this skill owns everything the project leaves unsaid.

Use the matching reference sections for a concrete Go concern. Reuse current context; do not load every reference merely because Go appears in the task. Project conventions and supported Go versions govern applicability.

## The floor

Non-negotiables for every line of Go, regardless of branch:

1. Every error is handled or carries a written justification at the discard site — never a bare `_`.
2. Errors wrap with `fmt.Errorf("context: %w", err)` and are matched with `errors.Is`/`errors.As` — never by string comparison.
3. `ctx context.Context` is the first parameter of any function that does I/O, blocks, or crosses an API boundary, and the caller's ctx is propagated — never a fresh `context.Background()` mid-path.
4. Every goroutine has an owner, an exit path, and a way to be waited on.
5. `panic` is reserved for impossible states; expected failures return errors.
6. Check uncertain type assertions. Initialize maps before writes; use nil/zero-value slices when their semantics fit the contract.
7. Use compile-time interface assertions for an intentional implementation contract; do not invent interfaces just to add an assertion.
8. Tests prove the owning invariant. Use tables for multiple cases and race checks for concurrency or when the project requires them.
9. Apply gofmt and the project's owning lint/test gates; reuse current evidence instead of running repository-wide vet for every edit.
10. Operational values (timeouts, limits, addresses) come from configuration or options — never hardcoded.

## Branches

| When the task involves… | Read |
| --- | --- |
| Creating, wrapping, matching, logging, or joining errors; panic/recover policy | [references/errors.md](references/errors.md) |
| Goroutines, channels, select, sync primitives, errgroup, worker pools, races | [references/concurrency.md](references/concurrency.md) |
| Cancellation, timeouts, deadlines, context values, detached background work | [references/context.md](references/context.md) |
| Nil traps, append aliasing, map access, numeric conversion, defer-in-loop, zero values | [references/safety.md](references/safety.md) |
| Designing structs, interfaces, embedding, receivers, field tags, generics | [references/interfaces-generics.md](references/interfaces-generics.md) |
| Naming anything, control-flow shape, constructors, functional options, `init()` | [references/style-naming.md](references/style-naming.md) |
| Writing or reviewing tests, benchmarks, fuzzing, integration build tags | [references/testing.md](references/testing.md) |
| Profiling, allocation hunting, GC tuning, benchmark comparison | [references/performance.md](references/performance.md) |
| Old-style patterns, deprecated APIs, Go version upgrades | [references/modernize.md](references/modernize.md) |
| New module, directory layout, `cmd`/`internal`/`pkg`, workspaces | [references/layout.md](references/layout.md) |

Concurrency work that cancels anything reads both `concurrency.md` and `context.md`. A review reads the relevant sections needed to assess the changed behavior.

## Tripwires

Investigate these signals when they occur in the changed path; they are not categorical defects:

- A goroutine spawned with no `ctx.Done()`, channel close, or `WaitGroup` path — it outlives its caller.
- `time.After` inside a loop — a timer allocation per iteration; use `time.NewTimer` + `Reset`.
- A typed nil pointer returned as an interface — the result is `!= nil`.
- An `append` result kept alongside the original slice — shared backing array, silent co-mutation.
- `defer` inside a loop body — resources accumulate until function exit; extract the body.
- A narrowing integer conversion without a bounds check — silent wraparound.
- An error logged *and* returned — duplicate reports upstream; pick exactly one.
- An interface returned from a constructor, or defined beside its implementation instead of its consumer.
- Tests with unexplained shared mutable state or concurrency behavior without race coverage.
- `any` where a type parameter or concrete type is known.
