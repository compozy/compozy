# Internal Backend (Go)

The Go runtime — `internal/*` packages composed by `internal/daemon`, plus the API transports under `internal/api/*`. ACP subprocess management, SQLite persistence, HTTP/SSE + UDS APIs, autonomy kernel, AGH Network. Entry binary lives in `cmd/agh`.

Repo-wide rules (Critical Rules, Workflow, Build, Commits, Skill Dispatch, Memory & Skills RFC, CI/Release) live in the **root `CLAUDE.md`**. This file owns architecture, package boundaries, autonomy contracts, security invariants, and `internal/`-specific debugging/forensics.

## Architecture

### Principles

- **Designed for incremental extension** — new capabilities arrive as new packages wired into `daemon/`, without modifying existing packages. Small interfaces + dependency injection. Every capability plan decides which extension points, hooks, capabilities, tools/resources, bundles, registries, bridge SDKs, and docs must be added, updated, or removed.
- **Pragmatic Flat with Discipline** — packages under `internal/`, API transports grouped under `api/`, no domain/infra split, no event bus.
- **`daemon/` is the sole composition root** — the only package that imports all others. Reconciliation logic running at boot belongs to composition root and is not "legacy support".
- **No package imports `daemon/`, `api/`, or `cli/`** — dependencies flow downward only.
- **Interfaces defined where consumed** (Go-style) — `session/` defines `AgentDriver`, `acp/` implements it.
- **Direct function calls through interfaces** — no event bus or reflection-based routing. Network acceptance commits before in-process notification; daemon packages communicate through interfaces and the Notifier pattern.
- **Notifier pattern for fan-out** — typed interface for observability and SSE, not a generic bus.
- **No back-pointers between packages** — inject callbacks or interfaces.
- **Functional options for constructors** — `NewManager(opts ...Option)`.
- **Maps for <10 items** — no registry interfaces for small collections.
- **File-level organization** within packages — sub-packages only when complexity justifies it. One cohesive responsibility per file, hard cap 500 lines for production Go (root CLAUDE.md Critical Rules: "No god files"): contract/types, registry/wiring, each implementation, and shared helpers live in separate files, split at design time — never accreted into one file and split later.
- **CI-enforceable boundaries** — `mage Boundaries` rules prevent import cycles. Update `magefiles/boundaries.go` in the same commit that introduces a new `internal/api/*` subpackage.
- **Build/CI plumbing tests are exceptional.** Do not parse `.github`, `Makefile`, `magefiles/`, `package.json`, or `turbo.json` to pin exact commands unless the command is a public operator/agent entry point or release contract. Prefer real or dry-run command smoke (`make -n`, `mage -l`, `make verify`, `make codegen-check`, `goreleaser check`) and assert observable lane behavior. Literal script tests must name the contract they protect.
- **`internal/api/core` is the canonical handler home.** REST/UDS endpoints exist as shared `BaseHandlers` methods; HTTP and UDS only choose registration and authentication. No transport-duplicated parsing/validation.
- **Authoritative primitives are exclusive.** When a primitive owns a state transition (`task.Service.ClaimNextRun`, `Spawn`, `EnsureMigration`), no peer package may replicate it. Wake/observe/sweep are allowed; claim/own is not. The mechanical scheduler does not call `ClaimNextRun`.
- **Hooks are typed dispatch, not an event bus.** Dispatch at the call site that owns the state transition. Never tail event/log tables to fire hooks. Hooks may deny/narrow/annotate but cannot bypass safety primitives (claim tokens, leases, TTL, lineage, spawn caps, permission narrowing).
- **Agent-manageable by default.** User-visible runtime capabilities must expose stable machine-readable control surfaces for agents: CLI verbs with `-o json`/`-o jsonl` where relevant, HTTP/UDS parity when state crosses the daemon boundary, discoverable status/config output, and docs that describe the agent path. UI-only manageability is incomplete.
- **No partial-surface completions.** Any change touching a public surface closes the loop end-to-end in one pass: contract → HTTP handler → UDS handler → CLI client → CLI command → extension/config/docs surfaces → tests → docs.
- **Public read boundaries activate `agh-data-boundaries`.** Preserve authorization, scope, ordering, completeness, cursor/fence metadata, and surface parity from store projection through Web cache.

