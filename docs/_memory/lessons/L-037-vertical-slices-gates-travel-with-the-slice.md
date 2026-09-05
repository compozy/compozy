# L-037 — Deliver in vertical slices; verification gates travel with the slice

**Class:** Spec authoring / Workflow / Process
**Date discovered:** 2026-09-01
**Evidence sources:** agent-comms post-mortem 2026-09-01. Spec set (task graph): [`compozy-specs/_archived/2026-08-19-agent-comms/`](https://github.com/compozy/compozy-specs/tree/main/_archived/2026-08-19-agent-comms). Execution trail (`state.yaml`, `memory/task_06.md`, `reviews/summary.md`): `.compozy/tasks/agent-comms/` on the unmerged `agent-comms` branch — remote only, draft PR [compozy/compozy#497](https://github.com/compozy/compozy/pull/497).

## Context

The agent-comms task graph was layer-horizontal: foundations → backend domain → mailbox → surfaces → **all web in one task** → docs → QA pair at the tail. All 7 implementation tasks ran in one 17.5-hour day; the entire six-surface UI was one 7-hour task, second to last. The first browser verification happened ~28 hours after the first commit. task_06's memory recorded the gate relocation verbatim: "Screenshots and Visual Contract evidence remain deferred to task 09" — but task_09 (QA execution) never cited Visual Contracts, so 22 declared VC rows had **zero** evidence bundles at SHIP; the 20 bundles only appeared three days later, after the owner rejected the UI. The result: one 61-commit / 999-file / +103k-line PR, 9 feature commits and 52 repair commits, ~700 review findings with ~350 raised after SHIP, and an unreviewable diff whose review skill degraded to sampling by its own rules.

The failure was codified, not accidental: `cy-create-tasks` listed "Domain: backend vs frontend vs SDK vs docs" as a split boundary and capped breakdowns at 3-7 tasks; `cy-loop-tasks` ordered "Skip any per-task QA-walk step the task file requests"; the QA pair was mandated as the last two rows; the PR could only exist after all tasks + QA + SHIP. The internal review's own contradiction ruling — "Relocar o gate não é authorized difference" — never became doctrine.

## Root cause

Layer-horizontal decomposition with tail-only verification makes integration and user-visible truth unobservable until ~85% of the work is spent, and structurally cannot produce a small reviewable increment: PR size is a symptom of the cut, not its own disease. A gate whose owning task defers it to a task that does not name it has no owner at all.

## Rule

> Prefer shippable increments with an observable outcome and the cheapest check that can expose failure. Verification travels with the changed outcome; broader integration QA covers gaps rather than repeating current slice evidence.

## Operationalization

- Size work by risk and dependencies, not a fixed slice quota. A foundation task identifies its consuming outcome and verifies its own boundary.
- A requested full `cy-loop-tasks` workflow keeps its checkpoint/phase contract; ordinary fixes do not need that workflow. Its trailing QA phase owns remaining integration journeys once.
- A slice uses its declared `gate`, `probe`, or `smoke` evidence. Named-reference visual rows derive from the actual UI contract.
- SD-012 owns current execution policy; branch/PR options belong to the invoked workflow.

## Anti-pattern

- "All backend → all frontend → docs → QA" graphs; a single task owning every UI surface.
- Deferring smoke/VC evidence to "the QA phase" — the exact relocation this lesson exists to prevent.
- Reading a giant PR by sampling instead of recommending the split.

## Source

- Layer-split graph: `_archived/2026-08-19-agent-comms/_tasks.md` in the compozy-specs repo (link above).
- Execution evidence on the `agent-comms` branch under `.compozy/tasks/agent-comms/`: `state.yaml` (iterations; 24 consecutive `partial` repair loops post-SHIP), `memory/task_06.md:94` (deferral), `reviews/summary.md` P0 #14 and contradiction #11.
