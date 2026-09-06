# Herdr delegation lanes

Use `herdr-orchestration` for generic transport, configured models, named tabs,
preflight, prompts, waits, evidence verification, and retirement. Read the common
contract and active lane once; this reference supplies only Compozy loop policy.
Do not duplicate transport commands or re-run preflight for each packet.

## Common dispatch contract

- Give each worker a named tab using the generic skill. Reuse a suitable live
  worker for related implementation and integration follow-ups.
- The orchestrator owns task/state completion and commits. Workers update their
  assigned files and task memory, report evidence, and never commit or push.
- Capture HEAD before dispatch and verify it is unchanged afterward. The diff
  and reported checks must cover the current working-tree inputs as well.
- Inspect worker evidence; rerun only missing, invalidated, or unreliable checks
  plus integration checks the worker did not cover. A handoff does not require
  a second `cy-final-verify` cycle or a new QA lab.
- A normal wait timeout is not a worker failure. Read status, continue useful
  independent work, and observe again. Recover missing evidence through the
  existing worker before considering a relaunch.

## Delegation packet

Include repo root, slug, task/slice path, owned files, out-of-scope surfaces,
dependencies/current interfaces, shared/current memory paths from
`memory-protocol.md`, and any caller overrides. Tell the worker it shares the
codebase, must preserve others' edits, and must report interface changes that
could invalidate dependent work.

Assign the task's focused checks and explicit task-owned visual/probe acceptance.
Name the Phase C integration checks and Phase E delivery gates retained by the
orchestrator; do not ask the worker to perform them as extra task gates. Include
available evidence with checked inputs, required artifact paths, stop conditions,
and **do not commit or push; leave the owned diff for the orchestrator**.

## Frontend lane (Phase B)

Active when `state.frontend_agent` was selected with `--frontend`.

- In tasks mode, trust `lane=frontend agent=<x>` from `detect-phase.py` for
  frontmatter `type: frontend`.
- In free mode, delegate slices whose owned paths are exclusively `web/**`,
  `packages/ui/**`, or `packages/site/**`; mixed slices run locally.
- Use the configured `claude` or `cursor` runtime/model. Launch through the
  generic skill's TUI procedure, not a headless command or a split pane.

The packet names the relevant task/contracts, scoped `AGENTS.md`/`CLAUDE.md`,
`cy-execute-task`, and supplied `cy-workflow-memory` paths. Reuse grounded
preflight facts. Frontend checks run through root Turborepo with affected-package
filters. Apply `cy-final-verify` to the resulting focused evidence within that
validation step. Inspect a representative visual state early when a reference
exists; complete task-owned rows and flag integration-owned rows for Phase C.

Complete the Phase B action after the worker reports done, its owned outcome and
memory have been inspected, task-scope PASS evidence is valid, and HEAD is
unchanged. The orchestrator satisfies the repository commit gate, marks the task
complete, and checkpoints. Missing evidence keeps the action open; ask the same
worker for the missing item rather than redispatching the entire task.

## QA-report lane (Phase C, optional)

Use a configured Claude worker in direct mode for substantial independent QA
planning or explicit delegation. Small plan updates and evidence reconciliation
run locally; no worker is needed to record that all applicable evidence is current.

The packet names `qa-report`, `qa-docs-path=docs/qa`, affected scenarios, existing
plans/evidence, and only the remaining changed/integration journeys and visual
rows. Update journey maps/charters only where missing or changed. Do not start a
lab, walk journeys, or run full suites to produce the plan.

Before recording `--qa-report-done`, verify the scoped plan or no-work disposition,
any changed artifact paths, current memory, and unchanged HEAD. Retain the worker
for planned follow-ups; retire it when its assignment is settled. The orchestrator
runs the selected QA execution and final delivery.
