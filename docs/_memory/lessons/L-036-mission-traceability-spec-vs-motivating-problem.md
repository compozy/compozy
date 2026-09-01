# L-036 — A spec must prove which slice solves its motivating problem

**Class:** Spec authoring / Decision process
**Date discovered:** 2026-09-01
**Evidence sources:** agent-comms post-mortem 2026-09-01. Spec set (spec, ADRs, tasks): [`compozy-specs/_archived/2026-08-19-agent-comms/`](https://github.com/compozy/compozy-specs/tree/main/_archived/2026-08-19-agent-comms). Execution trail (`HANDOFF.md`, reviews, QA): `.compozy/tasks/agent-comms/` on the unmerged `agent-comms` branch — remote only, draft PR [compozy/compozy#497](https://github.com/compozy/compozy/pull/497).

## Context

The agent-comms handoff opened with its mission: "Typed structured output on every return path — a **loop node**, a task run, or a child session finishing must hand back a schema-validated payload the caller can trust (today the caller LLM-parses prose)" (`.compozy/tasks/agent-comms/HANDOFF.md:9` on the branch). One day later, ADR-012 scoped loop nodes out of the call mechanism ("No call records for loop nodes in v1"), leaving loops with only a relocated prose extractor — byte-identical behavior to `main`. The spec grew to 8 features and shipped +102,883 lines, and the motivating problem was never delivered.

Nothing caught it: 3 spec peer-review rounds, an Opus completeness audit, a 369-finding deep review, a 32/32 QA pass, and two final reviews — ~700 findings total — produced **zero** findings about the mission. The one QA scenario touching the area (`LP-loop-contract-regime-adoption`) certified that the scope-out worked as specified.

## Root cause

Every verifier in the pipeline terminated at spec conformance: `cy-final-verify` defined "the original specification" as the artifacts in the spec directory, `cy-execute-task` ranked ADRs as binding authority, and review rounds cross-checked implementation-vs-spec and spec-vs-boards. The problem *behind* the spec was out of every verifier's frame, so an ADR that quietly narrowed it became self-certifying law. Every round audited implementation-vs-spec; none audited spec-vs-mission.

## Rule

> The spec's Overview opens with the Motivating Problem stated as an outcome observable from outside the system. Exactly one slice — the earliest the dependency graph allows — solves it end-to-end, named in the MVP Boundary and audited by `cy-create-tasks` and `cy-spec-preflight`. An ADR that narrows or defers the Motivating Problem is valid only with the user's sign-off recorded in the ADR itself, and reviews treat an unsigned narrowing as a finding.

## Operationalization

- `spec-template.md` Overview carries the Motivating Problem bullet; `cy-spec-preflight` phase `spec` cross-checks every ADR against it, and phase `tasks` audits mission traceability in the breakdown.
- `cy-review-round` and `cy-spec-peer-review` carry an explicit mission-fit check; `cy-final-verify` requires workstream claims to name the solving slice.

## Anti-pattern

- Treating ADR authority as a substitute for mission review — "the ADR says loops are out, so out they are".
- A QA scenario that certifies a scope-out executed correctly while nothing asks whether it should exist.
- Measuring review depth by finding count (~700) while the single most expensive defect is a frame no finding can land in.

## Source

- ADR and spec text: `_archived/2026-08-19-agent-comms/adrs/adr-012.md` and `_spec.md` in the compozy-specs repo (link above); exact line anchors (`HANDOFF.md:9`, `_spec.md:165`) resolve on the `agent-comms` branch under `.compozy/tasks/agent-comms/`.
- Post-mortem grep across all 21 review files + 10 `reviews-001` issues on that branch: 1 hit for the ADR (a files-read list), 0 findings.
