---
title: Parallel multi-run with git worktree isolation
type: feature
---

`compozy tasks run --multiple` can now execute the batched task workflows **in parallel**, each in its own isolated git worktree, instead of running them one at a time. This builds on the `--multiple` queue introduced in 0.2.8 and is fully additive — the default behavior (enqueued, one-at-a-time) is unchanged.

### Starting a parallel run

```bash
# Run children concurrently, each in its own worktree
compozy tasks run --multiple alpha,beta --parallel

# Bound how many children start at once (default 2)
compozy tasks run --multiple alpha,beta --parallel --parallel-limit 3
```

### Configuration

The mode and fanout limit can be set per workspace under `[tasks.run]`:

| Key                           | Flag override      | Default    | Meaning                                         |
| ----------------------------- | ------------------ | ---------- | ----------------------------------------------- |
| `run_multiple_mode`           | `--parallel`       | `enqueued` | `enqueued` or `parallel`                        |
| `run_multiple_parallel_limit` | `--parallel-limit` | `2`        | Max children started concurrently (must be > 0) |

`--parallel` and `--parallel-limit` are valid only with `--multiple`, and each flag overrides the corresponding config value for that invocation. Passing `--parallel-limit` while the resolved mode is `enqueued` is now rejected rather than silently ignored.

### Behavior

- **Worktree isolation.** Each child run gets a dedicated git worktree so concurrent agents never collide on the working tree. Worktree paths include a SHA-256 digest of the parent run id, so distinct parent runs can't collide on the same slug.
- **Bounded fanout with fail-late aggregation.** Children start up to the parallel limit at a time; a failing child no longer aborts its siblings — failures are aggregated and reported after the batch settles.
- **Worktree handoff surfaced.** The TUI and CLI render each child's worktree path so you know exactly where each run executed.
- **Contract aligned.** Multi-run item status now reports `running` (matching the daemon and `docs/events.md`) instead of the stale `active` that leaked into the OpenAPI schema and generated TypeScript client; a contract test now pins the runtime constants to the published enum so they can't drift again.