### Concurrency

Generic Go concurrency patterns (goroutine ownership, channels vs mutexes, `select`/`ctx.Done()` discipline, no `time.Sleep` in orchestration) live in `agh-code-guidelines`. Architectural invariants below are load-bearing for design decisions:

- **Goroutines spawned by `internal/session/manager_*.go` MUST be tracked by Manager-owned WaitGroup and joined in Manager shutdown.** Never put goroutine-owned channels in a struct field that another goroutine mutates — use a per-run handle.
- **Detached execution lifetime.** Any work that outlives an HTTP/UDS request — prompts, network channel sends, automation jobs — MUST detach via `context.WithoutCancel(ctx)`. Never tie execution lifetime to request lifetime. Expose explicit cancel endpoints (e.g., `POST /api/workspaces/:workspace_id/sessions/:id/prompt/cancel`).
- **`context.WithoutCancel` does NOT preserve deadlines.** Re-attach a deadline if needed.
- **Subprocess managed-stop** must respect `ctx.Done()` between Shutdown and Wait. Wrap `proc.Wait()` in `select { case <-proc.Done(): case <-ctx.Done(): }`.
- **Process-group supervision parity.** Unix uses process groups; Windows uses forced-exit fallback. Always cross-build with `GOOS=windows GOARCH=amd64 go build` before claiming subprocess work complete. Centralize signaling helpers in `internal/procutil`.

### Runtime

- Single-binary and local-first. Sidecars or external control planes require a written techspec.
- Keep execution paths deterministic and observable.
- **Daemon runs in background by default.** No daemon should require a foreground terminal.
- **`compozy exec` is headless.** `--format text` returns a single string; `--format json` returns a stream of valid JSON objects; the TUI is opt-in via `--tui`. `exec` does not persist artifacts to `.compozy/runs/` unless `--persist` is given.
- **Agent operations must not depend on the web UI.** If agents need to inspect, configure, start, stop, approve, claim, release, or repair a capability, the spec must provide a CLI/HTTP/UDS path with structured output and deterministic errors.

### Observability

- Every domain operation emits a canonical event with correlation keys (`workspace_id`, `session_id`, `parent_session_id`, `root_session_id`, `agent_name`, `task_id`, `run_id`, `claim_token_hash`, `lease_until`, `workflow_id`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `spawn_depth`, `actor_kind`, `actor_id`, `release_reason`).
- Cover with a coverage matrix test that fails if any required lifecycle path doesn't emit its canonical event.
- Append-only event store (`runtime.db`) is the canonical operational ledger; session DBs are projections, not authority.
- Live broadcasters publish only after durable append; reconnect/replay uses `after_seq`.

### Persistence

- **Goose owns runtime migration order.** Each physical SQLite scope has an embedded stream and dedicated Goose version table; append new migrations and never edit an applied identity.
- **Atlas owns schema drift.** Keep each stream's declarative schema and `atlas.sum` aligned with its Goose head; integrity or ahead errors remain hard failures.
- **sqlc owns static SQL.** Generated queries stay package-private behind repository mappings. Handwritten SQL is reserved for structural dynamic builders and must carry an ownership justification.
- **Pre-Goose databases are unsupported alpha state.** Open paths refuse them without mutation; do not add compatibility readers, import bridges, or schema fallbacks.

## Security Invariants

