---
name: cy-improve-architecture
description: Audits a target for shallow modules, produces visual and durable architecture reports plus an @import-safe depth map, and grills the top deepening candidate. Use when a developer wants to improve module depth, testability, or AI navigability. Do not use for line-level refactoring, general code smells, performance, security, or code changes.
argument-hint: [target]
---

# Improve Architecture

Audit one target through the deep/shallow-module lens. Produce a durable markdown report, its visual HTML twin, and a terse per-area depth-map section before offering to grill one recommendation. Keep the audit language-agnostic and emit no finding smaller than a module interface.

## Invariants

- Treat `.compozy/arch-reviews/<slug>.md` as the offline-safe source of truth and keep its candidate set identical to the HTML twin.
- Lead every non-empty report with exactly one dominant top-pick CTA. Present every other candidate as secondary.
- Give every candidate a module, a deletion-test verdict, a before/after structure, and concrete maintainability-cost evidence.
- Preserve prior good artifacts until a complete replacement is ready.
- Read `.compozy/DECISIONS.md` and `.compozy/ARCHITECTURE.md`; never write `.compozy/DECISIONS.md` or `.compozy/decisions/`.
- Modify only `.compozy/arch-reviews/<slug>.{html,md}`, the audited area in `.compozy/ARCHITECTURE.md`, and an accepted `.compozy/GLOSSARY.md` update. A user-confirmed durable cross-feature outcome may additionally create one workflow-scoped draft ADR under `.compozy/tasks/<workflow>/adrs/`; include the selected workflow and a concise draft summary in the run summary. Leave `.gitignore`, `CLAUDE.md`, every `AGENTS.md`, and source code unchanged.
- Treat co-located `AGENTS.md` guidance as deferred V1.1 behavior. Do not create or update it.

## Workflow

### 1. Resolve the workspace and target

1. Identify the workspace root and capture the existence and content state of `.gitignore`, `CLAUDE.md`, and every in-scope `AGENTS.md` so the run can confirm they remain unchanged.
2. Resolve the optional positional target before scanning source. When it is absent, ask which module, feature area, or whole project to audit and wait for that answer.
3. Read `references/audit-method.md` in full and apply its targeting, source-count, slug, collision, and early-exit rules.
4. Stop with `target not found` and write nothing when the chosen path does not exist. Stop with `nothing to audit` and write no runtime artifact when the resolved module has zero source files.

Done when the canonical area, deterministic slug, source scope, and overwrite disposition are known, or the run has exited without writes.

### 2. Load durable context and audit vocabulary

1. Load and apply the bundled `cy-codebase-design` skill before classifying any candidate. Use its exact terms: module, interface, implementation, depth, deep, shallow, seam, adapter, leverage, and locality.
2. Read the current audited-area section of `.compozy/ARCHITECTURE.md` when present. Treat active `avoid` entries as do-not-re-propose constraints and provenance comments as history.
3. Read `.compozy/DECISIONS.md` best-effort when present, using `references/audit-method.md`. Skip architecture covered by an active settled decision unless current friction provides concrete reopening evidence; then name the decision in a warning callout.
4. When `.compozy/DECISIONS.md` is absent, record one summary notice: `Settled-decision filtering skipped: .compozy/DECISIONS.md is absent; install and enable cy-capture-decisions to add it.` Continue the full audit.
5. When the decision index is malformed, retain every valid row, print one soft warning, and continue. Isolate any companion failure to this enhancement.

Done when the vocabulary, active avoidances, and usable settled-decision constraints are available without blocking the core audit.

### 3. Explore and rank module candidates

1. Apply `references/audit-method.md` in full. Delegate the read-only codebase walk to an Explore agent when available; otherwise perform the same bounded walk on the main thread.
2. Explore organically across module interfaces, callers, tests, and dependency seams. Keep the scan within the resolved target and its directly relevant call edges.
3. Apply the deletion test to every possible finding. Drop line-level smells, long methods, generic advice, and idiomatic thin modules whose deletion merely relocates pass-through code.
4. Merge overlapping candidates or cross-reference them under one primary module. Never present duplicate cards for the same structural problem.
5. Rank by fix-value with the deterministic tie-break from `references/audit-method.md`. Mark every uncertain verdict `Speculative`; never promote it to `Strong`.
6. Select one top pick when candidates exist. For a single candidate, make it the pick without rendering a `1 of 1` menu. For zero candidates, select no pick and prepare a healthy-target lead panel.

Done when every retained candidate names one module, includes evidence and a deletion-test verdict, has a stable rank, and the run has zero or one top pick.

### 4. Render and publish both reports

1. Read `references/report-format.md` and `references/html-report.md` in full.
2. Build one canonical candidate record set, then render both reports from it. Keep candidate IDs, order, titles, modules, badges, decision callouts, and deletion-test verdicts identical.
3. Stage both reports in temporary sibling files under `.compozy/arch-reviews/`. Validate complete markup, candidate parity, one lead outcome panel, and exactly one top-pick CTA when candidates exist.
4. Replace `.compozy/arch-reviews/<slug>.md` and `.compozy/arch-reviews/<slug>.html` only after the audit and both staged files succeed. Preserve prior good files on any earlier failure.
5. Detect whether `.compozy/**` is ignored. Write the reports regardless and add one run-summary warning when they remain untracked; never edit `.gitignore`.
6. Open the published HTML with `open`, `xdg-open`, or `start` according to the host. If no opener exists or it returns an error, print the absolute HTML path and continue successfully.

