---
name: cy-orchestrate-tasks
description: Conducts one spec's tasks by delegation — spawns a dedicated bounded worker session per task, dispatches the briefing, and accepts only the task file on disk as proof of completion. Use when a prompt names a spec slug under .compozy/tasks/ and asks for its tasks to be orchestrated across worker sessions. Do not use for implementing a task directly, for review remediation, or for QA and pull-request work.
---

# Orchestrate Spec Tasks

This session is the **conductor** of the spec: it conducts, it does not play. Every task under
`.compozy/tasks/<slug>/` is implemented by a dedicated CompozyOS worker session.

## Required Inputs

- `slug` — the directory name under `.compozy/tasks/<slug>/`, holding `_tasks.md` (the task graph)
  and `task_NN.md` files whose frontmatter carries `status`, `title`, and `type`.
- `implementer` — the exact validated Agent identifier supplied in the Goal objective. The Loop
  defaults it to `code_implementer`; the conductor never substitutes that default.
- `backend_runtime`, `frontend_runtime`, and `default_runtime` — optional runtime objects supplied
  in the Goal objective. Each can carry `provider`, `model`, `reasoning`, and `speed`.

## Workflow

### 1. Load the task graph

Read `.compozy/tasks/<slug>/_tasks.md` and derive the execution order from `graph.edges`. Read the
frontmatter of every `task_NN.md`. Queue only tasks whose `status` is `pending` or `in_progress`,
in graph order.

_Done when:_ the queue lists the id, `title`, and `status` of every queued task, in execution order.

### 2. Spawn the worker session

Choose the worker runtime by exact frontmatter type: `backend` uses `backend_runtime`, `frontend`
uses `frontend_runtime`, and every other value uses `default_runtime`. Merge the task's own
frontmatter `runtime` over that category object field by field before building spawn flags. Create
one bounded `spawned` session bound to this conductor. Bind the exact `implementer` value from the
Goal once, then pass it as one quoted argument. TTL and parent-stop are the containment contract;
`--ttl-seconds` is mandatory:

```bash
IMPLEMENTER="<exact implementer identifier from the Goal>"
compozy spawn --agent "$IMPLEMENTER" \
  --name "orchestrate-<slug>-<task_id>" \
  --role worker \
  --ttl-seconds 3600 \
  --auto-stop-on-parent=true \
  --idempotency-key "orchestrate-<slug>-<task_id>" \
  -o json
```

Append `--provider`, `--model`, `--reasoning-effort`, and `--speed` only for non-empty fields in the
selected runtime object; `reasoning` maps to `--reasoning-effort`. When every field is empty, omit
provider, model, and reasoning flags so the child resolves them through the selected Agent and
workspace. Under current spawn behavior, omitted speed inherits the parent session. Capture
`.session.id`.

When the spawn response is lost or ambiguous, reconcile before acting — a blind respawn creates a
second worker for one task:

```bash
compozy session list \
  --parent "$COMPOZY_SESSION_ID" \
  --type spawned \
  --state active \
  --query "orchestrate-<slug>-<task_id>" \
  -o json
```

Reuse the id only when exactly one returned session has `name` equal to
`orchestrate-<slug>-<task_id>` and `agent_name` equal to `$IMPLEMENTER`. Zero results after a
confirmed spawn failure blocks the task. A mismatched `agent_name` is never adopted or followed by
a blind respawn: stop the session only when its parent and exact name prove conductor ownership,
then block the task. More than one exact match: stop every conductor-owned match and block the task.

_Done when:_ exactly one worker session id is held for this task, or the task is marked blocked.

### 3. Dispatch the briefing and wait

Send the briefing and wait in the same command. `compozy session prompt` blocks until the worker's
turn ends in every output mode; `-o jsonl` is the form that also leaves a durable per-task event log
as evidence:

```bash
mkdir -p .compozy/tasks/<slug>/logs
compozy session prompt <session_id> "<briefing>" \
  -o jsonl > .compozy/tasks/<slug>/logs/<task_id>.jsonl 2>&1
```

`--queue`, `--interrupt`, and `--steer` return at admission instead of at turn end, so they cannot
carry a briefing this workflow waits on.

_Done when:_ the command has returned and its outcome is recorded, including a failed prompt.

### 4. Check the proof

Re-read the task file frontmatter. `status: completed` on disk is the only accepted proof — the
worker's closing message never completes a task.

If the status is anything else, send one corrective prompt in the **same** session, using the same
blocking form, naming exactly what is missing. A second failure produces a `blocked` result citing
`.compozy/tasks/<slug>/logs/<task_id>.jsonl` as evidence.

_Done when:_ the task reads `completed` on disk, or it is marked blocked with the log path cited.

### 5. Stop the worker session

Run `compozy session stop <session_id> -o json` before advancing to the next task and before
reporting any result — after success, after a prompt failure, after the corrective attempt, and
after a block. A failed stop is itself a `blocked` result and must be recorded with its error. TTL
and parent-stop contain abrupt cancellations; they do not replace this stop.

_Done when:_ the worker session is no longer active and the stop outcome is recorded.

### 6. Report

Answer with the structured result the loop asks for: `status`, optional `summary` (1–3 sentences),
and optional `tasks` — each task entry names the worker session that executed it. Use `complete`
or `blocked`; task-file frontmatter continues to use `completed`.

_Done when:_ every task in the queue appears in the report with its task id and session id.

## Worker briefing

Fill the fields and send this as the prompt body:

> Implement exactly task `<task_id>` — `<title>` — of the spec `.compozy/tasks/<slug>/`.
>
> Required skills:
>
> - `cy-workflow-memory`: use before editing code. Memory directory
>   `.compozy/tasks/<slug>/memory`, shared memory `.compozy/tasks/<slug>/memory/MEMORY.md`, task
>   memory `.compozy/tasks/<slug>/memory/<task_id>.md`.
> - `cy-execute-task`: the end-to-end execution workflow for this task.
> - `cy-final-verify`: use before any completion claim, to identify and run the repository's real
>   verification commands.
>
> Read the repository `AGENTS.md`/`CLAUDE.md` and surface-specific instructions, then
> `.compozy/tasks/<slug>/_spec.md` and `_tasks.md`; treat those plus the task file body as the
> source of truth. Keep scope tight to this task and record follow-up work instead of widening it.
> Preserve unrelated worktree changes. Run every Validation, Test Plan, or Testing item the task
> body lists and fix what fails. With verification clean, set the task file frontmatter to
> `status: completed`. Leave the changes uncommitted — commit and pull request belong to other
> surfaces.

## Rules

- The conductor conducts: in this session the work is spawn, dispatch, wait, check, stop. Code edits
  belong to workers.
- One task at a time, in graph order, with one worker session per task — reused only for that task's
  corrective prompt.
