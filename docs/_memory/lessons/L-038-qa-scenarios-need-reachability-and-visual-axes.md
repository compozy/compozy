# L-038 — QA scenarios need a reachability and visual-language axis, not only data truth

**Class:** Testing / QA / Design system
**Date discovered:** 2026-09-01
**Evidence sources:** agent-comms post-mortem 2026-09-01; `docs/qa/reports/2026-08-26-agent-comms.md`, the agent-comms scenario files, and `.compozy/tasks/agent-comms/reviews/summary.md` on the unmerged `agent-comms` branch — remote only, draft PR [compozy/compozy#497](https://github.com/compozy/compozy/pull/497). Spec set: [`compozy-specs/_archived/2026-08-19-agent-comms/`](https://github.com/compozy/compozy-specs/tree/main/_archived/2026-08-19-agent-comms).

## Context

The agent-comms QA pass was real work — 32/32 scenarios passed and it caught genuine runtime bugs (tool policy denying `call_return`, a 30s CLI timeout killing 55s waits, a parked-child idle clock erased by settlement). The owner then rejected the delivered UI wholesale.

Both were correct, because every scenario `expected:` field asserted **data truth only** — daemon counts, state vocabularies, verbatim wake lines, ownership propagation. Consequences on record: the Activity surface **passed QA reached via direct URL** while its real entry path (Dock → Agents → Activity) was hidden — the scenario never demanded the path; the wake row **passed while dumping model prompt XML**, because the scenario demanded the daemon's wake line "verbatim" and verbatim was the bug; and no agent-comms scenario referenced a design board at all. The QA lens set (functionality, accessibility, clarity, feedback, resilience, trust) contains no visual-language or reachability member, so a surface scored 6/6 while unreachable, hand-rolled, and off-board. The internal review later ruled: "Stale `pass` é pior que `untested`."

## Root cause

Daemon-agreement was the only truth axis in scenario authoring. A UI can agree with the daemon perfectly and still be unreachable through the product, violate the design grammar, and leak raw payloads — none of which any data assertion can fail.

## Rule

> A UI-bearing QA scenario enters through the product's real entry path — never a typed URL — and, when the surface has a design reference, asserts against the board's reading, not only the daemon's data. Scenario walks are the peer of the Visual Contract evidence, not a substitute for it: data truth, reachability, and visual language are three axes, and a pass claim covers only the axes the scenario actually asserted.

## Operationalization

- Scenario authoring (`qa-report`): UI scenarios name the entry path as a step and cite the owning board section when one exists.
- Per-slice `smoke` tier (L-037) owns visual-language evidence at build time; the QA walk re-proves reachability and flow, so neither axis waits for the other.
- "Verbatim" assertions on model- or payload-derived text carry a sanitization expectation — rendering raw payload verbatim is a finding, not a pass.

## Anti-pattern

- Walking a surface via URL because the dock/menu path is unfinished — that unfinished path is the bug.
- A 6-lens pass presented as whole-surface quality when no lens looked at the pixels.
- Treating a scenario `pass` recorded before a UI rework as still-valid evidence.

## Source

- On the `agent-comms` branch (remote): `docs/qa/reports/2026-08-26-agent-comms.md` (32/32), scenario reset note "The walk must start at Dock → Agents → Activity, not at the `/agents/activity` URL", `.compozy/tasks/agent-comms/reviews/summary.md` slice 06 ("Árvore construída e escondida … QA passou na URL secreta") and P0 #16.