Done when the markdown source of truth and HTML twin are complete, parity-checked, safely published, and either opened or reported by absolute path.

### 5. Regenerate the audited depth-map section

1. Read `references/architecture-map-format.md` in full immediately before editing `.compozy/ARCHITECTURE.md`.
2. Build the audited area's complete canonical section from the published markdown report and the retained avoidance history. Emit only the grammar in that reference.
3. When the audit found no candidates or seams, retain the section header and write `# no deepening opportunities as of <YYYY-MM-DD>` instead of omitting the area.
4. Replace only the audited area's keyed section. Preserve every other area's raw bytes and `avoid` history byte-for-byte. Preserve the audited area's active avoidances and superseded provenance unless this run explicitly changes their state.
5. Reconcile a confirmed area rename and interrupted or competing writes using the rules in `references/audit-method.md`. Keep area ordering valid and publish through a validated temporary sibling file so interruption cannot truncate the prior map.
6. Re-read the staged map and check every header, field arity, date, delimiter, area order, and `deep`/`seam`/`avoid` group against the grammar before replacement.

Done when `.compozy/ARCHITECTURE.md` represents one consistent section per area, the audited section points to the markdown report, and all untouched area bytes are unchanged.

### 6. Grill the selected deepening

1. Skip grilling only when no candidate exists, or when the run is non-interactive (for example an autonomous `compozy exec` pass). A non-interactive run publishes the report and depth map without the loop and records no decision.
2. Default to the top pick. Allow another candidate only when the user explicitly names it.
3. Run the built-in interrogation directly: walk the design tree one question at a time, each with a recommended answer, and explore the codebase to resolve codebase-answerable questions instead of asking the user. Cover constraints, dependencies, interface shape, seam placement, hidden implementation, adapters, and surviving tests. Use the bundled `cy-codebase-design` design-it-twice branch when materially different interfaces need comparison.
4. Treat a dedicated interactive interrogation skill such as `grill-me` as an optional enhancement: when one is installed, drive the loop through it for a richer walk; otherwise use the built-in interrogation from step 3. Grilling happens either way — the external skill is never required and is never a hard stop.
5. Note the grilling path once in the run summary, for example `Grilling: grill-me` or `Grilling: built-in (install grill-me for a richer interactive walk)`.
6. Isolate any external interrogation-skill error to the grilling enhancement, fall back to the built-in interrogation, and keep all core artifacts.

Done when the user proceeds, declines with a reason, or abandons the loop; a non-interactive run records no decision.

### 7. Record only accepted outcomes

1. On proceed, update the audited area's generated `deep` and `seam` guidance only when grilling materially sharpened it; republish the map with the same guarded section replacement.
2. On abandonment, record no decision and leave the pre-grilling artifacts intact.
3. On decline, distinguish a load-bearing reason from an ephemeral reason such as `not now`. Persist only load-bearing reasons as `avoid | <date> | <what> | <reason>` under the area.
4. When code changes invalidate an avoidance, demote the old line to the canonical `# superseded ...` provenance form and retain it. Never delete the history.
5. For a durable cross-feature outcome, ask the user to confirm the target workflow and concise draft summary. After confirmation, create one workflow-scoped draft ADR under `.compozy/tasks/<workflow>/adrs/` for later `cy-capture-decisions` promotion, and include the selected workflow and summary in the run summary. Never create a `proven` record or write the durable decision log.
6. When grilling crystallizes a project-specific domain concept, offer the bundled `cy-domain-modeling` skill. Apply an accepted, non-duplicate update only to `.compozy/GLOSSARY.md`; a decline writes nothing, and no `docs/adr/` or `CONTEXT.md` file is created.
7. Isolate any optional companion error and retain the core artifacts.

Done when the depth map and optional glossary reflect only accepted durable outcomes and the durable decision log remains untouched.

### 8. Verify and summarize the run

1. Confirm `.gitignore`, `CLAUDE.md`, and every captured `AGENTS.md` are unchanged.
2. Confirm the markdown and HTML reports have the same candidate IDs and ordering, and confirm the depth-map section conforms to `references/architecture-map-format.md`.
3. Print a compact summary containing the target and slug, candidate count, top pick or `none (healthy target)`, artifact paths written, depth-map area updated, ignored/untracked status, and each skipped or failed companion with its reason.
4. State that imported depth-map guidance remains advisory when stale and that re-auditing the area is the refresh path. Treat a missing report body as a soft link failure; the `deep`, `seam`, and `avoid` imperatives still stand.

Done when the user can identify the result, the single next action, every artifact, and every degraded enhancement from the printed summary.

## Error Handling

- Leave all prior artifacts untouched when target resolution, exploration, rendering, parity checks, or grammar checks fail before publication.
- On a write conflict, re-read the latest complete depth map and reapply the audited-area replacement once. If consistency still cannot be proven, report the conflict and preserve the latest complete file.
- Treat missing optional companions and malformed optional inputs as scoped warnings. Deliver the report and depth map whenever core exploration and publication succeed.

> _Adapted from Matt Pocock's MIT-licensed [`improve-codebase-architecture`](https://github.com/mattpocock/skills/tree/main/skills/engineering/improve-codebase-architecture) skill. See the extension `NOTICE` for the upstream copyright and license._
