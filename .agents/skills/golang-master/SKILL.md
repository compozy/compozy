---
name: golang-master
description: Go engineering doctrine for writing and reviewing production Go — errors, concurrency, safety, types, tests, performance. Use when wrapping or matching errors, spawning goroutines or picking channel vs mutex vs atomic, propagating context and cancellation, designing interfaces or generics, naming and structuring declarations, auditing nil/slice/map/numeric safety, shaping table-driven tests and benchmarks, chasing allocations with pprof, modernizing pre-1.21 idioms, or laying out a module. Don't use for a framework's or repo's own conventions (project skills own those), non-Go code, or fetching third-party library docs.
allowed-tools: Read, Grep, Glob, Bash(go:*)
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
  credits: Distilled from samber/cc-skills-golang and Jeffallan's golang-pro (both MIT)
---

# Golang Master

Language-level doctrine for **Go 1.21+** (1.22–1.26 features flagged inline) in any Go codebase. This skill is the generic floor; a project's own guidelines skill overrides it wherever the two conflict, and this skill owns everything the project leaves unsaid.

Match the task to one or more Branches rows and read every listed reference **in full** before producing output — the references are the contract; the floor and tripwires below apply to every branch.

## The floor

Non-negotiables for every line of Go, regardless of branch:

1. Every error is handled or carries a written justification at the discard site — never a bare `_`.
2. Errors wrap with `fmt.Errorf("context: %w", err)` and are matched with `errors.Is`/`errors.As` — never by string comparison.
3. `ctx context.Context` is the first parameter of any function that does I/O, blocks, or crosses an API boundary, and the caller's ctx is propagated — never a fresh `context.Background()` mid-path.
4. Every goroutine has an owner, an exit path, and a way to be waited on.
5. `panic` is reserved for impossible states; expected failures return errors.
6. Type assertions use comma-ok; maps and slices are initialized before use.
7. Exported types implementing an interface carry `var _ Interface = (*Type)(nil)` beside the definition.
8. Tests are table-driven with named subtests and run under `-race`.
9. `gofmt`, `go vet ./...`, and the project's linter pass with zero findings before hand-off.
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

Concurrency work that cancels anything reads both `concurrency.md` and `context.md`. A review reads every reference whose subject appears in the diff.

## Tripwires

Final self-check — each of these has shipped a real bug:

- A goroutine spawned with no `ctx.Done()`, channel close, or `WaitGroup` path — it outlives its caller.
- `time.After` inside a loop — a timer allocation per iteration; use `time.NewTimer` + `Reset`.
- A typed nil pointer returned as an interface — the result is `!= nil`.
- An `append` result kept alongside the original slice — shared backing array, silent co-mutation.
- `defer` inside a loop body — resources accumulate until function exit; extract the body.
- A narrowing integer conversion without a bounds check — silent wraparound.
- An error logged *and* returned — duplicate reports upstream; pick exactly one.
- An interface returned from a constructor, or defined beside its implementation instead of its consumer.
- Independent subtests without `t.Parallel()`, or a suite that has never run under `-race`.
- `any` where a type parameter or concrete type is known.
