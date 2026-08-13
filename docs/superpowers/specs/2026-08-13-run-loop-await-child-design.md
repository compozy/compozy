# Durable `run-loop` Await Semantics

**Status:** Approved

**Issue:** [#386](https://github.com/compozy/compozy/issues/386)

## Minimal Generic Reproduction

The defect requires no extension, agent session, provider, skill, or imported task. It can be reproduced with two workspace-authored Loops and the public CLI.

The child Loop contains only a durable wait, keeping it live long enough to inspect the parent:

```yaml
apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: await-child-hold
  description: Generic child Loop that stays live long enough to inspect its parent.
concurrency: allow
contract:
  goal: Hold a child run in a live state.
  definition_of_done: The durable wait finishes.
  stop_when: "nodes.hold.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2, hash_fields: ["nodes.hold.status"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: hold
      class: control
      kind: wait
      params: { for: 10m }
  edges: []
start: [{ kind: manual }, { kind: http }, { kind: uds }]
```

The parent starts that child twice, with an authored edge requiring strict order:

```yaml
apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: await-parent-probe
  description: Generic parent with two sequential awaited child Loops.
concurrency: allow
contract:
  goal: Run two child Loops in strict sequence.
  definition_of_done: Both awaited child Loops finish in authored order.
  stop_when: "nodes.second_child.status == 'succeeded'"
  verification: []
  terminal_states: [done, failed, blocked, exhausted, stalled]
  iteration_cap: 1
  no_progress: { window: 2, hash_fields: ["nodes.first_child.status", "nodes.second_child.status"] }
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: halt }
graph:
  nodes:
    - id: first_child
      class: action
      kind: run-loop
      params: { loop: await-child-hold, mode: await }
    - id: second_child
      class: action
      kind: run-loop
      params: { loop: await-child-hold, mode: await }
  edges:
    - { from: first_child, to: second_child }
start: [{ kind: manual }, { kind: http }, { kind: uds }]
```

After saving the files as `await-child-hold.yaml` and `await-parent-probe.yaml`, validate, publish, run, and inspect them:

```bash
WORKSPACE=/path/to/workspace

compozy loop validate --workspace "$WORKSPACE" --file await-child-hold.yaml -o json
compozy loop validate --workspace "$WORKSPACE" --file await-parent-probe.yaml -o json
compozy loop create --workspace "$WORKSPACE" --file await-child-hold.yaml -o json
compozy loop create --workspace "$WORKSPACE" --file await-parent-probe.yaml -o json

PARENT_RUN_ID="$(compozy loop run \
  --workspace "$WORKSPACE" \
  --name await-parent-probe \
  -o json | jq -r '.run.id')"

compozy loop status --workspace "$WORKSPACE" --run-id "$PARENT_RUN_ID" -o json
compozy loop runs --workspace "$WORKSPACE" -o json \
  | jq '.runs[] | select(.loop_name == "await-child-hold")'
```

On the affected runtime, both child runs start and remain live while the parent becomes `done`. Its generation records `first_child` and `second_child` as `succeeded`, even though each structured output says `status: "awaiting_child"`. The second child therefore starts before the first child terminates.

The corrected behavior is one live child, a live parent whose `first_child` node is `awaiting_child`, and no `second_child` run until the first child reaches an accepted terminal outcome.

Both standalone definitions were checked with `compozy loop validate` before publication of the reproduction.

## Real-World Discovery Evidence

The generic defect was originally exposed at 2026-08-13 15:06 America/Sao_Paulo by an installed resource extension. An extension-published agent session started this delivery graph:

```text
input -> import_tasks -> implement(run-loop, await) -> review(run-loop, await)
```

The parent run `looprun-ae54eccd8e6a361b` reached `done` while both child runs were still live:

- implementation: `looprun-8458a0aea51bca58`
- review: `looprun-9205fd25577f44e2`

The parent generation reported both nodes as `succeeded`, but each structured output contained `status: "awaiting_child"`. Review started before implementation completed and inspected an unimplemented repository.

The evidence was read through public structured surfaces:

```bash
compozy loop status --run-id looprun-ae54eccd8e6a361b -o json
compozy loop status --run-id looprun-8458a0aea51bca58 -o json
compozy loop status --run-id looprun-9205fd25577f44e2 -o json
compozy loop runs --workspace /home/franciscpd/Projects/batuta-smoke-lab -o json
```

The extension is discovery evidence, not a reproduction dependency. The promoted build and its predecessor have no diff under `internal/loop`, so the smoke exposed a pre-existing runtime defect rather than a regression from the extension-session changes.

## Root Cause

`RunLoopActionExecutor.Execute` starts the child and returns a structured result containing the child run ID and `awaiting_child`. `executeActionTaskRun` persists that value as the completed worker task result.

When the coordinator later refreshes the generation, `refreshCompletedTaskRunOutput` treats every successful mechanical action task as terminal success. It does not interpret the reserved `run-loop` result. The generation snapshot therefore records `succeeded` without copying the child run ID into `GenerationOutput.ChildLoopRunID`.

The existing `refreshAwaitingChildOutput` logic is correct once a snapshot already contains `status: awaiting_child` and `child_loop_run_id`. Existing tests begin at that later state, so they do not cover the broken completed-task-to-generation-output transition.

## Goals

1. Preserve the live `awaiting_child` state when an awaited child has started but has not terminated.
2. Preserve the child run ID in the parent generation snapshot.
3. Prevent downstream nodes and parent terminal settlement while an awaited child is live.
4. Restore the wait from durable task and generation state after daemon restart without submitting another child.
5. Propagate accepted child terminal outcomes before scheduling dependents.
6. Fail closed when the completed `run-loop` result is malformed or inconsistent with the authored mode.
7. Keep the primary reproduction independent of any extension while preserving the extension-driven discovery evidence.

## Non-Goals

- Changing `detach` semantics.
- Changing Loop, extension, native-tool, HTTP, UDS, or CLI schemas.
- Adding polling, sleeps, or extension-specific behavior.
- Fixing transcript assembly that drops whitespace-only message chunks.
- Fixing provider/model routing diagnostics found during the same smoke.
- Claiming atomic recovery from a process crash between child creation and durable completion of the action task. That is a separate transaction-boundary problem and requires separate evidence and design.

## Design

### Typed reserved-action result

Introduce an internal typed decoder for the completed `run-loop` action result. It consumes the persisted task result and validates:

- the owning graph node exists and is `kind: run-loop`;
- the authored or defaulted mode is `await`;
- `status` is exactly `awaiting_child`;
- `loop_run_id` is non-empty.

After decoding, the coordinator immediately resolves the referenced child through the existing `refreshAwaitingChildOutput` path. That lookup validates that the child belongs to the same workspace and names the current parent run before any dependent can be scheduled.

This decoder is internal to `internal/loop`; it does not alter the public result shape.

### Coordinator transition

Before generic mechanical-action success handling, the coordinator recognizes a completed awaited `run-loop` action and transforms the parent output candidate to:

```text
Status         = awaiting_child
ChildLoopRunID = <persisted child run ID>
OutputRef      = <persisted structured action result>
```

The same refresh then passes the candidate through `refreshAwaitingChildOutput`. A live child makes the refresh report the output as live. The graph scheduler therefore cannot reserve dependents and the parent cannot settle terminally.

On later coordinator passes, the existing `refreshAwaitingChildOutput` path remains the sole owner of child terminal interpretation:

- `done` or `no-op` -> parent node `succeeded`;
- every other child terminal outcome -> parent node `failed` with the existing classified child reference;
- live child -> keep `awaiting_child` and yield;
- timeout -> fail and request the existing child stop behavior.

### Restart recovery

The recovery test will stop runtime components only after the child ID is durable in either the completed action task result or the generation snapshot, then reopen the same database and rebuild the coordinator/runtime composition.

On recovery:

1. The completed action task is not executed again.
2. The coordinator reconstructs or reads the same `awaiting_child` output.
3. The same child run ID remains attached to the parent cell.
4. No second child run exists for that cell.
5. The next graph node remains unscheduled until the original child terminates.

This is deterministic state recovery, not time-based polling.

### Invalid results

An awaited `run-loop` completed task with missing, empty, malformed, or contradictory child identity must not become `succeeded`. The coordinator records a classified failure with a stable internal reason and does not schedule dependents.

A `detach` result remains ordinary successful action output and never enters the wait path.

## Test Ownership

The invariant belongs to the Loop coordinator because it maps durable action task results into generation state.

The canonical coordinator suite will add RED coverage for:

1. completed awaited result -> `awaiting_child` with exact child ID;
2. live child -> yield and no dependent reservation;
3. child `done` and `no-op` -> dependent becomes eligible;
4. child failure/cancellation -> fail closed;
5. malformed awaited result -> fail closed;
6. detached child -> immediate success.

The canonical daemon Loop integration suite will add a real-store restart case:

1. start an ordered parent with two child Loop nodes;
2. hold the first child live;
3. persist the completed action result and observed wait state;
4. rebuild the runtime against the same database;
5. assert one original child, parent still live, and second child absent;
6. complete the first child and assert the second starts exactly once;
7. hold the second child and assert the parent remains live;
8. complete the second child and assert the parent reaches the correct terminal outcome.

Tests use conditions and durable reads, never sleeps as synchronization.

## Documentation And QA

Issue #386 leads with the standalone authored-Loop reproduction and keeps the real extension-driven sequence as secondary discovery evidence. It carries the run IDs, executed graph, observed output, expected contract, and impact. The PR will link that issue and include the executable regression commands.

The official Compozy Loop skill and site documentation will state that an awaited child keeps the
parent and dependents nonterminal, survives restart with the same child identity, and maps child
terminal outcomes without coercion.

A fresh isolated QA lab will first reproduce the standalone two-Loop flow with unique `COMPOZY_HOME`, ports, and UDS path, then use an installed extension as a compatibility canary. Evidence will include the ordered run timeline, exact parent/child IDs, restart boundary, absence of duplicate children, terminal states, and clean teardown.

## Compozy Impact Audit

- **Native tools:** No ToolID, descriptor, schema, risk, capability, or reason-code change. `compozy__loop_status` exposes corrected existing state.
- **Extensibility and hooks:** Corrects generic nested Loop behavior for every extension and authored Loop. No manifest, resource, hook, capability, registry, MCP, or config lifecycle change.
- **Workspace data isolation:** Child lookup and parent validation remain bound to the parent run's workspace. Tests must reject a child from another workspace.
- **Official Compozy skill:** Update `skills/compozy/references/loops.md` with ordering, restart, and child-terminal mapping.

## Web And Docs Impact

No Web component or route changes. Existing Loop run inspectors consume the same status fields and
will display corrected runtime truth. The Loop guardrails documentation is updated with ordering,
restart, and terminal-mapping behavior; no API or schema documentation changes.

## Rollout And Rollback

The change is internal runtime behavior with no migration or config change. Rollout uses the normal daemon binary replacement and a fresh nested-Loop smoke. Rollback restores the prior binary; persisted runs retain existing schema compatibility.

## Alternatives Rejected

### Extend `ActionControl`

`ActionControl` currently owns Goal pause and terminal dispositions. Expanding it for child-loop liveness would broaden an unrelated protocol and duplicate the existing generation-level `awaiting_child` state.

### Keep the worker task open until the child terminates

This would consume a long-lived lease and process, complicate cancellation and restart, and move graph coordination into the action worker. The coordinator already owns durable waiting and should remain the authority.

### Special-case the extension

The failure is in the generic `run-loop` contract. Extension-specific ordering or prompt rules would hide the runtime defect and leave every other caller exposed.
