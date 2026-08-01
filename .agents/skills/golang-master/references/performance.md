# Performance

Profile first — intuition about bottlenecks is wrong most of the time. One change at a time, measured before and after; an optimization without numbers is a readability regression.

## Rule out external bottlenecks

If 90% of latency is a slow query or upstream call, no amount of allocation tuning helps. Before optimizing Go code: goroutine profile showing stacks blocked in `net` or `database/sql` means the wait is external — fix the query, add caching, tune the pool, and stop.

## The cycle

1. **Pick the metric** — latency, throughput, memory, or CPU; without a target, changes are random.
2. **Baseline** — `go test -bench=BenchmarkX -benchmem -count=10 ./pkg | tee old.txt`.
3. **Diagnose** — profile (below) to find where the metric goes.
4. **Change one thing**, with a comment explaining why it's shaped that way.
5. **Compare** — `benchstat old.txt new.txt`; claim only statistically significant deltas (no `~` rows), and paste the benchstat table into the commit body.
6. Repeat until the target is met — then stop; unneeded optimization is debt.

## Profiling

```bash
go test -bench=BenchmarkParse -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg
go tool pprof cpu.prof                 # top, list Func, web
go tool pprof -alloc_objects mem.prof  # GC churn: what allocates most often
go tool pprof -inuse_space mem.prof    # leaks: what holds memory now
go tool trace trace.out                # scheduling, GC phases, blocking — the "when"
```

Live services expose `net/http/pprof` on a private port; capture with `go tool pprof http://host:6060/debug/pprof/profile?seconds=30`. Mutex/block profiles find lock contention; the goroutine profile finds leaks and external waits.

## Reading the signal

| Signal | Diagnosis | Action |
| --- | --- | --- |
| High `alloc_objects` in hot path | Allocation churn | Reduce allocations (below) |
| One function dominates CPU | Hot loop | Algorithmic fix first, micro-opt second |
| High GC %, OOM kills in containers | GC pressure / no memory ceiling | `GOMEMLIMIT` to 80–90% of the container limit; investigate retention |
| Goroutines parked on I/O | External latency | Fix the upstream; connection pooling; timeouts |
| Mutex profile hot | Lock contention | Narrow critical sections; shard; atomics |
| Same computation repeated | Missing cache | Memoize; `singleflight` for concurrent duplicate calls |
| O(n²) where O(n) exists | Wrong algorithm | Fix the algorithm before touching allocations |

## Allocation reduction

The usual biggest ROI — Go's GC is fast, not free:

- Preallocate when size is known: `make([]T, 0, len(src))`, `make(map[K]V, n)`.
- `strings.Builder` (with `Grow`) for string assembly; avoid `[]byte`↔`string` ping-pong — every conversion copies.
- Reuse hot temporary buffers with `sync.Pool` — always reset before `Put`; pooled objects must carry no per-use state.
- Avoid boxing into `any`/interfaces in hot paths (every boxing may allocate); generics keep call sites unboxed.
- Escape analysis explains surprise heap allocations: `go build -gcflags='-m'` — returned pointers, captured closures, and interface conversions send values to the heap.
- Struct field order affects size (alignment padding); `fieldalignment` finds waste — only worth fixing on high-volume structs.
- Compile regexps once at package level; `strconv` over `fmt` in hot paths; `slog.LogAttrs` at guarded levels when logging cannot leave the loop.

## Runtime tuning

- `GOMEMLIMIT` is the container-safety knob (soft ceiling; GC works harder as it approaches). `GOGC` trades memory for GC frequency — tune only with profile evidence.
- `GOMAXPROCS` respects container CPU quotas automatically since Go 1.25.
- PGO: drop a production CPU profile at `default.pgo` next to `main` and the compiler optimizes hot paths (~2–7% typical) — free once profiles are routine.

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Optimizing without a profile | pprof first — then decide |
| Trusting one benchmark run | `-count=10` + `benchstat`; variance is real |
| Default `http.Client`/Transport at scale | Set `MaxIdleConnsPerHost` near your concurrency; add timeouts |
| `reflect.DeepEqual` in production paths | `slices.Equal`, `maps.Equal`, `bytes.Equal`, or `go-cmp` in tests |
| Logging inside hot loops | Hoist, sample, or `slog.LogAttrs` behind a level check |
| `panic`/`recover` as control flow | Error returns — panic unwinds are expensive |
| Micro-optimizing an O(n²) | Fix the complexity first |
| Pooling everything | `sync.Pool` pays off only for measured-hot, uniform, resettable objects |
