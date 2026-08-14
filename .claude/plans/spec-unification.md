# Plan: Spec Unification — PRD + TechSpec → `_spec.md` + `cy-create-spec`

**Status:** executed 2026-08-13 · full gate pass (fingerprint 427cb765, log .cache/gate/logs/full-1786674585.log)
**Goal:** Replace the two-document PRD/TechSpec pipeline with a single `_spec.md` authored by a new `cy-create-spec` skill, add two experience companions (`_dx.md` always, `_uiux.md` UI-bearing only), and hard-cut every coupling — no aliases, no dual fields.

## Design decisions

1. **`_spec.md`** has two first-class parts — **Product** (overview, goals, business rules, non-goals) and **Technical** (architecture, implementation design, impact analysis, sequencing, six quality markers) — written in two stages with an explicit grill checkpoint between them.
2. **Phased grill** (informed by mattpocock `grilling` + `grill-with-docs` skills): stage 1 grills business/behavior/scope; checkpoint; stage 2 opens with `_dx.md` (+ `_uiux.md` if UI-bearing) surface drafts, grills ergonomics, then designs internals to serve the frozen surface.
3. **`_dx.md`** (always — agent-manageability premise makes it unconditional): final public interfaces written as if shipped — golden path first, then per surface (YAML, CLI + output, HTTP/UDS, SDK, config.toml, `compozy__*` native tools). Zero internals. Quality test: "could this be the docs usage page?"
4. **`_uiux.md`** (only when feature touches `web/`): surface map (S1..SN change map), component plan with design→production mapping, reuse-before-create discipline, names the opendesign visual reference + authorized deltas. Its presence is the deterministic UI-bearing signal for the QA tail.
5. **Companions retained:** `_user_stories.md` closes stage 1; `_tests.md` closes stage 2; unified ADR template.
6. **Approval model** (resolves playbook:47 vs skill "no draft-approval loops" contradiction): grill convergence at each stage checkpoint IS the approval; files are generated at stage end; user reviews generated files. No extra draft loops.
7. **Peer review** stays opt-in, re-scoped to the Technical part of `_spec.md`; `_uiux.md` reviews fold into existing review flows, not a new authoring artifact.
8. **`cy-research-issues`** lite path: `_techspec.md` lite → `_spec.md` lite (Technical part only).

## Delete targets

- `.agents/skills/cy-create-prd/` + `.agents/skills/cy-create-techspec/` (and `extensions/dev-cycle/skills/` mirrors)
- `.claude/skills/` symlinks for both
- `cy-spec-preflight/scripts/check-prd-implementation-leak.py` + `references/prd-checks.md`
- Playbook §2 (PRD) + §3 (TechSpec) as separate phases → collapsed into one Spec phase (two stages)

## Update targets (coupling map from exploration)

**Skills (both trees: `.agents/skills/` + `extensions/dev-cycle/skills/` where mirrored):**

- NEW `cy-create-spec/` — SKILL.md + references: spec-template, dx-template, uiux-template, user-stories-template, tests-template, adr-template, grill protocol
- `cy-spec-preflight` — phase blocks (prd|techspec → spec), six-markers check targets Technical part of `_spec.md`, tasks-checks input names
- `cy-create-tasks` — reads `_spec.md`, `_dx.md`, `_uiux.md`
- `cy-tasks-tail-qa-pair` — UI-bearing signal = `_uiux.md` presence
- `cy-spec-peer-review` — resolve `_spec.md`, `review_kind: spec`, validate-findings.sh
- `cy-loop-tasks` — SKILL.md, `scripts/init-state.py` exit 3 (`_techspec.md` → `_spec.md`), references/*
- `cy-execute-task`, `cy-review-round`, `cy-research-issues`, `cy-web-docs-impact`, `cy-workflow-memory`, `deep-review/references/context-pack.md`

**Runtime / Go / product:**

- `extensions/dev-cycle/embed_test.go` + `internal/daemon/agent_skill_resources_integration_test.go` (bundledSkillNames + `_techspec.md` assertions)
- `extensions/dev-cycle/loops/implement-tasks/loop.yaml` prompt; `extensions/dev-cycle/agents/code_implementer/AGENT.md`
- `web/src/systems/loops/mocks/fixture-implement-tasks.ts`

**Docs / governance:**

- `docs/_memory/spec-authoring-playbook.md` (collapse §2/§3, add `_dx.md`/`_uiux.md` guidance)
- `CLAUDE.md` (dispatch rows PRD/TechSpec → Spec, workflow rule :29, playbook description :184)
- `MIGRATION_GUIDE.md`, `packages/site/content/docs/migration/index.mdx`, `packages/site/content/docs/examples/implement-tasks-loop.mdx`
- `skills/compozy/references/tools-and-skills.md` (dev-cycle published skills list)

**State:**

- Active slugs: `worktree-support` (mechanically merge `_prd.md` + `_techspec.md` → `_spec.md`, delete originals), `_toplan`, `agent-plugins-interop` (prose mentions)
- `skills-lock.json` — investigate mechanism, update entries
- `docs/qa/scenarios/` — flag changed dev-cycle skill surface (extension kit resources renamed)

## Compozy Impact Audit

- Native tools: no impact — no `compozy__*` IDs/toolsets/schemas touched (verify at close)
- Extensibility and hooks: **impact** — dev-cycle extension kit skill resources renamed (2 removed, 1 added); embed + daemon publication tests updated
- Workspace data isolation: no impact — no runtime data ownership changes; artifacts are workspace files under `.compozy/tasks/`
- Official Compozy skill: **impact** — `skills/compozy/references/tools-and-skills.md` dev-cycle skill list updated

## Gates

`make gate` during iteration (Go tests + web fixture + site touched → likely escalates); `make gate-full` once at close. QA scenario walk per qa-execution contract for the flagged scenario before completion.
