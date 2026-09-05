---
name: cy-spec-preflight
description: Check the applicable contracts of a Compozy spec or task artifact using current project policy and existing context. Use within spec authoring; not a separate research or execution gate.
---

# Spec Preflight

Use inside spec authoring for the active `spec`, `tasks`, or `task-body` phase. Infer the phase from the requested artifact; this is a check within the authoring step, not a separate research or approval cycle.

- Reuse the current spec, user decisions, and contract index. Read the relevant section of `docs/_memory/spec-authoring-playbook.md`; consult matching directives/lessons only for the changed design question. SD-013 governs compatibility. Use `references/phase-lessons.md` as a selective index.
- Read the artifact being changed and its actual contract dependencies. Already-read/current ADRs, analysis, glossary entries, and companion sections need not be reloaded. Expand the inventory when a dependency or ambiguity is unresolved.

## Spec

Product content states outcome, users, scope, and acceptance; implementation terms are acceptable when they describe an actual public contract or fixed constraint. `references/spec-part1-checks.md` and the read-only `.agents/skills/cy-spec-preflight/scripts/check-spec-part1-leak.py <spec_path>` are advisory terminology aids, not lexical approval gates.

Technical content makes applicable interfaces, data ownership, storage decisions, and safety invariants concrete. Read `references/spec-six-markers.md` for applicability. The read-only `.agents/skills/cy-spec-preflight/scripts/check-spec-markers.py <spec_path>` checks common outcome/boundary markers; add `--require <marker>` for each additional contract actually changed. Review the meaning, since heuristic matches are not proof of completeness.

Changed user state needs a lossless migration; public surfaces follow the SD-013 deprecation ladder; only internal code carries no-compat hard cuts. Record delete targets and affected config/extension/agent/Web/Docs surfaces once. Preserve `_dx.md` and UI-bearing `_uiux.md` contracts when applicable; `_tests.md` owns concrete test cases. ADRs may narrow an accepted goal only with the user's recorded decision.

## Tasks and Task Bodies

Use `references/tasks-checks.md` for graph shape and current `cy-create-tasks` metadata. Check dependency consistency, outcome coverage, canonical references, and one owner per assigned test ID. Tests reflect distinct invariants, not fixed density or category quotas. An inapplicable companion/impact section needs at most a brief reason or link.

Named-reference UI tasks carry their touched visual rows. A requested full loop keeps its required QA tail pair and phase schema; do not impose that loop on an ordinary task. Routine task bodies need no repeated critical-reminder block or full corpus survey.

Repair substantive inconsistencies at their owner and recheck affected facts. Do not block on a missing template phrase, force repeated questions, invent a contract, or refuse an explicit user decision. After a saved spec is approved, peer review is offered only as opt-in.
