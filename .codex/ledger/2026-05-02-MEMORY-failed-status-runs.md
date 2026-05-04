Goal (incl. success criteria):

- Diagnose why `cy tasks run`/Compozy runs against `../agh/.compozy/tasks/agent-soul` report `failed status`.
- Success: identify the source artifact/log/code path producing failed status and explain root cause with evidence.
  Constraints/Assumptions:
- Must not run destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit permission.
- Use local code search only for repo code.
- Follow systematic-debugging before proposing fixes.
  Key decisions:
- Start with persisted run/task artifacts, then trace into looper/Compozy status handling if needed.
  State:
- Root cause identified for observed failed statuses.
  Done:
- Read relevant skill guidance: systematic-debugging, no-workarounds, compozy.
- Scanned ledger directory for cross-agent awareness; no own ledger existed before this turn.
- Located relevant runs:
  - `tasks-agent-soul-653749-20260502-044224-000000000`: all 17 jobs failed immediately at session creation due required hook `agent.pre_session_create` in `cy-qa-workflow` returning `-32602 Invalid params: illegal base64 data at input byte 0`.
  - `tasks-agent-soul-653749-20260502-044956-000000000`: task_01 failed after ACP session update status `failed`; persisted cause is Codex ACP `-32603` from remote compact request `401 Unauthorized`, `token_revoked`. Later daemon force stop canceled remaining jobs.
  - `tasks-agent-soul-653749-20260502-043447-000000000`: succeeded, not the failing screenshot.
- Traced UI text `Session reported failed status` to looper code:
  - `internal/core/run/transcript/model.go:101`
  - `internal/core/run/internal/acpshared/session_handler.go:245`
    Now:
- Prepare concise explanation with evidence.
  Next:
- None unless user asks to fix one of the two root causes.
  Open questions (UNCONFIRMED if needed):
- UNCONFIRMED: whether user wants a code fix for hook base64 handling, Codex auth refresh, or daemon shutdown behavior.
  Working set (files/ids/commands):
- `.codex/ledger/2026-05-02-MEMORY-failed-status-runs.md`
- Target task root: `../agh/.compozy/tasks/agent-soul`
- Runs:
  - `/Users/pedronauck/.compozy/runs/tasks-agent-soul-653749-20260502-044224-000000000`
  - `/Users/pedronauck/.compozy/runs/tasks-agent-soul-653749-20260502-044956-000000000`
