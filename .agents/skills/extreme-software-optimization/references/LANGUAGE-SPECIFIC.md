# Language-Specific Profiling & Trouble Spots

## Contents

1. [Rust](#rust)
2. [Go](#go)
3. [TypeScript](#typescript)
4. [Python](#python)
5. [Universal Patterns](#universal-patterns)

---

## Rust

### Profiling

```bash
# CPU flamegraph (best first tool)
cargo flamegraph --root -- ./target/release/binary <args>

# Allocation
heaptrack ./binary <args> && heaptrack_gui heaptrack.binary.*.zst
# Or DHAT (add dhat to Cargo.toml with optional feature)
DHAT_LOG_FILE=dhat.out cargo run --release --features dhat-heap -- <args>

# perf (Linux)
perf record -g --call-graph dwarf ./binary <args> && perf report

# macOS
cargo instruments -t "Time Profiler" --release -- <args>

# Cache misses
valgrind --tool=cachegrind ./binary <args>
```

### Trouble Spots

| Pattern | Problem | Fix |
|---------|---------|-----|
| `.clone()` in loops | Allocs | Refs, `Cow<T>`, `Rc<T>` |
| `String` vs `&str` | Allocs | Accept `&str`, return `Cow<str>` |
| `Vec::push` loop | Reallocations | `Vec::with_capacity(n)` |
| `Box<dyn Trait>` | Vtable + heap | Generics or enum dispatch |
| `Mutex` contention | Lock wait | `RwLock`, sharding, lock-free |
| `.collect::<Vec<_>>()` | Materializes | Keep as iterator |
| `format!()` hot path | Allocs | Pre-alloc buffer, `write!()` |
| Default hasher | SipHash slow | `ahash`, `rustc-hash` |
| `async` overhead | Future alloc | Sync if not I/O bound |

### Grep for Issues

```bash
rg '\.clone\(\)' --type rust -c | sort -t: -k2 -rn | head -20
rg '\.unwrap\(\)' src/ --type rust
rg 'String::from|\.to_string\(\)|format!' --type rust -c
rg 'Box<dyn|&dyn|Arc<dyn' --type rust
rg 'Mutex::new|RwLock::new' --type rust
rg 'Vec::new\(\)' --type rust  # Check if followed by push loop
```

### Optimizations

```rust
// Fast hasher
use rustc_hash::FxHashMap;
let map: FxHashMap<K, V> = FxHashMap::default();

// Stack for small collections
use smallvec::SmallVec;
let items: SmallVec<[Item; 8]> = SmallVec::new();

// Conditional ownership
fn process(input: &str) -> Cow<str> {
    if needs_change { Cow::Owned(modify(input)) }
    else { Cow::Borrowed(input) }
}

// Pre-size
let mut results = Vec::with_capacity(items.len());
let mut map = HashMap::with_capacity(expected_size);

// Avoid format! in hot path
let mut buf = String::with_capacity(100);
write!(buf, "{}: {}", key, value)?;

// Inline hot functions
#[inline]
fn hot_function() { /* ... */ }
```

### Profile-Guided Optimization (PGO)

```bash
# Step 1: Build instrumented binary
RUSTFLAGS="-Cprofile-generate=/tmp/pgo-data" cargo build --release

# Step 2: Run with representative workload
./target/release/binary <typical-args>
./target/release/binary <another-workload>

# Step 3: Merge profile data
llvm-profdata merge -o /tmp/pgo-data/merged.profdata /tmp/pgo-data

# Step 4: Build optimized binary
RUSTFLAGS="-Cprofile-use=/tmp/pgo-data/merged.profdata" cargo build --release
```
**Typical gains:** 10-20% for hot paths
**When to use:** Production binaries, after other optimizations exhausted

---

## Go

### Profiling

```bash
# Add to main.go
import _ "net/http/pprof"
go func() { http.ListenAndServe("localhost:6060", nil) }()

# CPU
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine/blocking
go tool pprof http://localhost:6060/debug/pprof/goroutine
go tool pprof http://localhost:6060/debug/pprof/block

# Tracer
curl -o trace.out http://localhost:6060/debug/pprof/trace?seconds=5
go tool trace trace.out

# Escape analysis
go build -gcflags='-m -m' 2>&1 | grep "escapes to heap"
```

### Trouble Spots

| Pattern | Problem | Fix |
|---------|---------|-----|
| `interface{}` | Boxing | Generics (1.18+), concrete |
| Small allocs in loops | GC pressure | `sync.Pool`, pre-allocate |
| `defer` in hot loops | Overhead | Move outside loop |
| String `+` | Allocs | `strings.Builder` |
| `[]byte` ↔ `string` | Copies | `unsafe` if safe |
| Slice append | Reallocations | `make([]T, 0, cap)` |
| Channel for single val | Overhead | Direct return or atomic |
| Mutex for read-heavy | Block all | `sync.RWMutex` |
| `fmt.Sprintf` hot path | Reflection | `strconv` functions |
| Goroutine per request | Scheduler | Worker pool |

### Grep for Issues

```bash
rg 'interface\{\}|any\b' --type go
rg '\+.*string|string.*\+' --type go
rg 'for.*\{' -A5 --type go | rg 'defer'
rg 'make\(chan.*,\s*[01]\)' --type go
rg 'var.*sync\.(Mutex|RWMutex)' --type go
rg 'fmt\.Sprintf' --type go
rg 'reflect\.' --type go
```

### Optimizations

```go
// Pre-allocate
items := make([]Item, 0, expectedSize)

// strings.Builder
var b strings.Builder
b.Grow(100)
b.WriteString(s1)
b.WriteString(s2)

// sync.Pool
var bufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
buf := bufPool.Get().(*bytes.Buffer)
defer bufPool.Put(buf)
buf.Reset()

// RWMutex for read-heavy
var mu sync.RWMutex
mu.RLock()
defer mu.RUnlock()

// Atomic counters
var counter atomic.Int64
counter.Add(1)

// strconv over fmt
s := strconv.Itoa(n)  // Not fmt.Sprintf("%d", n)
```

### GC Debugging

```bash
# Trace GC activity
GODEBUG=gctrace=1 ./binary

# Output format:
# gc 1 @0.012s 2%: 0.026+0.44+0.003 ms clock, 0.10+0.32/0.40/0+0.012 ms cpu, 4->4->0 MB, 5 MB goal, 4 P
#    │   │   │    │                      │                             │           │        └── processors
#    │   │   │    │                      │                             │           └── target heap
#    │   │   │    │                      │                             └── heap before->after->live
#    │   │   │    │                      └── CPU times (assist/bg/idle mark)
#    │   │   │    └── wall-clock times (sweep/mark/term)
#    │   │   └── CPU % in GC
#    │   └── time since start
#    └── GC cycle number

# Memory allocation tracking
GODEBUG=allocfreetrace=1 ./binary  # Very verbose - every alloc/free

# Schedule tracing (goroutine scheduling)
GODEBUG=schedtrace=1000 ./binary   # Every 1000ms
```
**Use when:** High GC pause times, memory pressure, allocation debugging

### Race Detection

```bash
# Build with race detector
go build -race ./...

# Test with race detector (recommended)
go test -race ./...

# Run with race detector
go run -race main.go
```
**Note:** 10-20x slowdown, 5-10x memory overhead; use in CI, not production
**Finds:** Data races, concurrent map access, improper channel use

---

## TypeScript

### Profiling

```bash
# V8 profiler
node --prof app.js && node --prof-process isolate-*.log > profile.txt

# Chrome DevTools
node --inspect app.js
# Open chrome://inspect

# clinic.js (best)
npm install -g clinic
clinic doctor -- node app.js
clinic flame -- node app.js
clinic bubbleprof -- node app.js

# 0x flamegraphs
npm install -g 0x && 0x app.js

# Event loop
const { monitorEventLoopDelay } = require('perf_hooks');
const h = monitorEventLoopDelay({ resolution: 20 });
h.enable();
setInterval(() => console.log(h.percentile(99)), 1000);
```

### Trouble Spots

| Pattern | Problem | Fix |
|---------|---------|-----|
| `JSON.parse/stringify` loop | CPU | Stream parsing, cache |
| Sync fs | Blocks event loop | `fs.promises` |
| `await` in loop | Sequential | `Promise.all()` |
| `.map().filter()` chain | Multiple passes | Single pass |
| `new Date()` hot path | Alloc | Cache timestamp |
| Regex without compile | Recompile | Const outside function |
| Object spread in loop | Copy | Mutate or pre-allocate |
| `console.log` production | I/O | Remove or log levels |
| `Array.includes` | O(n) | Use `Set` |

### Grep for Issues

```bash
rg 'fs\.(readFileSync|writeFileSync|existsSync)' --type ts
rg 'for.*await|while.*await' --type ts
rg 'JSON\.(parse|stringify)' --type ts
rg 'console\.(log|info|warn|error)' --type ts
rg 'new RegExp' --type ts
rg '\.(map|filter|reduce)\(.*\)\.(map|filter|reduce)' --type ts
```

### Optimizations

```typescript
// Set for membership
const seen = new Set<string>();
if (seen.has(item)) { /* O(1) vs includes O(n) */ }

// Parallel async
const results = await Promise.all(items.map(processAsync));

// Stream large JSON
import { createReadStream } from 'fs';
import { parser } from 'stream-json';
createReadStream('large.json').pipe(parser()).on('data', process);

// Pre-compile regex
const PATTERN = /\d+/g;  // Outside function

// Map over object
const cache = new Map<string, Result>();  // Faster for dynamic keys

// TypedArrays
const data = new Float64Array(1000);  // Faster than number[]

// Worker threads for CPU
import { Worker, isMainThread, parentPort } from 'worker_threads';
```

---

## Python

### Profiling

```bash
# cProfile
python -m cProfile -s cumtime script.py > profile.txt

# snakeviz (visual)
pip install snakeviz && python -m cProfile -o output.prof script.py && snakeviz output.prof

# line_profiler (decorate with @profile)
pip install line_profiler && kernprof -l -v script.py

# py-spy (no code changes, best)
pip install py-spy
py-spy record -o profile.svg -- python script.py
py-spy top --pid <PID>

# memory_profiler
pip install memory_profiler && python -m memory_profiler script.py

# scalene (CPU + memory + GPU)
pip install scalene && scalene script.py
```

### Trouble Spots

| Pattern | Problem | Fix |
|---------|---------|-----|
| String `+=` in loop | O(n²) | `''.join()` |
| `list.append` loop | Resize | List comprehension |
| Global lookup | Slower | Cache in local |
| `in list` | O(n) | `set` |
| Attr access in loop | Dict lookup | Cache as local |
| `import` in function | Overhead | Module level |
| Object creation in loop | Allocs | Reuse or pool |
| `re.match()` | Recompiles | `re.compile()` |
| `pandas.iterrows()` | Extremely slow | Vectorize |

### Grep for Issues

```bash
rg 'for.*:' -A5 --type py | rg '\+='  # String concat
rg '\.append\(' --type py -c | sort -t: -k2 -rn
rg 'in \[|in list' --type py
rg '^\s+import |^\s+from .* import' --type py
rg 're\.(match|search|findall|sub)\(' --type py
rg '\.iterrows\(\)|\.itertuples\(\)' --type py
```

### Optimizations

```python
# String join
result = ''.join(items)  # Not += loop

# List comprehension
result = [x * 2 for x in items]  # Not append loop

# Set for membership
valid = set(valid_items)
if item in valid:  # O(1)

# Local caching
def process(items):
    local_func = expensive_module.func
    for item in items:
        local_func(item)

# Generator for large data
def process_large(items):
    for item in items:
        yield transform(item)

# Compile regex
PATTERN = re.compile(r'\d+')

# __slots__ for many instances
class Point:
    __slots__ = ['x', 'y']

# NumPy vectorize
np.arange(1000000) ** 2  # Not list comprehension

# lru_cache
from functools import lru_cache
@lru_cache(maxsize=128)
def expensive(n): return compute(n)

# Avoid pandas iteration
df['new'] = df['old'].apply(func)  # Not iterrows
df['new'] = np.where(df['old'] > 0, df['old'] * 2, 0)  # Better
```

### GC Debugging

```python
import gc

# Enable GC stats
gc.set_debug(gc.DEBUG_STATS)  # Print collection stats

# More verbose - show collectable objects
gc.set_debug(gc.DEBUG_COLLECTABLE)

# Show uncollectable objects (reference cycles that can't be freed)
gc.set_debug(gc.DEBUG_UNCOLLECTABLE)

# All debug flags
gc.set_debug(gc.DEBUG_STATS | gc.DEBUG_COLLECTABLE | gc.DEBUG_UNCOLLECTABLE)

# Manual GC control
gc.disable()           # Disable automatic GC
gc.collect()           # Force collection
gc.enable()            # Re-enable

# Get GC stats
print(gc.get_stats())  # Generation stats
print(gc.get_count())  # Objects in each generation

# Find reference cycles
gc.collect()           # Collect first
print(gc.garbage)      # Uncollectable objects (reference cycles with __del__)
```
**Use when:** Memory leaks, high GC pause times, debugging reference cycles

### objgraph (Visual Memory Debugging)

```python
import objgraph

# Find what's creating the most objects
objgraph.show_most_common_types(limit=20)

# Find growth between two points
objgraph.show_growth()  # Call multiple times to see delta

# Find references to specific objects
objgraph.show_backrefs([obj], filename='refs.png')

# Find reference chains (why isn't this GC'd?)
objgraph.find_backref_chain(obj, objgraph.is_proper_module)
```
**Install:** `pip install objgraph`

---

## Universal Patterns

### Find Hot Loops

```bash
rg 'for\s*\(|for\s+\w+\s+in|while\s*\(' --type-add 'code:*.{rs,go,ts,py}' -t code
```

### Nested Loops (O(n²))

```bash
rg 'for.*\{' -A20 | rg 'for.*\{'
```

### By Symptom

| Symptom | Grep |
|---------|------|
| High CPU | `clone\|copy\|Clone\|deepcopy` |
| Memory growth | `append\|push\|Add\|\.add\(` |
| Lock contention | `Mutex\|Lock\|synchronized` |
| I/O bound | `read\|write\|fetch\|http` |
| GC pressure | `new \|make\(\|Box::new\|alloc` |
| Serialization | `JSON\|json\|serde\|marshal` |

### Red Flags

| Language | Red Flag |
|----------|----------|
| Rust | `.clone()` 50+ times, `Box<dyn>` hot path, default hasher |
| Go | `interface{}` everywhere, no `sync.Pool`, `defer` in loops |
| TS | `JSON.parse` in handlers, `await` in loops, sync fs |
| Python | `+=` strings, `iterrows()`, `in list` |

---

## Profiling Cheatsheet

| Lang | CPU | Memory | Live |
|------|-----|--------|------|
| Rust | `cargo flamegraph` | `heaptrack` | `perf top` |
| Go | `pprof /profile` | `pprof /heap` | `pprof /goroutine` |
| TS | `clinic flame` | DevTools heap | `clinic doctor` |
| Python | `py-spy record` | `scalene` | `py-spy top` |
