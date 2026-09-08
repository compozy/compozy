# Go Runtime

Applies to `internal/` and `cmd/compozy`. Root `CLAUDE.md` owns compatibility, delivery, and test placement. Use `eng-code-guidelines` for project Go conventions and `eng-test-conventions` when editing Go tests; load other references for the particular problem.

## Architecture

- `internal/daemon` is the composition root. Other packages do not import `daemon`, `api`, or `cli`; inject consumed interfaces/callbacks and keep dependencies downward. Boot reconciliation belongs to the composition root.
- Keep packages flat where practical, interfaces at their consumer, constructors with functional options, and files cohesive. Update `magefiles/boundaries.go` when adding a boundary it governs.
- Domain calls use typed interfaces; Notifier fan-out carries observability/SSE. Do not introduce a generic event bus or reflection router. Network acceptance commits before notification.
- `internal/api/core` owns shared `BaseHandlers`; HTTP/UDS choose registration and authentication rather than duplicate parsing/validation.
- Authoritative state transitions have one owner. Peers may observe/wake/sweep, but do not reproduce claim/spawn/migration ownership. The mechanical scheduler does not call `ClaimNextRun`.
- Hooks dispatch at the owning transition, not by tailing logs. They may deny/narrow/annotate but cannot bypass claims, leases, TTL, lineage, spawn limits, or permission narrowing.
- Runtime capabilities expose structured agent operations; co-ship only the contracts, transports, CLI, config, extensions, docs, and tests affected by the change. Audit read authorization, scope, ordering, pagination, and cache completeness with `eng-data-boundaries` when those contracts change.

## Lifetime and Events

- Session Manager owns and joins its spawned goroutines. Keep goroutine channels in per-run handles instead of mutable shared fields.
- Work that outlives a request uses an independent execution lifetime (`context.WithoutCancel` where appropriate), explicit cancellation, and any needed replacement deadline. Client disconnect stops streaming, not the execution.
- Managed subprocess stop respects cancellation between Shutdown and Wait. Centralize signaling in `internal/procutil`; preserve Unix process-group and Windows forced-exit behavior. Cross-build affected subprocess code for Windows before claiming platform parity.
- Append canonical lifecycle events durably before broadcasting. Preserve applicable correlation/ownership keys, token hashes, and `after_seq` replay fences; verify changed emit paths in the existing lifecycle suite.
- The append-only runtime event ledger is authoritative; projections do not replace it. Confirm the owning stream and paths in current code rather than infer them from historical filenames.
- Single-binary/local-first is the runtime boundary. New sidecars/control planes require an explicit design decision. Daemon operation is background by default; preserve `compozy exec` headless text/JSON and opt-in TUI/persistence contracts.

## Persistence

- Use `eng-schema-migration` for SQLite shape changes: update the owning declarative schema, append the next gap-free Goose migration, regenerate `atlas.sum`/sqlc via `make codegen`, and pass `make codegen-check`.
- Existing migration bytes, versions, order, and recorded history are immutable. Integrity/ahead failures remain explicit; never bypass checks or repair schema opportunistically at boot.
- Released user databases must upgrade losslessly under SD-013. If an older shape lacks an upgrade path, design an explicit migration with preserved-data evidence; do not call it disposable alpha state or silently reset it. Unknown/corrupt state remains unchanged while diagnosed.
- sqlc owns static SQL, kept package-private behind repository mappings. Structural dynamic SQL needs a named owner and reason.
- Extend the owning fresh/reopen/ahead/integrity/equivalence suites for the changed invariant. Keep `t.Run("Should …")`, parallel-safe defaults, `CGO_ENABLED=1`/`-race`, integration/E2E build tags, status-plus-body assertions, and the project coverage floor; reuse current suite coverage instead of duplicating tests.

## Security

- Raw claim tokens, MCP auth tokens, OAuth codes, PKCE verifiers, and bound secrets stay out of logs, status/error payloads, SSE, UI, and memory. Expose hash forms; reject raw claim tokens in network metadata.
- Resolve symlinks and enforce approved-root containment for skill/extension paths, including canonicalized macOS temporary roots. User/agent-controlled paths use the existing sanitization and deepest-existing-realpath helpers.
- A signed-format network identity without valid proof is rejected, never downgraded to unverified.
- Outbound calls use explicit timeouts. Non-bundled skills run `internal/skills.VerifyContent` on load; preserve the configured severity decisions and bundled immutability exception.
- Provider authentication ownership is explicit: `native_cli` uses native login without Compozy credential slots; `bound_secret` injects only declared resolved secrets; `none` injects neither. Preserve filtered/isolated environment policies and provider-home isolation without copying operator credentials. Public config changes follow SD-013 boundary translation.

## Targeted Runtime References

Read current skill-loader/config code when changing precedence: Bundled → Marketplace → User → Profile → Additional → Workspace → Workspace-Profile → Agent-local; configured overlay roots replace hardcoded paths. Preserve shadow audit trails.

For memory changes, preserve `user|feedback|project|reference` types, `agent|workspace|global` scopes, per-agent write scope, and the Time → Sessions → Lock consolidation gate order. For lifecycle hooks, preserve hierarchy/alphabetical order, configurable timeout, JSON stdin, and fail-open error reporting.

Bug fixes use a narrow reproduction or existing incident evidence; record uncertainty when reproduction is unavailable. Startup-pending sessions are not crashed, and stale ACP session IDs require classification before a fresh-start fallback. Add broader investigation only when the local evidence does not explain the failure.