- **`claim_token` redaction is non-negotiable.** Raw `claim_token` (`agh_claim_*`), MCP auth tokens, OAuth codes, PKCE verifiers, and secret bindings MUST NEVER appear in logs, status APIs, settings views, error payloads, channel messages, SSE, web UI, or memory. Use hash forms (`claim_token_hash`) over the wire. Network layer rejects raw `claim_token` in metadata.
- **Symlink escape hardening.** Skill sidecars, skill files, managed-extension dependency copies, and bundle install paths MUST verify resolved targets remain inside approved roots. Use `EvalSymlinks` + path-prefix check, not naive joins. Handle macOS `/private/var/folders` quirk (canonicalize source root before containment check).
- **Path security helpers.** Filesystem helpers resolving user-controlled or agent-controlled paths use the `sanitizePathKey` + `realpathDeepestExisting` pattern (defenses against null-byte, URL-encoded traversal, Unicode normalization, symlink-escape).
- **Identity proof-stripping defense.** In any signed-message processing path (AGH Network v1), an identity in verified format (`nickname@fingerprint`) without valid `proof` MUST classify as `rejected`, not `unverified`.
- **External-call timeouts.** Outbound HTTP/network calls MUST use a client with an explicit timeout. `http.DefaultClient` is forbidden in production code paths.
- **Load-time security scan.** Every non-bundled skill is scanned via `internal/skills.VerifyContent` on every load (not just install). Critical findings block; warning findings log; info findings log silently. Bundled skills are exempt because `go:embed` provides immutability.
- **Provider auth boundary.** Native ACP providers (`auth_mode = native_cli`) use provider-owned
  CLI login/session state and MUST NOT require AGH-bound API-key `credential_slots`. Bound secrets
  are legal only under `auth_mode = bound_secret`, where AGH resolves `env:` or
  `vault:providers/<provider>/<slot>` refs and injects exactly those values. Provider env/home
  policy is part of this security boundary: `env_policy = filtered` strips secret-shaped daemon
  variables, `env_policy = isolated` starts from a minimal allowlist, and
  `home_policy = isolated` points providers at `$AGH_HOME/providers/<provider>` without copying
  operator credentials.

## Package Layout

