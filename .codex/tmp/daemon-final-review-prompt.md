Review the daemon specification at `.compozy/tasks/daemon/_techspec.md` against the current codebases in:

- `/Users/pedronauck/Dev/compozy/looper`
- `/Users/pedronauck/dev/compozy/agh`

You MUST use multiple subagents.

Minimum delegation:

- one subagent focused on current `looper` architecture and migration seams
- one subagent focused on AGH patterns and concrete reuse opportunities
- one subagent focused on finding architectural gaps, contradictions, or missing rollout details in the TechSpec

Review scope:

- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/adrs/`
- `docs/plans/2026-04-17-compozy-daemon-design.md`

Expectations:

- do not modify any files
- produce the final answer in pt-BR
- findings first, ordered by severity
- focus on wrong assumptions, missing migrations, storage gaps, transport gaps, CLI inconsistencies, sync/archive edge cases, and places where the spec should borrow more directly from AGH
- include concrete file references from both repositories when relevant
- if there are no major issues, say that clearly, then list residual risks and improvements
- keep the review pragmatic and implementation-oriented

Important context that the review must respect:

- this is a greenfield-friendly redesign
- Compozy keeps its workflow model and TUI-first posture
- `_tasks.md` and `_meta.md` do not survive in the daemonized model
- runtime state moves to `~/.compozy`
- operational storage is `global.db` plus `~/.compozy/runs/<run-id>/run.db`
- CLI and web talk to the daemon via AGH-aligned REST over UDS and localhost HTTP, with SSE for streaming
- extensions stay run-scoped subprocesses in v1
- same `run_id` in parallel is invalid; different run IDs may run concurrently

Deliverable format:

1. Verdict
2. Findings
3. Residual risks
4. Suggested spec adjustments
