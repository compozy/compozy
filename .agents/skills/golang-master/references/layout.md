# Module and Package Layout

Right-size structure to the problem: a 100-line tool stays flat; layers appear when complexity demands them, not before. Never impose clean/hexagonal/DDD scaffolding on a small project — and when a project already has an architecture, follow it.

## Module

- Module path matches the repository URL, lowercase, hyphens for multi-word: `github.com/org/payment-processor`.
- The `go` directive states the real minimum version the code needs — it gates which stdlib features compile.
- `go get` / `go mod tidy` manage `go.mod`; hand-editing dependency lines invites resolution drift.
- Tool dependencies use `tool` directives (1.24+), not blank-import `tools.go` files.

## Directories

| Directory | Holds | Rule |
| --- | --- | --- |
| `cmd/<name>/` | One `main` package per binary | Thin: parse flags, wire dependencies, call `Run()` — business logic lives elsewhere |
| `internal/` | Private packages | The compiler enforces privacy; default home for application code |
| `pkg/` | Deliberately public library code | Only when external consumers are a goal; skip it otherwise |
| `testdata/` | Test fixtures | Ignored by the toolchain |
| `api/`, `web/`, `docs/` | Contracts, assets, docs | As needed |

- Packages are named for what they provide, not what they contain: `session`, `billing` — never `utils`, `helpers`, `common`, `models`. A grab-bag package accretes forever and imports everything.
- Package name == directory name, lowercase, singular.
- One package per responsibility; split when a package serves two audiences or its name needs "and". File-level organization inside a package scales a long way — sub-packages are for genuine boundaries, not tidiness.
- No cyclic escape hatches: if two packages need each other, the boundary is wrong — extract the shared type downward or merge.

## Application shape

- `main` wires the object graph explicitly: read config, construct dependencies bottom-up through constructors, inject via interfaces, run, shut down gracefully on signal. No `init()` wiring, no global registries.
- Follow 12-factor for services: config from environment/flags, logs to stdout, stateless processes, graceful shutdown, one-off admin tasks as their own `cmd/` binaries.
- Libraries never call `log.Fatal`/`os.Exit` outside `main`, never print, never parse flags — they return errors and accept configuration.

## Workspaces

`go.work` for developing multiple local modules together (monorepo or cross-repo work): `go work init ./svc-a ./svc-b`. Workspaces are a local development convenience — `go.work` stays out of version control unless the repo is genuinely a multi-module monorepo that builds through it.
