# Context and decisions

Normal preparation reads the generated `knowledge.md` queue. This reference defines the small semantic input to `prepare_review.py`; scripts own the expanded registry, context and plan.

## Discovery

Instruction discovery checks root and ancestors of selected paths, including AGENTS.md and CLAUDE.md aliases. One canonical source retains its lexical aliases, scope and precedence. Local skill roots are catalogued by metadata, including linked installations; arbitrary words in lockfiles do not make a skill applicable.

`knowledge.json` stores canonical source IDs, content/scope fingerprints and shared path sets. `knowledge.md` presents metadata and pending decisions. References are discovered only after the parent router is applied. Shared reference bindings survive exclusion of one parent; all parents must be explicitly excluded before automatic not-applicable accounting. A missing referenced file named by an applied router is a diagnostic, never silently dropped.

## Decision file

Copy `decisions.template.json` to `decisions.json`, edit semantic fields and pass it to preparation. Preserve `scope_fingerprint`, `source_fingerprints` and row fingerprints: these bind decisions to the current selection and source bytes. Refreshed templates carry newly discovered reference fingerprints, prior decisions and additional fields. Copy the updated template for the next stage and edit pending entries; do not rewrite boilerplate or invent fingerprints.

Each source has `source` (canonical path or compact source ID), `status` (`pending`, `applied`, `not-applicable`) and `reason`. Applied means the relevant material was read; name the sections if only part applies. An exclusion needs a concrete scope/routing reason and does not require reading an irrelevant body.

For batch exclusions, `groups` accepts `{ "sources": ["s...", "s..."], "status": "not-applicable", "reason": "..." }`. This resolves those pending template rows. Complete conflicting decisions are rejected. The compiled template expands groups so subsequent stages need no repeated group editing.

Additional fields:

| Field | Contract |
| --- | --- |
| `intent` | Stated change intent: user request, PR description or commit digest; concise prose |
| `rules` | Exact source-backed rules, described below |
| `linters` | Detected/relevant lane results: `name`, `command`, `status` (`ran`, `reused`, `unavailable`), `result`, optional `scope` globs; ran/reused require matching `input_snapshot` |
| `sweeps` | Default empty; objects with `key`, concrete cross-cohort `hypothesis`, optional custom `lens` |
| `spec_artifacts` | Additional canonical `{path, role}` entries, including dependencies named by the requested spec |
| `mode` | Recorded engine, e.g. `agent-fallback`, `workflow`, `subagent:codex` |
| `plan` | Optional complete semantic override of generated cohorts; ordinary runs omit it |

Preparation suggests repository-owned Makefile/package linter lanes. Also record relevant lanes identified from project instructions or other tool configurations. Respect repository policy; reuse only evidence for the same inputs. Failed/missing tools may be `unavailable` with a reason; never install them merely to populate a lane.

## Rules

Extract only verdict-bearing rules. Operational directions can leave an applied source with zero rules when its reason explains why. Rule precedence: path instructions in review config; deepest applicable instructions; root instructions; relevant skills; scoped learnings.

A rule uses `id`, `source`, `scope` globs and either exact `guideline` text or one-based inclusive `start_line`/`end_line`. The compiler copies spans verbatim and rejects text not found in the applied source, invalid spans, unbound scopes and stale evidence.

```json
{
  "id": "R01",
  "source": "s<id from template>",
  "scope": ["internal/store/**"],
  "start_line": 12,
  "end_line": 14,
  "lanes": ["defect"],
  "sweeps": ["migrations"]
}
```

`lanes` binds defect and/or polish assessments; omitted means both for existing registries. Every rule keeps at least one local lane owner. Use defect for correctness/security/contracts/test efficacy, polish for maintainability/naming/idioms, both when the obligation spans both. `sweeps` names the lenses needing that rule; omitted means no extra global compliance repetition. Sweep investigation still judges general correctness through its lens.

## Generated artifacts

`prepare_review.py` assembles `rules.json`, `review-context.json`, `context-pack.md` and a default package-oriented plan. Reviewers receive only their applicable rules and linter context in their per-job contract. Complete source accounting stays on disk. The global gate rechecks source fingerprints, including externally symlinked skill files.

`--spec PATH` resolves a file or recursively inventories Markdown documents in a directory; append other canonical dependencies via `spec_artifacts`. Spec artifacts are the contract under test, not rubric law. The compiler includes the spec-parity sweep and the report's conformance section. The walkthrough is rendered after review from intent, cohorts, summaries and optional flow diagrams.
