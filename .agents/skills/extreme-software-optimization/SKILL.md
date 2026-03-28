---
name: extreme-software-optimization
description: >-
  Profile-driven performance optimization with behavior proofs. Use when: optimize,
  slow, bottleneck, hotspot, profile, p95, latency, throughput, or algorithmic improvements.
---

# Extreme Software Optimization

> **The One Rule:** Profile first. Prove behavior unchanged. One change at a time.

## The Loop (Mandatory)

```
1. BASELINE    → hyperfine --warmup 3 --runs 10 'command'
2. PROFILE     → cargo flamegraph / py-spy / clinic flame
3. PROVE       → Golden outputs + isomorphism proof per change
4. IMPLEMENT   → Score ≥ 2.0 only, one lever per commit
5. VERIFY      → sha256sum -c golden_checksums.txt
6. REPEAT      → Re-profile (bottlenecks shift)
```

## Opportunity Matrix

| Hotspot | Impact (1-5) | Confidence (1-5) | Effort (1-5) | Score |
|---------|--------------|------------------|--------------|-------|
| *func:line* | × | × | ÷ | Impact×Conf/Effort |

**Rule:** Only implement Score ≥ 2.0

## Isomorphism Proof Template

For EVERY change, document:
```
## Change: [description]
- Ordering preserved:     [yes/no + why]
- Tie-breaking unchanged: [yes/no + why]
- Floating-point:         [identical/N/A]
- RNG seeds:              [unchanged/N/A]
- Golden outputs:         sha256sum -c golden_checksums.txt ✓
```

---

## Pattern Tiers (Quick Reference)

### Tier 1: Low-Hanging Fruit

| Pattern | When | Isomorphism |
|---------|------|-------------|
| N+1 → Batch | Sequential fetches | Same results, fewer round-trips |
| Linear → HashMap | Keyed lookups | O(n)→O(1), order may change |
| Lazy eval | Maybe-unused values | Same final values |
| Memoization | Repeated pure calls | Cached = recomputed |
| Buffer reuse | Alloc per iteration | Zero-copy in loop |

### Tier 2: Algorithmic

| Pattern | Change | Check |
|---------|--------|-------|
| Binary search | O(n)→O(log n) | Sorted input |
| Two-pointer | O(n²)→O(n) | Structured input |
| Prefix sums | O(n)→O(1) query | Static data |
| Priority queue | O(n)→O(log n) | Top-k/scheduling |

### Tier 3: Data Structures

| Structure | Use Case |
|-----------|----------|
| HashMap | Point lookups |
| BTreeMap | Range queries |
| SmallVec | Usually-small collections |
| Arena | Many allocations, bulk free |
| Bloom filter | Membership pre-filter |

**Full catalog:** [TECHNIQUES.md](references/TECHNIQUES.md)

---

## Language Cheatsheet

| Lang | CPU Profile | Trouble Spot Grep |
|------|-------------|-------------------|
| Rust | `cargo flamegraph` | `rg '\.clone\(\)' --type rust` |
| Go | `go tool pprof /debug/pprof/profile` | `rg 'interface\{\}' --type go` |
| TS | `clinic flame -- node app.js` | `rg 'JSON\.(parse\|stringify)' --type ts` |
| Python | `py-spy record -o flame.svg -- python script.py` | `rg '\.iterrows\(\)' --type py` |

**Full language guides:** [LANGUAGE-SPECIFIC.md](references/LANGUAGE-SPECIFIC.md)

---

## Anti-Patterns (Never Do)

| ✗ | Why |
|---|-----|
| Optimize without profiling | Wastes effort on non-hotspots |
| Multiple changes per commit | Can't isolate regressions |
| Assume improvement | Must measure before/after |
| Change behavior "while we're here" | Breaks isomorphism guarantee |
| Skip golden output capture | No regression detection |

---

## Checklist (Before Any Optimization)

- [ ] Baseline captured (p50/p95/p99, throughput, memory)
- [ ] Profiled: hotspot in top 5 by % time
- [ ] Opportunity score ≥ 2.0
- [ ] Golden outputs saved
- [ ] Isomorphism proof written
- [ ] Single lever only
- [ ] Rollback plan: `git revert <sha>`

---

## Tool Commands

```bash
# Benchmark
hyperfine --warmup 3 --runs 10 'command'

# Profile
cargo flamegraph                           # Rust CPU
heaptrack ./binary                         # Allocation
strace -c ./binary                         # Syscalls

# Verify
sha256sum golden_outputs/* > golden_checksums.txt
sha256sum -c golden_checksums.txt          # After changes
```

---

## References

| Need | Reference |
|------|-----------|
| Complete technique catalog | [TECHNIQUES.md](references/TECHNIQUES.md) |
| Step-by-step methodology | [METHODOLOGY.md](references/METHODOLOGY.md) |
| Language-specific guides | [LANGUAGE-SPECIFIC.md](references/LANGUAGE-SPECIFIC.md) |
| Advanced (Round 2+) | [ADVANCED.md](references/ADVANCED.md) |

## Iteration Rounds

- **Round 1:** Standard (N+1, indexes, batching, memoization)
- **Round 2:** Algorithmic (DP, convex, semirings) → [ADVANCED.md](references/ADVANCED.md)
- **Round 3:** Exotic (suffix automata, link-cut trees)

Each round: fresh profile → new hotspots → new matrix.
