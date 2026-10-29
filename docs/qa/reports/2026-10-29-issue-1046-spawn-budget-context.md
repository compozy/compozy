# QA report: issue #1046 spawn budget in agent context

- Date: 2026-10-29
- Scenario: `RT-spawn-budget-context`
- Lab: isolated `COMPOZY_HOME` (allocate-isolation slug `lab1046`), own HTTP port and UDS socket,
  provider `claude` / `claude-opus-5-5` / reasoning `medium`, workspace `/tmp/lab1046/ws`.
- Binaries: `main` at 55ff6b776 ("before") and this branch ("after").

## Before (main)

Session `forked-worker` was created with `--parent` on a root user session. Its live context had
`lineage.spawn_budget: {max_children: 0, max_depth: 0, ttl_seconds: 0}` and `lineage.spawn_depth: 1`
next to `limits: {max_children: 5, max_spawn_depth: 1}`. Asked to run the frontend on
`claude-sonnet-5-5` and the backend on `claude-haiku-4-5` in parallel child sessions, the agent
refused without calling the spawn tool:

> I can't create any child sessions. I'm a forked worker already at `spawn_depth: 1`, which equals
> the workspace `max_spawn_depth: 1`. My own lineage also sets `spawn_budget` to
> `max_children: 0, max_depth: 0`.

The engine would have admitted both children: the session is not spawned-type, so its governed
depth is 0, and the zero budget resolves to 5/1.

## After (branch)

Same setup, fresh `--parent` session. Context now shows `spawn_budget: {max_children: 5,
max_depth: 1}` and `limits: {max_children: 5, max_spawn_depth: 1, governed_spawn_depth: 0,
can_spawn_children: true, ...}`. The agent answered:

> My governed spawn depth is actually 0 despite the displayed depth of 1, so I can spawn up to 5
> children (max depth 1), though those children won't be able to spawn further themselves.

It spawned both children in parallel; `frontend/HELLO.md` and `backend/HELLO.md` were written by
the children on the requested models.

Depth path: a spawned child (`spawn-limit-probe`) quoted its limits verbatim:
`{"max_children":5,"max_spawn_depth":1,"can_spawn_children":false,"spawn_blocked_reason":"Spawned
child sessions cannot spawn governed children: this is a spawned child session and max_depth is 1.
Use compozy__subagent_delegate instead.",...}`. The forced engine refusal (`SpawnDepthError`) and
its tool recovery are covered by `TestManagerSpawnReportsTheExhaustedBudget` and the native
`session_spawn` depth case.

Model override path: the first after-run showed the agent's `session_spawn` calls with `model` and
no `provider` failing as `config_validation_failed: invalid tool request`, with no cause. The tool
now returns `provider is required when model is set` with a recovery to pass `provider`; covered by
the native `session_spawn` runtime-override case.

## Verdict

PASS. Remaining limitation: `governed_spawn_depth` for spawned sessions is reported as the lower
bound 1; it does not change `can_spawn_children`, because every daemon-written budget has
`max_depth` 1.
