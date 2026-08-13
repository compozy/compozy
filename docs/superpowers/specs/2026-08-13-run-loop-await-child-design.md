# Durable `run-loop` Await Semantics

**Status:** Approved

**Issue:** [#386](https://github.com/compozy/compozy/issues/386)

## Confirmed Reproduction

At 2026-08-13 15:06 America/Sao_Paulo, an installed resource extension started a delivery Loop from an extension-published agent session. The executed graph was:

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

The promoted build and its predecessor have no diff under `internal/loop`, so the smoke exposed a pre-existing runtime defect rather than a regression from the extension-session changes.

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
7. Document a reproducible extension-driven flow without making the fix specific to Batuta.

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

Issue #386 carries the real extension-driven sequence, run IDs, executed graph, observed output, expected contract, and impact. The PR will link that issue and include the executable regression commands.

The official Compozy Loop skill will be checked against the corrected behavior. It already states that `awaiting_child` is node-level and live; it needs no wording change unless implementation review finds a mismatch.

A fresh isolated QA lab will reproduce the extension flow with unique `COMPOZY_HOME`, ports, and UDS path. Evidence will include the ordered run timeline, exact parent/child IDs, restart boundary, absence of duplicate children, terminal states, and clean teardown.

## Compozy Impact Audit

- **Native tools:** No ToolID, descriptor, schema, risk, capability, or reason-code change. `compozy__loop_status` exposes corrected existing state.
- **Extensibility and hooks:** Corrects generic nested Loop behavior for every extension and authored Loop. No manifest, resource, hook, capability, registry, MCP, or config lifecycle change.
- **Workspace data isolation:** Child lookup and parent validation remain bound to the parent run's workspace. Tests must reject a child from another workspace.
- **Official Compozy skill:** Checked `skills/compozy/references/loops.md`; its current live-state contract already matches the intended behavior.

## Web And Docs Impact

No Web component or route changes. Existing Loop run inspectors consume the same status fields and will display corrected runtime truth. No public documentation change is required beyond issue and QA/PR evidence because the documented `mode: await` contract is already correct.

## Rollout And Rollback

The change is internal runtime behavior with no migration or config change. Rollout uses the normal daemon binary replacement and a fresh nested-Loop smoke. Rollback restores the prior binary; persisted runs retain existing schema compatibility.

## Alternatives Rejected

### Extend `ActionControl`

`ActionControl` currently owns Goal pause and terminal dispositions. Expanding it for child-loop liveness would broaden an unrelated protocol and duplicate the existing generation-level `awaiting_child` state.

### Keep the worker task open until the child terminates

This would consume a long-lived lease and process, complicate cancellation and restart, and move graph coordination into the action worker. The coordinator already owns durable waiting and should remain the authority.

### Special-case the extension

The failure is in the generic `run-loop` contract. Extension-specific ordering or prompt rules would hide the runtime defect and leave every other caller exposed.
