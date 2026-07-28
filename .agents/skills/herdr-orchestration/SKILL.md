---
name: herdr-orchestration
description: Orchestrate Claude and Codex worker TUIs from a controller agent through herdr panes and the herdr socket CLI. Use when delegating bounded tasks to herdr worker panes, running user-activated plan-first delegations (Claude Code plan mode, Codex Plan mode), waiting on native agent status (idle, working, blocked, done), or verifying worker reports. Workers launch as interactive TUIs via herdr agent start — never through headless runners (compozy exec, claude -p, codex exec). Not for cmux workspaces (see cmux-orchestration) and not for end-user herdr control.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# herdr Orchestration

The top-level agent is the **controller** (Codex, Claude, Fable, or another);
it delegates to **worker** TUIs that gather evidence, draft bounded patches,
run tests, and report. The controller owns assignment, state, conflict
control, verification, integration, and the final user-facing answer. herdr is
the substrate: panes host the worker TUIs, and the socket CLI (`herdr …`)
gives the controller placement, input, screen reads, and native agent-status
waits.

## Workers are TUIs — no headless runners

A worker is an interactive TUI — `claude` or `codex` — launched in a herdr
pane with `herdr agent start`. The whole orchestration loop hangs on this:
herdr's agent-state integrations hook the interactive TUIs, so
`wait agent-status`, `agent list`, and blocked/done detection exist only while
a real TUI is on screen.

Headless runners — `compozy exec`, `claude -p`, `codex exec`, anything that
streams JSON events into a pane — are not workers. They never report state,
`wait agent-status` never fires, and the delegation dies silently. Recognize
the failure on sight: a worker pane must show the TUI banner and input box; a
pane filling with raw JSON event lines is a broken delegation — interrupt it
(`rtk herdr pane send-keys <pane_id> ctrl+c`) and relaunch the worker with
`herdr agent start`.

## Invariants

- Scope every action to the caller workspace unless the user names another
  target.
- Resolve caller context from `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`,
  `HERDR_PANE_ID`, and `HERDR_SOCKET_PATH` (injected into every herdr pane)
  before any focused-window fallback.
- Address panes by explicit ids (`w2:p3` — workspace-qualified) captured from
  command JSON output, never by guessed position.
- Pass `--no-focus` on every creating verb (`agent start`, `pane split`,
  `tab create`, `workspace create`, `worktree create`); use focus verbs only
  when the user asks.
- Retire every worker you launch: once its report is verified and its
  disposition recorded, close its pane (see Retire workers).

## Preflight

Inspect the daemon, integrations, and caller context:

```bash
rtk herdr status                 # server running + socket path
rtk herdr integration status     # claude and codex must show `current`
rtk herdr pane current           # caller pane / tab / workspace ids
rtk herdr agent list             # agents already running
```

`integration status` must show `claude` and `codex` installed — that is what
makes agent status authoritative instead of screen-scraped. If either is
missing, run `rtk herdr integration install claude` / `… install codex` before
launching workers.

## Launch workers

Two default profiles — Claude Code on `opus` (**claude-opus**) and Codex on
`gpt-5.5` (**codex-gpt-5.5**). `agent start` creates the pane, runs the TUI
argv directly, and returns the worker's `pane_id` in JSON — capture it in the
registry:

```bash
rtk herdr agent start claude-opus --workspace "$HERDR_WORKSPACE_ID" \
  --cwd "$PWD" --split right --no-focus -- \
  claude --dangerously-skip-permissions --model opus "<packet>"

rtk herdr agent start codex-gpt-5.5 --workspace "$HERDR_WORKSPACE_ID" \
  --cwd "$PWD" --split right --no-focus -- \
  codex --yolo -m gpt-5.5 -C "$PWD"
```

Claude always launches with `--dangerously-skip-permissions` and Codex with
`--yolo` — workers run unattended and must not stall on permission prompts.
Plan-first runs add Claude's `--permission-mode plan` — the flags compose (see
Plan-first delegation).

A launch is confirmed only when `rtk herdr pane read <pane_id> --source
visible` shows the TUI banner and input box, and `rtk herdr agent list` shows
the worker leaving `unknown`. A pane scrolling text while its status stays
`unknown` is the headless-runner failure above — interrupt and relaunch.

## Sending prompts to running TUIs

Prefer passing the initial prompt as the launch argv (shown above) — the
session starts working immediately and skips the TUI-ready race. Exception:
Codex plan-first launches bare (see Plan-first delegation).

For any follow-up to a running TUI, use `pane run` — it types the text and
presses Enter atomically:

```bash
rtk herdr pane run <pane_id> "<follow-up prompt>"
```

A trailing `\n` inside `send-text` does not submit — a TUI renders it as a
soft newline and the prompt sits unsubmitted. When a prompt must be staged and
submitted separately, pair the text with an explicit key:

```bash
rtk herdr pane send-text <pane_id> "<prompt>"
rtk herdr pane send-keys <pane_id> enter
```

