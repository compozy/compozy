# Modernize

Replace outdated patterns with their modern equivalents when touching code — and only the code being touched, unless a full modernization sweep was explicitly requested. Check `go.mod`'s `go` directive first; suggest nothing the target version can't compile. The `modernize` analyzer (golangci-lint ≥2.6, `gopls`, and Go 1.26 `go fix`) automates most of these detections.

## Baseline (pre-1.21, still commonly missed)

- `interface{}` → `any`
- Direct error comparison / type assertion on errors → `errors.Is` / `errors.As`
- `strings.Split(s, sep)[0]`-style parsing → `strings.Cut`
- `ioutil.*` → `os` / `io` equivalents

## By version

| Version | Adopt | Replaces |
| --- | --- | --- |
| 1.21 | `slices.Contains/Sort/SortFunc/Index/Clone`, `maps.Clone/Keys/Values` | Hand-rolled loops, `sort.Slice` |
| 1.21 | `min`/`max` builtins, `clear(m)` | Manual comparisons, delete loops |
| 1.21 | `sync.OnceFunc/OnceValue/OnceValues` | `sync.Once` + closure boilerplate |
| 1.21 | `log/slog` | `log.Printf`, unstructured third-party loggers |
| 1.21 | `context.WithoutCancel`, `context.AfterFunc` | Hand-rolled detachment/cleanup goroutines |
| 1.22 | Per-iteration loop variables | `v := v` shadow copies before closures — delete them |
| 1.22 | `for range n` | `for i := 0; i < n; i++` when the index is unused |
| 1.22 | `math/rand/v2` | `math/rand` (+ delete `rand.Seed`) |
| 1.22 | `cmp.Or(v, fallback)` | `if v == "" { v = fallback }` chains |
| 1.23 | Iterators (`iter.Seq`, `slices.Values/Collect`, `maps.Keys`) | Channel-based generators, materialized intermediate slices |
| 1.24 | `os.Root` for user-supplied paths | Manual path-traversal guards |
| 1.24 | `b.Loop()` in benchmarks, `t.Context()` in tests | `b.N` loops with `ResetTimer`/sinks, manual test contexts |
| 1.24 | `tool` directives in `go.mod` | `tools.go` blank-import files |
| 1.24 | `runtime.AddCleanup` | `runtime.SetFinalizer` |
| 1.25 | `wg.Go(fn)` | `wg.Add(1)` + `go func(){ defer wg.Done() … }` |
| 1.25 | `testing/synctest.Test` | Sleep-based concurrent test timing |
| 1.25 | `reflect.TypeAssert[T]` | `value.Interface().(T)` |
| 1.26 | `errors.AsType[T]` | `errors.As` with a declared target variable |
| 1.26 | `ReverseProxy.Rewrite` | `ReverseProxy.Director` |

Deprecated crypto worth flagging on sight: `x/crypto/{sha3,hkdf,pbkdf2}` → stdlib `crypto/*` (1.24); new RSA encryption uses OAEP, not PKCS1v15 (1.26).

## Priority when sweeping

1. **Correctness/safety** — loop-var shadow copies (delete them), `errors.Is/As`, `os.Root` for untrusted paths, deprecated crypto, `govulncheck` findings.
2. **Readability** — `slices`/`maps`, `min`/`max`, `any`, `cmp.Or`, `wg.Go`, `OnceValue`, `range n`.
3. **Gradual** — slog migration, iterator adoption, `b.Loop()`/`t.Context()` in existing tests, PGO, tool directives.

A version upgrade itself: bump the `go` directive, run the full test suite with `-race`, then apply the newly unlocked rows — never the reverse order.
