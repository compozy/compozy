# Detailed Optimization Methodology

> Step-by-step process for rigorous, provably correct performance optimization.

## Phase A: Baseline

### A1. Test Suite

```bash
cargo test --release 2>&1 | tee baseline_tests.txt
grep -E "(FAILED|error)" baseline_tests.txt && echo "FIX TESTS FIRST" && exit 1
```

### A2. Performance Metrics

```bash
# Latency (p50/p95/p99)
hyperfine --warmup 3 --runs 30 --export-json baseline.json './binary <args>'
cat baseline.json | jq '.results[0].times | sort |
  {p50: .[length/2], p95: .[length*0.95|floor], p99: .[length*0.99|floor]}'

# Peak memory
/usr/bin/time -v ./binary <args> 2>&1 | grep "Maximum resident"
```

### A3. Document

```markdown
## Baseline (DATE)
| Metric | Value |
|--------|-------|
| p50 | X ms |
| p95 | X ms |
| p99 | X ms |
| Peak memory | X MB |
| Tests | PASS |
```

---

## Phase B: Profile

### B1. CPU (Flamegraph)

```bash
cargo flamegraph --root --release -- <args>
# Alternative
perf record -g ./binary <args> && perf script | inferno-collapse-perf | inferno-flamegraph > flame.svg
```

**Analysis:** Widest bars = most time. Deep stacks = call overhead.

### B2. Allocation

```bash
# DHAT
DHAT_LOG_FILE=dhat.out cargo +nightly run --release --features dhat-heap -- <args>
dhat-viewer dhat.out

# heaptrack
heaptrack ./binary <args>
heaptrack_gui heaptrack.binary.*.zst
```

### B3. I/O

```bash
strace -c ./binary <args>
strace -T -e read,write,open,close ./binary <args> 2>&1 | head -1000
```

### B4. Hotspot Table

```markdown
| Rank | Location | % Time | Category |
|------|----------|--------|----------|
| 1 | `file.rs:123` | 35% | CPU |
| 2 | `file.rs:456` | 22% | Alloc |
| 3 | `file.rs:789` | 15% | I/O |
```

---

## Phase C: Equivalence Oracle

### C1. Golden Outputs

```bash
for input in test_inputs/*; do
  ./binary "$input" > "golden_outputs/$(basename $input).out"
done
sha256sum golden_outputs/* > golden_checksums.txt
```

### C2. Invariants

Document:
1. **Ordering:** Results sorted by [field]
2. **Tie-breaking:** Equal items ordered by [secondary]
3. **Floating-point:** IEEE 754, no NaN
4. **RNG seeds:** Deterministic given seed X

### C3. Property Tests

```rust
use proptest::prelude::*;

proptest! {
    #[test]
    fn deterministic(input in any::<Vec<u8>>()) {
        assert_eq!(process(&input), process(&input));
    }

    #[test]
    fn ordering_preserved(items in prop::collection::vec(any::<Item>(), 0..1000)) {
        let result = process(&items);
        assert!(result.windows(2).all(|w| w[0] <= w[1]));
    }
}
```

---

## Phase D: Isomorphism Proof

### Template

```markdown
## Change: [description]

### What Changes
- Before: [code/behavior]
- After: [code/behavior]

### Proof
1. **I/O Equivalence:** Same inputs → same outputs because [reason]
2. **Ordering:** Preserved because [reason or N/A]
3. **Tie-breaking:** Unchanged because [reason or N/A]
4. **Floating-point:** [identical/N/A]
5. **RNG:** [unchanged/N/A]

### Verification
- [ ] `sha256sum -c golden_checksums.txt`
- [ ] `cargo test`
- [ ] `diff <(./old input) <(./new input)`
```

### Common Proofs

| Pattern | Proof |
|---------|-------|
| Memoization | Pure function, same results cached |
| Index lookup | Same data, different access pattern |
| Batching | Same operations, collected in order |
| Parallelization | Commutative/associative OR sorted merge |

---

## Phase E: Opportunity Matrix

### Scoring

```
Score = (Impact × Confidence) / Effort

Impact (1-5): 5=>50%+, 4=25-50%, 3=10-25%, 2=5-10%, 1=<5%
Confidence (1-5): 5=profiler confirms, 3=likely, 1=speculative
Effort (1-5): 5=>1 day, 3=hours, 1=minutes
```

### Matrix

| Opportunity | Impact | Conf | Effort | Score |
|-------------|--------|------|--------|-------|
| HashMap lookup | 4 | 5 | 2 | 10.0 |
| Memoize `expensive_fn` | 3 | 4 | 2 | 6.0 |
| Batch queries | 3 | 3 | 3 | 3.0 |

**Rule:** Only implement Score ≥ 2.0

---

## Phase F: Implementation

### One Lever Per Change

```bash
git checkout -b perf/add-hashmap-index
# Make ONLY the optimization change
# NO: refactoring, cleanup, style
git diff --stat  # Minimal files
```

### Checklist

- [ ] Single technique applied
- [ ] No unrelated refactors
- [ ] Commit message explains perf rationale

### Rollback Plan

```markdown
## Rollback
- Command: `git revert <sha>`
- Risk: None / Low / Medium
- Post-rollback tests: [list]
```

---

## Phase G: Regression Guardrails

### Benchmarks

```rust
use criterion::{criterion_group, criterion_main, Criterion};

fn bench_critical(c: &mut Criterion) {
    let input = setup_input();
    c.bench_function("critical_function", |b| b.iter(|| critical_function(&input)));
}

criterion_group!(benches, bench_critical);
criterion_main!(benches);
```

### CI Gate

```yaml
- name: Check regression
  run: |
    cargo bench -- --baseline main --save-baseline current
    cargo benchcmp main current --threshold 10  # Fail if >10% regression
```

### Monitoring

```rust
use metrics::{histogram, counter};

fn critical_function(input: &Input) -> Output {
    let start = Instant::now();
    let result = do_work(input);
    histogram!("critical_function_ms", start.elapsed().as_millis() as f64);
    result
}
```

---

## Iteration Protocol

After each cycle:
1. **Re-baseline** → new metrics
2. **Re-profile** → new hotspots (bottlenecks shift)
3. **Update matrix** → recalculate scores
4. **Repeat** → until no Score ≥ 2.0

```markdown
## History
| Round | Change | p95 Before | p95 After | Δ |
|-------|--------|------------|-----------|---|
| 1 | HashMap | 50ms | 35ms | -30% |
| 2 | Memoize | 35ms | 28ms | -20% |
```

---

## Command Reference

```bash
# Profile
cargo flamegraph --root --release -- <args>
heaptrack ./binary <args>
strace -c ./binary <args>

# Benchmark
hyperfine --warmup 3 --runs 30 './binary <args>'
cargo bench

# Memory
/usr/bin/time -v ./binary <args>
valgrind --tool=massif ./binary <args>

# Verify
sha256sum -c golden_checksums.txt
cargo test
diff <(./old input) <(./new input)

# Git
git checkout -b perf/description
git revert <sha>
```
