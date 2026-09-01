---
name: cy-create-spec
description: Creates the unified Spec (_spec.md with Product and Technical parts) plus its companion catalogs — _user_stories.md, the _dx.md developer-experience contract, the _uiux.md UI change map for UI-bearing features, and the _tests.md test contract — through a two-stage frontier grill with parallel codebase and market research. Use when starting a new feature or product, specifying requirements or technical design, or updating an existing spec and its companions. Do not use for task breakdown, review remediation, or direct code implementation.
---

# Create Spec

Author one `_spec.md` carrying the full arc from product intent to implementation design, plus the companion catalogs. Every artifact is written for the LLM agents that consume it downstream (`cy-create-tasks`, `cy-execute-task`, review rounds): supply business rules, domain behavior, product intent, and implementation design. KPIs, success metrics, timelines, and rollout phases have no consumer in this pipeline — leave them out. All artifacts in English, active voice; every sentence earns its place.

The spec has two parts written in two stages, separated by a checkpoint:

- **Part I — Product**: WHAT users need, WHY it provides value, WHO the users are. Closes with `_user_stories.md`.
- **Part II — Technical**: first the public surface (`_dx.md`, and `_uiux.md` when UI-bearing), then HOW to implement it. Closes with `_tests.md`.

<HARD-GATE>
- Research before questions: every stage's grill MUST be fed by research first — codebase and market for Stage 1, architecture exploration for Stage 2.
- Grill before writing: the user MUST shape each part by answering grill rounds — for every spec, however simple. "Simple" features are where unexamined assumptions cause the most rework; a short grill is fine, a skipped grill is not.
- Checkpoint between stages: Stage 2 opens only after the user confirms the Stage 1 summary.
- Surface before internals: the grilled `_dx.md`/`_uiux.md` freeze before internal design begins; internals are designed to serve the frozen surface, never the reverse.
- Decide, then write: once a stage's frontier is empty and its ADRs are recorded, write that stage's files directly. The user reviews the generated files and requests changes afterward — no draft-approval loops.
</HARD-GATE>

## Full Scope, Sliced Delivery

Capture the complete scope the user wants, however large it grows — a capability leaves scope only when the user decides against it (record that in Non-Goals). Scope lives in the documents; delivery lives in shippable slices:

- The spec opens with its **Motivating Problem** — the concrete pain that triggered it — and the simplest end-to-end behavior that solves it. That behavior is what `cy-create-tasks` turns into slice 1; every later capability extends it. An ADR that narrows or defers the Motivating Problem is valid only with the user's sign-off recorded in the ADR itself.
- When the scope holds more than one distinct product problem, or an honest breakdown would exceed the slice budget (default 5 shippable slices; `slice_budget: N` per invocation), present it at the checkpoint as a sequenced program of specs — each independently shippable — and let the user choose between one spec and the program.
- YAGNI applies to invention: challenge features the user never asked for; keep every one they did.
- When the user adds scope mid-conversation, fold it in — and re-check the slice budget at the next checkpoint.
- Design minimalism applies to Part II — to the design, never the scope: include no component, interface, or abstraction the design does not strictly need, and prefer adding a file to an existing package over proposing new packages.

## Grilling

Read `references/grill-protocol.md` before the first round and run every grill with its frontier method: map the stage into a design tree, ask the whole frontier each round (numbered questions, each led by a recommendation), dispatch read-only subagents for facts, chase vague answers, challenge vocabulary against `docs/_memory/glossary.md`, and capture an ADR (per `references/adr-template.md`) the moment a significant decision crystallizes. A grill is done when its frontier is empty — every branch resolved or explicitly parked in Open Questions with the user's consent.

## Required Inputs

- Feature name or product idea.
- Optional: `_idea.md` at the slug as primary context.
- Optional: existing `_spec.md` or companions for update mode.

## Workflow

Track each step as a task in the runtime's task tracker when one is available, and complete the steps in order.

### Stage 1 — Product

1. Resolve the working directory.
   - Derive the slug from the feature name; the target directory is `.compozy/tasks/<slug>/` with an `adrs/` subdirectory. Create both if missing.
   - Read `_idea.md` there if present. If `_spec.md` exists, read it and operate in update mode.

