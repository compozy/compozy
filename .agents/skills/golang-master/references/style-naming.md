# Style and Naming

> "Clear is better than clever." — Go Proverbs

Linters own formatting; this file owns the judgment calls.

## Naming quick reference

| Element | Convention | Example |
| --- | --- | --- |
| Package | lowercase, one word, singular | `json`, `tabwriter` — never `utils`, `helpers`, plurals |
| Exported / unexported | `UpperCamel` / `lowerCamel` — never underscores | `ReadAll`, `parseToken` |
| Interface | method + `-er` | `Reader`, `Stringer` |
| Constant | MixedCaps — never `ALL_CAPS` | `MaxRetries`, `defaultTimeout` |
| Receiver | 1–2 letters, consistent across methods | `func (s *Server)` — never `this`/`self` |
| Error variable / type | `Err` prefix / `Error` suffix | `ErrNotFound` / `PathError` |
| Constructor | `New` (single primary type) or `NewTypeName` (several) | `apiclient.New()`, `http.NewRequest` |
| Boolean field/method | `is`/`has`/`can` prefix | `isReady`, `HasPrefix()` |
| Getter / setter | no `Get` prefix / `Set` prefix | `user.Name()` / `user.SetName()` |
| Acronym | all caps or all lower | `HTTPServer`, `xmlParser` — never `HttpServer` |
| Option func | `With` + field | `WithTimeout(d)` |
| Panicking variant | `Must` prefix | `MustParse` |
| Format-string variant | `f` suffix | `Errorf`, `Logf` |
| Enum | type-name prefix, sentinel at 0 | `StatusUnknown = iota`, then `StatusActive` |

High-frequency misses:

- **No stuttering** — call sites include the package name, so `http.HTTPClient`, `user.NewUser()`, `dbpool.PoolStatus` all repeat themselves. Write `http.Client`, `user.New()`, `dbpool.Status`. Applies to every exported name, not just the primary type.
- **Name length matches scope** — `i` for a three-line loop, descriptive names at package level.
- **Error strings fully lowercase, acronyms included, no trailing punctuation** — they compose mid-chain when wrapped.
- **Enum zero value is a sentinel** — `var s Status` is silently 0; if 0 is a real state, uninitialized reads pass as deliberate choices.
- One name per concept — `user`/`account`/`person` for the same thing forces readers to track synonyms.

## Control flow

- **Early return.** Handle errors and edge cases first; the happy path stays at minimal indentation.
- **No `else` after a terminating `if`** (`return`/`continue`/`break` — drop the `else`). For value selection, assign the default first, then override with `switch`/independent `if`s.
- **3+ operands in a condition → named booleans.** `isAdmin || isOwner || isPublicVerified` reads; a wall of `&&`/`||` hides the business rule. Keep expensive checks inline for short-circuiting.
- Scope check-only variables to the `if`: `if err := validate(in); err != nil { return err }`.
- Same variable compared repeatedly → `switch`, with a `default` that handles (or deliberately panics on) the impossible case.
- `range` over index loops; `for range n` (1.22+) for counting.

## Declarations

- `var x T` for deliberate zero values; `:=` for computed ones — the form signals intent.
- Composite literals use field names — positional literals break on the next added field.
- Maps and slices that will be written are initialized (`make`, literal), with capacity when known: `make([]T, 0, len(src))`. No speculative capacities.
- Functions take ≤4 parameters — beyond that, an options struct or functional options. `ctx` first, then inputs, then destinations.
- Small values (`string`, `int`, `time.Time`) pass by value; pointers mean mutation, large structs, or meaningful nil.
- Blank imports (`_ "pkg"`) only in `main` or tests — side-effect registration stays visible at the root. Dot imports never.
- Unexport aggressively — exporting later is free, unexporting is a breaking change.

## Constructors and options

Functional options for any type with optional configuration — they extend without breaking call sites:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 5 * time.Second} // defaults first
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

When option validation can fail, options return `error` and `NewServer` returns `(*Server, error)` — bad config dies at construction, not at first use.

**No `init()`, no mutable package globals.** `init` runs implicitly in unspecified cross-file order, cannot return errors, and fires before tests configure anything. Wire dependencies explicitly in `main` through constructors. Legitimate package-level state is limited to immutable values: `var emailRe = regexp.MustCompile(...)` — compiled once, never reassigned.

## Comments

- Default to none — well-named identifiers carry the *what*.
- Comment the *why* that code cannot express: invariants, workarounds with their trigger, surprising behavior.
- Every exported name in a library gets a doc comment starting with the identifier: `// Serve accepts incoming connections…`.
- Never reference the current task, ticket, or caller ("added for the retry flow") — those rot on the next refactor.

## Misc

- `strconv` over `fmt.Sprintf` for single conversions; `%q` in errors to make string boundaries visible.
- `strings.Builder` for concatenation in loops; `+` for a couple of pieces.
- `[]byte` for I/O and mutation, `string` for keys and display — each conversion between them allocates; stay in one.
- `//go:embed` for static assets — compile-time inclusion beats runtime file reads.
- "A little copying is better than a little dependency" — a 20-line helper does not justify a new module.