Confirm delivery with `rtk herdr pane read <pane_id> --source visible` (input
line empty, working indicator active) or `rtk herdr wait agent-status
<pane_id> --status working --timeout 15000`. An unconfirmed send is not
delivered.

## Plan-first delegation (opt-in)

Plan-first runs only when the user activates it — an explicit ask ("plan
first", "plan mode") or a `--plan-mode` flag on the invocation. The default
delegation is direct: launch and run hands-off. For investigation-heavy
slices (root-cause fixes, multi-file or cross-package changes, unfamiliar
code), offer plan-first and let the user decide — never switch a worker into
plan mode without that signal.

Once activated, the worker plans, the controller reviews and accepts, then it
runs hands-off. The launch flags, shift+tab sequences, status checks, and
acceptance menus are exact and differ per TUI. **Read
[references/plan-mode.md](references/plan-mode.md) in full before launching a
plan-first worker.**

## Delegation packets

Send each worker prompt as a standalone contract. Include:

- repo path and exact objective
- worker role and model
- execution mode: direct (default) or plan-first (user-activated)
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

## Worktree isolation

When workers must edit overlapping files in parallel, give each an isolated
worktree and point the worker's `--cwd` at it — herdr manages worktrees
natively:

```bash
rtk herdr worktree create --workspace "$HERDR_WORKSPACE_ID" \
  --branch <slug> --no-focus --json
```

## Track workers

Orchestrations are long-running — worker slices take minutes to hours, and a
worker is never abandoned because a wait expired. Block on herdr's native
agent status in **check-in intervals** of 3–5 minutes (sized to the slice —
longer for heavier slices) instead of polling screens:

```bash
rtk herdr wait agent-status <pane_id> --status done --timeout 300000     # completion check-in
rtk herdr wait agent-status <pane_id> --status blocked --timeout 300000  # question or menu
rtk herdr wait output <pane_id> --match "<text>" --timeout 300000        # specific screen string
```

`--timeout` is the check-in interval, never a deadline on the worker. On
expiry (exit 1, `timed out waiting for agent status change`), read the screen
(`rtk herdr pane read <pane_id> --source recent --lines 120`) and re-enter the
wait — loop until the worker reaches a terminal state, asks a question, or a
stop condition fires. When a reported status looks wrong, debug detection with
`rtk herdr agent explain <pane_id> --json`.

Maintain a compact registry in the task ledger or handoff: controller
identity; worker name, role, model; workspace/tab/pane ids; objective sent and
start time; status (starting, planning, plan-review, plan-accepted, running,
blocked, reported, verified, retired); claimed files or work slice; worker-reported
commands and results; controller verification performed; final disposition
(accepted, rejected, superseded, or blocked).

Label worker panes so the herd stays legible, and announce milestones without
stealing focus:

```bash
rtk herdr pane rename <pane_id> "claude-opus: fix loops"
rtk herdr notification show "Workers done" --body "2/2 verified" --sound done
```

## Verify worker output

Treat worker output as untrusted until verified. Monitor and verify without
changing focus:

- Read worker panes read-only (`pane read --source recent --lines 200`); ask a
  worker for a concise status summary when output is unclear.
- Re-open cited files locally to verify high-impact findings.
- Re-run claimed test results with fresh controller commands when the result
  gates completion.
- Review the final diff before accepting any worker patch.
- If a worker edits outside its claim, pause integration and decide: accept,
  request user-approved cleanup, or supersede with controller edits.

## Retire workers

A delegation ends with the worker retired — pane closed — not just its report
read. Retire each worker the moment its disposition is recorded and no
follow-up prompt is planned, as part of handling that worker's completion,
never deferred to an end-of-run sweep:

```bash
rtk herdr pane close <pane_id>
```

Closing the pane ends the TUI session and its scrollback, so record what the
registry needs first — report text, cited line refs, command output. Files,
diffs, and worktrees survive on disk.

A worker stays open only while mid-run or blocked, while its screen is
evidence for an unresolved failure, or when the user asks to inspect it —
record the reason in the registry. The orchestration is complete only when
`rtk herdr agent list` shows none of this controller's workers still running:
every worker retired, or its open pane justified in the registry.

## Stop conditions

Stop and report instead of improvising when:

- herdr rejects a valid workspace, tab, or pane ref, or `rtk herdr status`
  shows no running server.
- a worker TUI rejects the model alias, auth state, working directory, or a
  startup flag — report the exact blocker, and get user approval before
  downgrading a model or switching tools.
- authentication blocks the TUI.
- a worker's status stays `unknown` while output scrolls and
  `agent explain` shows no matched rule — detection is broken or the pane is
  running a headless process.
- a plan-mode status line or acceptance menu cannot be confirmed via
  `pane read` or `wait output`, or a worker's plan stays out of scope after a
  re-planning round.
- workers need overlapping edits the controller cannot integrate safely.
- a task needs focus-changing herdr verbs the user has not approved.