2. Discover context through two parallel research tracks. Both MUST finish before any question is asked; run them in parallel (e.g., two Agent tool calls).
   - Track A — Codebase: files, patterns, data models, and integration points related to the request; summarize in 3-5 bullets.
   - Track B — Market: 3-5 web searches on trends, competing products, and user expectations; summarize in 3-5 bullets. If web search tools are unavailable, note the limitation and proceed with Track A only.
   - Present the merged findings to the user before the first round.

3. Grill the product through the product lens: WHAT users need, WHY it provides value, WHO the users are. When the feature name sounds technical ("webhook notifications"), translate it into the user-experience question behind it ("which events should notify the user?") — implementation questions wait for Stage 2. Done when the product frontier is empty.

4. Write the user-story catalog.
   - Read `references/user-stories-template.md` and write `.compozy/tasks/<slug>/_user_stories.md`.
   - Cover every persona — secondary ones included — and every core feature; run the template's edge-case sweep against every story.
   - Done when every story has verifiable acceptance criteria plus edge cases with expected behavior, and every edge-case class has been probed against every story.

5. Write Part I of `_spec.md`.
   - Read `references/spec-template.md` and fill every Part I section with the decided direction and confirmed answers; the template carries the per-section rules.
   - Present the Stage 1 summary: decided direction, non-goals, parked questions, file paths.

**Checkpoint**: proceed to Stage 2 only after the user confirms the Stage 1 summary; fold requested adjustments in first.

### Stage 2 — Surface, then internals

6. Explore the architecture: spawn a read-only subagent to map patterns, existing components, dependencies, and technology stack relevant to the design; have it return every load-bearing path annotated with why it matters — repo files, and `.resources/<repo>/` slices when competitor sources inform the design — as the seed for the File References section. Read every `adrs/*.md` and `analysis/*.md` under the slug.

7. Draft the public surface as if the feature already shipped:
   - Read `references/dx-template.md` and draft `.compozy/tasks/<slug>/_dx.md` — always.
   - Read `references/uiux-template.md` and draft `.compozy/tasks/<slug>/_uiux.md` — only when the feature touches `web/` surfaces. Its presence marks the feature UI-bearing for the QA tail.

8. Grill the surface: rounds on the drafts themselves — naming, invocation shape, YAML feel, output ergonomics, golden-path friction, failure copy. Rework the drafts between rounds; the draft is the question. Done when the surface frontier is empty — the surfaces are now frozen.

9. Grill the internals through the technical lens: HOW to implement, WHERE components live, WHICH technologies to use — architecture and component boundaries, data models and storage, integration points, testing strategy. Never spend a question on what the codebase can answer. Done when the technical frontier is empty.

10. Write Part II of `_spec.md`.
    - Fill every applicable Part II section per the template: map every Part I goal, every story in `_user_stories.md`, and every surface in `_dx.md`/`_uiux.md` to a technical component; reference Part I sections by name without duplicating them.
    - The template carries the six quality markers and the CompozyOS mandatory sections — fill each or state explicit no-impact evidence.
    - Fill File References with every path the design depends on — repo files, `.resources/<repo>/` slices, analysis files, design artboards — each with its one-clause read-reason; tasks copy their subsets from this index.
    - List every ADR from both stages in the Architecture Decision Records section; if none exists yet, record the primary-technical-approach ADR first.

11. Write the test contract.
    - Read `references/tests-template.md` and write `.compozy/tasks/<slug>/_tests.md`.
    - Derive unit cases from every component and interface in Part II including every error path; integration cases from every component boundary; E2E cases from every journey — CLI/API journeys use the exact commands and routes from `_dx.md`, browser journeys follow the `_uiux.md` surface map.
    - Done when the coverage matrix satisfies the template's Coverage Demands and every case meets its Case-Writing Rules.

12. Hand off: confirm every file path to the user, invite change requests directly on the generated files, and point to `cy-create-tasks` as the next step.

## Error Handling

- Insufficient context for a section: note it in Open Questions rather than guessing.
- Web research tools unavailable: proceed with codebase findings and state the limitation.
- Target directory cannot be created: stop and report the filesystem error.
- Conflicting architectural patterns in the codebase: document both and recommend one with rationale.
- Update mode: preserve sections the user has not asked to change, and mirror every behavior or interface change into the affected companions (`_user_stories.md`, `_dx.md`, `_uiux.md`, `_tests.md`) so the set stays in sync.
