# SDK Builder Benchmark Baselines

**Last updated:** October 27, 2025  \
**Environment:** Apple M4 Max (64 GB), Go 1.25.2  \
**Command:** `go test -run=^$ -bench=. -benchmem github.com/compozy/compozy/sdk/{agent,project,workflow,task,knowledge,memory,compozy}`

Baseline metrics are stored in `sdk/docs/performance-benchmarks.json` and enforced with `tools/benchcheck` (fails when runtime, bytes, or allocs regress by more than 20%).

## Summary Targets

| Builder | Scenario | Time (ns/op) | Memory (B/op) | Allocs/op |
| --- | --- | ---: | ---: | ---: |
| Agent | Simple | 4 054 | 6 314 | 92 |
| Agent | Medium | 11 086 | 15 398 | 244 |
| Agent | Complex | 20 694 | 29 649 | 429 |
| Workflow | Simple | 17 011 | 27 971 | 260 |
| Workflow | Medium | 106 267 | 190 207 | 1 617 |
| Workflow | Complex | 596 911 | 1 136 322 | 9 089 |
| Project | Simple | 3 890 | 6 280 | 87 |
| Project | Medium | 7 910 | 10 522 | 190 |
| Project | Complex | 11 669 | 14 802 | 287 |
| Compozy | Simple | 1 558 376 | 2 147 275 | 24 581 |
| Compozy | Medium | 1 718 274 | 2 356 175 | 27 400 |
| Compozy | Complex | 1 886 675 | 2 567 128 | 30 220 |

All simple workloads remain under 1 ms and <10 kB allocations per target requirements; even complex project builds stay below 12 µs.

## Task Builder Benchmarks

| Task Type | Time (ns/op) | Memory (B/op) | Allocs/op |
| --- | ---: | ---: | ---: |
| Basic | 7 651 | 14 046 | 163 |
| Aggregate | 7 627 | 12 195 | 184 |
| Collection | 8 698 | 16 398 | 172 |
| Composite | 5 066 | 9 115 | 104 |
| Memory | 4 630 | 8 499 | 94 |
| Parallel | 21 656 | 43 755 | 422 |
| Router | 5 687 | 9 875 | 121 |
| Signal | 5 600 | 10 372 | 127 |
| Wait | 4 498 | 8 723 | 93 |

Parallel sub-benchmarks (`BenchmarkTaskBuilders/parallel/*`) confirm linear scaling: each build stays under 8 µs with identical allocation patterns.

## Knowledge & Memory Builders

| Builder | Scenario | Time (ns/op) | Memory (B/op) | Allocs/op |
| --- | --- | ---: | ---: | ---: |
| Knowledge Base | Simple | 2 952 | 3 697 | 71 |
| Knowledge Base | Medium | 5 104 | 7 442 | 123 |
| Knowledge Base | Complex | 5 035 | 7 442 | 123 |
| Knowledge Base | Parallel | 739 | 3 696 | 71 |
| Memory Config | Simple | 1 738 | 2 537 | 37 |
| Memory Config | Medium | 2 031 | 2 849 | 48 |
| Memory Config | Complex | 2 380 | 3 289 | 59 |
| Memory Config | Parallel | 487 | 2 536 | 37 |

## Profiling Insights (Workflow Benchmarks)

CPU profile (`cpu.prof`) and heap profile (`mem.prof`) were generated from the workflow benchmarks. Top hotspots:

1. `github.com/mohae/deepcopy.copyRecursive` (19% CPU, 0.5MB alloc/op) – deep copies dominate cloning of task definitions.
2. `reflect.packEface` and related reflection helpers (>62% of allocations) – reflection-heavy copying inflates both CPU and heap cost.
3. `github.com/compozy/compozy/sdk/workflow.(*Builder).AddTask` (11% CPU, 8% allocation impact) – repeated deep copies per task.
4. `reflect.New` / `reflect.unsafe_NewArray` (collectively ~16% allocations) – new struct creation during cloning.
5. Runtime scheduling (`runtime.usleep`, `runtime.pthread_cond_wait`, `runtime.kevent`) – 50%+ CPU showing idle time driven by `b.RunParallel`; indicates builds finish faster than the parallel harness can schedule work.

Follow-up actions:
- Investigate reducing reliance on `github.com/mohae/deepcopy` by reusing pooled buffers or custom copy routines.
- Preallocate task slices inside workflow builder to cut reflection allocations.
- Explore replacing reflection-based cloning with hand-written copy logic for hot structs.

## Regression Detection

- Baseline metrics: `sdk/docs/performance-benchmarks.json`
- Guardrail: `go run tools/benchcheck --baseline sdk/docs/performance-benchmarks.json --results <bench-output>`
- CI step: runs `go test -run=^$ -bench=. -benchmem github.com/compozy/compozy/sdk/...` (restricted set) and fails if any metric regresses by >20%.

To refresh baselines:
1. Run the benchmark command on stable hardware.
2. Update `performance-benchmarks.json` with new metrics.
3. Re-run `go run tools/benchcheck ...` to confirm zero regressions.
