# Spec Authoring Playbook

Use the current phase and changed contract to select guidance. This playbook coordinates authoring; it does not require a separate full read/research pass for each task or checkpoint.

## Shared Decisions

- State the motivating problem, observable outcome, scope, and non-goals. Preserve accepted user goals; record user decisions that narrow them.
- Apply SD-013 to compatibility: user state migrates losslessly, public surfaces deprecate before deletion except documented experimental contracts, and internals hard-cut. List delete targets and regimes where breaking changes occur. L-040 explains the current policy.
- Record extensibility, agent operations, config lifecycle, workspace isolation, and Web/Docs impact once at the owning artifact (`change-impact.md`). Consumers link to it and update only affected entries.
- Reuse approved research, current code findings, and relevant ADRs. Consult competitors only when the task needs that evidence; record actual source paths. Market research is conditional, not a default cost for every spec.

## Product and Surface Design

Write WHAT/WHY/WHO with verifiable acceptance. Technical terms may remain when they are the public contract or an explicit constraint; incidental implementation goes in the technical part (L-013).

Use `_user_stories.md` for distinct journeys. `_dx.md` records agent/operator interactions; an internal-only change may state no public surface. UI-bearing features use `_uiux.md` to map changed surfaces, production owners, and any named visual reference. Do not invent prototype content, controls, or new primitives to match a reference.

Ask only for unresolved product choices after available evidence is used. Approved direction satisfies earlier checkpoints. ADRs record consequential alternatives, not every ordinary implementation decision. Save the spec before offering optional peer review.

## Technical Design

Always make the outcome and architectural boundaries clear. Other markers are conditional (L-012):

| Change | Required design detail |
| --- | --- |
| Interface/API | Concrete changed signatures/schema and consumers |
| Persistent/config data | Field purpose, ownership, defaults, and SD-013 upgrade plan |
| Queryable metadata/state | Side-table/JSON choice where there is a real ownership/query trade-off |
| Concurrency, ownership, permissions | Applicable invariants and failure behavior |
| Public/runtime surface | Agent/extension/config/Web/Docs impact and compatibility |

A missing applicable contract is a design gap. An irrelevant marker needs at most a short no-impact entry, never invented interfaces, side tables, or lease rules. Templates and heuristic keyword checks support this review; they do not override semantics.

File References names the relevant contract/code/competitor paths and why each matters. `_tests.md` defines concrete cases with canonical suite ownership; existing coverage or stronger gates may already satisfy an invariant. Testing strategy belongs in the spec, full cases in the catalog, assigned IDs in tasks.

## Compozy Spec Rules

`cy-create-spec` is generic. These rules supply what it leaves to the target project.

- **Vocabulary:** use `docs/_memory/glossary.md` terms and the `COPY.md` register. When a spec coins or shifts a meaning, propose the glossary edit before it closes.
- **Compatibility (SD-013):** classify every delete target as user state, public surface, or internal code. User state migrates losslessly in the same change. Public surfaces auto-migrate, or keep the old shape one release behind the new one through a boundary shim with a named removal release. Internal code hard-cuts in the same change, with "no fallback, no shim" clauses where drift is likely, and specs and `.compozy/tasks/*` artifacts move with it. ADRs may narrow an accepted goal only with the user's recorded decision.
- **Boundaries:** reference the `internal/daemon` composition root, and name new internal packages. Critical Go interfaces appear as code, not prose.
- **Storage:** list new SQLite columns, frontmatter fields, and config keys with purpose and shape. Use side tables for matchable state and JSON for opaque metadata only.
- **Sources:** competitor paths stay relative to `.resources/`, never paraphrased.

### Peer Review Constraints

Add these to the reviewer prompt's project rules:

- `task_runs` is the single durable queue, and `ClaimNextRun` the only authoritative claim primitive. Reject a parallel queue or peer claimer.
- Manual operator paths converge with autonomous ones on the same primitives.
- Hooks dispatch at the call site and never tail event tables.
- A raw `claim_token` never crosses transport, channel, log, or memory.
- Generated artifacts co-ship with their source change.
- Reject partial-surface completion, such as CLI and HTTP now with UDS, docs, or codegen later.

### Lesson Index

Read a lesson only when its question occurs. Paths are relative to `docs/_memory/lessons/`; the README resolves each filename.

| Question | Lessons |
| --- | --- |
| Product vs technical requirements | L-013 |
| Missing concrete technical contracts | L-012 |
| Request and execution lifetime | L-001 |
| Durable work queue and authoritative ownership | L-003, L-004, L-005 |
| Deletion, public compatibility, user upgrades | L-006, L-040, SD-013 |
| SQLite migrations | L-008, L-021 |
| Test ownership and useful coverage | L-002, L-007, L-011 |
| Concurrent QA and teardown | L-009, L-029 |
| Outcomes and proportional verification | L-036, L-037, L-039 |

## Slices

`_slices.md` from `cy-create-spec` owns the delivery graph. Cut by useful outcomes and dependencies; do not force a fixed number of slices or repeat research for each one. A foundation slice names its consumer and validates its own boundary.

Each test ID belongs to exactly one slice, the one completing its behavior. No happy/failure/concurrency quota: add cases for distinct risks. A slice that changes many behaviors with one or two cases records who owns the rest (L-011). For named visual references, enumerate the affected states/viewports and their evidence owner. Reuse shared impact and compatibility analysis through links.

Remaining integration journeys are verified once, after integration. Ordinary fixes use focused tests/probes without a spec.

## Execution

Slice briefs point to the canonical artifacts their worker must read. Reuse a current inventory; expand only for stale, missing, or conflicting dependencies. Do not re-audit unchanged contracts at every checkpoint.

Use the root's scoped verification/delivery rules and each slice's changed-behavior evidence. L-029 teardown applies whenever a lab is created. Historical synthesis and incident counts explain these decisions; they are not a second executable backlog.
