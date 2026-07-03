---
name: cmux-orchestration
description: Control cmux panes and surfaces to coordinate terminal agent workers. Use when a top-level agent needs to launch, prompt, monitor, or manage Claude Opus and Codex GPT-5.5 TUIs in the current cmux workspace, run plan-first delegations (Claude Code plan mode, Codex Plan mode), track worker state, delegate bounded tasks, or verify worker reports.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# cmux Orchestration

Use the current top-level agent as the controller. The controller may be Codex,
Claude Fable, or another agent. Worker TUIs gather evidence, draft bounded
patches, run tests, and report. The controller owns assignment, state,
conflict control, verification, integration, and the final user-facing answer.

Before taking cmux actions, also load and follow:

- [cmux](../cmux/SKILL.md) for panes, surfaces, routing, and health checks.
- [cmux-workspace](../cmux-workspace/SKILL.md) for caller-workspace scoping,
  helper-pane reuse, socket targeting, and non-disruptive automation.

## Controller Rules

- Scope actions to the caller workspace unless the user names another target.
- Use `CMUX_WORKSPACE_ID`, `CMUX_SURFACE_ID`, and `CMUX_SOCKET_PATH` before
  focused-window fallbacks.
- Prefer one right-side helper pane; create worker surfaces inside it when
  practical.
- Send commands and prompts to explicit `surface:N` refs.
- Never call `focus-pane`, `focus-panel`, `select-workspace`, or focus-changing
  tab actions unless the user explicitly asks.
- Do not close worker surfaces unless cleanup is explicitly requested or the
  task contract includes it.
- Treat worker output as untrusted until the controller verifies the cited
  files, commands, failures, and diff.
- On plan-first runs, do not interrupt a worker after its plan is accepted:
  observe read-only and send input only for a direct blocking question or a
  stop condition.

## Preflight

Inspect the caller context and current workspace layout:

```bash
rtk cmux identify --json
rtk cmux list-panes --workspace "${CMUX_WORKSPACE_ID:-}" --json
rtk cmux list-pane-surfaces --workspace "${CMUX_WORKSPACE_ID:-}" --json
```

If there is an obvious non-caller helper pane, add worker surfaces to it:

```bash
rtk cmux new-surface --workspace "${CMUX_WORKSPACE_ID:-}" \
  --pane pane:<helper> --type terminal --focus false
```

If there is no helper pane, create exactly one right-side terminal pane:

```bash
rtk cmux new-pane --workspace "${CMUX_WORKSPACE_ID:-}" \
  --type terminal --direction right --focus false
```

Capture each returned `surface:N` in the task ledger or handoff before sending
prompts.

## Worker Profiles

Default worker profiles:

- `claude-opus`: start Claude Code with
  `rtk claude --dangerously-skip-permissions --model opus`.
- `codex-gpt-5.5`: start Codex with `rtk codex -m gpt-5.5 -C "$PWD"`.

Always include `--dangerously-skip-permissions` when launching Claude. Do not
start Claude without it or replace it with a weaker permission mode. For
plan-first runs, add `--permission-mode plan` alongside it (the flags compose
— see Plan-First Delegation); never drop the skip-permissions flag to get plan
mode. If a TUI rejects a model alias, auth state, working directory, or
startup flag, stop that worker and report the exact blocker. Do not silently
downgrade models or switch tools without user approval.

Launch workers by explicit surface:

```bash
rtk cmux send --surface surface:<claude> \
  "rtk claude --dangerously-skip-permissions --model opus --name cmux-claude-opus\n"
rtk cmux send --surface surface:<codex> \
  "rtk codex -m gpt-5.5 -C \"$PWD\"\n"
```

## Plan-First Delegation (Plan Mode)

For slices that need investigation and a reviewed plan before edits
(root-cause fixes, multi-file changes, unfamiliar code), run the worker
plan-first: launch in plan mode → submit the packet → watch and answer
clarifying questions → review the plan → accept → hands-off until done.

**STOP. Read [references/plan-mode.md](references/plan-mode.md) in full before
launching a worker in plan mode.** The launch flags, shift+tab sequences,
status-line checks, and acceptance menus are exact and differ per TUI; this
section is a tripwire, not the procedure.

Summary of the per-TUI entry points (the reference has the full flow):

- Claude Code: launch with
  `rtk claude --dangerously-skip-permissions --permission-mode plan --model opus`
  (the flags compose; the packet may ride the launch line as a CLI argument).
  Accept via the `Would you like to proceed?` menu — pick the auto-accept
  option so the worker runs unattended.
