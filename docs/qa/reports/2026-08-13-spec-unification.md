# QA Run Report — 2026-08-13 — spec-unification

Scope: spec-unification hard cut — `cy-create-prd` + `cy-create-techspec` replaced by `cy-create-spec`; dev-cycle bundle shrinks from nine to eight published skills.

## Scenarios walked

| Scenario | Verdict | Notes |
| --- | --- | --- |
| ET-dev-cycle-skill-bundle | pass | Exact eight-skill bundle verified via CLI against a fresh daemon in an isolated lab. |

## Evidence

- Lab: `/Users/pedronauck/dev/qa-labs/compozy-spec-unification-skill-bundle-20260814-022518-316742-lab` (profile targeted, surfaces cli+runtime)
- `qa-artifacts/qa/skill-list.json` — exactly `compozy`, seven `cy-*` skills, and `git-rebase`; `cy-create-prd`/`cy-create-techspec` absent.
- `qa-artifacts/qa/skill-view-cy-create-spec.txt` — bundled body renders through `compozy skill view`.
- Retired skill view returns deterministic `skill not found: "cy-create-prd"`.
- `qa-artifacts/qa/teardown.json` — `"clean": true`, zero survivors.

Runtime-side publication and boot rebuild are additionally covered by `TestAgentSkillPublicationAndBootRebuild` (`internal/daemon`, integration tag) and the dev-cycle embed contract suite (102 tests), both green on this tree.
