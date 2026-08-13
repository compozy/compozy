---
name: herdr-orchestration
description: Orchestrate Claude and Codex worker TUIs from a controller agent — one worker per named herdr tab, driven over the herdr socket CLI.
disable-model-invocation: true
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
the substrate: each worker TUI owns a named tab, and the socket CLI (`herdr …`)
gives the controller placement, input, screen reads, and native agent-status
waits.

## One worker, one named tab

Every worker gets its own tab, labeled at creation with `tab create --label`.
The tab bar is the controller's dashboard: a labeled tab says which model is
running which slice, and stays readable no matter how many workers are live.

Tabs also cost no screen area. A split pane divides the caller's screen with
every worker, and on a laptop the third worker leaves every pane too narrow to
read. Split a pane only when the user explicitly asks to watch a worker beside
the caller.

Two names per worker, both required at launch:

- **tab label** — `<model>: <slice>`, e.g. `opus: fix loops`,
  `gpt-5.5: audit auth`. Free text; keep it short, tab bars truncate.
- **agent name** — the slice as a slug, e.g. `fix-loops`. Must match
  `[a-z][a-z0-9_-]{0,31}` and be unique among live agents. Every `agent …` verb
  accepts it in place of a pane id.

## Workers are TUIs — no headless runners

A worker is an interactive TUI — `claude` or `codex` — started with
`herdr agent start`, which validates the agent's identity and returns only once
herdr sees it ready for input. herdr's agent-state integrations hook those
TUIs, so `agent wait`, `agent list`, and blocked/done detection exist only
while a real TUI is on screen.

Headless runners — `compozy exec`, `claude -p`, `codex exec`, anything that
streams JSON events into a pane — never report state, so waits never fire and
the delegation dies silently. A worker tab filling with raw JSON event lines is
a broken delegation: interrupt it (`rtk herdr pane send-keys <pane_id> ctrl+c`)
and relaunch through `agent start`.

## Invariants

- Scope every action to the caller workspace unless the user names another
  target.
- Resolve caller context from `HERDR_WORKSPACE_ID`, `HERDR_TAB_ID`, and
  `HERDR_PANE_ID` (injected into every herdr pane) before any focused-window
  fallback.
- Address workers by ids parsed from command JSON — agent name for `agent …`
  verbs, `w2:t3` / `w2:p4` for tab and pane verbs — never by guessed position.
- Pass `--no-focus` on every creating verb (`tab create`, `workspace create`,
  `worktree create`, `pane split`); use focus verbs only when the user asks.
- Retire every worker you launch: once its report is verified and its
  disposition recorded, close its tab (see Retire workers).

## Preflight

Inspect the daemon, integrations, and caller context:

```bash
rtk herdr status                 # server running + socket path
rtk herdr integration status     # claude and codex must show `current`
rtk herdr pane current --current # caller pane / tab / workspace ids
rtk herdr agent list             # agents already running
```

`integration status` must show `claude` and `codex` installed — that is what
makes agent status authoritative instead of screen-scraped. If either is
missing, run `rtk herdr integration install claude` / `… install codex` before
launching workers.

## Launch workers

Two default profiles — Claude Code on `opus` and Codex on `gpt-5.5`. Launch is
two commands: create the named tab, then start the TUI in that tab's root pane.

```bash
# 1. named tab — returns .result.tab.tab_id and .result.root_pane.pane_id
rtk herdr tab create --workspace "$HERDR_WORKSPACE_ID" --cwd "$PWD" \
  --label "opus: fix loops" --no-focus

# 2. worker TUI in that tab's root pane; native flags follow `--`
rtk herdr agent start fix-loops --kind claude --pane <root_pane_id> -- \
  --dangerously-skip-permissions --model opus "<packet>"

rtk herdr agent start audit-auth --kind codex --pane <root_pane_id> -- \
  --yolo -m gpt-5.5
```

Capture `tab_id`, `pane_id`, and the agent name in the registry — retiring and
screen reads need all three.

Claude always launches with `--dangerously-skip-permissions` and Codex with
`--yolo` — workers run unattended and must not stall on permission prompts.
Plan-first runs add Claude's `--permission-mode plan`; the flags compose (see
Plan-first delegation).

`agent start` returns success only after herdr detects the agent ready, so a
non-zero exit is the launch failure — read the tab's root pane
(`rtk herdr pane read <pane_id> --source visible`) before retrying. Confirm the
label stuck with `rtk herdr tab list --workspace "$HERDR_WORKSPACE_ID"`; if the
tab shows a default label, restore it:

