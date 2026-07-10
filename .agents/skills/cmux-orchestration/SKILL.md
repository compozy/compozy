---
name: cmux-orchestration
description: Orchestrate Claude Opus and Codex GPT-5.5 worker TUIs from a controller agent in the current cmux workspace. Use when delegating bounded tasks to worker panes, running plan-first delegations (Claude Code plan mode, Codex Plan mode), monitoring and tracking worker state, or verifying worker reports. Not for end-user cmux control (see cmux, cmux-workspace) — this drives agent workers.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# cmux Orchestration

The top-level agent is the **controller** (Codex, Claude, Fable, or another); it
delegates to **worker** TUIs that gather evidence, draft bounded patches, run
tests, and report. The controller owns assignment, state, conflict control,
verification, integration, and the final user-facing answer.

Every run also loads:

- [cmux](../cmux/SKILL.md) — panes, surfaces, routing, health checks.
- [cmux-workspace](../cmux-workspace/SKILL.md) — caller-workspace scoping,
  helper-pane reuse, socket targeting, non-disruptive automation.

## Invariants

- Scope every action to the caller workspace unless the user names another
  target.
- Target `CMUX_WORKSPACE_ID`, `CMUX_SURFACE_ID`, and `CMUX_SOCKET_PATH` before
  any focused-window fallback.
- Drive workers through explicit `surface:N` refs.
- Keep focus where it is — use `focus-pane`, `focus-panel`, `select-workspace`,
  and focus-changing tab actions only when the user asks.
- Leave worker surfaces open; close them only when cleanup is explicitly
  requested or part of the task contract.

## Preflight

Inspect the caller context and current layout:

```bash
rtk cmux identify --json
rtk cmux list-panes --workspace "${CMUX_WORKSPACE_ID:-}" --json
rtk cmux list-pane-surfaces --workspace "${CMUX_WORKSPACE_ID:-}" --json
```

Prefer one right-side helper pane. If an obvious non-caller helper pane exists,
add worker surfaces to it; otherwise create exactly one right-side terminal
pane:

```bash
rtk cmux new-surface --workspace "${CMUX_WORKSPACE_ID:-}" \
  --pane pane:<helper> --type terminal --focus false

rtk cmux new-pane --workspace "${CMUX_WORKSPACE_ID:-}" \
  --type terminal --direction right --focus false
```

Capture each returned `surface:N` in the state registry before sending prompts.

## Launch workers

Two default profiles — Claude Code on `opus` (**claude-opus**) and Codex on
`gpt-5.5` (**codex-gpt-5.5**) — launched by explicit surface:

```bash
rtk cmux send --surface surface:<claude> \
  "rtk claude --dangerously-skip-permissions --model opus --name cmux-claude-opus\n"
rtk cmux send --surface surface:<codex> \
  "rtk codex --yolo -m gpt-5.5 -C \"$PWD\"\n"
```

Claude always launches with `--dangerously-skip-permissions` and Codex with
`--yolo` — workers run unattended and must not stall on permission prompts.
Plan-first runs add Claude's `--permission-mode plan` — the flags compose
(see Plan-first delegation).

## Sending prompts to running TUIs

`cmux send "...\n"` submits only in a plain shell (the launch lines above).
Inside a running TUI (Claude Code, Codex) the trailing `\n` is pasted as a soft
newline — the prompt sits unsubmitted and the worker stalls silently.

- Prefer passing the initial prompt as a launch CLI argument
  (`rtk claude --dangerously-skip-permissions "…prompt…"`,
  `rtk codex "…prompt…"`) — this skips the TUI-ready race and the submission
  problem. Exception: Codex plan-first launches bare (see Plan-first
  delegation).
- For any follow-up to a running TUI, send the text, then submit with an
  explicit Enter:

  ```bash
  rtk cmux send --surface surface:<worker> "…follow-up prompt…"
  rtk cmux send-key --surface surface:<worker> enter
  ```

- Confirm with `rtk cmux read-screen --surface surface:<worker>`: input line
  empty, spinner/working indicator active. An unconfirmed send is not delivered.

## Plan-first delegation

For slices needing investigation before edits — root-cause fixes, multi-file or
cross-package changes, unfamiliar code — run the worker plan-first: it plans,
you review and accept, then it runs hands-off. The launch flags, shift+tab
sequences, status-line checks, and acceptance menus are exact and differ per
TUI. **When delegating plan-first, read
[references/plan-mode.md](references/plan-mode.md) in full before launching the
worker.**

## Delegation packets

Send each worker prompt as a standalone contract. Include:

- repo path and exact objective
- worker role and model
- execution mode: plan-first or direct
- files, packages, or surfaces in scope
- files and behaviors explicitly out of scope
- claimed files or work slice, to avoid conflicts
- expected evidence: files, line refs, commands, diffs, failures, screenshots,
  and stated uncertainty
- verification commands or browser flows to run
- stop conditions: unexpected code shape, repeated command failure, auth/model
  blocker, or need for out-of-scope edits

Prefer small, independent slices. Assign overlapping file claims only when the
controller will integrate and resolve conflicts immediately.

## State registry

Maintain a compact registry in the task ledger or handoff:

- controller identity
- worker role and model
- `workspace:N`, `pane:N`, `surface:N`
- prompt/objective sent and start time
- status: starting, planning, plan-review, plan-accepted, running, waiting,
  blocked, reported, verified
- claimed files or work slice
- worker-reported commands and results
- controller verification performed
- final disposition: accepted, rejected, superseded, or blocked

For long runs, attach progress to the caller workspace with cmux sidebar state,
and clear it when orchestration finishes or is abandoned:

```bash
rtk cmux set-status orchestration "running" \
  --workspace "${CMUX_WORKSPACE_ID:-}" --color "#ff9500"
rtk cmux set-progress 0.4 --label "Workers running" \
  --workspace "${CMUX_WORKSPACE_ID:-}"
rtk cmux log --workspace "${CMUX_WORKSPACE_ID:-}" --level info -- \
  "Started cmux orchestration"
```

## Verify worker output

Treat worker output as untrusted until verified. Monitor and verify without
changing focus:

- Poll or inspect worker surfaces read-only; ask a worker for a concise status
  summary when output is unclear.
- Re-open cited files locally to verify high-impact findings.
- Re-run claimed test results with fresh controller commands when the result
  gates completion.
- Review the final diff before accepting any worker patch.
- If a worker edits outside its claim, pause integration and decide: accept,
  request user-approved cleanup, or supersede with controller edits.

## Stop conditions

Stop and report instead of improvising when:

- cmux rejects a valid workspace, pane, or surface ref.
- a worker TUI rejects the model alias, auth state, working directory, or a
  startup flag — report the exact blocker, and get user approval before
  downgrading a model or switching tools.
- authentication blocks the TUI.
- worker output cannot be observed or controlled reliably.
- a plan-mode status line or acceptance menu cannot be confirmed via
  `read-screen`, or a worker's plan stays out of scope after a re-planning
  round.
- workers need overlapping edits the controller cannot integrate safely.
- a task needs focus-changing cmux commands the user has not approved.

## Diagram

`assets/fable-orchestrator.excalidraw` (or its PNG variants) diagrams the
controller/worker topology when a visual explanation helps.
