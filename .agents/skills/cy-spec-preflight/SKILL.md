---
name: cy-spec-preflight
description: >-
  Loads the Compozy spec authoring playbook plus relevant lessons, standing
  directives, glossary, and active context before cy-create-spec or
  cy-create-tasks runs. Applies phase-specific checks: Part I of the spec
  stays business-focused, Part II carries the six quality markers, the
  _dx.md/_uiux.md surface contracts exist and stay consistent, and every
  spec/task captures extensibility integration, agent-manageability, config
  lifecycle, QA tail coverage, and Web/Docs Impact. Use whenever a Compozy
  spec authoring skill is about to run. Do not use for spec execution, review
  remediation, or non-spec brainstorming output.
trigger: implicit
argument-hint: "[phase]"
---

# Spec Preflight

Authors of Compozy specs and `_tasks.md` repeatedly produce drafts that miss project-specific directives — frameworks named in the Product part, prose-only technical design, "fraco" test coverage. This skill loads project memory before handing off to `cy-create-spec` or `cy-create-tasks`, then runs the relevant post-draft checks before approval.

## Required Inputs

- **phase** (optional): one of `spec`, `tasks`, or `task-body`. When omitted, infer from the active `cy-create-*` skill or from the artifact path (`_spec.md`, `_tasks.md`, `task_NN.md`).

## Procedures

**Step 1: Load Project Memory**

1. Read `docs/_memory/spec-authoring-playbook.md` in full.
2. Read `docs/_memory/standing_directives.md` (SD-001..SD-011).
3. Read `docs/_memory/glossary.md` (vocabulary discipline — `capability` vs `recipe`, Compozy is/is-not).
4. Read the matching lessons by phase. Read `references/phase-lessons.md` for the phase → lesson mapping.
5. Read `CLAUDE.md` Authoring Posture, Architecture Principles, Autonomy Contracts, Security Invariants sections.

**Step 2: Load Active Project Context**

1. Resolve the active task slug: the `.compozy/tasks/<slug>/` directory the artifact targets.
2. If `_spec.md` exists at the slug, read it before authoring tasks.
3. If `adrs/*.md` exist, read every one before authoring Part II or tasks.
4. If `analysis/*.md` exist under the slug, read before authoring Part II.
5. Read every companion that already exists (`_user_stories.md`, `_dx.md`, `_uiux.md`, `_tests.md`) before the phase that consumes it.

**Step 3: Apply Phase-Specific Checks**

Phase-specific checks below. Run only the relevant block. Use the "before authoring" checks before the inner skill writes a draft, and the "after draft" checks before user approval.

### Phase: `spec`

Stage 1 — Part I (Product):

1. Read `references/spec-part1-checks.md`.
2. Before authoring, confirm the active idea is framed as WHAT/WHY/WHO and not implementation detail.
3. After Part I is written, run `python3 scripts/check-spec-part1-leak.py <spec_path>` to surface framework/storage/error-code/file-format names. Move every match to Part II unless the spec is *about* the named technology.
4. Confirm Part I lists explicit Goals, Non-Goals, and Open Questions, and states the agent/operator manageability outcome and extension ecosystem expectation without naming implementation details.
5. Confirm the Overview opens with the Motivating Problem and the simplest end-to-end behavior that solves it, stated as an outcome observable from outside the system. Cross-check every ADR against it: an ADR that narrows or defers the Motivating Problem must carry the user's recorded sign-off.

Stage 2 — Surface + Part II (Technical):

6. Read `references/spec-six-markers.md`.
7. Confirm `_dx.md` exists and is written as-if-shipped (paired write→see examples, zero internals). For features touching `web/`, confirm `_uiux.md` exists — its presence is the UI-bearing signal the QA tail consumes.
8. After Part II is written, run `python3 scripts/check-spec-markers.py <spec_path>` to verify the six markers are present.
9. Confirm "No fallback / no compat shim / no placeholder" clauses are present where breaking changes apply, and delete targets are listed.
10. Confirm Testing Approach is strategy-only and `_tests.md` carries the concrete cases derived from `_user_stories.md`, `_dx.md`, and `_uiux.md`.
11. Confirm the Agent Manageability Plan and API sections are consistent with `_dx.md` (routes, payloads, CLI verbs, config keys — no divergence between promise and design).
12. Confirm the Extensibility Integration Plan enumerates extension manifests, hooks, skills/capabilities, tools/resources, registries, bridge SDKs, MCP sidecars, and protocol docs that are added/changed/removed or explicitly unaffected.
13. Confirm the Config Lifecycle section enumerates `config.toml` keys/defaults, merge/overlay behavior, validation, examples, generated CLI/site docs, and tests that are added/changed/removed or explicitly unaffected.
14. Confirm File References is filled as the read-first index: repo files, `.resources/<competitor>/path` entries when the design drew on competitors, and `analysis/`/design sources — each annotated with why to read it.
15. Confirm the Assumptions and Defaults section closes the spec.
16. Confirm Web/Docs Impact is captured if any contract surface is touched (activate `cy-web-docs-impact`).
17. After the user approves the complete spec and it has been saved, offer `cy-spec-peer-review`. Invoke it only if the user explicitly opts in.