```bash
rtk herdr tab rename <tab_id> "opus: fix loops"
```

## Sending prompts to running TUIs

Prefer passing the initial packet as launch argv (shown above) — the session
starts working immediately and skips the TUI-ready race. Exception: Codex
plan-first launches bare (see Plan-first delegation).

For any follow-up, use `agent prompt` — it submits text plus Enter atomically,
honoring the pane's live bracketed-paste mode, and `--wait` blocks until the
worker settles:

```bash
rtk herdr agent prompt fix-loops "<follow-up prompt>" --wait --timeout 300000
```

A trailing `\n` in `pane send-text` does not submit — a TUI renders it as a
soft newline and the prompt sits unsubmitted. An unconfirmed send is not
delivered: if `--wait` returns `agent_prompt_stalled`, the worker never changed
state — read the screen and resend.

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
worktree — herdr manages them natively, as its own labeled workspace:

```bash
rtk herdr worktree create --workspace "$HERDR_WORKSPACE_ID" \
  --branch <slug> --label "<slug>" --no-focus --json
```

Create the worker's tab inside that workspace (`tab create --workspace
<worktree_workspace_id>`) so its cwd is the isolated checkout.

## Track workers

Orchestrations are long-running — worker slices take minutes to hours, and a
worker is never abandoned because a wait expired. Block on herdr's native agent
status in **check-in intervals** of 3–5 minutes (sized to the slice — longer
for heavier slices) instead of polling screens:

```bash
rtk herdr agent wait fix-loops --until done --timeout 300000     # completion check-in
rtk herdr agent wait fix-loops --until blocked --timeout 300000  # question or menu
rtk herdr pane wait-output <pane_id> --match "<text>" --timeout 300000
```

Controller reads never mark a tab seen, so an unfocused worker settles as
`done`, not `idle` — wait on `done`. `--timeout` is the check-in interval,
never a deadline on the worker. On expiry, read the screen
(`rtk herdr agent read fix-loops --source recent-unwrapped --lines 120`) and
re-enter the wait — loop until the worker reaches a terminal state, asks a
question, or a stop condition fires. When a reported status looks wrong, debug
detection with `rtk herdr agent explain <pane_id> --json`.

Maintain a compact registry in the task ledger or handoff: controller
identity; worker agent name, tab label, role, model; workspace/tab/pane ids;
objective sent and start time; status (starting, planning, plan-review,
plan-accepted, running, blocked, reported, verified, retired); claimed files or
work slice; worker-reported commands and results; controller verification
performed; final disposition (accepted, rejected, superseded, or blocked).

Keep the tab label current as a slice changes, and announce milestones without
stealing focus:

```bash
rtk herdr tab rename <tab_id> "opus: fix loops (verifying)"
rtk herdr notification show "Workers done" --body "2/2 verified" --sound done
```

## Verify worker output

Treat worker output as untrusted until verified. Monitor and verify without
changing focus:

- Read worker tabs read-only (`agent read <name> --source recent-unwrapped
  --lines 200`); ask a worker for a concise status summary when output is
  unclear.
- Re-open cited files locally to verify high-impact findings.
- Re-run claimed test results with fresh controller commands when the result
  gates completion.
- Review the final diff before accepting any worker patch.
- If a worker edits outside its claim, pause integration and decide: accept,
  request user-approved cleanup, or supersede with controller edits.

When `--lines` stops revealing more of a completed response, the TUI is on the
terminal's alternate screen and those rows are gone. Ask the worker to write
its full report as Markdown in a temp directory and reply with the path, then
read the file.

## Retire workers

A delegation ends with the worker retired — tab closed — not just its report
read. Retire each worker the moment its disposition is recorded and no
follow-up prompt is planned, as part of handling that worker's completion,
never deferred to an end-of-run sweep:

```bash
rtk herdr tab close <tab_id>
```

Closing the tab ends the TUI session and its scrollback, so record what the
registry needs first — report text, cited line refs, command output. Files,
diffs, and worktrees survive on disk.

A worker stays open only while mid-run or blocked, while its screen is
evidence for an unresolved failure, or when the user asks to inspect it —
record the reason in the registry. The orchestration is complete only when
`rtk herdr agent list` shows none of this controller's workers still running:
every worker retired, or its open tab justified in the registry.

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
  `agent read` or `pane wait-output`, or a worker's plan stays out of scope
  after a re-planning round.
- workers need overlapping edits the controller cannot integrate safely.
- a task needs focus-changing herdr verbs the user has not approved.
