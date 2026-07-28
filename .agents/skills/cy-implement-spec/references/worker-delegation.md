# Worker delegation — QA-report lane

Read this file in full before dispatching the worker. It builds on the
`herdr-orchestration` skill — activate that skill for pane/socket mechanics
(preflight, `agent start`, waits, screen reads). This file fixes what THIS
loop dispatches, with which argv, and what evidence gates completion.

`qa_report` is the loop's only delegated action, and it is delegated for
judgment independence, not capacity: the orchestrator implemented the whole
spec, so a fresh worker plans QA with unbiased eyes. The orchestrator never
authors `qa_report` output itself. Every other iteration runs locally.

## Dispatch contract

1. Preflight: `rtk herdr status`, `rtk herdr integration status` (the
   `claude` integration must show `current`), and `rtk herdr pane current`
   for caller ids.
2. Compose the delegation packet (below) and launch the worker as an
   interactive TUI with the packet as the initial prompt:

   ```bash
   rtk herdr agent start <worker-name> --workspace "$HERDR_WORKSPACE_ID" \
     --cwd "$PWD" --split right --no-focus -- \
     claude --permission-mode auto --model claude-fable-5 "<packet>"
   ```

   Direct execution — never plan-first: no plan permission mode, no
   plan-mode key sequences. Workers are TUIs. Headless runners
   (`compozy exec`, `claude -p`, `codex exec`) never report agent status
   and kill the delegation silently. A pane filling with raw JSON event
   lines is that failure on sight: interrupt
   (`rtk herdr pane send-keys <pane_id> ctrl+c`) and relaunch with
   `rtk herdr agent start`.
3. Confirm launch: `rtk herdr pane read <pane_id> --source visible` shows the
   TUI banner and input box, and `rtk herdr agent list` shows the worker
   leaving `unknown`.
4. Track with native status waits, answering questions via
   `rtk herdr pane run <pane_id> "<answer>"` when the worker blocks:

   ```bash
   rtk herdr wait agent-status <pane_id> --status done --timeout 900000
   rtk herdr wait agent-status <pane_id> --status blocked --timeout 900000
   ```

5. Verify: worker output is untrusted until verified. Read the report
   (`rtk herdr pane read <pane_id> --source recent --lines 200`), re-open
   cited files, and re-run gating commands locally.
6. Commit gate: capture `git rev-parse HEAD` before the dispatch and compare
   after — identical, or the worker breached contract. The orchestrator owns
   the checkpoint commit.
7. Leave the worker pane open unless cleanup is explicitly requested.

## Delegation packet

The packet is a standalone contract naming: repo root; slug; the action
(`qa-report`); the spec documents in scope (`_techspec.md` plus companions);
out-of-scope surfaces; shared and current memory paths (per
`references/memory-protocol.md`); skills to activate (`qa-report` with
`qa-docs-path=docs/qa`); expected evidence (artifact paths written); stop
conditions; and the hard rule: **do not commit — leave the worktree dirty
for the orchestrator's checkpoint.**

Packet additions for this lane: update journey flows (`docs/qa/journeys/`),
scenario files (`docs/qa/scenarios/`), and cycle charters
(`docs/qa/charters/`); register any found bugs in the content-addressed
registry; update the provided memory paths; print the exact artifact paths
written.

## Completion gate

Record `--qa-report-done` only when:

- every artifact path the worker reported exists on disk
- the affected `docs/qa/scenarios/` files reflect the cycle plan
- the memory paths were updated
- HEAD is unchanged (no worker commit)

Missing any item keeps the Phase C action open. Follow
`references/recovery-loop.md`, repair or rerun the lane, and recheck every
item before updating state.
