Goal (incl. success criteria):

- Execute AGH tools-registry Task 16 real-scenario QA end to end against a fresh isolated lab, fix reproduced P0/P1 defects if any, write verification evidence, update task tracking, and create one local commit only after clean verification.

Constraints/Assumptions:

- Must use cy-workflow-memory, cy-execute-task, qa-execution, and cy-final-verify per task prompt.
- Must read workflow memory, AGH guidance, PRD docs, ADRs, and QA test plans before code or QA evidence edits.
- Must not run destructive git commands without explicit permission.
- Browser-use requested for highest-risk UI flow; availability is UNCONFIRMED.

Key decisions:

- Session ledger lives in looper `.codex/ledger/` per governing instructions; PRD workflow memory remains in AGH task memory files.

State:

- Corrected fresh QA lab bootstrapped; baseline gate uncovered and fixed BUG-001/BUG-002, smoke uncovered and fixed BUG-003.

Done:

- Read cy-workflow-memory, cy-execute-task, and qa-execution skill instructions.
- Read AGH root `AGENTS.md`/`CLAUDE.md`, standing directives index, AGH-local `agh-qa-bootstrap`, `real-scenario-qa`, and `agh-worktree-isolation` skills.
- Read workflow shared memory and task_16 memory.
- Read Task 16, Task 15, `_tasks.md`, key `_techspec.md` sections, all six QA test plan files, and ADR-001 through ADR-011.
- Ran QA contract discovery via the installed qa-execution helper; canonical gate is `make verify`, E2E is supported, web UI exists.
- Bootstrapped initial lab `tools-registry-task16-20260429-073832-489478`, then replaced it after socket path validation showed the generated UDS/provider paths exceeded macOS portable limits.
- Bootstrapped corrected lab `tools-registry-task16-20260429-075857-781754` with short runtime/provider homes, port `64177`, valid UDS/tmux paths, and browser mode `browser-use`.
- Mirrored bootstrap manifest into AGH task QA artifacts and seeded redaction sentinels plus behavioral scenario charter.
- Reproduced baseline `make verify` blocker: UDS API constructor kept process-home daemon socket after `WithHomePaths`, causing portable Unix socket length failure in the isolated provider home.
- Fixed UDS/HTTP API server constructors so `WithHomePaths` realigns default config unless `WithConfig` was explicit; added focused regression tests and corrected UDS tests to use `shortSocketPath`.
- Filed `.compozy/tasks/tools-registry/qa/issues/BUG-001.md`; targeted rerun passed for UDS/HTTP/config affected tests.
- Fixed `.agents/skills/agh-qa-bootstrap/scripts/bootstrap-qa-env.py` to validate portable Unix socket paths and allocate short runtime/provider homes; filed `.compozy/tasks/tools-registry/qa/issues/BUG-002.md`.
- Baseline `make verify` rerun passed under isolated provider HOME/CODEX_HOME with Go lint `0 issues`, `DONE 6970 tests`, and boundaries respected.
- Smoke TC-SEC-001 initially failed because `TestPolicyDenyAll` did not exist and the command ran no tests; added cross-backend deny-all dispatch coverage and filed `.compozy/tasks/tools-registry/qa/issues/BUG-003.md`.

Now:

- Rerun the smoke P0 lane from the top with the real-test guard enabled.

Next:

- Build visible execution checklist, seed redaction fixtures, run smoke P0 lane first, then continue through targeted/full/security/web/docs gates if smoke passes.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether browser-use:browser callable tooling is available in this session.
- UNCONFIRMED whether local AGH custom skills exist and are usable from repository files.
- `scripts/check-test-conventions.py` is absent in AGH, so that optional skill check is blocked.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/task_16.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/memory/MEMORY.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/memory/task_16.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/qa/bootstrap-manifest.json`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/qa/fixtures/redaction-sentinels.json`
- `/Users/pedronauck/dev/qa-labs/agh-tools-registry-task16-20260429-073832-489478-lab/qa-artifacts/qa/bootstrap-manifest.json`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/server.go`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/server_test.go`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/server.go`
- `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/server_test.go`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/qa/issues/BUG-001.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/qa/issues/BUG-002.md`
- `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-registry/qa/issues/BUG-003.md`
- `/Users/pedronauck/Dev/compozy/agh/.agents/skills/agh-qa-bootstrap/scripts/bootstrap-qa-env.py`
- `/Users/pedronauck/Dev/compozy/agh/internal/tools/dispatch_test.go`
