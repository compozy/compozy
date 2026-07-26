---
name: cy-spec-preflight
description: >-
  Loads the AGH spec authoring playbook plus relevant lessons, standing
  directives, glossary, and active context before cy-create-prd,
  cy-create-techspec, or cy-create-tasks runs. Applies phase-specific checks:
  PRDs stay business-focused, TechSpecs carry the six quality markers, and
  every spec/task captures extensibility integration, agent-manageability,
  config lifecycle, QA tail coverage, and Web/Docs Impact. Use whenever an AGH
  spec authoring skill is about to run. Do not use for spec execution, review
  remediation, or non-spec brainstorming
  output.
trigger: implicit
argument-hint: "[phase]"
---

# Spec Preflight

Authors of AGH PRDs, TechSpecs, and `_tasks.md` repeatedly produce drafts that miss project-specific directives — frameworks named in PRDs, prose-only TechSpecs, "fraco" test coverage. This skill loads project memory before handing off to `cy-create-prd`, `cy-create-techspec`, or `cy-create-tasks`, then runs the relevant post-draft checks before approval.

## Required Inputs

- **phase** (optional): one of `prd`, `techspec`, `tasks`, or `task-body`. When omitted, infer from the active `cy-create-*` skill or from the artifact path (`_prd.md`, `_techspec.md`, `_tasks.md`, `task_NN.md`).

## Procedures

**Step 1: Load Project Memory**

1. Read `docs/_memory/spec-authoring-playbook.md` in full.
2. Read `docs/_memory/standing_directives.md` (SD-001..SD-011).
3. Read `docs/_memory/glossary.md` (vocabulary discipline — `capability` vs `recipe`, AGH is/is-not).
4. Read the matching lessons by phase. Read `references/phase-lessons.md` for the phase → lesson mapping.
5. Read `CLAUDE.md` Authoring Posture, Architecture Principles, Autonomy Contracts, Security Invariants sections.

**Step 2: Load Active Project Context**

1. Resolve the active task slug: the `.compozy/tasks/<slug>/` directory the artifact targets.
2. If a `_techspec.md` exists at the slug, read it before authoring tasks.
3. If `adrs/*.md` exist, read every one before authoring techspec/tasks.
4. If `analysis/*.md` exist under the slug, read before authoring techspec.
5. If a prior phase artifact exists (PRD before TechSpec, TechSpec before Tasks), read it.

**Step 3: Apply Phase-Specific Checks**

Phase-specific checks below. Run only the relevant block. Use the "before authoring" checks before the inner skill writes a draft, and the "after draft" checks before user approval.

### Phase: `prd`

1. Read `references/prd-checks.md`.
2. Before authoring, confirm the active idea is framed as WHAT/WHY/WHO and not implementation detail.
3. After the draft is produced, run `python3 scripts/check-prd-implementation-leak.py <prd_path>` to surface framework/storage/error-code/file-format names. Strip every match unless the PRD is *about* the named technology.
4. Confirm the PRD lists explicit Goals, Non-Goals, Success Metrics, and Open Questions using the canonical `cy-create-prd` template.
5. Confirm the PRD states the agent/operator manageability outcome and extension ecosystem expectation without naming implementation details.
6. Do not invoke `cy-spec-peer-review` for PRDs. Peer review is TechSpec-only and user-directed.

### Phase: `techspec`

1. Read `references/techspec-six-markers.md`.
2. After draft is produced, run `python3 scripts/check-techspec-markers.py <techspec_path>` to verify the six markers are present.
3. Confirm "No fallback / no compat shim / no placeholder" clauses are present where breaking changes apply.
4. Confirm Test Plan is per-section bullet list with concrete assertions and verification commands.
5. Confirm Public Interfaces / Types section enumerates routes, payloads, CLI verbs, config keys.
6. Confirm Extensibility Integration Plan enumerates extension manifests, hooks, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, and protocol docs that are added/changed/removed or explicitly unaffected.
7. Confirm Agent Manageability Plan enumerates CLI verbs, HTTP endpoints, UDS routes, structured outputs, status/config discovery, and deterministic errors agents will use.
8. Confirm Config Lifecycle section enumerates `config.toml` keys/defaults, merge/overlay behavior, validation, examples, generated CLI/site docs, and tests that are added/changed/removed or explicitly unaffected.
9. Confirm Assumptions/Defaults section closes the spec.
10. Confirm Web/Docs Impact is captured if any contract surface is touched (activate `cy-web-docs-impact`).
11. After the user approves the baseline TechSpec draft and it has been saved, offer `cy-spec-peer-review`. Invoke it only if the user explicitly opts in.

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
10. Confirm `.resources/<competitor>/path` references are cited per task when the TechSpec drew on competitors.
11. Confirm no TBD / placeholder rows.

### Phase: `task-body`

1. Confirm `<critical>ALWAYS READ _techspec.md ...</critical>` block at the top.
2. Confirm `<critical>MINIMIZE CODE, TESTS REQUIRED, NO WORKAROUNDS</critical>` block.
3. Confirm Files / Surfaces section enumerates touched files.
4. Confirm Tests section enumerates assertions covering happy path + failure paths + concurrency stress + contract redaction (when relevant).
5. Confirm Web/Docs Impact subitem.
6. Confirm Extensibility / Agent Manageability / Config Lifecycle subitem.
7. Confirm References section cites `.resources/<competitor>/path` paths from the TechSpec.

**Step 4: Coordinate With the Inner Skill**

1. Before authoring checks pass: hand off to the inner `cy-create-*` skill.
2. The inner skill produces the artifact; this preflight skill is not the author.
3. After the draft exists: run the after-draft checks above before user approval or task execution.

## Error Handling

- **Phase cannot be inferred:** ask the user explicitly. Do not guess.
- **Playbook missing:** halt. The playbook is mandatory context. Direct the user to restore from git or re-run the synthesis.
- **`scripts/check-*.py` fail with structural errors:** the artifact does not match the expected shape. Surface the path that broke; do not auto-fix.
- **PRD names AGH-Network wire format:** allowed exception per `lessons/L-013` — confirm with user before stripping.
- **TechSpec missing markers:** do not let the user skip. Pedro will reject the spec; resolve missing markers first.
- **`_tasks.md` missing QA pair:** auto-invoke `cy-tasks-tail-qa-pair` to repair.
- **`_tasks.md` missing Web/Docs Impact subitems:** auto-invoke `cy-web-docs-impact` to populate.
- **TechSpec/task lacks extensibility, agent-manageability, or config lifecycle analysis:** block approval until the artifact names the impacted surfaces or gives explicit no-impact evidence.
