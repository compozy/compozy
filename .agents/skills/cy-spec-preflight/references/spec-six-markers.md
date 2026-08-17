# Spec Six Markers

A Spec's Part II is "ready for review" only when ALL six markers are present. Run `scripts/check-spec-markers.py <spec_path>` to validate. Source: `docs/_memory/lessons/L-012-techspec-prose-only-rework.md` and `docs/_memory/analysis/analysis_compozy_tasks.md` spec quality patterns.

## Marker 1 — MVP Boundary statement

A sentence near the top of Part II spelling out which numbered tasks compose MVP, what is post-MVP, what is out of scope.

Example:
> "MVP boundary: tasks 01-16 implement the autonomy kernel. Tasks 17-18 prepare and execute QA. Post-MVP network evolution, broad memory scopes, self-correction telemetry, eval/replay, and broad web visibility remain follow-up specs unless explicitly pulled into scope."

## Marker 2 — Architectural Boundaries section

A first-class section enumerating which packages can/cannot import which. Names new internal packages explicitly. References `daemon/` composition root.

## Marker 3 — Concrete Go interface signatures

Critical Go interfaces are pasted as code blocks. Every method signature final.

```go
type TaskClaimer interface {
    ClaimNextRun(ctx context.Context, criteria ClaimCriteria) (*ClaimedRun, error)
    Heartbeat(ctx context.Context, runID string, token ClaimToken) error
    Release(ctx context.Context, runID string, token ClaimToken, reason ReleaseReason) error
}
```

## Marker 4 — Data-model field rationale

Any new SQLite columns, frontmatter fields, or config keys are listed with purpose and shape.

```
task_runs.claim_token      TEXT NOT NULL DEFAULT ''  -- opaque ownership token (compozy_claim_*)
task_runs.lease_until       TIMESTAMP NOT NULL DEFAULT 0  -- lease deadline; 0 = unleased
task_runs.coordination_channel_id TEXT  -- workspace-scoped run only; null otherwise
```

## Marker 5 — Side-table-vs-JSON decision

For every new domain entity that could be either a typed column/side-table or a JSON-bag entry, the spec states which choice and why.

> "Capability matching uses `task_run_required_capabilities` and `task_run_preferred_capabilities` side tables (exact-match rows). JSON metadata blobs are forbidden for matchable state because `ClaimNextRun(criteria)` filters via SQL."

## Marker 6 — Lease / safety invariants numbered

Concurrency- or ownership-sensitive code paths spell out invariants as a numbered list rather than prose.

```
1. Exactly one active claim token per non-terminal run.
2. Heartbeat compares both run owner and claim token.
3. Stale/late after recovery fails explicitly.
4. Sweep + heartbeat serialize via SQLite tx (BEGIN IMMEDIATE).
5. Boot recovery before scheduler accepts wake/claim traffic.
6. Lease extension bounded by config.
7. One active lease per session in MVP.
```

## Validation

The check script greps for:

- "MVP boundary" or "MVP Boundary" near the top.
- "Architectural Boundaries" section heading.
- At least one ` ```go` block containing `interface` or `type`.
- A list of fields with rationale (heuristic: a column-style table or a `* field — purpose` list).
- "side-table" or "JSON" keywords in a decision context.
- A numbered list of invariants under a "Invariants" / "Lease" / "Safety" heading.

If any marker fails, print which one and refuse to mark the spec ready.

## Compozy-specific mandatory sections

The six quality markers are not the whole approval gate. Compozy Spec Part II must also include:

- **Extensibility Integration Plan** — extension manifests, hooks, skills/capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, and protocol docs added/changed/removed or explicitly unaffected.
- **Agent Manageability Plan** — CLI verbs, HTTP endpoints, UDS routes, structured outputs, status/config discovery, and deterministic errors agents will use to operate the feature. Must be consistent with `_dx.md`.
- **Config Lifecycle** — `config.toml` keys/defaults, merge/overlay behavior, validation, examples, generated CLI/site docs, and tests added/changed/removed or explicitly unaffected.
- **File References** — the read-first index: repo files, `.resources/<competitor>/path` entries when the design drew on competitors, and `analysis/`/design sources, each annotated with why to read it. Tasks copy their subsets from here.

If a section says "no impact", it must list the surfaces checked and why no change is needed.
