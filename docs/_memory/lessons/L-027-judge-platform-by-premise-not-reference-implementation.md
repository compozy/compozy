# L-027 — Judge a platform against its premise, not its reference implementation

**Class:** Architecture / Spec authoring
**Date discovered:** 2026-07-07
**Evidence sources:** Loops × task-domain architecture session (2026-07-07); coupling audit of `internal/loop`; `extensions/dev-cycle/loops/software-delivery/loop.yaml`.

## Context

The loop engine's premise is a **flexible loop-engineering + DAG system** — many loop shapes (pure run-agent chains, tool pipelines, judge panels, watch loops, task-delegating loops) are all first-class, and `software-delivery` is only the first reference example. In one session, three wrong judgments were produced by evaluating the engine through that single shipped example:

1. Task delegation was promoted to a system norm ("run-agent as durable-work executor dies") when it is an opt-in composition pattern chosen per YAML.
2. "Two human-attention surfaces" (loop gate decisions vs task inbox) was read as a defect requiring unification machinery, when each is its domain-native primitive and the author chooses which to use.
3. The example's non-need for mid-flow human gates was used to argue for shrinking/removing the core human gate — which generic loop-engineering demonstrably requires (deploy/publish/spend approval gates, autonomy ratchets, last-checkpoint patterns).

Meanwhile the *actual* defect ran in the opposite direction and was initially missed: the reference example had contaminated the core. The Compozy product format `md_tasks` is hardcoded in the generic engine and is even the **default** parse mode (`internal/loop/control_file_import.go:87` — empty `parse` falls through to `md_tasks`; closed vocabulary enforced at `internal/loop/linter.go:327-332`).

## Root cause

When a platform has exactly one shipped example, the example becomes a mental stand-in for the system. Traits of the example get promoted to rules of the system; gaps in the example get read as defects of the engine; non-needs of the example get used to justify deleting core primitives. The critique flows the wrong way — the right direction is checking whether the example left product traces in the core.

## Rule

> Before proposing a core change, classify the defect: engine defect vs reference-implementation defect. Composition patterns remain opt-in authorial choices — never system norms. A reference example must leave zero product traces in the core: no formats, no defaults, no vocabulary.

## Examples (from this incident)

| Claim made from the example | Premise-derived resolution |
| --- | --- |
| "Durable work should always delegate to native tasks" | Delegation is opt-in composition (`agh__task_create` + event watch); run-agent chains stay first-class |
| "Two attention surfaces are a defect — unify them" | Domain-native surfaces; author picks per YAML; at most a read-only aggregated UI view |
| "Human gate should shrink to contract boundaries" | Gate criteria `{command, agent-judge, human, extension}` are the loop's exit-condition algebra — the autonomy dial; keep in core |
| (missed) "md_tasks in core is fine" | Product format as default parse in a generic engine → evict to the extension |

## Anti-pattern

- Promoting reference-example traits into engine rules or defaults.
- Deleting or shrinking a core primitive because the current example doesn't exercise it.
- Letting the first-party example define parse formats, default values, or vocabulary inside the generic engine instead of inside its own extension.

## Source

- `internal/loop/control_file_import.go:87` (md_tasks as default parse), `internal/loop/linter.go:327-332` (closed parse vocabulary).
- `.compozy/tasks/_archived/1783422111817-2d088f48-loops/adrs/adr-021.md` — the taxonomy already supported the semantic composition path (open ToolID action class); the reference loop simply didn't use it.
- `extensions/dev-cycle/loops/software-delivery/loop.yaml` — `execute_task` run-agent node carrying a ~100-line prompt re-implementing task lifecycle conventions in prose.
