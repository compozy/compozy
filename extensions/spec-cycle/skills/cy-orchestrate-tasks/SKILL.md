---
name: cy-orchestrate-tasks
description: Conducts one spec's tasks by delegation — starts one bounded worker call per task and accepts only the task file on disk as proof. Use when a prompt names a spec slug under .compozy/tasks/ and asks for orchestration across worker sessions. Do not use for direct implementation, review remediation, QA, or pull-request work.
---

# Orchestrate Spec Tasks

This session is the **conductor** of the spec: it conducts, it does not play. Every task under
`.compozy/tasks/<slug>/` is implemented by a dedicated CompozyOS worker session.

## Required Inputs

- `slug` — the directory name under `.compozy/tasks/<slug>/`, holding `_tasks.md` (the task graph)
  and `task_NN.md` files whose frontmatter carries `status`, `title`, and `type`.
- `backend_runtime`, `frontend_runtime`, and `default_runtime` — optional runtime objects supplied
  in the Goal objective. Each can carry `provider`, `model`, `reasoning`, and `speed`.

## Workflow

### 1. Load the task graph

Read `.compozy/tasks/<slug>/_tasks.md` and derive the execution order from `graph.edges`. Read the
frontmatter of every `task_NN.md`. Queue only tasks whose `status` is `pending` or `in_progress`,
in graph order.

_Done when:_ the queue lists the id, `title`, and `status` of every queued task, in execution order.

### 2. Call the worker

Choose the worker runtime by exact frontmatter type: `backend` uses `backend_runtime`, `frontend`
uses `frontend_runtime`, and every other value uses `default_runtime`. Merge the task's own
frontmatter `runtime` over that category object field by field. Join provider, model, reasoning,
and speed as `provider/model/reasoning/speed`, preserving empty positions. Build the complete task
briefing, then create one typed call to the `code_implementer` agent. The call lineage and idle TTL
are the containment contract; `--workspace`, `--idle-ttl`, and `--idempotency-key` are mandatory:

```bash
compozy call code_implementer "<briefing>" \
  --workspace "<workspace_id>" \
  --idle-ttl 1h \
  --idempotency-key "orchestrate-<slug>-<task_id>" \
  -o json
```

Append `--runtime "<provider>/<model>/<reasoning>/<speed>"` only when at least one selected field is
non-empty. When every field is empty, omit `--runtime` so the child resolves from the
`code_implementer` definition and workspace defaults. Capture `.call_id` and `.child_session_id`.

When the response is lost or ambiguous, repeat the same command with the same idempotency key. The
daemon returns the original call instead of creating a second worker.

_Done when:_ exactly one call id and child session id are held for the task, or the task is marked
blocked with the typed call error.

### 3. Wait for the result

Wait on the durable call and save the receipt as evidence:

```bash
mkdir -p .compozy/tasks/<slug>/logs
compozy call await <call_id> --timeout 1h \
  -o jsonl > .compozy/tasks/<slug>/logs/<task_id>.jsonl 2>&1
```

_Done when:_ the call settled or reached its timeout checkpoint and the outcome is recorded.

### 4. Check the proof

Re-read the task file frontmatter. `status: completed` on disk is the only accepted proof — the
worker's closing message never completes a task.

If the status is anything else, create one follow-up call to `<child_session_id>`, naming exactly
what is missing, and await it in the same blocking form. A second failure produces a `blocked`
result citing `.compozy/tasks/<slug>/logs/<task_id>.jsonl` as evidence.

_Done when:_ the task reads `completed` on disk, or it is marked blocked with the log path cited.

### 5. Stop the worker session

Run `compozy session stop <child_session_id> -o json` before advancing to the next task and before
reporting any result — after success, after a prompt failure, after the corrective attempt, and
after a block. A failed stop is itself a `blocked` result and must be recorded with its error. The
idle TTL contains abrupt cancellations; it does not replace this stop.

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

- The conductor conducts: in this session the work is call, wait, check, stop. Code edits
  belong to workers.
- One task at a time, in graph order, with one worker session per task — reused only for that task's
  corrective prompt.
