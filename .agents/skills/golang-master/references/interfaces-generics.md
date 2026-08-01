# Interfaces, Structs, and Generics

> "The bigger the interface, the weaker the abstraction." — Go Proverbs

## Interface design

- **1–3 methods.** Larger contracts compose from small ones (`ReadWriteCloser` = `Reader` + `Writer` + `Closer`).
- **Defined where consumed, not where implemented.** The consumer declares only what it needs; implementors stay ignorant of the interface and don't get imported for their types:

```go
// package notification — the consumer owns the contract
type Sender interface {
    Send(ctx context.Context, to, body string) error
}

type Service struct{ sender Sender }
```

- **Accept interfaces, return structs.** Constructors return `*Service`, never `ServiceInterface` — callers get the full concrete API and can still assign to an interface.
- **Discover interfaces, don't design them.** No interface until a second implementation or a test boundary demands one; a one-implementation interface is indirection with no payoff.
- Honor canonical method names — a `String() string` must satisfy `fmt.Stringer`; never invent `ToString()`.
- **Compile-time check** beside every exported type that exists to satisfy an interface:

```go
var _ io.ReadWriter = (*MyBuffer)(nil)
```

- Optional capability via assertion — the stdlib pattern (`http.Flusher`, `io.ReaderFrom`):

```go
if f, ok := w.(interface{ Flush() error }); ok {
    return f.Flush()
}
```

## Struct design

### Embedding vs named field

| Use | When |
| --- | --- |
| Embed | The outer type should expose the inner type's full API — "is an enhanced" |
| Named field | The inner type is an internal dependency — "has a" |

Promoted methods keep the *inner* type as receiver; the outer type overrides by redeclaring. Embedding to save typing on two delegating methods is a named field's job.

### Receivers

| Pointer `(s *Server)` | Value `(s Server)` |
| --- | --- |
| Method mutates the receiver | Small immutable receiver |
| Receiver holds a mutex/channel/noCopy | Basic types, tiny structs |
| Large struct (copy cost) | Map/func/chan receivers (already references) |

One type, one receiver kind — if any method needs a pointer, all methods take pointers.

### Field tags

Every exported field on a serialized struct is tagged; untagged fields silently leak with Go-cased names:

```go
type Order struct {
    ID        string    `json:"id"`
    Total     float64   `json:"total"`
    Internal  string    `json:"-"`
    Note      string    `json:"note,omitempty"`
}
```

## Dependency injection

Constructor injection through interfaces is the whole pattern — no framework required:

```go
type UserStore interface {
    FindByID(ctx context.Context, id string) (*User, error)
}

func NewUserService(store UserStore) *UserService {
    return &UserService{store: store}
}
```

Tests hand in a stub satisfying `UserStore`. Package-level singletons and `init()` wiring hide the graph and serialize tests — inject explicitly from `main`.

## Generics

Reach for a type parameter when the **type set is known** and the logic is identical across it; reach for an interface when behavior varies by type. `any` survives only at true unknowns (JSON decoding, reflection boundaries).

```go
// Type-safe, no boxing, no assertions at call sites
func Contains[T comparable](s []T, target T) bool

// Constraint from the stdlib (Go 1.21+): cmp.Ordered
func Max[T cmp.Ordered](a, b T) T {
    if a > b {
        return a
    }
    return b
}
```

### Constraints

| Constraint | Admits | Source |
| --- | --- | --- |
| `any` | Everything | builtin |
| `comparable` | Types supporting `==`/`!=` (map keys) | builtin |
| `cmp.Ordered` | Types supporting `<` `>` `<=` `>=` | stdlib 1.21+ |
| `interface{ A \| B }` | Union of exact types | declared |
| `interface{ ~int }` | `int` and every `type X int` | declared |
| Method sets | Types with those methods | declared |

- `~T` (approximation) is what makes constraints work with named types — a union without `~` rejects `type Duration int64`.
- Mixed unions can't carry methods; switch on `any(v).(type)` inside when per-type behavior is unavoidable.

### Patterns

- Zero value of a type parameter: `var zero T; return zero, false`.
- Generic containers hold the type parameter on the struct: `type Stack[T any] struct{ items []T }`; methods are declared on `Stack[T]`.
- Let inference work — call `Max(3, 5)`, not `Max[int](3, 5)`; spell type arguments only when inference fails (return-type-only parameters).
- Before writing a generic slice/map helper, check `slices` and `maps` (stdlib 1.21+) — `slices.Contains`, `slices.SortFunc`, `maps.Keys` already exist.
- Don't genericize an algorithm used with exactly one type — the concrete version is simpler and compiles faster. Extract the type parameter when the second type arrives.

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Interface with 5+ methods | Split and compose |
| Interface declared beside its implementation | Move to the consumer package |
| Constructor returning an interface | Return the concrete type |
| Premature interface, one implementation | Start concrete; extract later |
| Mixed pointer/value receivers on one type | Unify |
| Untagged exported fields on serialized structs | Tag every field |
| `any` where the type set is known | Type parameter or concrete type |
| Hand-rolled `Contains`/`Keys` | `slices`/`maps` stdlib packages |
