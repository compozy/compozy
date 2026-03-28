# Advanced Optimization Techniques

> Round 2+ patterns for when standard techniques are exhausted.

## Contents

1. [Mathematical Recastings](#mathematical-recastings)
2. [Advanced DP](#advanced-dp)
3. [Exotic Data Structures](#exotic-data-structures)
4. [Streaming/Sublinear](#streamingsublinear)
5. [Algebraic Techniques](#algebraic-techniques)
6. [Graph Optimizations](#graph-optimizations)
7. [Cache-Oblivious Design](#cache-oblivious-design)
8. [Randomized Algorithms](#randomized-algorithms)
9. [Quick Reference](#quick-reference)

---

## Mathematical Recastings

### Convex Optimization

**When:** Brute-forcing allocation/scheduling/fitting

```rust
use minilp::{Problem, OptimizationDirection, ComparisonOp};

let mut problem = Problem::new(OptimizationDirection::Minimize);
let vars: Vec<_> = costs.iter().map(|&c| problem.add_var(c, (0.0, f64::INFINITY))).collect();
for (row, &limit) in constraints.iter().zip(limits.iter()) {
    problem.add_constraint(row.iter().zip(&vars).map(|(&a, &v)| (v, a)), ComparisonOp::Le, limit);
}
let solution = problem.solve().unwrap();
```
**Libraries:** `minilp`, `good_lp`, `osqp`

### Submodular Greedy

**When:** Diminishing returns set function (f(A∪{x})-f(A) ≥ f(B∪{x})-f(B) when A⊆B)

```rust
fn greedy_submodular<F>(n: usize, k: usize, f: F) -> Vec<usize>
where F: Fn(&[usize]) -> f64 {
    let mut selected = Vec::with_capacity(k);
    let mut remaining: Vec<_> = (0..n).collect();
    for _ in 0..k {
        let (best_idx, _) = remaining.iter().enumerate()
            .map(|(i, &e)| { let mut c = selected.clone(); c.push(e); (i, f(&c) - f(&selected)) })
            .max_by(|a, b| a.1.partial_cmp(&b.1).unwrap()).unwrap();
        selected.push(remaining.remove(best_idx));
    }
    selected
}
```
**Guarantee:** 63% of optimal for monotone submodular

### Semiring Generalization

**When:** Path computations (shortest path, transitive closure, dataflow)

```rust
trait Semiring {
    fn zero() -> Self;
    fn one() -> Self;
    fn add(&self, other: &Self) -> Self;  // ⊕
    fn mul(&self, other: &Self) -> Self;  // ⊗
}

// Tropical (shortest paths)
impl Semiring for MinPlus {
    fn zero() -> Self { MinPlus(f64::INFINITY) }
    fn one() -> Self { MinPlus(0.0) }
    fn add(&self, other: &Self) -> Self { MinPlus(self.0.min(other.0)) }
    fn mul(&self, other: &Self) -> Self { MinPlus(self.0 + other.0) }
}

// Boolean (transitive closure)
impl Semiring for bool {
    fn zero() -> Self { false }
    fn one() -> Self { true }
    fn add(&self, other: &Self) -> Self { *self || *other }
    fn mul(&self, other: &Self) -> Self { *self && *other }
}
```

### Matroid Recognition

**When:** Greedy is provably optimal

**Recognition:** Independent sets satisfy hereditary property + exchange property

**Examples:**
- Graphic matroids (spanning trees)
- Partition matroids (select k from each category)
- Linear matroids (linearly independent vectors)

```rust
fn greedy_matroid<I: Matroid>(elements: &[Element], matroid: &I) -> Vec<Element> {
    let mut result = Vec::new();
    let mut sorted: Vec<_> = elements.iter().collect();
    sorted.sort_by(|a, b| b.weight.cmp(&a.weight));  // Descending
    for elem in sorted {
        if matroid.is_independent(&result, elem) {
            result.push(elem.clone());
        }
    }
    result
}
```

### Min-Cost Max-Flow

**When:** Assignment, scheduling, resource allocation

```rust
// Model as graph:
// Source → workers (cap 1, cost 0)
// Workers → tasks (cap 1, cost = -preference)
// Tasks → sink (cap 1, cost 0)
use pathfinding::prelude::*;
let (flow, cost, paths) = edmonds_karp(&graph, source, sink);
```
**Libraries:** `pathfinding`, `petgraph`

### 2-SAT Reduction

**When:** Configuration validity, implication graphs, pairwise boolean constraints

```rust
// (x ∨ y) becomes (¬x → y) ∧ (¬y → x)
fn solve_2sat(clauses: &[(Lit, Lit)]) -> Option<Vec<bool>> {
    let n = /* number of variables */;
    let mut graph = ImplicationGraph::new(n);

    for &(a, b) in clauses {
        graph.add_implication(!a, b);
        graph.add_implication(!b, a);
    }

    let sccs = kosaraju(&graph);

    // Check: x and ¬x in same SCC = UNSAT
    for var in 0..n {
        if sccs[var] == sccs[var + n] { return None; }
    }

    // Assign: choose literal whose SCC comes later in reverse topo order
    Some((0..n).map(|var| sccs[var] > sccs[var + n]).collect())
}
```

---

## Advanced DP

### DP as Shortest Path in Implicit DAG

**When:** DP with DAG structure, non-uniform transition costs

```rust
fn dp_dijkstra(start: State, goal: State) -> Cost {
    let mut dist = HashMap::new();
    let mut heap = BinaryHeap::new();
    dist.insert(start, 0);
    heap.push(Reverse((0, start)));

    while let Some(Reverse((d, state))) = heap.pop() {
        if state == goal { return d; }
        if d > *dist.get(&state).unwrap_or(&Cost::MAX) { continue; }

        for (next_state, cost) in transitions(&state) {
            let new_dist = d + cost;
            if new_dist < *dist.get(&next_state).unwrap_or(&Cost::MAX) {
                dist.insert(next_state, new_dist);
                heap.push(Reverse((new_dist, next_state)));
            }
        }
    }
    Cost::MAX
}
```

### Convex Hull Trick

**When:** DP recurrence dp[i] = min(m[j]·x[i] + c[j]) — O(n²) → O(n log n)

```rust
struct CHT { lines: VecDeque<(i64, i64)> }
impl CHT {
    fn add_line(&mut self, m: i64, c: i64) {
        while self.lines.len() >= 2 {
            let (m1, c1) = self.lines[self.lines.len() - 2];
            let (m2, c2) = self.lines[self.lines.len() - 1];
            if (c - c2) * (m1 - m2) <= (c2 - c1) * (m - m2) { self.lines.pop_back(); }
            else { break; }
        }
        self.lines.push_back((m, c));
    }
    fn query(&mut self, x: i64) -> i64 {
        while self.lines.len() >= 2 {
            let (m1, c1) = self.lines[0];
            let (m2, c2) = self.lines[1];
            if m1 * x + c1 >= m2 * x + c2 { self.lines.pop_front(); }
            else { break; }
        }
        let (m, c) = self.lines[0];
        m * x + c
    }
}
```

### Knuth's Optimization

**When:** dp[i][j] = min(dp[i][k] + dp[k][j] + cost[i][j]), optimal split monotonic — O(n³) → O(n²)

**Condition:** cost satisfies quadrangle inequality, opt[i][j-1] ≤ opt[i][j] ≤ opt[i+1][j]

```rust
fn knuth_optimization(n: usize, cost: &[Vec<i64>]) -> Vec<Vec<i64>> {
    let mut dp = vec![vec![0; n]; n];
    let mut opt = vec![vec![0; n]; n];

    for i in 0..n { opt[i][i] = i; }

    for len in 2..=n {
        for i in 0..=n-len {
            let j = i + len - 1;
            dp[i][j] = i64::MAX;
            let lo = opt[i][j-1];
            let hi = if j + 1 < n { opt[i+1][j] } else { j };

            for k in lo..=hi.min(j) {
                let candidate = dp[i][k] + dp[k+1][j] + cost[i][j];
                if candidate < dp[i][j] {
                    dp[i][j] = candidate;
                    opt[i][j] = k;
                }
            }
        }
    }
    dp
}
```

### Divide & Conquer DP

**When:** Similar conditions, 1D DP — O(n²) → O(n log n)

```rust
fn dc_dp(dp: &mut [i64], prev: &[i64], cost: impl Fn(usize, usize) -> i64,
         lo: usize, hi: usize, opt_lo: usize, opt_hi: usize) {
    if lo > hi { return; }
    let mid = (lo + hi) / 2;
    let mut best = (i64::MAX, opt_lo);

    for k in opt_lo..=opt_hi.min(mid) {
        let candidate = prev[k] + cost(k, mid);
        if candidate < best.0 { best = (candidate, k); }
    }

    dp[mid] = best.0;
    dc_dp(dp, prev, &cost, lo, mid.saturating_sub(1), opt_lo, best.1);
    dc_dp(dp, prev, &cost, mid + 1, hi, best.1, opt_hi);
}
```

---

## Exotic Data Structures

### Suffix Array + LCP

**When:** Substring queries, longest common substring, pattern matching

```rust
use suffix_array::SuffixArray;
let sa = SuffixArray::new(text);
let lcp = sa.lcp_array();
let range = sa.search(pattern);  // O(m log n)
```
**Libraries:** `suffix_array`, `cdivsufsort`

### Wavelet Trees

**When:** Rank, select, quantile queries on sequences — O(log σ)

```rust
// Operations:
// rank(c, i): count of c in prefix [0, i)
// select(c, k): position of k-th occurrence of c
// quantile(l, r, k): k-th smallest in range [l, r)
```
**Libraries:** `wavelet-matrix`

### Link-Cut Trees

**When:** Dynamic tree connectivity, path queries on changing trees — O(log n)

```rust
// Operations:
// link(u, v): Add edge
// cut(u, v): Remove edge
// path_query(u, v): Aggregate on u-v path
// find_root(u): Find tree root
```
**Libraries:** `link_cut_tree` crate or implement from scratch

### Heavy-Light Decomposition

**When:** Path queries on static trees — O(log² n) per query

```rust
struct HLD {
    parent: Vec<usize>,
    depth: Vec<usize>,
    heavy: Vec<Option<usize>>,
    head: Vec<usize>,
    pos: Vec<usize>,
}
// Decompose tree into heavy chains
// Path query = O(log n) chains × O(log n) segment tree
```

### Monotone Deque

**When:** Sliding window min/max — O(1) amortized

```rust
struct MonotoneDeque { deque: VecDeque<(i64, usize)> }
impl MonotoneDeque {
    fn push(&mut self, val: i64, idx: usize) {
        while self.deque.back().map_or(false, |&(v, _)| v >= val) { self.deque.pop_back(); }
        self.deque.push_back((val, idx));
    }
    fn pop_expired(&mut self, min_idx: usize) {
        while self.deque.front().map_or(false, |&(_, i)| i < min_idx) { self.deque.pop_front(); }
    }
    fn min(&self) -> Option<i64> { self.deque.front().map(|&(v, _)| v) }
}
```

### Segment Tree + Lazy Propagation

**When:** Range updates + range queries

```rust
struct LazySegTree<T, L> {
    tree: Vec<T>,
    lazy: Vec<L>,
}

impl<T, L> LazySegTree<T, L> {
    fn push_down(&mut self, node: usize) {
        // Propagate lazy value to children
    }

    fn update_range(&mut self, node: usize, l: usize, r: usize, ql: usize, qr: usize, val: L) {
        if qr < l || r < ql { return; }
        if ql <= l && r <= qr {
            self.apply_lazy(node, val);
            return;
        }
        self.push_down(node);
        let mid = (l + r) / 2;
        self.update_range(2*node, l, mid, ql, qr, val.clone());
        self.update_range(2*node+1, mid+1, r, ql, qr, val);
        self.pull_up(node);
    }
}
```

---

## Streaming/Sublinear

### Bloom Filters

**When:** Probabilistic membership, no false negatives

```rust
use bloom::BloomFilter;
let mut filter = BloomFilter::new(expected_items, false_positive_rate);
filter.insert(&item);
if filter.may_contain(&item) { /* check authoritative source */ }
```

### Count-Min Sketch

**When:** Frequency estimation — O(1) query, O(k) space

```rust
struct CountMinSketch { counters: Vec<Vec<u64>>, hash_fns: Vec<Box<dyn Fn(&[u8]) -> usize>> }
impl CountMinSketch {
    fn insert(&mut self, item: &[u8]) {
        for (i, hf) in self.hash_fns.iter().enumerate() {
            self.counters[i][hf(item) % self.counters[i].len()] += 1;
        }
    }
    fn estimate(&self, item: &[u8]) -> u64 {
        self.hash_fns.iter().enumerate()
            .map(|(i, hf)| self.counters[i][hf(item) % self.counters[i].len()])
            .min().unwrap()
    }
}
```

### HyperLogLog

**When:** Count distinct — O(log log n) space

```rust
use hyperloglogplus::HyperLogLog;
let mut hll: HyperLogLog<str> = HyperLogLog::new(14).unwrap();  // 2^14 registers
for item in stream { hll.insert(&item); }
let cardinality = hll.count();
```
**Libraries:** `hyperloglogplus`

### Locality-Sensitive Hashing (LSH)

**When:** Approximate nearest neighbor search

```rust
// For cosine similarity: random hyperplane LSH
// For Jaccard similarity: MinHash

use minhash::MinHash;
let mh1 = MinHash::new(128);  // 128 hash functions
let sig1 = mh1.signature(&set1);
let sig2 = mh1.signature(&set2);
let approx_jaccard = mh1.similarity(&sig1, &sig2);
```

### Cuckoo Filter

**When:** Membership with deletion (Bloom alternative), better space at low FP rates

```rust
struct CuckooFilter {
    buckets: Vec<[Fingerprint; 4]>,  // 4 slots per bucket
    num_items: usize,
}

impl CuckooFilter {
    fn fingerprint(item: &[u8]) -> Fingerprint { hash(item) as Fingerprint }

    fn bucket_indices(&self, item: &[u8]) -> (usize, usize) {
        let fp = Self::fingerprint(item);
        let i1 = hash(item) % self.buckets.len();
        let i2 = (i1 ^ hash(&fp.to_le_bytes())) % self.buckets.len();
        (i1, i2)
    }

    fn insert(&mut self, item: &[u8]) -> bool {
        let fp = Self::fingerprint(item);
        let (i1, i2) = self.bucket_indices(item);
        // Try both buckets, then cuckoo displacement
    }

    fn contains(&self, item: &[u8]) -> bool {
        let fp = Self::fingerprint(item);
        let (i1, i2) = self.bucket_indices(item);
        self.buckets[i1].contains(&fp) || self.buckets[i2].contains(&fp)
    }

    fn delete(&mut self, item: &[u8]) -> bool {
        // Find and remove fingerprint from either bucket
    }
}
```
**Libraries:** `cuckoofilter`, `xorf`

### Minimal Perfect Hashing

**When:** Static key set, guaranteed O(1), ~2-3 bits/key

```rust
// CHD algorithm: n keys → [0, n) with no collisions
struct MinimalPerfectHash {
    pilots: Vec<u32>,      // Pilot values per bucket
    values: Vec<Value>,    // Direct-indexed values
}

impl MinimalPerfectHash {
    fn build(keys: &[Key]) -> Self {
        // Group keys by initial hash
        // Sort buckets by size (largest first)
        // Find pilot for each bucket that avoids collisions
    }

    fn get(&self, key: &Key) -> Option<&Value> {
        let bucket = hash1(key) % self.pilots.len();
        let pilot = self.pilots[bucket];
        let pos = (hash1(key) ^ hash2(key).wrapping_mul(pilot as u64)) as usize % self.values.len();
        Some(&self.values[pos])
    }
}
```
**Libraries:** `phf` (compile-time), `boomphf`, `ph`

### Fractional Cascading

**When:** Searching same value across multiple sorted lists — O(k log n) → O(log n + k)

```rust
struct FractionalCascade {
    augmented: Vec<Vec<(i64, usize)>>,  // (value, pointer to next list)
}

impl FractionalCascade {
    fn build(lists: Vec<Vec<i64>>) -> Self {
        // Start from last list
        // Build backwards, interleaving every other element from next list
        // Each element has pointer to corresponding position in next list
    }

    fn search(&self, x: i64) -> Vec<usize> {
        // Binary search in first list
        // Follow pointers for remaining lists (adjust by at most 2)
    }
}
```
**Use cases:** Computational geometry, database range queries

---

## Algebraic Techniques

### FFT/NTT Convolution

**When:** Polynomial multiplication — O(n²) → O(n log n)

```rust
use rustfft::{FftPlanner, num_complex::Complex};

fn convolve(a: &[f64], b: &[f64]) -> Vec<f64> {
    let n = (a.len() + b.len() - 1).next_power_of_two();
    let mut planner = FftPlanner::new();
    let fft = planner.plan_fft_forward(n);
    let ifft = planner.plan_fft_inverse(n);

    let mut a_fft: Vec<_> = a.iter().map(|&x| Complex::new(x, 0.0)).collect();
    let mut b_fft: Vec<_> = b.iter().map(|&x| Complex::new(x, 0.0)).collect();
    a_fft.resize(n, Complex::new(0.0, 0.0));
    b_fft.resize(n, Complex::new(0.0, 0.0));

    fft.process(&mut a_fft);
    fft.process(&mut b_fft);

    let mut result: Vec<_> = a_fft.iter().zip(&b_fft).map(|(a, b)| a * b).collect();
    ifft.process(&mut result);

    result.iter().map(|c| c.re / n as f64).take(a.len() + b.len() - 1).collect()
}
```

### Matrix Exponentiation

**When:** Linear recurrence — O(n) → O(log n)

```rust
type Matrix = [[i64; 2]; 2];

fn mat_mul(a: &Matrix, b: &Matrix, m: i64) -> Matrix {
    let mut c = [[0; 2]; 2];
    for i in 0..2 {
        for j in 0..2 {
            for k in 0..2 {
                c[i][j] = (c[i][j] + a[i][k] * b[k][j]) % m;
            }
        }
    }
    c
}

fn mat_pow(mut base: Matrix, mut exp: u64, m: i64) -> Matrix {
    let mut result = [[1, 0], [0, 1]];  // Identity
    while exp > 0 {
        if exp & 1 == 1 { result = mat_mul(&result, &base, m); }
        base = mat_mul(&base, &base, m);
        exp >>= 1;
    }
    result
}

fn fibonacci(n: u64, m: i64) -> i64 {
    if n == 0 { return 0; }
    mat_pow([[1, 1], [1, 0]], n, m)[0][1]
}
```

### Möbius Transform / Subset Convolution

**When:** Sum over subsets, subset DP

```rust
// Zeta transform: f'[S] = Σ_{T⊆S} f[T]
fn zeta_transform(f: &mut [i64]) {
    let n = f.len().trailing_zeros() as usize;
    for i in 0..n {
        for mask in 0..f.len() {
            if mask & (1 << i) != 0 {
                f[mask] += f[mask ^ (1 << i)];
            }
        }
    }
}

// Möbius transform (inverse): f[S] = Σ_{T⊆S} (-1)^{|S|-|T|} f'[T]
fn mobius_transform(f: &mut [i64]) {
    let n = f.len().trailing_zeros() as usize;
    for i in 0..n {
        for mask in 0..f.len() {
            if mask & (1 << i) != 0 {
                f[mask] -= f[mask ^ (1 << i)];
            }
        }
    }
}
```

### Linear Algebra over GF(2)

**When:** XOR systems, toggle problems, error correction

```rust
// Solve Ax = b over GF(2) using Gaussian elimination
fn solve_gf2(mut a: Vec<Vec<bool>>, mut b: Vec<bool>) -> Option<Vec<bool>> {
    let n = a.len();
    let m = a[0].len();
    let mut pivot_col = 0;

    for row in 0..n {
        if pivot_col >= m { break; }

        // Find pivot
        let pivot_row = (row..n).find(|&r| a[r][pivot_col])?;
        a.swap(row, pivot_row);
        b.swap(row, pivot_row);

        // Eliminate
        for r in 0..n {
            if r != row && a[r][pivot_col] {
                for c in 0..m { a[r][c] ^= a[row][c]; }
                b[r] ^= b[row];
            }
        }
        pivot_col += 1;
    }
    Some(b)
}
```

---

## Graph Optimizations

### Bidirectional Search / Meet-in-the-Middle

**When:** Exponential branching — O(b^d) → O(b^{d/2})

```rust
fn bidirectional_bfs(start: Node, goal: Node, graph: &Graph) -> Option<usize> {
    let mut forward = HashMap::from([(start, 0)]);
    let mut backward = HashMap::from([(goal, 0)]);
    let mut forward_frontier = vec![start];
    let mut backward_frontier = vec![goal];

    while !forward_frontier.is_empty() && !backward_frontier.is_empty() {
        // Expand smaller frontier
        if forward_frontier.len() <= backward_frontier.len() {
            let mut next = Vec::new();
            for node in forward_frontier {
                let dist = forward[&node];
                for neighbor in graph.neighbors(node) {
                    if let Some(&bd) = backward.get(&neighbor) {
                        return Some(dist + 1 + bd);  // Found!
                    }
                    if !forward.contains_key(&neighbor) {
                        forward.insert(neighbor, dist + 1);
                        next.push(neighbor);
                    }
                }
            }
            forward_frontier = next;
        } else {
            // Similar for backward
        }
    }
    None
}
```

### Mo's Algorithm

**When:** Offline range queries — O(qn) → O((n+q)√n)

```rust
fn mo_algorithm(arr: &[i64], queries: &[(usize, usize)]) -> Vec<i64> {
    let n = arr.len();
    let block = (n as f64).sqrt() as usize + 1;

    // Sort queries by (l/block, r)
    let mut indexed: Vec<_> = queries.iter().enumerate()
        .map(|(i, &(l, r))| (l / block, r, l, i))
        .collect();
    indexed.sort();

    let mut results = vec![0; queries.len()];
    let mut current = 0i64;
    let mut cur_l = 0;
    let mut cur_r = 0;

    for (_, r, l, idx) in indexed {
        while cur_r < r { current += add(arr[cur_r]); cur_r += 1; }
        while cur_l > l { cur_l -= 1; current += add(arr[cur_l]); }
        while cur_r > r { cur_r -= 1; current -= remove(arr[cur_r]); }
        while cur_l < l { current -= remove(arr[cur_l]); cur_l += 1; }
        results[idx] = current;
    }
    results
}
```

### Centroid Decomposition

**When:** Path queries on trees, divide & conquer

```rust
fn centroid_decomposition(tree: &Tree) -> CentroidTree {
    fn find_centroid(tree: &Tree, root: usize, removed: &[bool]) -> usize {
        let size = subtree_size(tree, root, removed);
        let mut cur = root;
        loop {
            let heavy_child = tree.children(cur)
                .filter(|&c| !removed[c])
                .find(|&c| subtree_size(tree, c, removed) > size / 2);
            match heavy_child {
                Some(c) => cur = c,
                None => return cur,
            }
        }
    }

    fn decompose(tree: &Tree, root: usize, removed: &mut [bool]) -> CentroidNode {
        let centroid = find_centroid(tree, root, removed);
        removed[centroid] = true;
        let children: Vec<_> = tree.children(centroid)
            .filter(|&c| !removed[c])
            .map(|c| decompose(tree, c, removed))
            .collect();
        CentroidNode { id: centroid, children }
    }

    decompose(tree, 0, &mut vec![false; tree.len()])
}
```

### Union-Find with Rollback

**When:** Undo connectivity (no path compression for rollback)

```rust
struct UFRollback {
    parent: Vec<usize>,
    rank: Vec<usize>,
    history: Vec<(usize, usize, usize)>,  // (node, old_parent, old_rank)
}

impl UFRollback {
    fn new(n: usize) -> Self {
        Self { parent: (0..n).collect(), rank: vec![0; n], history: Vec::new() }
    }

    fn find(&self, mut x: usize) -> usize {
        while self.parent[x] != x { x = self.parent[x]; }
        x
    }

    fn union(&mut self, x: usize, y: usize) -> bool {
        let (rx, ry) = (self.find(x), self.find(y));
        if rx == ry { return false; }

        let (small, large) = if self.rank[rx] < self.rank[ry] { (rx, ry) } else { (ry, rx) };
        self.history.push((small, self.parent[small], self.rank[large]));

        self.parent[small] = large;
        if self.rank[rx] == self.rank[ry] { self.rank[large] += 1; }
        true
    }

    fn checkpoint(&self) -> usize { self.history.len() }

    fn rollback(&mut self, checkpoint: usize) {
        while self.history.len() > checkpoint {
            let (node, old_parent, old_rank) = self.history.pop().unwrap();
            let large = self.parent[node];
            self.parent[node] = old_parent;
            self.rank[large] = old_rank;
        }
    }
}
```

---

## Cache-Oblivious Design

### Principles

1. **Recursive decomposition:** Problems splitting into n/2 subproblems naturally fit cache
2. **Van Emde Boas layout:** Recursive memory layout for trees
3. **Blocking without block size:** Algorithms that work for any cache size

### Cache-Oblivious Matrix Transpose

```rust
fn transpose_recursive(
    src: &[f64], dst: &mut [f64],
    n: usize, m: usize,
    src_stride: usize, dst_stride: usize,
    si: usize, sj: usize, di: usize, dj: usize,
) {
    if n <= 32 && m <= 32 {
        // Base case: fits in cache
        for i in 0..n {
            for j in 0..m {
                dst[(di + j) * dst_stride + dj + i] = src[(si + i) * src_stride + sj + j];
            }
        }
    } else if n >= m {
        // Split rows
        let mid = n / 2;
        transpose_recursive(src, dst, mid, m, src_stride, dst_stride, si, sj, di, dj);
        transpose_recursive(src, dst, n - mid, m, src_stride, dst_stride, si + mid, sj, di, dj + mid);
    } else {
        // Split columns
        let mid = m / 2;
        transpose_recursive(src, dst, n, mid, src_stride, dst_stride, si, sj, di, dj);
        transpose_recursive(src, dst, n, m - mid, src_stride, dst_stride, si, sj + mid, di + mid, dj);
    }
}
```

---

## Randomized Algorithms

### Reservoir Sampling

**When:** Uniform random sample from stream of unknown size

```rust
fn reservoir_sample<T: Clone>(stream: impl Iterator<Item = T>, k: usize) -> Vec<T> {
    let mut reservoir = Vec::with_capacity(k);
    let mut rng = rand::thread_rng();

    for (i, item) in stream.enumerate() {
        if i < k {
            reservoir.push(item);
        } else {
            let j = rng.gen_range(0..=i);
            if j < k { reservoir[j] = item; }
        }
    }
    reservoir
}
```

### Randomized Quickselect

**When:** Finding kth smallest element — O(n) expected

```rust
fn quickselect<T: Ord + Clone>(arr: &mut [T], k: usize) -> T {
    let mut rng = rand::thread_rng();

    fn partition<T: Ord>(arr: &mut [T], pivot_idx: usize) -> usize {
        arr.swap(pivot_idx, arr.len() - 1);
        let mut i = 0;
        for j in 0..arr.len() - 1 {
            if arr[j] < arr[arr.len() - 1] {
                arr.swap(i, j);
                i += 1;
            }
        }
        arr.swap(i, arr.len() - 1);
        i
    }

    let pivot_idx = rng.gen_range(0..arr.len());
    let pivot_pos = partition(arr, pivot_idx);

    match pivot_pos.cmp(&k) {
        Ordering::Equal => arr[k].clone(),
        Ordering::Greater => quickselect(&mut arr[..pivot_pos], k),
        Ordering::Less => quickselect(&mut arr[pivot_pos + 1..], k - pivot_pos - 1),
    }
}
```

### Amortized Analysis Patterns

**Key insight:** Don't optimize operations that are already amortized O(1)

**Examples:**
- **Dynamic array resize:** O(1) amortized push despite O(n) occasional resize
- **Union-Find path compression:** O(α(n)) amortized despite O(log n) worst case
- **Splay tree:** O(log n) amortized despite O(n) worst case

**Potential method:** Expensive operations are rare and "pay" for future cheap operations

### Hirschberg's Algorithm

**When:** Sequence alignment when O(n²) space is prohibitive — O(nm) space → O(min(n,m))

```rust
fn hirschberg(a: &[u8], b: &[u8]) -> Vec<Edit> {
    if a.is_empty() { return b.iter().map(|&c| Edit::Insert(c)).collect(); }
    if b.is_empty() { return a.iter().map(|_| Edit::Delete).collect(); }
    if a.len() == 1 { return base_case(a, b); }

    let mid = a.len() / 2;
    let (a1, a2) = a.split_at(mid);

    let score_l = nw_score_linear_space(a1, b);
    let score_r = nw_score_linear_space_rev(a2, b);

    // Find optimal split point in b
    let split = (0..=b.len())
        .max_by_key(|&j| score_l[j] + score_r[b.len() - j])
        .unwrap();

    let (b1, b2) = b.split_at(split);
    let mut result = hirschberg(a1, b1);
    result.extend(hirschberg(a2, b2));
    result
}

fn nw_score_linear_space(a: &[u8], b: &[u8]) -> Vec<i32> {
    let mut prev = (0..=b.len() as i32).map(|i| -i).collect::<Vec<_>>();
    let mut curr = vec![0; b.len() + 1];

    for (i, &ca) in a.iter().enumerate() {
        curr[0] = -(i as i32 + 1);
        for (j, &cb) in b.iter().enumerate() {
            let match_score = if ca == cb { 1 } else { -1 };
            curr[j + 1] = (prev[j] + match_score)
                .max(prev[j + 1] - 1)
                .max(curr[j] - 1);
        }
        std::mem::swap(&mut prev, &mut curr);
    }
    prev
}
```
**Use cases:** Bioinformatics, diff algorithms, sequences >10K elements

---

## Quick Reference

| Technique | Recognition | Complexity Change |
|-----------|-------------|-------------------|
| Convex optimization | Continuous params + convex constraints | Brute → poly |
| Submodular greedy | Diminishing returns | Optimal (63%) |
| Matroid greedy | Hereditary + exchange property | Optimal |
| 2-SAT | Pairwise boolean constraints | O(n+m) |
| DP as shortest path | DAG + non-uniform costs | Dijkstra-style |
| CHT / Li Chao | Linear cost DP | O(n²) → O(n log n) |
| Knuth optimization | Monotonic optimal split | O(n³) → O(n²) |
| D&C DP | Similar to Knuth, 1D | O(n²) → O(n log n) |
| FFT convolution | Polynomial multiply | O(n²) → O(n log n) |
| Matrix exponentiation | Linear recurrence | O(n) → O(log n) |
| Möbius transform | Subset sums | O(3^n) → O(n·2^n) |
| GF(2) linear algebra | XOR systems | Gaussian elim |
| HyperLogLog | Count distinct | O(n) → O(log log n) space |
| LSH | Approximate NN | Sublinear query |
| Bidirectional search | Exponential branch | O(b^d) → O(b^{d/2}) |
| Mo's algorithm | Offline range queries | O(qn) → O((n+q)√n) |
| Fractional cascading | Multi-list search | O(k log n) → O(log n + k) |
| Cuckoo filter | Membership + delete | Better than Bloom |
| Minimal perfect hash | Static keys | O(1), 2-3 bits/key |
| Reservoir sampling | Stream sampling | O(n) time, O(k) space |
| Quickselect | kth element | O(n) expected |
| Hirschberg | Alignment | O(nm) time, O(n) space |
| Cache-oblivious | Any cache size | Recursive blocking |

### Libraries

| Category | Libraries |
|----------|-----------|
| LP/Optimization | `minilp`, `good_lp`, `osqp` |
| Graph | `pathfinding`, `petgraph` |
| Strings | `suffix_array`, `cdivsufsort` |
| Probabilistic | `hyperloglogplus`, `cuckoofilter`, `bloom` |
| FFT | `rustfft` |
| Perfect hash | `phf`, `boomphf` |
| Wavelets | `wavelet-matrix` |
