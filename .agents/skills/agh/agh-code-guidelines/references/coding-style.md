# AGH Coding Style — Canonical Rules

Verbatim canonical rules. Reviewers will quote these.

## Errors

- Wrap with context using `%w`: `fmt.Errorf("operation: %w", err)`.
- Match with `errors.Is` and `errors.As`. **`strings.Contains(err.Error(), …)` is forbidden.**
- Never ignore an error with `_`. Every error is handled or has a written justification on the line.
- No `panic()` or `log.Fatal()` in production paths — only for unrecoverable startup failures inside `main`.

## Cleanup

- Pair `defer cancel()` immediately after `WithCancel` / `WithTimeout` / `WithDeadline`.
- Every error-return path that previously created or extended a context, registered a resource, opened a connection, or spawned a subprocess MUST `cancel()`, `Close()`, `Stop()`, or release its lease before returning. See `agh-cleanup-failure-paths` for the full audit pattern.

## Logging

- `log/slog` for structured logging. `log.Printf`, `fmt.Println`, `fmt.Printf` are forbidden in operational paths.
- Include correlation keys when relevant: `workspace_id`, `session_id`, `parent_session_id`, `root_session_id`, `agent_name`, `task_id`, `run_id`, `claim_token_hash`, `lease_until`, `workflow_id`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `spawn_depth`, `actor_kind`, `actor_id`, `release_reason`.

## Context

- `context.Context` is the first argument of any function that crosses a runtime boundary.
- `context.Background()` is forbidden outside `main` and focused tests.
- Detached execution (work that outlives the request) uses `context.WithoutCancel(ctx)`. `WithoutCancel` does NOT preserve deadlines — re-attach with `WithDeadline` if needed.

## Types and Interfaces

- New exported types implementing an interface get `var _ Interface = (*Type)(nil)` adjacent to the type. **Mandatory.**
- No `interface{}` / `any` when a concrete type is known.
- No reflection without a written performance justification.
- No defensive `if x == nil` checks after `make(...)`. Lint flags this as unreachable.

## Configuration

- Never hardcode operational values. Use TOML config (`internal/config`) or functional options (`NewManager(opts ...Option)`).
- Disable / zero-value semantics must be explicit. Document whether `0` means "off" or "use default".
- Resolution chains (env → flag → config → default) are documented as ordered fallbacks ending in actionable errors.
- Config lifecycle is part of the feature lifecycle: structs, defaults, merge/overlay behavior, validation, examples, `config.toml` docs, generated CLI/site docs, and tests update in the same change. If no config change is needed, the TechSpec says why explicitly.

## CLI Boundary

- Distinguish "flag not set" from "flag set to zero value" via `cmd.Flags().Changed(name)` (Cobra) or equivalent. Silently ignoring an explicit flag is a bug.
- String-slice inputs (capabilities, IDs, tags, paths) trim and drop empty entries before sending to the daemon. Whitespace-only strings must not be pushed as "validation problems".
- `-o json` and `-o jsonl` are compatibility contracts. No command aliases (no `done`, no `pass`).
- Operator endpoints MUST NOT infer agent identity from environment variables — that path belongs to `internal/agentidentity` for agent-facing CLI.

## Comments

- Default: write no comments. Well-named identifiers carry the WHAT.
- Comments capture WHY when non-obvious: hidden constraints, invariants, workarounds for a specific bug, surprising behavior.
- Never reference the current task, fix, callers, or issue number ("used by X", "added for Y flow", "handles the case from issue #123"). Those rot.
- No multi-paragraph docstrings or multi-line comment blocks. One short line max.

## Outbound Calls

- `http.DefaultClient` is forbidden in production paths.
- Every outbound HTTP/network call uses a client with an explicit timeout.
- Drain response bodies (`io.Copy(io.Discard, resp.Body)` then `resp.Body.Close()`) — do not skip the drain.

## Architecture Discipline (cross-package)

- Interfaces defined where consumed (Go-style): `session/` defines `AgentDriver`, `acp/` implements it.
- Direct function calls through interfaces. Outside the `internal/network` NATS boundary, do not add event buses, NATS routing, or reflection-based routing.
- No back-pointers between packages — inject callbacks or interfaces.
- Functional options for constructors: `NewManager(opts ...Option)`.
- Maps for <10 items — no registry interfaces for small collections.
- File-level organization within packages — sub-packages only when complexity justifies it.
- `internal/api/core` is the canonical handler home. REST/UDS endpoints exist as shared `BaseHandlers` methods; HTTP and UDS only choose registration and authentication. No transport-duplicated parsing/validation.
- New `internal/api/*` subpackage requires updating `magefiles/boundaries.go` in the same commit (CI-enforceable boundaries prevent import cycles).
