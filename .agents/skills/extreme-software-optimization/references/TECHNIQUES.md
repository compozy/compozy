# Optimization Techniques Catalog

> Scan for applicable patterns after profiling identifies hotspots.

## Contents

1. [I/O & Network](#io--network)
2. [Memory & Allocation](#memory--allocation)
3. [Concurrency](#concurrency)
4. [Algorithms](#algorithms)
5. [Data Structures](#data-structures)
6. [Caching](#caching)
7. [Serialization](#serialization)
8. [Strings](#strings)

---

## I/O & Network

### N+1 Elimination

```rust
// BAD: N round-trips
for id in ids { db.get(id).await; }

// GOOD: 1 round-trip
let items = db.get_many(&ids).await;
```
**Isomorphism:** Same results, order may change unless preserved.

### Buffer Reuse

```rust
// BAD
loop { let buf = vec![0u8; 4096]; file.read(&mut buf)?; }

// GOOD
let mut buf = vec![0u8; 4096];
loop { file.read(&mut buf)?; }
```

### Vectored I/O

```rust
// BAD: Multiple syscalls
socket.write(&header)?; socket.write(&body)?;

// GOOD: Single syscall
let iov = [IoSlice::new(&header), IoSlice::new(&body)];
socket.write_vectored(&iov)?;
```

### Async Batching

```rust
// BAD: Sequential
for item in items { process(item).await; }

// GOOD: Parallel
futures::future::join_all(items.iter().map(process)).await;

// BETTER: Bounded concurrency
futures::stream::iter(items).map(process).buffer_unordered(10).collect().await;
```

### Bounded Queues

```rust
// BAD: Unbounded (memory blowup)
let (tx, rx) = mpsc::unbounded_channel();

// GOOD: Backpressure
let (tx, rx) = mpsc::channel(1000);
```

---

## Memory & Allocation

### Pooling

```rust
let pool = Pool::new(|| expensive_create(), 10);
let obj = pool.get();  // Reuse, don't create
```
**Libraries:** `deadpool`, `bb8`, `r2d2`

### Arena Allocation

```rust
let arena = bumpalo::Bump::new();
for _ in 0..1000 { arena.alloc_str("item"); }  // Fast bump
// All freed when arena drops
```
**Libraries:** `bumpalo`, `typed-arena`

### SmallVec

```rust
use smallvec::SmallVec;
let items: SmallVec<[Item; 8]> = SmallVec::new();  // Stack up to 8
```

### Cow<str>

```rust
fn process(input: &str) -> Cow<str> {
    if needs_change(input) { Cow::Owned(modify(input)) }
    else { Cow::Borrowed(input) }  // Zero-copy
}
```

### SoA Layout

```rust
// AoS - poor cache locality
struct Point { x: f32, y: f32, z: f32 }
let points: Vec<Point>;

// SoA - excellent for field iteration
struct Points { xs: Vec<f32>, ys: Vec<f32>, zs: Vec<f32> }
```

---

## Concurrency

### Sharded Locks

```rust
// BAD: Single contention point
let map = Mutex::new(HashMap::new());

// GOOD: 16 shards
let shards: [Mutex<HashMap<K, V>>; 16];
fn shard(key: &K) -> usize { hash(key) % 16 }
```
**Libraries:** `dashmap`, `flurry`

### Lock-Free

```rust
use crossbeam::queue::SegQueue;
let queue = SegQueue::new();  // Lock-free MPMC
```
**Libraries:** `crossbeam`, `parking_lot`

### Work-Stealing

```rust
rayon::scope(|s| {
    s.spawn(|s| recursive_task(s));  // Automatic load balancing
});
```

---

## Algorithms

### Binary Search

```rust
// On data
let idx = data.binary_search(&target);

// On answer space (parametric)
let answer = (lo..hi).binary_search_by(|mid| {
    if predicate(mid) { Ordering::Less } else { Ordering::Greater }
});
```
**O(n) → O(log n)**

### Two-Pointer

```rust
fn two_sum_sorted(arr: &[i32], target: i32) -> Option<(usize, usize)> {
    let (mut lo, mut hi) = (0, arr.len() - 1);
    while lo < hi {
        match (arr[lo] + arr[hi]).cmp(&target) {
            Ordering::Equal => return Some((lo, hi)),
            Ordering::Less => lo += 1,
            Ordering::Greater => hi -= 1,
        }
    }
    None
}
```
**O(n²) → O(n)**

### Sliding Window

```rust
fn max_sum_k(arr: &[i32], k: usize) -> i32 {
    let mut sum: i32 = arr[..k].iter().sum();
    let mut max = sum;
    for i in k..arr.len() {
        sum += arr[i] - arr[i - k];
        max = max.max(sum);
    }
    max
}
```

### Prefix Sums

```rust
// Build O(n), query O(1)
let prefix: Vec<i64> = arr.iter().scan(0, |acc, &x| { *acc += x; Some(*acc) }).collect();
fn range_sum(prefix: &[i64], l: usize, r: usize) -> i64 {
    prefix[r] - if l > 0 { prefix[l - 1] } else { 0 }
}
```

### Union-Find

```rust
struct UnionFind { parent: Vec<usize>, rank: Vec<usize> }
impl UnionFind {
    fn find(&mut self, x: usize) -> usize {
        if self.parent[x] != x { self.parent[x] = self.find(self.parent[x]); }
        self.parent[x]
    }
    fn union(&mut self, x: usize, y: usize) {
        let (rx, ry) = (self.find(x), self.find(y));
        if rx != ry {
            match self.rank[rx].cmp(&self.rank[ry]) {
                Ordering::Less => self.parent[rx] = ry,
                Ordering::Greater => self.parent[ry] = rx,
                Ordering::Equal => { self.parent[ry] = rx; self.rank[rx] += 1; }
            }
        }
    }
}
```
**O(α(n)) per operation**

### Dijkstra

```rust
fn dijkstra(graph: &Graph, start: Node) -> HashMap<Node, Cost> {
    let mut dist = HashMap::new();
    let mut heap = BinaryHeap::new();
    dist.insert(start, 0);
    heap.push(Reverse((0, start)));
    while let Some(Reverse((d, u))) = heap.pop() {
        if d > *dist.get(&u).unwrap_or(&Cost::MAX) { continue; }
        for (v, weight) in graph.edges(u) {
            let new_dist = d + weight;
            if new_dist < *dist.get(&v).unwrap_or(&Cost::MAX) {
                dist.insert(v, new_dist);
                heap.push(Reverse((new_dist, v)));
            }
        }
    }
    dist
}
```

### Topological Sort (Kahn's Algorithm)

```rust
fn topological_sort(graph: &Graph) -> Option<Vec<Node>> {
    let mut in_degree: HashMap<Node, usize> = HashMap::new();
    for node in graph.nodes() {
        in_degree.entry(node).or_insert(0);
        for neighbor in graph.neighbors(node) {
            *in_degree.entry(neighbor).or_insert(0) += 1;
        }
    }

    let mut queue: VecDeque<Node> = in_degree.iter()
        .filter(|(_, &d)| d == 0)
        .map(|(&n, _)| n)
        .collect();

    let mut result = Vec::with_capacity(graph.node_count());
    while let Some(node) = queue.pop_front() {
        result.push(node);
        for neighbor in graph.neighbors(node) {
            let deg = in_degree.get_mut(&neighbor).unwrap();
            *deg -= 1;
            if *deg == 0 { queue.push_back(neighbor); }
        }
    }

    if result.len() == graph.node_count() { Some(result) } else { None }  // None = cycle
}
```
**Use when:** DAG processing, dependency resolution, build systems
**O(V + E)**

### Graph Traversal with Early Termination

```rust
// BFS with early exit - find first match
fn bfs_find<F>(graph: &Graph, start: Node, predicate: F) -> Option<Node>
where F: Fn(&Node) -> bool
{
    let mut visited = HashSet::new();
    let mut queue = VecDeque::new();
    queue.push_back(start);
    visited.insert(start);

    while let Some(node) = queue.pop_front() {
        if predicate(&node) { return Some(node); }  // Early exit
        for neighbor in graph.neighbors(node) {
            if visited.insert(neighbor) {
                queue.push_back(neighbor);
            }
        }
    }
    None
}

// DFS with early exit - find any path
fn dfs_path(graph: &Graph, start: Node, goal: Node) -> Option<Vec<Node>> {
    let mut visited = HashSet::new();
    let mut path = Vec::new();

    fn dfs(graph: &Graph, node: Node, goal: Node, visited: &mut HashSet<Node>, path: &mut Vec<Node>) -> bool {
        if node == goal { path.push(node); return true; }
        if !visited.insert(node) { return false; }
        path.push(node);
        for neighbor in graph.neighbors(node) {
            if dfs(graph, neighbor, goal, visited, path) { return true; }
        }
        path.pop();
        false
    }

    if dfs(graph, start, goal, &mut visited, &mut path) { Some(path) } else { None }
}
```
**Pattern:** Return early when condition met; avoid visiting entire graph
**Use when:** Finding existence, first match, any valid path

---

## Data Structures

### Selection Matrix

| Access Pattern | Structure | Complexity |
|----------------|-----------|------------|
| Key → Value | `HashMap` | O(1) avg |
| Ordered iteration | `BTreeMap` | O(log n) |
| Range queries | `BTreeMap` | O(log n + k) |
| Prefix matching | Trie | O(key length) |
| Membership (approx) | Bloom filter | O(k) |
| Min/max extraction | `BinaryHeap` | O(log n) |
| FIFO | `VecDeque` | O(1) |
| Set operations | `HashSet` | O(n) |

### Trie

```rust
use radix_trie::Trie;
let mut trie = Trie::new();
trie.insert("hello", 1);
for (k, v) in trie.iter_prefix("hel") { /* ... */ }
```
**Libraries:** `radix_trie`, `fst`

### Bloom Filter

```rust
let mut filter = BloomFilter::new(1000, 0.01);  // 1% false positive
filter.insert(&"item");
if filter.contains(&"item") { /* maybe present */ }
```
**Libraries:** `bloom`, `probabilistic-collections`

---

## Caching

### LRU

```rust
use lru::LruCache;
let mut cache = LruCache::new(NonZeroUsize::new(100).unwrap());
cache.put("key", expensive_compute());
```
**Libraries:** `lru`, `cached`

### Memoization

```rust
use cached::proc_macro::cached;
#[cached(size = 100, time = 60)]  // 100 entries, 60s TTL
fn expensive(x: u64) -> u64 { heavy_computation(x) }
```

**Invalidation:** TTL, LRU, write-through, cache-aside

---

## Serialization

### Format Comparison

| Format | Parse | Size | Schema |
|--------|-------|------|--------|
| bincode | Fastest | Small | No |
| MessagePack | Fast | Small | No |
| protobuf | Fast | Small | Yes |
| simd-json | Medium | Large | No |
| serde_json | Slow | Large | No |

**Rule:** `bincode`/`rkyv` internal, JSON external only.

### Zero-Copy

```rust
use rkyv::{Archive, Deserialize, Serialize};
#[derive(Archive, Deserialize, Serialize)]
struct Data { values: Vec<u64> }

let archived = rkyv::check_archived_root::<Data>(&bytes).unwrap();
println!("{}", archived.values[0]);  // Direct memory access
```
**Libraries:** `rkyv`, `zerocopy`

---

## Strings

### Interning

```rust
use string_interner::{StringInterner, DefaultSymbol};
let mut interner = StringInterner::default();
let sym1 = interner.get_or_intern("hello");
let sym2 = interner.get_or_intern("hello");
assert_eq!(sym1, sym2);  // O(1) comparison
```

### Regex

```rust
// BAD: Compile per iteration
for line in lines { Regex::new(r"\d+").unwrap().find(line); }

// GOOD: Compile once
lazy_static! { static ref RE: Regex = Regex::new(r"\d+").unwrap(); }
for line in lines { RE.find(line); }

// BETTER: RegexSet for multiple patterns
let set = RegexSet::new(&[r"\d+", r"\w+", r"[a-z]+"]).unwrap();
```

### SIMD Search

```rust
use memchr::memmem;
let finder = memmem::Finder::new(b"pattern");
if let Some(pos) = finder.find(haystack) { /* ... */ }
```
**Libraries:** `memchr`, `aho-corasick`

---

## Applicability Checks

### DP
- [ ] Overlapping subproblems? → Memoize
- [ ] Optimal partitioning? → Interval DP
- [ ] DAG with repeated traversal? → Topological DP

### Data Structure Selection
- [ ] Point lookups? → HashMap
- [ ] Range queries? → BTreeMap / Segment tree
- [ ] Prefix ops? → Trie
- [ ] Approximate membership? → Bloom filter

---

## Library Quick Reference

| Category | Libraries |
|----------|-----------|
| Hashing | `ahash`, `rustc-hash` |
| Concurrency | `rayon`, `crossbeam` |
| Serialization | `bincode`, `rkyv` |
| Strings | `memchr`, `aho-corasick` |
| Collections | `smallvec`, `indexmap` |
| Caching | `lru`, `cached` |
| Allocation | `bumpalo`, `typed-arena` |
