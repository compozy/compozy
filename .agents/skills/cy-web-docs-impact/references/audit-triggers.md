# Audit Triggers

A backend task has downstream web/docs, agent-manageability, extensibility, or config lifecycle impact whenever it touches one of the surfaces below. The skill scans the task's "Files / Surfaces" or equivalent section for these triggers.

## Backend triggers (mandatory audit)

- `internal/api/contract/**` — any change here forces a `make codegen` co-ship.
- `internal/api/core/**` — `BaseHandlers` are shared across HTTP and UDS; web hooks consume them.
- `internal/api/httpapi/**` — HTTP/SSE responses feed the React SPA.
- `internal/api/udsapi/**` — UDS responses feed CLI consumers; web indirectly via HTTP parity.
- `internal/cli/**` — every new or renamed CLI verb regenerates `packages/site/content/runtime/cli/` via `make cli-docs`.
- `cmd/agh/**` — top-level binary changes affect docs/getting-started/snippets.
- `internal/config/**` — new TOML keys or sections affect `packages/site/content/runtime/configuration/`.
- `internal/extension/**`, `internal/bridgesdk/**`, `internal/bridges/**` — extension and bridge surfaces affect the runtime extensibility contract.
- `internal/hooks/**` — hook taxonomy or payload changes affect extension authors and agent workflows.
- `internal/skills/**`, `internal/tools/**`, `internal/resources/**`, `internal/bundles/**`, `internal/registry/**`, `internal/mcp/**` — agent capability and extensibility surfaces require docs and manageability review.
- `openapi/agh.json` — direct OpenAPI edits are the same as `internal/api/contract/**`.
- Any new exported type with `var _ Interface = (*Type)(nil)` that crosses the contract boundary.

## Soft triggers (audit recommended)

- `internal/observe/**` — new metrics or log fields surface in operator dashboards (`web/src/systems/observability`).
- `internal/automation/**` — automation lifecycle changes affect `/jobs` and `/triggers` UI.
- `internal/network/**` — wire-format or capability changes affect `web/src/systems/network`, agent operations, and protocol docs.
- `internal/memory/**` — memory health/history changes affect memory views.
- `internal/scheduler/**`, `internal/task/**`, `internal/coordinator/**` — autonomy changes propagate to Tasks/Sessions/Coordinator UI per the contract co-ship rule.

## No-trigger (typical no-impact)

- `internal/{procutil,fileutil,filesnap,frontmatter,registry,workref,retry}` — pure infrastructure.
- `internal/testutil/**` and `internal/api/testutil/**` — test scaffolding only.
- `internal/store/{globaldb,sessiondb}` — schema changes audit through `internal/api/contract` if they reach the wire; otherwise no-impact.
- `internal/version`, `internal/logger` — infrastructure with no UI surface.

## CLI documentation special case

- New cobra commands or new flags MUST regenerate `make cli-docs`. The task's Tests section MUST list this regeneration.
- Renamed CLI verbs are hard-cuts: every doc page using the old verb is rewritten in the same change.

## Config special case

- New TOML keys MUST update structs, defaults, merge/overlay behavior, validation, examples, tests, `packages/site/content/runtime/configuration/`, and relevant runtime/protocol docs that reference defaults.
- Changed defaults or semantics MUST update examples and docs in the same PR.
- Removed keys MUST be deleted from structs, merge/overlay behavior, validation, examples, and docs in the same PR (no "deprecated" mentions).

## Agent-manageability special case

- New user-visible capabilities MUST provide a CLI/HTTP/UDS operation path for agents unless the TechSpec gives a concrete exception.
- CLI commands MUST expose structured output (`-o json` or `-o jsonl`) where agents need to consume state.
- HTTP and UDS routes that represent the same daemon state MUST stay behaviorally aligned.
