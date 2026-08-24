---
name: batuta-routing
description: Default cost/complexity routing table for the batuta conductor. Read at bootstrap as a starting point, validated against the live provider catalog, then stored as the per-workspace loop configuration; the stored workspace override is authoritative afterwards.
---

# Batuta Routing Table

Batuta's core opinion: route every task to the cheapest executor that can
handle it. Lanes use the `complexity` vocabulary that `cy-create-tasks`
writes into task frontmatter (`low`, `medium`, `high`, `critical`) — the
same vocabulary `runtime_rules[].match.complexity` matches on.

## Lane semantics (the durable opinion)

| Lane       | Intent                                | Selection rule                                             |
| ---------- | ------------------------------------- | ---------------------------------------------------------- |
| `low`      | Contained change, well-trodden paths  | Cheapest coding-capable model in the catalog               |
| `medium`   | New interfaces, moderate coordination | Mid-tier coding model; raise reasoning before raising cost |
| `high`     | New subsystem, heavy reasoning        | Strong coding model, premium tier acceptable               |
| `critical` | Cross-cutting, high regression risk   | The operator's most trusted frontier model                 |

## How batuta derives the concrete table (never copy an example)

1. `compozy__provider_models_list` (with costs) is the ONLY source of
   concrete provider/model IDs — it reflects the CLIs actually installed
   and the models actually discovered on this machine. A provider absent
   from the catalog is not installed; never route to it.
2. Map each lane's selection rule onto the catalog using the cost fields
   (`input_per_million` / `output_per_million`) as evidence.
3. Model enablement is account-side and invisible to the daemon — present
   the derived table (with costs) to the operator for confirmation before
   storing; ask what their accounts enable when in doubt.

### Example only — derived on one machine on 2026-08-11, DO NOT reuse

On that machine the derivation produced: `low → codex/gpt-5.6-luna`,
`medium → codex/gpt-5.6-terra@high`, `high → codex/gpt-5.6-sol`,
`critical → claude/claude-opus-4-8`. Your catalog will differ; derive, do
not copy.

## Canonical rule shape

This is the exact JSON SHAPE batuta writes with `compozy__loop_configure`
(stored per-workspace override for `implement-tasks`) after deriving the
values from the catalog — the model/provider strings below are the same
dated example as above and MUST be replaced by the derived ones. The stored
override is what `run-loop` children resolve at execution — batuta never
sends per-run rules on dispatch, because per-run rules freeze into the run
and are not inherited by `run-loop` children anyway. Rule matching
inside the stored layer accepts exactly `id`, `type`, `complexity`, or
`type + complexity`; the conjunction is AND and `id` is exclusive.
Specificity is `id > type + complexity > type > complexity`. Matching
rules merge runtime fields independently, and a later equal-specificity
rule wins only the non-empty fields it sets.

```json runtime_rules
[
  { "match": { "complexity": "low" }, "runtime": { "provider": "codex", "model": "gpt-5.6-luna" } },
  {
    "match": { "complexity": "medium" },
    "runtime": { "provider": "codex", "model": "gpt-5.6-terra", "reasoning": "high" }
  },
  { "match": { "complexity": "high" }, "runtime": { "provider": "codex", "model": "gpt-5.6-sol" } },
  {
    "match": { "complexity": "critical" },
    "runtime": { "provider": "claude", "model": "claude-opus-4-8" }
  }
]
```

### Conjunctive overlay shape (placeholder values, never store them)

For a `frontend/high` task, these rules resolve provider from the `type`
rule, reasoning from the matrix rule, and model from the later matrix rule:

```json runtime_rules
[
  {
    "match": { "type": "frontend" },
    "runtime": { "provider": "derived-provider", "model": "type-model" }
  },
  { "match": { "type": "frontend", "complexity": "high" }, "runtime": { "reasoning": "high" } },
  { "match": { "type": "frontend", "complexity": "high" }, "runtime": { "model": "matrix-model" } }
]
```

## Provider quirks

- Some providers multiplex upstreams and require the model field to carry a
  prefix — e.g. `opencode` only binds `opencode/kimi-k2.5`, never bare
  `kimi-k2.5`. The catalog's exact `model_id` is authoritative; copy it
  verbatim into the rule.
- A model can exist in the catalog and still be disabled for the operator's
  account at the provider (invisible to the daemon). When a lane fails its
  bind with zero tokens, ask the operator what their account enables.

## Escalation and reclassification

- Repeated failure in a lane: write a surgical `id` rule one lane up into
  the STORED override (`compozy__loop_configure` on `implement-tasks`, e.g.
  `{"match":{"id":"task_NN"},"runtime":{...}}` prepended to the rules), then
  re-dispatch `batuta-deliver`. An `id` rule beats matrix and single-selector
  rules for each non-empty runtime field it sets; remove it after the task lands.
- Operator reclassification in conversation ("use luna for this one")
  becomes the same stored `id` rule before the next dispatch.
- The daemon persists `resolved_runtime` with per-field provenance on every
  generation — routing decisions are auditable via `compozy__loop_status`,
  never narrated.
