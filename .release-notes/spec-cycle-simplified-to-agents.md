---
title: Spec delivery moves from Loops to agents
type: breaking
---

The `spec-cycle` extension now ships 4 agents and 2 skills, with no Loops and no tools. Deliver a spec by starting `spec_conductor`, and hand small work to the stays-on `orchestrator`. Each agent carries its own procedure, so picking the agent is enough. This is a hard cut with no deprecation window.

Upgrading needs no manual step. The daemon starts normally, and nothing under `~/.compozy` needs deleting. Existing sessions, automations, stored Loop overrides and `.compozy/tasks` files are kept, and anything pointing at a removed resource fails only when it is used, with a clear error.

| Removed                                                                             | Use instead                               |
| ----------------------------------------------------------------------------------- | ----------------------------------------- |
| Loop `implement-tasks`                                                              | Agent `spec_conductor`                    |
| Loop `review-and-fix`                                                               | Agent `reviewer`, then `code_implementer` |
| Agent `review_fixer`                                                                | Agent `code_implementer`                  |
| `ext__spec_cycle__import_tasks`                                                     | `_slices.md` read by `spec_conductor`     |
| `ext__spec_cycle__write_review_artifacts`, `ext__spec_cycle__finalize_review_round` | Agent `reviewer` findings file            |
| Skill `cy-create-tasks`                                                             | `_slices.md` in `cy-create-spec`          |
| Skills `cy-execute-task`, `cy-final-verify`, `cy-workflow-memory`                   | Agent `code_implementer`                  |
| Skill `cy-orchestrate-tasks`                                                        | Agent `spec_conductor`                    |
| Skills `cy-review-round`, `cy-fix-reviews`                                          | Agents `reviewer` and `code_implementer`  |

- `cy-create-spec` now writes `_slices.md` and offers peer review after you approve a spec.
- A new heartbeat source, `work_log`, surfaces accepted work in `.compozy/work-log.md` that never started.
- Running a removed Loop, starting a session with a removed agent, or calling a removed tool fails with a not found error. Automations that target them stay saved, and each run fails on its own.
- A run of a removed Loop that is active during the upgrade fails at its first removed step with a clear error. Its history stays, and other runs are unaffected.
- A stays-on instance of a removed agent pauses as `definition_missing`, and nothing is deleted.
- A workspace Loop with one of the removed names keeps working.

**Batuta leaves the catalog.** Batuta is built on the removed Loops, so the catalog no longer offers it. An installed copy keeps loading, and only `batuta-deliver` fails when run.