| Path                            | Responsibility                                                                |
| ------------------------------- | ----------------------------------------------------------------------------- |
| `cmd/agh`                       | Main entry point, CLI binary                                                  |
| `internal/config`               | TOML loading, validation, merge, home paths, agent def parsing                |
| `internal/acp`                  | ACP client: subprocess spawn, JSON-RPC over stdio                             |
| `internal/agentidentity`        | Caller-identity inference from `AGH_SESSION_ID`/`AGH_AGENT`                   |
| `internal/automation`           | Cron, webhook, and scheduled triggers; durable scheduler state                |
| `internal/bridges`              | External messaging adapters (Slack, Telegram, etc.)                           |
| `internal/bridgesdk`            | Bridge SDK / contract types                                                   |
| `internal/bundles`              | Bundle activation projector                                                   |
| `internal/cli`                  | Cobra commands                                                                |
| `internal/codegen`              | OpenAPI → TS generator helpers                                                |
| `internal/coordinator`          | Coordinator-agent bootstrap and lifecycle                                     |
| `internal/daemon`               | Composition root, lock, boot, shutdown                                        |
| `internal/diagnostics`          | Diagnostics + health probes                                                   |
| `internal/e2elane`              | E2E lane harness wiring                                                       |
| `internal/sandbox`              | Sandbox profile resolution and provider runtime                               |
| `internal/extension`            | Extension manifest, registry, host API, install runtime                       |
| `internal/extensiontest`        | Extension test harness                                                        |
| `internal/filesnap`             | File snapshot utilities                                                       |
| `internal/fileutil`             | Shared filesystem helpers                                                     |
| `internal/frontmatter`          | YAML frontmatter parsing                                                      |
| `internal/hooks`                | Typed hook taxonomy + dispatch                                                |
| `internal/logger`               | Structured logging (slog)                                                     |
| `internal/mcp`                  | MCP server lifecycle / sidecars                                               |
| `internal/memory`               | Persistent dual-scope memory (global + workspace + agent), provenance, recall |
| `internal/memory/consolidation` | Dream consolidation runtime (Time → Sessions → Lock gate cascade)             |
| `internal/network`              | AGH Network participation, conversations, durable acceptance, bounded wakes   |
| `internal/observe`              | Event recording, health metrics, query engine                                 |
| `internal/procutil`             | Process utilities, process-group signaling, Windows fallback                  |
| `internal/providerenv`          | Provider env/home isolation helpers shared by session launch and CLI auth     |
| `internal/registry`             | Skill/agent/capability registry helpers                                       |
| `internal/resources`            | Resource projector / codec / validate                                         |
| `internal/retry`                | Retry primitives                                                              |
| `internal/scheduler`            | Mechanical scheduler (idle registry, wakeups, sweep, recovery)                |
| `internal/session`              | Session lifecycle, Manager, state machine                                     |
| `internal/settings`             | Settings overlay/projection                                                   |
| `internal/situation`            | Situation surface providers (`/agent/context`)                                |
| `internal/skills`               | Skills catalog, loader, `VerifyContent`, MCP/hook decl, provenance            |
| `skills/` (repo-root)           | Embedded bundled skill definitions and resources                              |
| `internal/sse`                  | Shared SSE helpers                                                            |
| `internal/store`                | SQLite helpers, migration streams/engine, integrity validation                |
| `internal/store/globaldb`       | Global catalog (`agh.db`): sessions, metadata                                 |
| `internal/store/sessiondb`      | Per-session event store (`events.db`)                                         |
| `internal/subprocess`           | Subprocess signaling primitives                                               |
| `internal/task`                 | Task domain, `task_runs` ownership, `ClaimNextRun`                            |
| `internal/testutil`             | Shared test helpers                                                           |
| `internal/api/contract`         | Shared daemon/CLI/HTTP contract types                                         |
| `internal/api/core`             | Shared handler types (`BaseHandlers`), error mapping, SSE helpers             |
| `internal/api/httpapi`          | HTTP/SSE server (Gin) for web UI                                              |
| `internal/api/udsapi`           | UDS server for CLI IPC                                                        |
| `internal/api/testutil`         | Test helpers for the API layer                                                |
| `internal/toolruntime`          | Tool process registry + interrupts                                            |
| `internal/tools`                | Tool definitions and dispatch                                                 |
| `internal/transcript`           | Canonical replay message assembly from persisted events                       |
| `internal/version`              | Build metadata                                                                |
| `internal/workref`              | Work reference helpers                                                        |
| `internal/workspace`            | Workspace resolver and entity management                                      |

## Memory & Skills Runtime (RFC-backed)

- **Five-layer skill/memory/agent precedence**: Bundled → Marketplace → User → Additional → Workspace, with agent-local overriding all. Higher precedence wins on collision; an audit trail logs every shadow.
- **Memory taxonomy**: `user | feedback | project | reference` types; scopes `agent | workspace | global`. Default write scope declared per agent in `memory.scope`.
- **Memory consolidation gates**: Time → Sessions → Lock cascade ordered by computational cost. Default gates: 24h, 5 touched sessions, file-lock. Never replace gates with naive heuristics.
- **Lifecycle hooks** (`on_session_created`, `on_session_stopped`) execute in hierarchy precedence then alphabetical order; configurable timeout (default 5s); fail-open semantics (errors logged, never block); JSON over stdin.

## Forensic Bug Fixes

- **Bug-fix plans open with confirmed reproduction** (timestamp, command, observed evidence) BEFORE listing changes. "I think" or "probably" is forbidden at the top of a fix plan.
- **Inactive metadata repair must distinguish startup-pending from crashed.** Sessions in `m.pending` are still starting, not failed.
- **Stale ACP session ids must be classified, not propagated.** Convert `Resource not found` to fresh-start fallback.
