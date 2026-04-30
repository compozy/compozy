Goal (incl. success criteria):

- Execute AGH tools-refac Task 02: dynamic per-call policy resolver, runtime discovery defaults for `agh__bootstrap`/`agh__catalog`, policy-sensitive projection cache keys, tests, tracking updates, and one local commit after clean verification.

Constraints/Assumptions:

- Must use cy-workflow-memory, cy-execute-task, cy-final-verify, golang-pro, testing-anti-patterns, no-workarounds, and systematic-debugging as applicable.
- Must read AGH guidance, PRD docs, `_techspec.md`, `_tasks.md`, ADR-001, ADR-002, and workflow memory before editing code.
- Must not run destructive git commands without explicit permission.
- Automatic commit is enabled only after clean `make verify`, self-review, memory/tracking updates.

Key decisions:

- Session ledger lives in looper `.codex/ledger/`; PRD workflow memory lives in AGH `.compozy/tasks/tools-refac/memory/`.

State:

- Initial context loading in progress.

Done:

- Loaded required workflow memory files for tools-refac Task 02.
- Loaded cy-workflow-memory, cy-execute-task, cy-final-verify, golang-pro, testing-anti-patterns, no-workarounds, and systematic-debugging skill instructions.
- Scanned existing looper ledgers; tools-registry QA ledger notes registry foundation risk and recent QA fixes.

Now:

- Read AGH guidance files, PRD docs, ADRs, and current code surfaces.

Next:

- Build the visible execution checklist and capture pre-change baseline before implementing.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether local AGH custom skill files exist and contain additional guidance beyond AGH/CLAUDE references.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/task_02.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/_techspec.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/_tasks.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/MEMORY.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_02.md`
