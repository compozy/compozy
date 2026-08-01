# Safety

Defensive coding against the bugs Go lets you write: panics, silent data corruption, and wraparound. Security handles attackers; safety handles ourselves.

## Nil

### The nil interface trap

An interface value is a (type, value) pair and equals `nil` only when **both** are nil. Returning a typed nil pointer sets the type descriptor, producing an interface that is `!= nil`:

```go
// Dangerous — returns interface{type: *MyHandler, value: nil}, which is != nil
func handler() http.Handler {
    var h *MyHandler
    if !enabled {
        return h
    }
    return h
}

// Right — return literal nil for the nil case
func handler() http.Handler {
    if !enabled {
        return nil
    }
    return &MyHandler{}
}
```

The same trap applies to errors: a function returning `*MyError` as `error` returns non-nil even when the pointer is nil — declare the return type as `error` and return literal `nil`.

### Nil zero-value behavior

| Type | Read/index nil | Write to nil | `len`/`cap` | `range` over nil |
| --- | --- | --- | --- | --- |
| Map | Zero value | **panic** | 0 | 0 iterations |
| Slice | **panic** (index) | **panic** (index) | 0 | 0 iterations |
| Channel | Blocks forever | Blocks forever | 0 | Blocks forever |

Initialize maps before writing, or lazy-init inside methods to keep the zero value usable:

```go
func (r *Registry) Add(name string, item Item) {
    if r.items == nil {
        r.items = make(map[string]Item)
    }
    r.items[name] = item
}
```

### Assertions

- Comma-ok always: `v, ok := x.(T)` — the bare form panics on mismatch.
- Reflection (1.25+): `reflect.TypeAssert[T](value)` over `value.Interface().(T)`.

## Slices and maps

### The append aliasing trap

`append` reuses the backing array when capacity allows — the old and new slice then share memory and mutate each other:

```go
a := make([]int, 3, 5)
b := append(a, 4)
b[0] = 99            // also changes a[0]

b := append(a[:len(a):len(a)], 4)   // full slice expression forces a fresh array
```

Corollaries:

- A subslice of a large buffer pins the entire backing array in memory — `slices.Clone` the piece you keep.
- Exported functions return **defensive copies** of internal slices/maps (`slices.Clone`, `maps.Clone`); returning the internal reference hands callers a mutation path into your struct.
- Function parameters of slice/map type are shared with the caller — mutate only when that is the documented contract.

### Concurrent map access

Concurrent read/write on a plain map is a hard runtime crash, not a race you might win. Guard with `sync.RWMutex` or use `sync.Map` for read-heavy cases — see `concurrency.md`.

## Numbers

- Narrowing conversions truncate silently — `int32(3_000_000_000)` wraps negative. Bounds-check against `math.MaxInt32`/`math.MinInt32` (or the target's limits) before converting anything externally sourced.
- Floats are inexact: `0.1+0.2 != 0.3`. Compare with an epsilon (`math.Abs(a-b) < 1e-9`) or use integers/`math/big` for money and counters.
- Integer division by zero panics; float division by zero yields `±Inf`/`NaN`. Guard denominators that can be zero.

## Resources

`defer` runs at **function** exit, not loop iteration — deferred cleanups accumulate across iterations until the function returns. Extract the loop body:

```go
for _, path := range paths {
    if err := processOne(path); err != nil {
        return err
    }
}

func processOne(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    return process(f)
}
```

- `defer x.Close()` goes on the line after the successful open — not fifty lines later.
- Close errors on **write** resources matter (buffered data flushes on close) — check them; read-only closes may be deferred unchecked with justification.

## Initialization

- Design zero values to be useful (`bytes.Buffer`, `sync.Mutex` work uninitialized). When a struct needs a live map or channel, give it a constructor or lazy-init and document that the zero value is not ready.
- `sync.Once`/`sync.OnceValue` for lazy init under concurrency — a boolean flag is a race.
- Structs containing a mutex, channel, or internal pointers must not be copied after first use. Embed a `noCopy` sentinel (`Lock()`/`Unlock()` no-op methods) so `go vet` flags copies, and pass such structs by pointer — the technique `sync.WaitGroup` and `strings.Builder` use.

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Bare assertion `v := x.(T)` | `v, ok := x.(T)` and handle `!ok` |
| Typed nil returned as interface | Return literal `nil` |
| Write to nil map | `make` it or lazy-init |
| Assuming `append` copies | `s[:len(s):len(s)]` to force a copy when the original lives on |
| `defer` in loop | Extract the body into a function |
| Narrowing conversion unchecked | Bounds-check first |
| `==` on floats | Epsilon comparison |
| Returning internal slice/map | Return `slices.Clone`/`maps.Clone` |
| `init()` ordering assumptions | Cross-file `init` order is unspecified — use explicit constructors |
| Send/receive on nil channel | Initialize before use; nil channels block forever |
