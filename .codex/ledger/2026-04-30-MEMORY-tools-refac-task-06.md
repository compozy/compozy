Goal (incl. success criteria):

- Execute AGH tools-refac Task 06: session-bound autonomy tools and claim-token hard cut, with tests/contract updates, tracking updates, clean verification, and one local commit if allowed by workflow gates.

Constraints/Assumptions:

- Must use cy-workflow-memory, cy-execute-task, cy-final-verify, golang-pro, testing-anti-patterns, no-workarounds, and systematic-debugging as applicable.
- Caller requires reading shared workflow memory and current task memory before editing code.
- Caller-provided current task memory path is `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_06.md`.
- Must not run destructive git commands without explicit permission.
- Automatic commit is enabled only after clean verification, self-review, memory/tracking updates.

Key decisions:

- Session ledger lives in looper `.codex/ledger/`; PRD workflow memory lives in AGH `.compozy/tasks/tools-refac/memory/`.
- No code edits may start until the missing current task memory file mismatch is resolved.

State:

- Blocked before implementation by missing workflow memory file.

Done:

- Loaded cy-workflow-memory, cy-execute-task, golang-pro, systematic-debugging, no-workarounds, testing-anti-patterns, and cy-final-verify skill instructions.
- Read shared workflow memory at `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/MEMORY.md`.
- Confirmed `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_06.md` is missing.
- Listed AGH tools-refac memory directory: only `MEMORY.md` and `task_04.md` exist.
- Scanned existing related ledgers for cross-agent awareness.

Now:

- Waiting for the current task memory path to be created or for explicit direction on how to resolve the mismatch.

Next:

- After memory path is available, read AGH guidance, PRD docs, task file, ADRs, current lease writers, then build the cy-execute-task checklist and capture pre-change baseline.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_06.md` should be initialized from a template or if another existing task memory file should be used.
- UNCONFIRMED whether the AGH-specific skills `agh-code-guidelines`, `agh-test-conventions`, and `agh-contract-codegen-coship` exist outside this session's installed skill list.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/MEMORY.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/memory/task_06.md` (missing)
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/task_06.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/_tasks.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac/_techspec.md`