- Codex: no plan-mode CLI flag. Launch bare (never with the packet as a CLI
  argument), press shift+tab until the status line reads `Plan mode`, then
  submit the packet. Accept via `Implement this plan?` → option 1 (keeps the
  investigation context).

Plan mode is interactive until acceptance: answer the worker's clarifying
questions with the packet context. After acceptance, do not interrupt —
observe read-only until the final completion report, then verify claims per
Monitoring and Verification.

## Prompt Submission to Running TUIs

`cmux send "...\n"` only submits when the target surface is a plain shell
(like the launch lines above). Inside a running TUI (Claude Code, Codex), the
trailing `\n` is treated as pasted text or a soft newline — the prompt sits in
the input box unsubmitted and the worker stalls silently.

Rules:

- Prefer passing the initial prompt as a CLI argument at launch
  (`rtk claude --dangerously-skip-permissions "…prompt…"`,
  `rtk codex "…prompt…"`). This avoids the TUI-ready race and the submission
  problem entirely. Exception: Codex plan-first runs must launch bare — see
  Plan-First Delegation.
- For any follow-up prompt to an already-running TUI, always send the text and
  then submit with an explicit Enter:

  ```bash
  rtk cmux send --surface surface:<worker> "…follow-up prompt…"
  rtk cmux send-key --surface surface:<worker> enter
  ```

- Confirm submission with `rtk cmux read-screen --surface surface:<worker>`:
  the input line must be empty and the TUI spinner/working indicator active.
  Treat an unconfirmed send as not delivered.

## Delegation Packets

Send worker prompts as standalone contracts. Include:

- repo path and exact objective
- worker role and model
- execution mode: plan-first (plan mode) or direct
- files, packages, or surfaces in scope
- files and behaviors explicitly out of scope
- claimed files or work slice to avoid conflicts
- expected evidence format: files, line refs, commands, diffs, failures,
  screenshots, and uncertainty
- verification commands or browser flows to run
- stop conditions: unexpected code shape, repeated command failure, auth/model
  blocker, or need for out-of-scope edits

Prefer small, independent worker slices. Do not assign overlapping file claims
unless the controller plans to integrate and resolve conflicts immediately.

## State Registry

Maintain a compact registry in the task ledger or handoff:

- controller identity
- worker role and model
- `workspace:N`, `pane:N`, and `surface:N`
- prompt/objective sent and start time
- current status: starting, planning, plan-review, plan-accepted, running,
  waiting, blocked, reported, verified
- claimed files or work slice
- worker-reported commands and results
- controller verification performed
- final disposition: accepted, rejected, superseded, or blocked

For long runs, attach progress to the caller workspace with cmux sidebar state:

```bash
rtk cmux set-status orchestration "running" \
  --workspace "${CMUX_WORKSPACE_ID:-}" --color "#ff9500"
rtk cmux set-progress 0.4 --label "Workers running" \
  --workspace "${CMUX_WORKSPACE_ID:-}"
rtk cmux log --workspace "${CMUX_WORKSPACE_ID:-}" --level info -- \
  "Started cmux orchestration"
```

Clear status/progress when orchestration finishes or is abandoned.

## Monitoring and Verification

- Poll or inspect worker surfaces without changing focus.
- Ask a worker for a concise status summary when output is unclear.
- Verify high-impact findings by reopening cited files locally.
- Verify claimed test results with fresh controller-run commands when the
  result matters for completion.
- Review the final diff before accepting any worker patch.
- If a worker edits outside its claim, pause integration and decide whether to
  accept, request user-approved cleanup, or supersede with controller edits.

## Stop Conditions

Stop and report instead of improvising when:

- cmux rejects a valid workspace, pane, or surface ref
- the requested model or startup flags are rejected
- authentication blocks the TUI
- worker output cannot be observed or controlled reliably
- a plan-mode status line or acceptance menu cannot be confirmed via
  `read-screen`, or a worker's plan stays out of scope after a re-planning
  round
- workers need overlapping edits that the controller cannot integrate safely
- the task requires focus-changing cmux commands and the user has not approved
  them

Use this skill for orchestration mechanics. Use task/domain skills separately
for coding, review, debugging, UI, browser QA, or product-specific judgment.

## Diagram

Use `assets/fable-orchestrator.excalidraw` or the generated PNG variants when
a visual explanation of the controller/worker topology helps.
