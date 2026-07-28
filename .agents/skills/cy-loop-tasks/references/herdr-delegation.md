# Herdr delegation lanes

Read this file in full before dispatching any worker. It builds on the
`herdr-orchestration` skill — activate that skill for pane/socket mechanics
(preflight, `agent start`, waits, screen reads, plan-mode details this loop
never uses). This file fixes what THIS loop dispatches, with which argv, and
what evidence gates completion.

## Common dispatch contract (both lanes)

1. Preflight: `rtk herdr status`, `rtk herdr integration status` (the
   integration for the worker's TUI — `claude` or `cursor` — must show
   `current`), and `rtk herdr pane current` for caller ids.
2. Compose the delegation packet (below) and launch the worker as an
   interactive TUI with the packet as the initial prompt:

   ```bash
   rtk herdr agent start <worker-name> --workspace "$HERDR_WORKSPACE_ID" \
     --cwd "$PWD" --split right --no-focus -- <worker argv> "<packet>"
   ```

   Workers are TUIs. Headless runners (`compozy exec`, `claude -p`,
   `codex exec`, `cursor-agent -p`) never report agent status and kill the
   delegation silently. A pane filling with raw JSON event lines is that
   failure on sight: interrupt (`rtk herdr pane send-keys <pane_id> ctrl+c`)
   and relaunch with `rtk herdr agent start`.
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

Every packet is a standalone contract naming: repo root; slug; the action
(`task_NN`, `free-iter-NNN`, or `qa-report`); exact task file path or slice
text in scope; out-of-scope surfaces; shared and current memory paths (per
`memory-protocol.md`); skills to activate; validation commands for the
touched surface; expected evidence (changed files, explicit PASS/FAIL verify
output, artifact paths); stop conditions; and the hard rule: **do not commit
— leave the worktree dirty for the orchestrator's checkpoint.**

## Frontend lane (Phase B)

Active only when `state.frontend_agent` is set (bootstrap `--frontend`).

Classification — what counts as frontend:

- mode=tasks: detect-phase prints `lane=frontend agent=<x>` when the task
  frontmatter `type:` is `frontend`. Trust the printed line.
- mode=free: the slice qualifies only when its owned paths are exclusively
  frontend surfaces — `web/**`, `packages/ui/**`, `packages/site/**`. Mixed
  backend/frontend slices run locally.

Worker argv by `frontend_agent` value:

```bash
# claude — Claude Code on Opus at xhigh effort
claude --dangerously-skip-permissions --model opus --effort xhigh "<packet>"

# cursor — Cursor agent on Grok 4.5
cursor-agent --yolo --model grok-4.5 "<packet>"
```

Packet additions for this lane: read the task file / `_techspec.md` / design
docs plus the scoped `AGENTS.md`/`CLAUDE.md` for the touched surfaces; apply
`cy-spec-preflight` (task-body) and `cy-execute-task` discipline; use
`cy-workflow-memory` with the provided memory paths; run the frontend
validation lane for the touched packages; run `cy-final-verify`; print
changed files plus explicit PASS/FAIL evidence.

Completion gate — mark the Phase B action complete only when ALL hold:

- the worker reached `done` (or reported completion verified on screen)
- the required memory files were updated
- task/status artifacts reflect completion
- the worker report contains explicit `cy-final-verify` PASS evidence
- HEAD is unchanged (no worker commit)

Missing any item → keep the phase action open and follow
`references/recovery-loop.md`: recover the evidence or rerun the lane before
advancing `state.yaml`. Record a blocker only when its external-blocker test
passes.

## QA-report lane (Phase C)

Always active — the orchestrator never authors `qa_report` output itself.

Worker argv — Claude Fable 5, direct execution (never plan-first: no plan
permission mode, no plan-mode key sequences):

```bash
claude --permission-mode auto --model claude-fable-5 "<packet>"
```

Packet additions for this lane: activate the `qa-report` skill with
`qa-docs-path=docs/qa`; update journey flows (`docs/qa/journeys/`), scenario
files (`docs/qa/scenarios/`), and cycle charters (`docs/qa/charters/`);
register any found bugs in the content-addressed registry; update the
provided memory paths; print the exact artifact paths written.

Completion gate — record `--qa-report-done` only when:

- every artifact path the worker reported exists on disk
- the affected `docs/qa/scenarios/` files reflect the cycle plan
- the memory paths were updated
- HEAD is unchanged (no worker commit)

Missing any item keeps the Phase C action open. Follow
`references/recovery-loop.md`, repair or rerun the lane, and recheck every
item before updating state.
