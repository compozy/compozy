# Phase → Lessons mapping

Read the matching lessons before authoring. All paths relative to `docs/_memory/lessons/`.

## Phase: `prd`

- `L-013-prd-must-not-name-implementation.md` — strip frameworks/storage/error codes/file formats.

## Phase: `techspec`

- `L-001-detached-prompt-lifetime.md` — request lifetime ≠ execution lifetime.
- `L-003-task-runs-single-queue.md` — no parallel queue alongside `task_runs`.
- `L-004-manual-equals-peer.md` — manual + autonomous converge on same primitives.
- `L-005-authoritative-primitive-exclusivity.md` — observe ≠ own.
- `L-006-greenfield-delete-not-adapt.md` — name delete targets explicitly.
- `L-008-schema-migrations-mandatory.md` — declarative schema + appended Goose SQL; no boot reconciliation.
- `L-012-techspec-prose-only-rework.md` — six quality markers required.

## Phase: `tasks`

- `L-002-tparallel-vs-tsetenv.md` — test plan must respect Go testing contract.
- `L-007-e2e-follows-runtime-contract.md` — E2E mocks ship with runtime contract changes.
- `L-009-concurrent-worktree-deadlock.md` — QA tasks include `agh-worktree-isolation`.
- `L-011-fraco-test-coverage-pushback.md` — test density must be proportional to behaviors.

## Phase: `task-body`

- `L-002-tparallel-vs-tsetenv.md`
- `L-008-schema-migrations-mandatory.md`
- `L-001-detached-prompt-lifetime.md`
- All applicable from the Tasks list above.

## Always-on (every phase)

- `docs/_memory/standing_directives.md` (SD-001..SD-011).
- `docs/_memory/glossary.md` (vocabulary discipline).
- `docs/_memory/spec-authoring-playbook.md` (canonical playbook).
- `CLAUDE.md` (Architecture, Autonomy Contracts, Security Invariants, Workflow Rules).
