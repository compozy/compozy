# Testing

Tests are executable specifications: they constrain observable behavior through the public contract, not implementation details, and they exist to fail on the named regression — never to satisfy a coverage number. The standard library plus `github.com/google/go-cmp` covers almost every assertion; adopt an assertion framework only when the project already uses one.

## Shape

Table-driven with named subtests is the default form:

```go
func TestCalculatePrice(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name     string
        quantity int
        want     float64
        wantErr  error
    }{
        {name: "single item", quantity: 1, want: 10.0},
        {name: "bulk discount", quantity: 100, want: 900.0},
        {name: "negative quantity", quantity: -1, wantErr: ErrInvalidQuantity},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := CalculatePrice(tt.quantity, 10.0)
            if !errors.Is(err, tt.wantErr) {
                t.Fatalf("CalculatePrice() error = %v, want %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("CalculatePrice() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

- Subtest names are lowercase descriptive phrases; error cases live in the same table as success cases.
- `t.Fatal` for preconditions the rest of the test depends on; `t.Error` to accumulate independent assertion failures.
- Failure messages state got and want — a bare "failed" forces a debugger session.
- Assert on behavior: returned values, emitted errors (`errors.Is`), observable state. Asserting call counts and internal sequencing freezes the implementation.

## Files and independence

- `foo.go` → `foo_test.go`, same order of test functions as source functions. Benchmarks may split into `foo_bench_test.go`.
- Same-package tests see unexported code (white-box); `package foo_test` exercises the public API (black-box) — prefer black-box for library surfaces.
- Every test runs alone and in any order: no shared mutable globals, no inter-test state, no execution-order dependence.
- `t.Parallel()` on every independent test and subtest. Tests that mutate env (`t.Setenv`) or global state stay serial — and `t.Setenv` enforces this by failing under `t.Parallel()`.
- `t.Context()` (1.24+) for a per-test context canceled at test end; `t.TempDir()` for scratch dirs; `t.Cleanup` over manual defers in helpers.
- Helpers call `t.Helper()` so failures report the caller's line.
- Fixtures live in `testdata/` (ignored by the toolchain).

## Concurrency in tests

- The suite runs under `-race` in CI, always — a race that only manifests under load is still a bug today.
- Packages that own goroutines verify leak-freedom: `goleak.VerifyTestMain(m)` in `TestMain`, or `defer goleak.VerifyNone(t)` per test.
- Time-dependent concurrent code uses `testing/synctest` (1.25+): inside `synctest.Test`, `time.Sleep`/timers advance a synthetic clock deterministically — no real sleeps, no flakes. Sleeps as synchronization are forbidden; a test that needs one has an ordering bug.

## Integration boundaries

- Build-tag integration tests apart from unit tests: `//go:build integration`, run via `go test -tags=integration ./...`. Unit tests stay fast (<1ms) and dependency-free.
- Mock at interfaces you consume, never concrete types — a hand-written stub struct satisfying the consumer-side interface is usually enough.
- Prefer real implementations when cheap: `httptest.NewServer` for HTTP, in-memory/on-disk SQLite for SQL, `net.Pipe` for connections. A mock of a protocol you don't control tests your guess, not the protocol.

## Fuzzing and examples

```go
func FuzzReverse(f *testing.F) {
    f.Add("hello")
    f.Fuzz(func(t *testing.T, in string) {
        if got := Reverse(Reverse(in)); got != in {
            t.Errorf("double reverse = %q, want %q", got, in)
        }
    })
}
```

Fuzz parsers, decoders, and anything consuming untrusted bytes; assert invariants (round-trip, no panic), not exact outputs. `Example*` functions with `// Output:` comments are compiler-verified documentation for library surfaces.

## Benchmarks

```go
func BenchmarkParse(b *testing.B) {
    data := loadFixture(b, "large.json") // setup excluded from timing
    b.ReportAllocs()
    for b.Loop() {                       // Go 1.24+; keeps inputs/results alive
        Parse(data)
    }
}
```

- `b.Loop()` (1.24+) over `b.N` loops — it times only the body and defeats dead-code elimination without sink variables.
- Size variants via `b.Run(fmt.Sprintf("size=%d", n), …)`.
- Comparison methodology, profiling from benchmarks, and `benchstat` live in `performance.md`.

## Commands

```bash
go test ./...                        # all
go test -run TestName/subtest ./...  # one subtest
go test -race ./...                  # race detector — the default posture
go test -tags=integration ./...      # integration lane
go test -bench=. -benchmem ./...     # benchmarks
go test -fuzz=FuzzName ./pkg         # fuzzing
go test -coverprofile=cover.out ./... && go tool cover -func=cover.out
```