### Phase: `tasks`

1. Read `references/tasks-checks.md`.
2. Confirm the table column order matches `cy-create-tasks`: `# | Title | Status | Complexity | Dependencies`.
3. Confirm an MVP Boundary statement above the table.
4. Confirm Dependencies column is populated for every row.
5. Confirm Complexity is rated `low | medium | high | critical`, with QA execution and safety primitives marked high/critical as appropriate.
6. Confirm last two rows are `qa-report` (high) + `qa-execution` (critical) per `cy-tasks-tail-qa-pair`.
7. Confirm Web/Docs Impact subsection exists in every backend task body (activate `cy-web-docs-impact` to populate).
8. Confirm Extensibility / Agent Manageability / Config Lifecycle subsections exist in every feature-bearing backend task body.
9. Confirm test density is proportional to behavior count per task. Reject "fraco" plans (1-2 tests for many behaviors).
10. Confirm each task's `### Competitor References` copies its `.resources/<competitor>/path` subset from `_spec.md` File References when the spec drew on competitors.
11. Confirm no TBD / placeholder rows.
12. Confirm every implementation task is a shippable slice: it carries a `## Shippable Outcome` with its verification tier, and no task is a layer grouping (all-backend / all-frontend / all-docs).
13. Confirm mission traceability: exactly one slice's Shippable Outcome solves the spec's Motivating Problem end-to-end, and it is the earliest slice the dependency graph allows.
14. When `_uiux.md` exists, confirm visual coverage: every artboard section of every mapped surface touched by a slice appears in that slice's Visual Contract.

### Phase: `task-body`

1. Confirm `<critical>ALWAYS READ _spec.md ...</critical>` block at the top.
2. Confirm `<critical>MINIMIZE CODE, TESTS REQUIRED, NO WORKAROUNDS</critical>` block.
3. Confirm Files / Surfaces section enumerates touched files.
4. Confirm Tests section enumerates assertions covering happy path + failure paths + concurrency stress + contract redaction (when relevant).
5. Confirm Web/Docs Impact subitem.
6. Confirm Extensibility / Agent Manageability / Config Lifecycle subitem.
7. Confirm `### Competitor References` cites this task's `.resources/<competitor>/path` subset copied from `_spec.md` File References.

**Step 4: Coordinate With the Inner Skill**

1. Before authoring checks pass: hand off to the inner `cy-create-*` skill.
2. The inner skill produces the artifact; this preflight skill is not the author.
3. After the draft exists: run the after-draft checks above before user approval or task execution.

## Error Handling

- **Phase cannot be inferred:** ask the user explicitly. Do not guess.
- **Playbook missing:** halt. The playbook is mandatory context. Direct the user to restore from git or re-run the synthesis.
- **`scripts/check-*.py` fail with structural errors:** the artifact does not match the expected shape. Surface the path that broke; do not auto-fix.
- **Part I names Compozy-Network wire format:** allowed exception per `lessons/L-013` — confirm with user before stripping.
- **Spec missing markers:** do not let the user skip. Pedro will reject the spec; resolve missing markers first.
- **UI-bearing feature without `_uiux.md`:** block the Stage 2 close until the file exists or the user states the feature ships without `web/` changes.
- **`_tasks.md` missing QA pair:** auto-invoke `cy-tasks-tail-qa-pair` to repair.
- **`_tasks.md` missing Web/Docs Impact subitems:** auto-invoke `cy-web-docs-impact` to populate.
- **Spec/task lacks extensibility, agent-manageability, or config lifecycle analysis:** block approval until the artifact names the impacted surfaces or gives explicit no-impact evidence.
